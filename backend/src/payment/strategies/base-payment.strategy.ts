import { Logger } from '@nestjs/common';
import { Repository } from 'typeorm';
import { Payment } from '../entities/payment.entity';
import { AppSettings } from '../../admin/entities/app-settings.entity';
import { CheckoutResult, WebhookAction, CreateCheckoutOptions } from './payment-strategy.interface';

/**
 * Base class every payment method extends. Owns the bits every strategy needs —
 * the AppSettings repository (live admin-set credentials), a logger named after
 * the concrete strategy, and a `getSettings()` helper — so subclasses only
 * implement the two things that actually differ: how to start a checkout and
 * how to interpret a provider webhook.
 *
 * NestJS DI note: `@InjectRepository(AppSettings)` must decorate the *concrete*
 * subclass's constructor param, which then calls `super(settingsRepo)`.
 */
export abstract class BasePaymentStrategy {
  protected readonly logger: Logger;

  protected constructor(protected readonly settingsRepo: Repository<AppSettings>) {
    this.logger = new Logger(this.constructor.name);
  }

  /** The single app-settings row holding all provider credentials. */
  protected getSettings(): Promise<AppSettings | null> {
    return this.settingsRepo.findOne({ where: {} });
  }

  abstract createCheckout(payment: Payment, options?: CreateCheckoutOptions): Promise<CheckoutResult>;
  abstract verifyWebhook(payload: any, headers: any): Promise<WebhookAction>;
}
