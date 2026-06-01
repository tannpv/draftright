import { Injectable, BadRequestException, NotFoundException, Logger } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository, MoreThanOrEqual } from 'typeorm';
import { Payment, PaymentMethod, PaymentStatus } from './entities/payment.entity';
// Default payment-method when nothing is configured. Single point of change.
const DEFAULT_PAYMENT_METHOD = PaymentMethod.STRIPE;
import { CheckoutResult, WebhookAction } from './strategies/payment-strategy.interface';
import { storeTypeForMethod } from './store-type-mapping';
import { BasePaymentStrategy } from './strategies/base-payment.strategy';
import { StripeStrategy } from './strategies/stripe.strategy';
import { VietQRStrategy } from './strategies/vietqr.strategy';
import { LemonSqueezyStrategy } from './strategies/lemonsqueezy.strategy';
import { PlansService } from '../plans/plans.service';
import { SubscriptionsService } from '../subscriptions/subscriptions.service';
import { User } from '../users/entities/user.entity';
import { ConfigService } from '@nestjs/config';
import { AppSettings } from '../admin/entities/app-settings.entity';
import { generatePaymentReference } from './payment-reference';
import { EmailService } from '../email/email.service';
import { PAYMENT_PENDING_TTL_MS } from '../common/app-config';
import { EnvSchema } from '../config/env.schema';

@Injectable()
export class PaymentService {
  private strategies: Map<string, BasePaymentStrategy>;

  private readonly logger = new Logger(PaymentService.name);

  constructor(
    @InjectRepository(Payment)
    private readonly paymentRepo: Repository<Payment>,
    @InjectRepository(User)
    private readonly userRepo: Repository<User>,
    @InjectRepository(AppSettings)
    private readonly settingsRepo: Repository<AppSettings>,
    private readonly plansService: PlansService,
    private readonly subscriptionsService: SubscriptionsService,
    private readonly stripeStrategy: StripeStrategy,
    private readonly vietqrStrategy: VietQRStrategy,
    private readonly lemonSqueezyStrategy: LemonSqueezyStrategy,
    private readonly emailService: EmailService,
    private readonly cfg: ConfigService<EnvSchema, true>,
  ) {
    // Use the PaymentMethod enum (entities/payment.entity.ts) as the keys —
    // the same enum the DB rows use, so adding a new method only requires one
    // edit per concern (enum value + strategy class), never a scatter of
    // string literals across the service / controller / DTOs.
    this.strategies = new Map<string, BasePaymentStrategy>([
      [PaymentMethod.STRIPE,        this.stripeStrategy],
      [PaymentMethod.VIETQR,        this.vietqrStrategy],
      [PaymentMethod.BANK_TRANSFER, this.vietqrStrategy],
      [PaymentMethod.LEMONSQUEEZY,  this.lemonSqueezyStrategy],
    ]);
  }

  /**
   * Enabled payment methods, admin-controlled via the
   * `payment_methods_enabled` setting (CSV). Falls back to the
   * PAYMENT_ENABLED_METHODS env var, then "stripe", when unset. Read fresh each
   * call so admin toggles take effect without a restart.
   * `bank_transfer` is implicitly enabled iff `vietqr` is.
   */
  async getEnabledMethods(): Promise<string[]> {
    const settings = await this.settingsRepo.findOne({ where: {} });
    const envMethods = this.cfg.get('PAYMENT_ENABLED_METHODS', { infer: true });
    const raw = (settings?.payment_methods_enabled || envMethods || DEFAULT_PAYMENT_METHOD).toLowerCase();
    const set = new Set(raw.split(',').map((s) => s.trim()).filter(Boolean));
    if (set.has(PaymentMethod.VIETQR)) set.add(PaymentMethod.BANK_TRANSFER);
    return [...set];
  }

  private async assertEnabled(method: string): Promise<void> {
    const enabled = await this.getEnabledMethods();
    if (!enabled.includes(method)) {
      throw new NotFoundException(`Payment method '${method}' is not enabled.`);
    }
  }

  // --- Generic: get strategy by method ---

  private getStrategy(method: string): BasePaymentStrategy {
    const strategy = this.strategies.get(method);
    if (!strategy) throw new BadRequestException(`Unsupported payment method: ${method}`);
    return strategy;
  }


  // --- Generic: create checkout ---

  async createCheckout(
    userId: string,
    planId: string,
    method: string,
    options?: { success_url?: string; cancel_url?: string },
  ): Promise<CheckoutResult> {
    const plan = await this.plansService.findById(planId);
    if (!plan) throw new NotFoundException('Plan not found');
    if (plan.price_cents === 0) throw new BadRequestException('Cannot purchase a free plan');

    await this.assertEnabled(method);
    const strategy = this.getStrategy(method);

    // Load user for Stripe customer reuse + email pre-fill
    const user = await this.userRepo.findOne({ where: { id: userId } });
    if (!user) throw new NotFoundException('User not found');

    // Create payment record. Currency is taken from the plan now (multi-currency).
    const payment = this.paymentRepo.create({
      user_id: userId,
      plan_id: planId,
      amount: plan.price_cents,
      currency: plan.currency || 'USD',
      method: method as PaymentMethod,
      status: PaymentStatus.PENDING,
      reference_code: generatePaymentReference(),
      expires_at: new Date(Date.now() + PAYMENT_PENDING_TTL_MS), // 30 min expiry
    });
    // Eagerly attach the plan so the strategy can read it without a re-fetch
    (payment as any).plan = plan;
    await this.paymentRepo.save(payment);

    // Delegate to strategy
    let result: CheckoutResult;
    try {
      result = await strategy.createCheckout(payment, {
        ...options,
        stripe_customer_id: user.stripe_customer_id,
        user_email: user.email,
      });
    } catch (err: any) {
      payment.status = PaymentStatus.FAILED;
      payment.notes = err.message;
      await this.paymentRepo.save(payment);
      throw new BadRequestException(err.message || 'Payment provider error');
    }

    // Save QR data if generated
    if (result.qr_data) {
      payment.qr_data = result.qr_data;
      await this.paymentRepo.save(payment);
    }

    return result;
  }

  // --- Customer Portal (unified across providers) ---

  /**
   * Mint a one-shot Customer Portal URL for the user's active
   * subscription.  Dispatches via the strategy registry — Stripe +
   * Lemon Squeezy each implement their own
   * `getCustomerPortalUrl(user)`; VietQR / bank-transfer have no
   * portal concept and return null.
   *
   * Throws NotFoundException when:
   *   - the user has no active subscription, or
   *   - the active subscription's store_type doesn't have a portal
   *     (admin-granted plans, mobile IAP, bank-transfer).
   */
  async getCustomerPortalUrl(userId: string): Promise<string> {
    const user = await this.userRepo.findOne({ where: { id: userId } });
    if (!user) throw new NotFoundException('User not found');

    const sub = await this.subscriptionsService.findActiveByUserId(userId);
    if (!sub) {
      throw new NotFoundException('No active subscription to manage');
    }

    // Map subscription.store_type → PaymentMethod registry key.
    // Keys hardcoded one-by-one rather than via PaymentMethod enum
    // because StoreType is a separate concept (e.g. ADMIN_GRANTED
    // has no payment method).
    let method: string | null = null;
    switch (sub.store_type) {
      case 'stripe':       method = PaymentMethod.STRIPE;       break;
      case 'lemonsqueezy': method = PaymentMethod.LEMONSQUEEZY; break;
      default:             method = null;
    }
    if (!method) {
      throw new NotFoundException(
        `Subscriptions sourced from '${sub.store_type}' have no self-service portal`,
      );
    }

    const strategy = this.getStrategy(method);
    const url = await strategy.getCustomerPortalUrl(user);
    if (!url) {
      throw new NotFoundException(
        'Customer portal is not available for this subscription',
      );
    }
    return url;
  }

  // --- Generic: handle webhook from any provider ---

  async handleWebhook(method: string, payload: any, headers: any): Promise<{ success: boolean; reference_code?: string }> {
    await this.assertEnabled(method);
    const strategy = this.getStrategy(method);
    const action: WebhookAction = await strategy.verifyWebhook(payload, headers);

    switch (action.type) {
      case 'payment_completed': {
        // Stripe: capture the new Customer ID for reuse on next checkout
        if (action.stripe_customer_id) {
          const payment = await this.paymentRepo.findOne({ where: { reference_code: action.reference_code } });
          if (payment?.user_id) {
            await this.userRepo.update(payment.user_id, { stripe_customer_id: action.stripe_customer_id });
            this.logger.log(`Saved stripe_customer_id ${action.stripe_customer_id} for user ${payment.user_id}`);
          }
        }
        // Stripe: stamp subscription_id on the granted subscription via store_transaction_id
        const completion = await this.completePayment(action.reference_code, 'completed');
        if (action.stripe_subscription_id && completion.success) {
          await this.subscriptionsService.stampStripeSubscription(
            action.reference_code,
            action.stripe_subscription_id,
          );
        }
        return completion;
      }

      case 'payment_failed':
        return this.completePayment(action.reference_code, 'failed');

      case 'subscription_renewed':
        await this.handleSubscriptionRenewed(action.stripe_subscription_id, action.current_period_end);
        return { success: true };

      case 'subscription_canceled':
        await this.handleSubscriptionCanceled(action.stripe_subscription_id);
        return { success: true };

      // Lemon Squeezy variants — same shape as Stripe but the
      // subscription ID lives in a different field per the LS API.
      // Handlers below delegate to the generic store-ref helpers in
      // SubscriptionsService.
      case 'lemonsqueezy_payment_success': {
        // First-cycle activation OR renewal — both emit
        // `subscription_payment_success`.  Path forks on whether a
        // PENDING payment with this reference_code still exists:
        //   - PENDING → first-cycle: complete payment + stamp store_ref
        //   - non-pending → renewal: extend expires_at via store_ref
        // Either way, also capture the LS customer_id for future
        // portal lookups + checkout reuse.
        if (action.lemonsqueezy_customer_id) {
          const payment = await this.paymentRepo.findOne({ where: { reference_code: action.reference_code } });
          if (payment?.user_id) {
            await this.userRepo.update(payment.user_id, { lemonsqueezy_customer_id: action.lemonsqueezy_customer_id });
          }
        }
        const pending = await this.paymentRepo.findOne({
          where: { reference_code: action.reference_code, status: PaymentStatus.PENDING },
        });
        if (pending) {
          // Defensive: re-resolve plan_id from the variant LS
          // actually charged.  In normal flow (post `enabled_variants`
          // lock) the variant matches what we picked, so this is a
          // no-op.  But if a legacy checkout URL lets the user
          // switch on the page, this corrects the plan_id BEFORE
          // activation so the subscription expires_at reflects the
          // billing period the user actually paid for.
          if (action.lemonsqueezy_variant_id) {
            const truePlanId = await this.resolvePlanIdFromLsVariant(
              action.lemonsqueezy_variant_id,
              pending.currency || 'USD',
            );
            if (truePlanId && truePlanId !== pending.plan_id) {
              await this.paymentRepo.update(
                { id: pending.id },
                { plan_id: truePlanId },
              );
              this.logger.warn(
                `LS variant switch detected: payment ${action.reference_code} ` +
                  `pre-charge plan=${pending.plan_id} post-charge plan=${truePlanId}. Corrected.`,
              );
            }
          }
          const completion = await this.completePayment(action.reference_code, 'completed');
          if (completion.success) {
            await this.subscriptionsService.stampStoreRef(
              action.reference_code,
              storeTypeForMethod(PaymentMethod.LEMONSQUEEZY),
              action.lemonsqueezy_subscription_id,
            );
          }
          return completion;
        }
        // Renewal — extend expiry.
        if (action.current_period_end > 0) {
          const expiresAt = new Date(action.current_period_end * 1000);
          const rows = await this.subscriptionsService.extendByStoreRef(
            storeTypeForMethod(PaymentMethod.LEMONSQUEEZY),
            action.lemonsqueezy_subscription_id,
            expiresAt,
          );
          this.logger.log(`Renewed LS sub ${action.lemonsqueezy_subscription_id} → expires ${expiresAt.toISOString()} (rows=${rows})`);
        }
        return { success: true, reference_code: action.reference_code };
      }

      case 'lemonsqueezy_payment_failed': {
        // LS dunning: a renewal charge failed.  Don't revoke access
        // (that comes via `subscription_expired` after final retry).
        // Email the user so they can update their card before then.
        // Lookup the subscription by store_transaction_id → user →
        // plan so the email carries the right context.
        const sub = await this.subscriptionsService.findByStoreRef(
          storeTypeForMethod(PaymentMethod.LEMONSQUEEZY),
          action.lemonsqueezy_subscription_id,
        );
        if (!sub) {
          this.logger.warn(`LS payment_failed for unknown sub ${action.lemonsqueezy_subscription_id} — skipped`);
          return { success: true };
        }
        try {
          const user = await this.userRepo.findOne({ where: { id: sub.user_id } });
          if (user?.email && sub.plan) {
            await this.emailService.sendPaymentFailed(
              user.email,
              user.name || user.email,
              sub.plan.name,
            );
          }
        } catch (err: any) {
          this.logger.warn(`Failed to email LS payment_failed: ${err.message}`);
        }
        this.logger.log(`Notified user of LS payment failure on sub ${action.lemonsqueezy_subscription_id}`);
        return { success: true };
      }

      case 'lemonsqueezy_subscription_canceled': {
        const rows = await this.subscriptionsService.cancelByStoreRef(
          storeTypeForMethod(PaymentMethod.LEMONSQUEEZY),
          action.lemonsqueezy_subscription_id,
        );
        this.logger.log(`Cancelled LS sub ${action.lemonsqueezy_subscription_id} (rows=${rows})`);
        return { success: true };
      }

      case 'lemonsqueezy_subscription_expired': {
        const rows = await this.subscriptionsService.expireByStoreRef(
          storeTypeForMethod(PaymentMethod.LEMONSQUEEZY),
          action.lemonsqueezy_subscription_id,
        );
        this.logger.log(`Expired LS sub ${action.lemonsqueezy_subscription_id} (rows=${rows})`);
        return { success: true };
      }

      case 'dispute_created':
        this.logger.warn(`Stripe dispute on charge ${action.stripe_charge_id}, amount ${action.amount}. Manual review required.`);
        return { success: true };

      case 'ignored':
      default:
        return { success: false };
    }
  }

  /**
   * Stripe `invoice.payment_succeeded` — extend the subscription's expires_at
   * to the new period end. Idempotent: setting same expiry twice is safe.
   */
  /**
   * Look up the local plan that matches the Lemon Squeezy variant
   * the user actually paid for.  Used by the webhook handler to
   * defensively correct payment.plan_id when LS charges a different
   * variant from what we sent at checkout creation.
   *
   * Resolution:
   *   1. Read app_settings.lemonsqueezy_variant_{monthly,yearly}
   *   2. Match the inbound variant_id → derive billing_period
   *   3. Find an active Pro plan with (billing_period + currency)
   *
   * Returns null if no match — caller keeps the original plan_id.
   */
  private async resolvePlanIdFromLsVariant(
    lsVariantId: string,
    currency: string,
  ): Promise<string | null> {
    const settings = await this.settingsRepo.findOne({ where: {} });
    if (!settings) return null;
    let billingPeriod: string | null = null;
    if (settings.lemonsqueezy_variant_monthly &&
        String(settings.lemonsqueezy_variant_monthly) === lsVariantId) {
      billingPeriod = 'monthly';
    } else if (settings.lemonsqueezy_variant_yearly &&
               String(settings.lemonsqueezy_variant_yearly) === lsVariantId) {
      billingPeriod = 'yearly';
    }
    if (!billingPeriod) return null;
    const plan = await this.plansService.findFirstActive({ billing_period: billingPeriod, currency });
    return plan?.id || null;
  }

  private async handleSubscriptionRenewed(stripeSubId: string, periodEndUnixSec: number): Promise<void> {
    const expiresAt = new Date(periodEndUnixSec * 1000);
    const updated = await this.subscriptionsService.extendByStripeSubId(stripeSubId, expiresAt);
    this.logger.log(`Renewed Stripe sub ${stripeSubId} → expires ${expiresAt.toISOString()} (rows=${updated})`);
  }

  /**
   * Stripe `customer.subscription.deleted` — mark the matching subscription
   * cancelled. Whether it was customer-initiated, dunning failure, or admin
   * doesn't change our action: stop renewing, let access lapse at expires_at.
   */
  private async handleSubscriptionCanceled(stripeSubId: string): Promise<void> {
    const updated = await this.subscriptionsService.cancelByStripeSubId(stripeSubId);
    this.logger.log(`Cancelled Stripe sub ${stripeSubId} (rows=${updated})`);
  }

  // --- Generic: complete/fail a payment ---

  async completePayment(referenceCode: string, status: 'completed' | 'failed'): Promise<{ success: boolean; reference_code: string }> {
    const payment = await this.paymentRepo.findOne({
      where: { reference_code: referenceCode },
      relations: ['plan'],
    });

    if (!payment) return { success: false, reference_code: referenceCode };
    if (payment.status !== PaymentStatus.PENDING) return { success: true, reference_code: referenceCode };

    if (status === 'completed') {
      payment.status = PaymentStatus.COMPLETED;
      payment.completed_at = new Date();
      await this.paymentRepo.save(payment);

      // Activate subscription
      await this.activateSubscription(payment);
    } else {
      payment.status = PaymentStatus.FAILED;
      await this.paymentRepo.save(payment);
    }

    return { success: true, reference_code: referenceCode };
  }

  // --- Generic: activate subscription after successful payment ---

  private async activateSubscription(payment: Payment): Promise<void> {
    if (!payment.plan) {
      const fullPayment = await this.paymentRepo.findOne({
        where: { id: payment.id },
        relations: ['plan'],
      });
      payment = fullPayment || payment;
    }

    const billingPeriod = payment.plan?.billing_period || 'monthly';
    const expiresAt = new Date();
    if (billingPeriod === 'yearly') {
      expiresAt.setFullYear(expiresAt.getFullYear() + 1);
    } else {
      expiresAt.setMonth(expiresAt.getMonth() + 1);
    }

    await this.subscriptionsService.grant(
      payment.user_id,
      payment.plan_id,
      expiresAt,
      storeTypeForMethod(payment.method),
    );

    // Best-effort "subscription active" email — never block activation on it.
    try {
      const user = await this.userRepo.findOne({ where: { id: payment.user_id } });
      if (user?.email) {
        await this.emailService.sendSubscriptionActivated(
          user.email, user.name, payment.plan?.name || 'Pro', expiresAt, payment.currency, payment.amount,
        );
      }
    } catch (e: any) {
      this.logger.warn(`subscription-activated email failed: ${e?.message}`);
    }
  }

  // --- Generic: get payment status ---

  async getStatus(referenceCode: string): Promise<Payment | null> {
    return this.paymentRepo.findOne({
      where: { reference_code: referenceCode },
      relations: ['plan'],
    });
  }

  // --- Generic: admin confirm (for bank transfers) ---

  async adminConfirm(paymentId: string, adminNotes?: string): Promise<Payment> {
    const payment = await this.paymentRepo.findOne({
      where: { id: paymentId },
      relations: ['plan'],
    });
    if (!payment) throw new NotFoundException('Payment not found');
    if (payment.status !== PaymentStatus.PENDING) throw new BadRequestException('Payment is not pending');

    payment.status = PaymentStatus.COMPLETED;
    payment.completed_at = new Date();
    payment.notes = adminNotes || 'Manually confirmed by admin';
    await this.paymentRepo.save(payment);

    await this.activateSubscription(payment);
    return payment;
  }

  // --- Generic: list payments (admin) ---

  async findAll(options: {
    page?: number;
    limit?: number;
    status?: string;
    search?: string;
    sort_by?: string;
    sort_order?: 'ASC' | 'DESC';
  }): Promise<{ payments: Payment[]; total: number }> {
    const { page = 1, limit = 20, status, search, sort_by, sort_order } = options;

    const qb = this.paymentRepo.createQueryBuilder('payment')
      .leftJoinAndSelect('payment.user', 'user')
      .leftJoinAndSelect('payment.plan', 'plan');

    if (status) qb.andWhere('payment.status = :status', { status });

    if (search?.trim()) {
      const term = `%${search.trim()}%`;
      qb.andWhere(
        '(payment.reference_code ILIKE :s OR user.email ILIKE :s OR user.name ILIKE :s OR plan.name ILIKE :s OR payment.method ILIKE :s)',
        { s: term },
      );
    }

    const sortMap: Record<string, string> = {
      reference_code: 'payment.reference_code',
      amount: 'payment.amount',
      method: 'payment.method',
      status: 'payment.status',
      created_at: 'payment.created_at',
      'user.email': 'user.email',
    };
    const sortField = (sort_by && sortMap[sort_by]) || 'payment.created_at';
    qb.orderBy(sortField, sort_order === 'ASC' ? 'ASC' : 'DESC');

    qb.skip((page - 1) * limit).take(limit);
    const [payments, total] = await qb.getManyAndCount();
    return { payments, total };
  }

  // --- Generic: user's payment history ---

  async findByUser(userId: string): Promise<Payment[]> {
    return this.paymentRepo.find({
      where: { user_id: userId },
      relations: ['plan'],
      order: { created_at: 'DESC' },
      take: 20,
    });
  }

  // --- Generic: expire stale pending payments ---

  async expireStalePayments(): Promise<number> {
    const result = await this.paymentRepo
      .createQueryBuilder()
      .update(Payment)
      .set({ status: PaymentStatus.EXPIRED })
      .where('status = :status AND expires_at < NOW()', { status: PaymentStatus.PENDING })
      .execute();
    return result.affected || 0;
  }

  // --- Refund (admin) ---

  /**
   * Issue a refund for a Stripe payment + cancel the linked subscription so
   * the user loses access immediately. Only Stripe is supported (the only
   * provider with subscriptions in Phase 3a).
   *
   * Idempotent-ish: re-running on a refunded payment is a no-op (returns the
   * existing row). Stripe refunds can fail silently if the charge is too old
   * (>180 days) or partially refunded — caller should surface error message.
   */
  async refund(paymentId: string, reason?: string): Promise<Payment> {
    const payment = await this.paymentRepo.findOne({
      where: { id: paymentId },
      relations: ['plan'],
    });
    if (!payment) throw new NotFoundException('Payment not found');
    if (payment.status === PaymentStatus.REFUNDED) return payment;
    if (payment.status !== PaymentStatus.COMPLETED) {
      throw new BadRequestException('Only completed payments can be refunded');
    }
    if (payment.method !== PaymentMethod.STRIPE) {
      throw new BadRequestException(`Refund not supported for method '${payment.method}'`);
    }

    const settings = await this.settingsRepo.findOne({ where: {} });
    const secretKey = settings?.stripe_secret_key || this.cfg.get('STRIPE_SECRET_KEY', { infer: true });
    if (!secretKey) throw new BadRequestException('Stripe secret_key is not configured');

    const Stripe = (await import('stripe')).default;
    const stripe = new Stripe(secretKey);

    // Find the matching Stripe subscription via subscriptions table.
    // store_transaction_id holds sub_XXXX after checkout.session.completed webhook.
    const sub = await this.subscriptionsService.findLatestStripeForUserPlan(payment.user_id, payment.plan_id);
    const stripeSubId = sub?.store_transaction_id;

    // Refund the charge. If we have the subscription, refund the latest invoice's charge.
    // Otherwise we need a charge_id, which we don't store — fall back to listing charges
    // by customer.
    let refundedCharge: string | undefined;
    if (stripeSubId) {
      const subObj: any = await stripe.subscriptions.retrieve(stripeSubId, { expand: ['latest_invoice'] });
      const invoiceObj: any = subObj.latest_invoice;
      const chargeId = invoiceObj?.charge || invoiceObj?.payments?.data?.[0]?.payment?.charge;
      if (chargeId) {
        const refund = await stripe.refunds.create({ charge: chargeId, reason: reason as any });
        refundedCharge = refund.id;
        this.logger.log(`Refunded charge ${chargeId} → refund ${refund.id} for payment ${paymentId}`);
      } else {
        this.logger.warn(`No charge found on latest invoice for sub ${stripeSubId}; skipping Stripe refund`);
      }
      // Cancel the subscription immediately. (cancel_at_period_end=false; access ends now.)
      try {
        await stripe.subscriptions.cancel(stripeSubId);
        this.logger.log(`Cancelled Stripe sub ${stripeSubId} for payment ${paymentId}`);
      } catch (err: any) {
        this.logger.warn(`Stripe sub cancel failed for ${stripeSubId}: ${err.message}`);
      }
    } else {
      this.logger.warn(`No Stripe subscription linked to payment ${paymentId}; refunding metadata only`);
    }

    payment.status = PaymentStatus.REFUNDED;
    payment.notes = [payment.notes, `Refunded by admin${reason ? ` (${reason})` : ''}${refundedCharge ? ` — Stripe refund ${refundedCharge}` : ''}`]
      .filter(Boolean)
      .join(' | ');
    await this.paymentRepo.save(payment);

    // Mark our subscription cancelled so quota guard demotes user.
    if (stripeSubId) {
      await this.subscriptionsService.cancelByStripeSubId(stripeSubId);
    }

    return payment;
  }

  // --- Stats ---

  async getStats(): Promise<{ total: number; completed: number; pending: number; revenue: number }> {
    const [total, completed, pending] = await Promise.all([
      this.paymentRepo.count(),
      this.paymentRepo.count({ where: { status: PaymentStatus.COMPLETED } }),
      this.paymentRepo.count({ where: { status: PaymentStatus.PENDING } }),
    ]);

    const revenueResult = await this.paymentRepo
      .createQueryBuilder('p')
      .select('COALESCE(SUM(p.amount), 0)', 'total')
      .where('p.status = :status', { status: PaymentStatus.COMPLETED })
      .getRawOne();

    return { total, completed, pending, revenue: parseInt(revenueResult?.total || '0') };
  }
}
