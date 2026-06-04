import { Injectable } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { Subscription, SubscriptionStatus, StoreType } from './entities/subscription.entity';
import { BillingPeriod } from '../plans/entities/plan.entity';
import { PlansService } from '../plans/plans.service';
import { Entitlement, EntitlementTier } from './entitlement';

@Injectable()
export class SubscriptionsService {
  constructor(
    @InjectRepository(Subscription)
    private readonly subsRepo: Repository<Subscription>,
    private readonly plansService: PlansService,
  ) {}

  async findActiveByUserId(userId: string): Promise<Subscription | null> {
    return this.subsRepo.findOne({
      where: { user_id: userId, status: SubscriptionStatus.ACTIVE },
      relations: ['plan'],
      order: { created_at: 'DESC' },
    });
  }

  /**
   * Single source of truth for what a user may do right now. An active
   * paid plan → PRO (unlimited). Everything else — active Free, EXPIRED,
   * CANCELLED, or no row at all — resolves to the canonical Free plan so
   * lapsed users keep 10/day instead of being locked out.
   */
  async resolveEntitlement(userId: string): Promise<Entitlement> {
    const active = await this.findActiveByUserId(userId);
    if (active?.plan && active.plan.billing_period !== BillingPeriod.NONE) {
      return {
        tier: EntitlementTier.PRO,
        dailyLimit: active.plan.daily_limit,
        status: active.status,
        expiresAt: active.expires_at ?? null,
        planName: active.plan.name,
      };
    }
    const free = await this.plansService.findFreePlan();
    return {
      tier: EntitlementTier.FREE,
      dailyLimit: free.daily_limit,
      status: active?.status ?? null,
      expiresAt: null,
      planName: free.name,
    };
  }

  async createFreeSubscription(userId: string, planId: string): Promise<Subscription> {
    const sub = this.subsRepo.create({
      user_id: userId,
      plan_id: planId,
      status: SubscriptionStatus.ACTIVE,
      store_type: StoreType.ADMIN_GRANTED,
      started_at: new Date(),
      expires_at: null,
    });
    return this.subsRepo.save(sub);
  }

  /**
   * Activate a subscription on the supplied [storeType] (defaults to
   * ADMIN_GRANTED so the admin "Grant Pro" button keeps working with
   * no caller changes).  Any prior active subscription for this user
   * is cancelled first so we never have two active rows.
   *
   * Payment flows pass the correct store_type (lemonsqueezy / stripe
   * / vietqr / bank_transfer / paypal) — see
   * `PaymentService.activateSubscription` for the method→store_type
   * mapping.  Keeping store_type accurate matters for analytics +
   * for `getCustomerPortalUrl` which dispatches the portal call by
   * store_type.
   */
  async grant(
    userId: string,
    planId: string,
    expiresAt?: Date,
    storeType: StoreType = StoreType.ADMIN_GRANTED,
  ): Promise<Subscription> {
    await this.subsRepo.update(
      { user_id: userId, status: SubscriptionStatus.ACTIVE },
      { status: SubscriptionStatus.CANCELLED },
    );
    const sub = this.subsRepo.create({
      user_id: userId,
      plan_id: planId,
      status: SubscriptionStatus.ACTIVE,
      store_type: storeType,
      started_at: new Date(),
      expires_at: expiresAt || null,
    });
    return this.subsRepo.save(sub);
  }

  async verifyAndActivate(
    userId: string, planId: string, storeType: StoreType, transactionId: string, expiresAt: Date,
  ): Promise<Subscription> {
    await this.subsRepo.update(
      { user_id: userId, status: SubscriptionStatus.ACTIVE },
      { status: SubscriptionStatus.CANCELLED },
    );
    const sub = this.subsRepo.create({
      user_id: userId, plan_id: planId, status: SubscriptionStatus.ACTIVE,
      store_type: storeType, store_transaction_id: transactionId,
      started_at: new Date(), expires_at: expiresAt,
    });
    return this.subsRepo.save(sub);
  }

  async countActive(): Promise<number> {
    return this.subsRepo.count({ where: { status: SubscriptionStatus.ACTIVE } });
  }

  // ── Generic store-ref helpers (any provider) ────────────────────────────

  /**
   * Stamp a subscription with its provider-side transaction ID so
   * later renewal/cancel webhooks can find it.  Looks up the
   * most-recent ACTIVE subscription belonging to the user that paid
   * `referenceCode` and writes `store_type` + `store_transaction_id`.
   *
   * One implementation per provider would duplicate the join + the
   * "find most recent" rule — centralise once.  Callers pass the
   * StoreType so the row's audit field matches the provider that
   * actually charged.
   */
  async stampStoreRef(referenceCode: string, storeType: StoreType, transactionId: string): Promise<void> {
    const recent = await this.subsRepo
      .createQueryBuilder('sub')
      .innerJoin('payments', 'pay', 'pay.user_id = sub.user_id AND pay.reference_code = :ref', { ref: referenceCode })
      .where('sub.status = :status', { status: SubscriptionStatus.ACTIVE })
      .orderBy('sub.created_at', 'DESC')
      .getOne();
    if (!recent) return;
    await this.subsRepo.update(
      { id: recent.id },
      { store_type: storeType, store_transaction_id: transactionId },
    );
  }

  /**
   * Extend a subscription's `expires_at` on a successful renewal.
   * Lookup keyed on `(store_type, store_transaction_id)` so the same
   * code path serves every provider — pass the provider-specific
   * StoreType.  Returns rows affected (0 = no matching row, webhook
   * arrived out-of-order or the stamp from `stampStoreRef` raced).
   */
  async extendByStoreRef(storeType: StoreType, transactionId: string, expiresAt: Date): Promise<number> {
    const result = await this.subsRepo.update(
      { store_type: storeType, store_transaction_id: transactionId, status: SubscriptionStatus.ACTIVE },
      { expires_at: expiresAt },
    );
    return result.affected ?? 0;
  }

  /**
   * Mark a subscription cancelled.  Access remains until expires_at —
   * the expiry cron flips it to EXPIRED after that.  Matches any
   * provider; keep this generic.
   */
  async cancelByStoreRef(storeType: StoreType, transactionId: string): Promise<number> {
    const result = await this.subsRepo.update(
      { store_type: storeType, store_transaction_id: transactionId, status: SubscriptionStatus.ACTIVE },
      { status: SubscriptionStatus.CANCELLED },
    );
    return result.affected ?? 0;
  }

  /**
   * Lookup a subscription by `(store_type, store_transaction_id)`
   * with the user + plan relations loaded — used by webhook handlers
   * that need to email the affected user (e.g. payment-failed).
   * Returns null when no matching row exists.
   */
  async findByStoreRef(storeType: StoreType, transactionId: string): Promise<Subscription | null> {
    return this.subsRepo.findOne({
      where: { store_type: storeType, store_transaction_id: transactionId },
      relations: ['user', 'plan'],
    });
  }

  /**
   * Mark a subscription expired immediately (used when the provider
   * reports the subscription has fully ended — e.g. after final
   * dunning retries failed).  Distinct from cancel: cancel keeps
   * access until expires_at; expire revokes access now.
   */
  async expireByStoreRef(storeType: StoreType, transactionId: string): Promise<number> {
    const result = await this.subsRepo.update(
      { store_type: storeType, store_transaction_id: transactionId },
      { status: SubscriptionStatus.EXPIRED, expires_at: new Date() },
    );
    return result.affected ?? 0;
  }

  // ── Stripe-specific helpers (Phase 3a) ──────────────────────────────────

  /**
   * Stripe-specific wrappers — kept for backward compat with the
   * Stripe webhook path that was already shipped (Phase 3a).  All
   * three delegate to the generic `stampStoreRef` / `extendByStoreRef`
   * / `cancelByStoreRef` helpers above; new providers should call
   * the generic versions directly.
   */
  async stampStripeSubscription(referenceCode: string, stripeSubId: string): Promise<void> {
    return this.stampStoreRef(referenceCode, StoreType.STRIPE, stripeSubId);
  }

  async extendByStripeSubId(stripeSubId: string, expiresAt: Date): Promise<number> {
    return this.extendByStoreRef(StoreType.STRIPE, stripeSubId, expiresAt);
  }

  async cancelByStripeSubId(stripeSubId: string): Promise<number> {
    return this.cancelByStoreRef(StoreType.STRIPE, stripeSubId);
  }

  /**
   * Find the most-recent Stripe subscription for a (user, plan) pair, regardless
   * of status. Used by the admin refund flow to locate the sub_XXXX to cancel.
   */
  async findLatestStripeForUserPlan(userId: string, planId: string): Promise<Subscription | null> {
    return this.subsRepo.findOne({
      where: { user_id: userId, plan_id: planId, store_type: StoreType.STRIPE },
      order: { created_at: 'DESC' },
    });
  }

  async findAllPaginated(options: { search?: string; page?: number; limit?: number }): Promise<{ subscriptions: Subscription[]; total: number }> {
    const { search, page = 1, limit = 20 } = options;
    const qb = this.subsRepo.createQueryBuilder('sub')
      .leftJoinAndSelect('sub.user', 'user')
      .leftJoinAndSelect('sub.plan', 'plan')
      .orderBy('sub.created_at', 'DESC')
      .skip((page - 1) * limit)
      .take(limit);

    if (search) {
      qb.where('user.email ILIKE :search OR user.name ILIKE :search', { search: `%${search}%` });
    }

    const [subscriptions, total] = await qb.getManyAndCount();
    return { subscriptions, total };
  }

  async getPlansBreakdown(): Promise<{ plan_name: string; active_count: number; price_cents: number }[]> {
    const result = await this.subsRepo.createQueryBuilder('sub')
      .leftJoin('sub.plan', 'plan')
      .select('plan.name', 'plan_name')
      .addSelect('plan.price_cents', 'price_cents')
      .addSelect('COUNT(*)', 'active_count')
      .where('sub.status = :status', { status: 'active' })
      .groupBy('plan.name')
      .addGroupBy('plan.price_cents')
      .getRawMany();
    return result.map(r => ({
      plan_name: r.plan_name,
      active_count: parseInt(r.active_count),
      price_cents: parseInt(r.price_cents) || 0,
    }));
  }

  async getMonthlyStats(months: number = 12): Promise<{ month: string; new_subscriptions: number; revenue_cents: number; churned: number }[]> {
    const stats: { month: string; new_subscriptions: number; revenue_cents: number; churned: number }[] = [];
    const now = new Date();

    for (let i = months - 1; i >= 0; i--) {
      const date = new Date(now.getFullYear(), now.getMonth() - i, 1);
      const monthStart = new Date(date.getFullYear(), date.getMonth(), 1);
      const monthEnd = new Date(date.getFullYear(), date.getMonth() + 1, 1);
      const monthStr = `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}`;

      const newSubs = await this.subsRepo.createQueryBuilder('sub')
        .leftJoin('sub.plan', 'plan')
        .where('sub.started_at >= :start AND sub.started_at < :end', { start: monthStart, end: monthEnd })
        .getCount();

      const revenueResult = await this.subsRepo.createQueryBuilder('sub')
        .leftJoin('sub.plan', 'plan')
        .select('COALESCE(SUM(plan.price_cents), 0)', 'total')
        .where('sub.started_at >= :start AND sub.started_at < :end', { start: monthStart, end: monthEnd })
        .getRawOne();

      const churned = await this.subsRepo.createQueryBuilder('sub')
        .where('sub.status IN (:...statuses)', { statuses: ['cancelled', 'expired'] })
        .andWhere('sub.updated_at >= :start AND sub.updated_at < :end', { start: monthStart, end: monthEnd })
        .getCount();

      stats.push({
        month: monthStr,
        new_subscriptions: newSubs,
        revenue_cents: parseInt(revenueResult?.total) || 0,
        churned,
      });
    }

    return stats;
  }
}
