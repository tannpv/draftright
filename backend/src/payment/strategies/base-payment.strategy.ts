import { Logger } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
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
}
