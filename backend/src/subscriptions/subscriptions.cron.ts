import { Injectable, Logger } from '@nestjs/common';
import { Cron, CronExpression } from '@nestjs/schedule';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository, Between, LessThan } from 'typeorm';
import { Subscription, SubscriptionStatus } from './entities/subscription.entity';
import { SubscriptionNotifier } from './subscription-notifier';
import { SubscriptionEvent } from './subscription-event';

/**
 * Subscription lifecycle cron — runs daily at 09:00 UTC.
 *
 * Two responsibilities:
 *   1. Renewal reminders: send a heads-up email to anyone whose ACTIVE
 *      subscription expires in roughly 3 days, so they can update their
 *      card or cancel before being charged.
 *   2. Lapse handling: any subscription past its expires_at gets flipped
 *      to EXPIRED so the rewrite quota guard demotes them to free-tier
 *      limits. Stripe-cancelled subs reach expires_at first; this is the
 *      cleanup pass.
 *
 * The cron is intentionally simple — Stripe Smart Retries handles dunning
 * (3 automatic retries before customer.subscription.deleted fires). We only
 * notify, we don't try to retry charges ourselves.
 */
@Injectable()
export class SubscriptionsCron {
  private readonly logger = new Logger(SubscriptionsCron.name);

  constructor(
    @InjectRepository(Subscription)
    private readonly subsRepo: Repository<Subscription>,
    private readonly notifier: SubscriptionNotifier,
  ) {}

  @Cron(CronExpression.EVERY_DAY_AT_9AM)
  async runDailyMaintenance(): Promise<void> {
    this.logger.log('Daily subscription maintenance running...');
    await this.sendRenewalReminders();
    await this.expireLapsedSubscriptions();
    this.logger.log('Daily subscription maintenance done.');
  }

  /**
   * Find ACTIVE subscriptions expiring in roughly 3 days (between 2.5 and 3.5
   * days from now), send a renewal reminder email. Window is wide enough to
   * tolerate the cron firing a few hours late.
   */
  private async sendRenewalReminders(): Promise<void> {
    const now = Date.now();
    const lower = new Date(now + 2.5 * 86400_000);
    const upper = new Date(now + 3.5 * 86400_000);

    const due = await this.subsRepo.find({
      where: {
        status: SubscriptionStatus.ACTIVE,
        expires_at: Between(lower, upper),
      },
      relations: ['user', 'plan'],
    });

    this.logger.log(`Renewal reminder window: ${due.length} subscription(s)`);

    for (const sub of due) {
      if (!sub.user?.email || !sub.plan || !sub.expires_at) continue;
      await this.notifier.notify(SubscriptionEvent.EXPIRING_SOON, {
        email: sub.user.email,
        name: sub.user.name || sub.user.email,
        planName: sub.plan.name,
        expiresAt: sub.expires_at,
        currency: (sub.plan as any).currency || 'USD',
        amountCents: sub.plan.price_cents,
      });
    }
  }

  /**
   * Flip any ACTIVE or CANCELLED row past its expires_at to EXPIRED.
   * After this, the rewrite-quota guard treats the user as free-tier.
   */
  private async expireLapsedSubscriptions(): Promise<void> {
    // Load the lapsed rows first (with user + plan) so we can email each one,
    // then flip them to EXPIRED.
    const lapsed = await this.subsRepo.find({
      where: [
        { status: SubscriptionStatus.ACTIVE, expires_at: LessThan(new Date()) },
        { status: SubscriptionStatus.CANCELLED, expires_at: LessThan(new Date()) },
      ],
      relations: ['user', 'plan'],
    });
    if (lapsed.length === 0) {
      this.logger.log('No lapsed subscriptions');
      return;
    }

    await this.subsRepo
      .createQueryBuilder()
      .update(Subscription)
      .set({ status: SubscriptionStatus.EXPIRED })
      .whereInIds(lapsed.map((s) => s.id))
      .execute();
    this.logger.log(`Expired ${lapsed.length} lapsed subscription(s)`);

    for (const sub of lapsed) {
      if (!sub.user?.email || !sub.plan) continue;
      await this.notifier.notify(SubscriptionEvent.EXPIRED, {
        email: sub.user.email,
        name: sub.user.name || sub.user.email,
        planName: sub.plan.name,
      });
    }
  }
}
