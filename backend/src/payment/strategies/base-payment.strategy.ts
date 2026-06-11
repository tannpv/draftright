import { Logger } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { timingSafeEqual } from 'crypto';
import { Repository } from 'typeorm';
import { Payment } from '../entities/payment.entity';
import { AppSettings } from '../../admin/entities/app-settings.entity';
import { EnvSchema } from '../../config/env.schema';
import { User } from '../../users/entities/user.entity';
import { CheckoutResult, WebhookAction, CreateCheckoutOptions } from './payment-strategy.interface';

/**
 * Base class every payment method extends. Owns the bits every strategy needs —
 * the AppSettings repository (live admin-set credentials), the typed
 * ConfigService (env fallback for first-deploy + dev), a logger named after
 * the concrete strategy, and a `getSettings()` helper — so subclasses only
 * implement the two things that actually differ: how to start a checkout and
 * how to interpret a provider webhook.
 *
 * NestJS DI note: subclasses inject `Repository<AppSettings>` + `ConfigService`
 * via `@InjectRepository(AppSettings)` decorators on their constructor params
 * and call `super(settingsRepo, cfg)`.
 */
export abstract class BasePaymentStrategy {
  protected readonly logger: Logger;

  protected constructor(
    protected readonly settingsRepo: Repository<AppSettings>,
    protected readonly cfg: ConfigService<EnvSchema, true>,
  ) {
    this.logger = new Logger(this.constructor.name);
  }

  /** The single app-settings row holding all provider credentials. */
  protected getSettings(): Promise<AppSettings | null> {
    return this.settingsRepo.findOne({ where: {} });
  }

  /**
   * Resolve a credential: DB value (admin-portal-set) takes priority,
   * then env var (first-deploy / dev), then empty string.  Concrete
   * strategies call this with the AppSettings field + env key so the
   * fallback ordering is consistent across providers.
   */
  protected resolveCredential(
    fromDb: string | null | undefined,
    envKey: keyof EnvSchema,
  ): string {
    if (fromDb && fromDb.length > 0) return fromDb;
    const envVal = this.cfg.get(envKey, { infer: true });
    return typeof envVal === 'string' ? envVal : '';
  }

  /**
   * Constant-time string comparison for webhook secrets / API keys. Returns
   * false on any length mismatch and never throws, so it's safe to feed
   * attacker-controlled input. Use this instead of `===` / `Array.includes`
   * when comparing a caller-supplied secret against the configured one —
   * `===` short-circuits on the first differing byte and leaks key bytes via
   * response timing.
   */
  protected timingSafeStrEqual(a: string, b: string): boolean {
    const ab = Buffer.from(a, 'utf8');
    const bb = Buffer.from(b, 'utf8');
    if (ab.length !== bb.length) return false;
    return timingSafeEqual(ab, bb);
  }

  abstract createCheckout(payment: Payment, options?: CreateCheckoutOptions): Promise<CheckoutResult>;
  abstract verifyWebhook(payload: any, headers: any): Promise<WebhookAction>;

  /**
   * Mint a one-shot Customer Portal URL for managing the existing
   * subscription (cancel, change plan, update card).
   *
   * Returns null when the strategy doesn't support a portal (VietQR /
   * bank-transfer have no concept of one — those users contact support
   * or transfer again).  The default implementation returns null so
   * strategies without a portal don't need to opt in.
   *
   * Implementations may throw if portal creation fails on the provider
   * side (e.g. Stripe key not configured) — PaymentService surfaces
   * the error to the controller.
   */
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  async getCustomerPortalUrl(_user: User): Promise<string | null> {
    return null;
  }

  /**
   * Cancel the supplied provider-side subscription so the user stops
   * being billed at the next cycle.  The provider's cancellation
   * webhook will follow shortly and stamp the local subscription
   * row's status — this method is responsible only for the upstream
   * cancel call.
   *
   * Returns:
   *   - `true`  when the provider accepted the cancel,
   *   - `false` when the strategy can't cancel programmatically
   *     (VietQR, bank-transfer, admin-granted — those caller should
   *     either reject the request or no-op locally).
   *
   * Throws when the provider call fails (network / 4xx).
   *
   * Default implementation returns false so strategies without
   * cancel support don't need to opt in.
   */
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  async cancelSubscription(_subscriptionId: string): Promise<boolean> {
    return false;
  }
}
