import { Injectable, Logger } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import {
  PaymentStrategy,
  CheckoutResult,
  WebhookAction,
  CreateCheckoutOptions,
} from './payment-strategy.interface';
import { Payment } from '../entities/payment.entity';
import { AppSettings } from '../../admin/entities/app-settings.entity';

/**
 * Stripe strategy — Phase 3a refactor.
 *
 * Differences from the old one-shot flow:
 *  - mode: 'subscription' (auto-renews; no manual repurchase)
 *  - line_items uses pre-created Stripe Price ID from plan.stripe_price_id
 *  - customer reuse: pass existing cus_XXXX or let Stripe create + capture from webhook
 *  - trial_period_days from plan.trial_days
 *  - verifyWebhook handles 5 event types (was 1)
 *
 * Credentials are read from AppSettings (admin-configurable via admin portal)
 * with env-var fallback so local dev still works without DB seeding.
 */
@Injectable()
export class StripeStrategy implements PaymentStrategy {
  private readonly logger = new Logger(StripeStrategy.name);

  constructor(
    @InjectRepository(AppSettings)
    private readonly settingsRepo: Repository<AppSettings>,
  ) {}

  /**
   * Pull credentials from AppSettings (set via admin portal). Falls back to env
   * vars if DB value is empty — useful for first deploy + tests.
   */
  private async getCredentials(): Promise<{ secretKey: string; webhookSecret: string }> {
    const settings = await this.settingsRepo.findOne({ where: {} });
    return {
      secretKey: settings?.stripe_secret_key || process.env.STRIPE_SECRET_KEY || '',
      webhookSecret: settings?.stripe_webhook_secret || process.env.STRIPE_WEBHOOK_SECRET || '',
    };
  }

  async createCheckout(payment: Payment, options?: CreateCheckoutOptions): Promise<CheckoutResult> {
    const { secretKey } = await this.getCredentials();
    if (!secretKey) {
      throw new Error('Stripe is not configured. Set STRIPE_SECRET_KEY in admin Settings → Payment.');
    }

    const plan: any = (payment as any).plan;
    if (!plan) {
      throw new Error('Stripe checkout requires payment.plan to be eagerly loaded.');
    }
    if (!plan.stripe_price_id) {
      throw new Error(`Plan ${plan.id} has no stripe_price_id. Run scripts/sync-stripe-prices.ts.`);
    }

    const Stripe = (await import('stripe')).default;
    const stripe = new Stripe(secretKey);

    const sessionParams: any = {
      mode: 'subscription',
      payment_method_types: ['card'],
      line_items: [{ price: plan.stripe_price_id, quantity: 1 }],
      // metadata propagates through to the subscription via subscription_data.metadata
      metadata: {
        reference_code: payment.reference_code,
        payment_id: payment.id,
        user_id: payment.user_id,
        plan_id: payment.plan_id,
      },
      subscription_data: {
        metadata: {
          reference_code: payment.reference_code,
          user_id: payment.user_id,
          plan_id: payment.plan_id,
        },
        // 0 trial_days = no trial, omit field; non-zero = pass through
        ...(plan.trial_days > 0 ? { trial_period_days: plan.trial_days } : {}),
      },
      success_url: options?.success_url
        || `${process.env.WEBSITE_URL || 'http://localhost:4000'}/payment/success?ref=${payment.reference_code}`,
      cancel_url: options?.cancel_url
        || `${process.env.WEBSITE_URL || 'http://localhost:4000'}/payment/cancel`,
      // Required so we can find the user later if Stripe creates a Customer fresh
      client_reference_id: payment.user_id,
    };

    // Customer reuse — single source of truth for billing/email/payment methods.
    // If we already have a Customer, pass it; otherwise pre-fill email so Stripe
    // creates a Customer with the right email.
    if (options?.stripe_customer_id) {
      sessionParams.customer = options.stripe_customer_id;
    } else if (options?.user_email) {
      sessionParams.customer_email = options.user_email;
    }

    const session = await stripe.checkout.sessions.create(sessionParams);

    return {
      payment,
      redirect_url: session.url || undefined,
    };
  }

  /**
   * Verify + dispatch Stripe webhook events. Handles 5 event types relevant to
   * subscription lifecycle. Other events return { type: 'ignored' }.
   */
  async verifyWebhook(payload: any, headers: any): Promise<WebhookAction> {
    const { secretKey, webhookSecret } = await this.getCredentials();
    if (!secretKey || !webhookSecret) {
      this.logger.warn('Stripe webhook called but secret_key or webhook_secret is unset.');
      return { type: 'ignored' };
    }

    const Stripe = (await import('stripe')).default;
    const stripe = new Stripe(secretKey);

    const sig = headers['stripe-signature'];
    let event: any;
    try {
      event = stripe.webhooks.constructEvent(payload, sig, webhookSecret);
    } catch (err: any) {
      this.logger.error(`Stripe webhook signature verification failed: ${err.message}`);
      return { type: 'ignored' };
    }

    this.logger.log(`Stripe webhook event: ${event.type} (id=${event.id})`);

    switch (event.type) {
      case 'checkout.session.completed': {
        // Initial checkout success — provisions subscription for the first time.
        // For trial subscriptions, this fires immediately even though no payment
        // was charged. invoice.payment_succeeded fires later when trial ends.
        const session = event.data.object;
        return {
          type: 'payment_completed',
          reference_code: session.metadata?.reference_code,
          stripe_subscription_id: session.subscription || undefined,
          stripe_customer_id: session.customer || undefined,
        };
      }

      case 'invoice.payment_succeeded': {
        // Subscription renewal — extends expiry. Also fires after trial → first paid charge.
        const invoice = event.data.object;
        const subscriptionId = invoice.subscription;
        const periodEnd = invoice.lines?.data?.[0]?.period?.end;
        if (!subscriptionId || !periodEnd) {
          this.logger.warn(`invoice.payment_succeeded missing subscription/period_end (id=${event.id})`);
          return { type: 'ignored' };
        }
        return {
          type: 'subscription_renewed',
          stripe_subscription_id: subscriptionId,
          current_period_end: periodEnd,
        };
      }

      case 'invoice.payment_failed': {
        // Renewal charge failed — let dunning cron handle (don't immediately cancel).
        // Stripe Smart Retries handles 3 retries automatically. Only mark failed
        // when customer.subscription.deleted fires.
        const invoice = event.data.object;
        return {
          type: 'payment_failed',
          reference_code: invoice.subscription_details?.metadata?.reference_code || invoice.id,
        };
      }

      case 'customer.subscription.deleted': {
        // Final cancellation — either by customer, dunning failure, or admin.
        const sub = event.data.object;
        return {
          type: 'subscription_canceled',
          stripe_subscription_id: sub.id,
        };
      }

      case 'charge.dispute.created': {
        // Chargeback — operationally critical. Pause the subscription pending review.
        const dispute = event.data.object;
        return {
          type: 'dispute_created',
          stripe_charge_id: dispute.charge,
          amount: dispute.amount,
        };
      }

      default:
        return { type: 'ignored' };
    }
  }
}
