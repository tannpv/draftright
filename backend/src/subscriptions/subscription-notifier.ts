import { Injectable, Logger } from '@nestjs/common';
import { EmailService } from '../email/email.service';
import { SubscriptionEvent, SubscriptionEventPayload } from './subscription-event';

/**
 * One place that maps a subscription lifecycle event to a notification
 * channel. Today: email. Adding push later = one extra send call here,
 * no caller changes. Sends are best-effort — a failure is logged, never
 * thrown, so a batch (cron) never aborts on one bad recipient.
 */
@Injectable()
export class SubscriptionNotifier {
  private readonly logger = new Logger(SubscriptionNotifier.name);

  constructor(private readonly email: EmailService) {}

  async notify(event: SubscriptionEvent, p: SubscriptionEventPayload): Promise<void> {
    try {
      switch (event) {
        case SubscriptionEvent.UPGRADED:
        case SubscriptionEvent.PLAN_CHANGED:
          await this.email.sendSubscriptionActivated(
            p.email, p.name, p.planName, p.expiresAt ?? new Date(), p.currency ?? 'USD', p.amountCents ?? 0,
          );
          break;
        case SubscriptionEvent.EXPIRING_SOON:
          await this.email.sendRenewalReminder(
            p.email, p.name, p.planName, p.expiresAt ?? new Date(), p.currency ?? 'USD', p.amountCents ?? 0,
          );
          break;
        case SubscriptionEvent.EXPIRED:
          await this.email.sendSubscriptionExpired(p.email, p.name, p.planName);
          break;
      }
    } catch (err: any) {
      this.logger.error(`notify(${event}) failed for ${p.email}: ${err?.message}`);
    }
  }
}
