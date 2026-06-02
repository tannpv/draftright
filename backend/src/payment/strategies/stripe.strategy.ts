import { Injectable } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import {
  CheckoutResult,
  WebhookAction,
  CreateCheckoutOptions,
} from './payment-strategy.interface';
import { BasePaymentStrategy } from './base-payment.strategy';
import { Payment } from '../entities/payment.entity';
import { AppSettings } from '../../admin/entities/app-settings.entity';
import { EnvSchema } from '../../config/env.schema';
import { User } from '../../users/entities/user.entity';
import { websiteUrl } from '../../common/app-config';
import { PaymentMethod } from '../entities/payment.entity';

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
export class StripeStrategy extends BasePaymentStrategy {
  constructor(
    @InjectRepository(AppSettings) settingsRepo: Repository<AppSettings>,
    cfg: ConfigService<EnvSchema, true>,
  ) {
    super(settingsRepo, cfg);
  }

  /**
   * Pull credentials from AppSettings (set via admin portal). Falls back to env
   * vars (typed via ConfigService) if DB value is empty — useful for first
   * deploy + tests.
   */
  private async getCredentials(): Promise<{ secretKey: string; webhookSecret: string }> {
    const settings = await this.getSettings();
    return {
      secretKey: this.resolveCredential(settings?.stripe_secret_key, 'STRIPE_SECRET_KEY'),
      webhookSecret: this.resolveCredential(settings?.stripe_webhook_secret, 'STRIPE_WEBHOOK_SECRET'),
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

    const Stripe = (await import('stripe')).default;
    const stripe = new Stripe(secretKey);

    // Native-wallet branch: when the inbound method is apple_pay or
    // google_pay we don't want a hosted Stripe Checkout (that would
    // bounce the user to a browser).  Instead create a PaymentIntent
    // and return its client_secret + merchant config; the mobile
    // SDK presents the native sheet and confirms client-side.  This
    // branch runs BEFORE the stripe_price_id check because wallets
    // bill an amount directly — they don't need a pre-created Price.
    if (payment.method === PaymentMethod.APPLE_PAY || payment.method === PaymentMethod.GOOGLE_PAY) {
      return this.createWalletPaymentIntent(payment, stripe, options);
    }

    if (!plan.stripe_price_id) {
      throw new Error(`Plan ${plan.id} has no stripe_price_id. Run scripts/sync-stripe-prices.ts.`);
    }

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
        || `${websiteUrl()}/payment/success?ref=${payment.reference_code}`,
      cancel_url: options?.cancel_url
        || `${websiteUrl()}/payment/cancel`,
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
   * Create a Stripe PaymentIntent for a native-wallet checkout
   * (Apple Pay / Google Pay).  Returns the client_secret the mobile
   * SDK needs to present the platform sheet — no redirect.  The
   * usual `payment_intent.succeeded` webhook activates the
   * subscription, identical to the hosted Checkout Session flow.
   *
   * `setup_future_usage: 'off_session'` saves the wallet's tokenised
   * payment method on the Stripe Customer so future renewals charge
   * silently (no second wallet prompt).
   */
  private async createWalletPaymentIntent(
    payment: Payment,
    stripe: import('stripe').default,
    options?: CreateCheckoutOptions,
  ): Promise<CheckoutResult> {
    const plan: any = (payment as any).plan;
    // Reuse the existing Stripe Customer or create one keyed by
    // user_email so renewals + admin lookups can find it later.
    let customerId = options?.stripe_customer_id;
    if (!customerId) {
      const customer = await stripe.customers.create({
        email: options?.user_email,
        metadata: { user_id: payment.user_id },
      });
      customerId = customer.id;
    }

    const intent = await stripe.paymentIntents.create({
      amount: payment.amount,
      currency: payment.currency.toLowerCase(),
      customer: customerId,
      setup_future_usage: 'off_session',
      payment_method_types: ['card'],
      metadata: {
        reference_code: payment.reference_code,
        payment_id: payment.id,
        user_id: payment.user_id,
        plan_id: payment.plan_id,
        method: payment.method,
      },
    });

    // Resolve env config for the wallet handshake.  Publishable key
    // is safe to ship to clients; the secret key stays server-side.
    // Apple merchant ID is whatever was registered in Stripe + Apple
    // Dev portal; Google Pay needs no merchant id (Stripe handles).
    const publishableKey = this.resolveCredential(
      // No DB column yet — env-only for now.
      null,
      'STRIPE_PUBLISHABLE_KEY',
    );
    const merchantIdentifier = this.resolveCredential(
      null,
      'APPLE_PAY_MERCHANT_ID',
    );

    return {
      payment,
      wallet_intent: {
        client_secret: intent.client_secret || '',
        publishable_key: publishableKey,
        merchant_identifier: merchantIdentifier || undefined,
        country_code: (plan?.country_code as string | undefined) || 'US',
        currency_code: payment.currency.toUpperCase(),
      },
    };
  }

  /**
   * One-shot Stripe Billing Portal URL.  Requires the user to have a
   * `stripe_customer_id` (every Stripe Checkout flow saves one onto
   * the user; see verifyWebhook).  Stripe's portal handles cancel /
   * plan change / card update without any custom UI on our side.
   */
  async getCustomerPortalUrl(user: User): Promise<string | null> {
    if (!user.stripe_customer_id) return null;
    const { secretKey } = await this.getCredentials();
    if (!secretKey) throw new Error('Stripe is not configured.');

    const Stripe = (await import('stripe')).default;
    const stripe = new Stripe(secretKey);
    const session = await stripe.billingPortal.sessions.create({
      customer: user.stripe_customer_id,
      return_url: `${websiteUrl()}/account?subscribed=1`,
    });
    return session.url || null;
  }

  /**
   * Cancel a Stripe subscription at period end via the Stripe API.
   * Mirrors LemonSqueezyStrategy.cancelSubscription so the unified
   * `/payment/cancel` endpoint dispatches through the strategy
   * registry instead of branching on store_type at the controller.
   */
  async cancelSubscription(stripeSubscriptionId: string): Promise<boolean> {
    const { secretKey } = await this.getCredentials();
    if (!secretKey) throw new Error('Stripe is not configured.');
    const Stripe = (await import('stripe')).default;
    const stripe = new Stripe(secretKey);
    // cancel_at_period_end keeps access until the renewal date and
    // fires `customer.subscription.updated` immediately + then
    // `customer.subscription.deleted` at period end — same shape as
    // LS's cancel/expire pair.
    await stripe.subscriptions.update(stripeSubscriptionId, {
      cancel_at_period_end: true,
    });
    return true;
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
        // API version 2026-04-22 moved invoice.subscription → invoice.parent.subscription_details.subscription
        // and lines[0].period → lines[0].parent.subscription_item_details.subscription_period.
        // We also accept the legacy paths so older API versions still work.
        const invoice = event.data.object;
        const subscriptionId =
          invoice.parent?.subscription_details?.subscription ||
          invoice.subscription;
        const line = invoice.lines?.data?.[0];
        const periodEnd =
          line?.period?.end ||
          line?.parent?.subscription_item_details?.subscription_period?.end ||
          invoice.period_end;
        if (!subscriptionId || !periodEnd) {
          // $0 trial-start invoice has no subscription on it — that's expected.
          // Real renewal will have one; warn so it's visible in logs.
          this.logger.warn(
            `invoice.payment_succeeded missing subscription/period_end (id=${event.id}, amount=${invoice.amount_paid}, billing_reason=${invoice.billing_reason})`
          );
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
        const refCode =
          invoice.parent?.subscription_details?.metadata?.reference_code ||
          invoice.subscription_details?.metadata?.reference_code ||
          invoice.id;
        return {
          type: 'payment_failed',
          reference_code: refCode,
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
