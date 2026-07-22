import { Injectable, BadRequestException } from '@nestjs/common';
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
import { appName, websiteUrl } from '../../common/app-config';

/**
 * PayPal strategy — recurring subscriptions via the PayPal REST API.
 *
 * Why PayPal (in addition to Lemon Squeezy): LS already accepts cards + PayPal
 * as a Merchant of Record, but takes a MoR cut on top of processing. A direct
 * PayPal integration pays out to the VN business's own PayPal balance at
 * PayPal's lower processing rate. USD only — PayPal does not support VND, so
 * VN-domestic buyers keep using VietQR.
 *
 * Flow (Model A, recurring):
 *  - createCheckout → POST /v1/billing/subscriptions with the billing-plan id
 *    (monthly|yearly per plan.billing_period). Our `reference_code` rides along
 *    as `custom_id`; PayPal echoes it on the ACTIVATED webhook so we re-find the
 *    pending Payment. Returns the `approve` link as the redirect URL.
 *  - verifyWebhook → verify via PayPal's verify-webhook-signature API (PayPal
 *    does NOT sign with HMAC), then dispatch on `event_type`.
 *
 * Auth: OAuth2 client-credentials. Access tokens are cached in-memory until a
 * safety margin before expiry. Credentials + billing-plan ids + mode live in
 * AppSettings (admin-editable, rotate without redeploy), env as first-deploy
 * fallback.
 */
@Injectable()
export class PayPalStrategy extends BasePaymentStrategy {
  // PayPal REST API hosts, selected by paypal_mode. Fixed infra endpoints
  // (like LemonSqueezyStrategy.API) — named here so no literal is repeated.
  private static readonly LIVE_API = 'https://api-m.paypal.com';
  private static readonly SANDBOX_API = 'https://api-m.sandbox.paypal.com';

  private tokenCache: { token: string; expiresAtMs: number } | null = null;

  constructor(
    @InjectRepository(AppSettings) settingsRepo: Repository<AppSettings>,
    cfg: ConfigService<EnvSchema, true>,
  ) {
    super(settingsRepo, cfg);
  }

  private async getCredentials(): Promise<{
    clientId: string;
    clientSecret: string;
    webhookId: string;
    planMonthly: string;
    planYearly: string;
    baseUrl: string;
  }> {
    const s = await this.getSettings();
    const mode =
      (s?.paypal_mode ||
        this.cfg.get('PAYPAL_MODE', { infer: true }) ||
        'sandbox').toLowerCase();
    return {
      clientId: this.resolveCredential(s?.paypal_client_id, 'PAYPAL_CLIENT_ID'),
      clientSecret: this.resolveCredential(s?.paypal_client_secret, 'PAYPAL_CLIENT_SECRET'),
      webhookId: this.resolveCredential(s?.paypal_webhook_id, 'PAYPAL_WEBHOOK_ID'),
      planMonthly: this.resolveCredential(s?.paypal_plan_monthly, 'PAYPAL_PLAN_MONTHLY'),
      planYearly: this.resolveCredential(s?.paypal_plan_yearly, 'PAYPAL_PLAN_YEARLY'),
      baseUrl:
        mode === 'live'
          ? PayPalStrategy.LIVE_API
          : PayPalStrategy.SANDBOX_API,
    };
  }

  /**
   * OAuth2 access token (client_credentials), cached in-memory until ~60s
   * before expiry. Every subsequent REST call authorizes with this Bearer.
   */
  private async getAccessToken(): Promise<string> {
    const now = Date.now();
    if (this.tokenCache && this.tokenCache.expiresAtMs > now) {
      return this.tokenCache.token;
    }
    const { clientId, clientSecret, baseUrl } = await this.getCredentials();
    if (!clientId || !clientSecret) {
      throw new Error('PayPal is not configured. Set the client ID + secret in admin Settings → Payment.');
    }
    const basic = Buffer.from(`${clientId}:${clientSecret}`).toString('base64');
    const res = await fetch(`${baseUrl}/v1/oauth2/token`, {
      method: 'POST',
      headers: {
        Authorization: `Basic ${basic}`,
        'Content-Type': 'application/x-www-form-urlencoded',
      },
      body: 'grant_type=client_credentials',
    });
    if (!res.ok) {
      const text = await res.text();
      this.logger.error(`PayPal token fetch failed: ${res.status} ${text.slice(0, 300)}`);
      throw new Error(`PayPal auth failed (${res.status})`);
    }
    const json: any = await res.json();
    const token = json?.access_token;
    const expiresIn = Number(json?.expires_in) || 0;
    if (!token) throw new Error('PayPal auth returned no access_token');
    this.tokenCache = { token, expiresAtMs: now + Math.max(0, (expiresIn - 60) * 1000) };
    return token;
  }

  async createCheckout(payment: Payment, options?: CreateCheckoutOptions): Promise<CheckoutResult> {
    const { planMonthly, planYearly, baseUrl } = await this.getCredentials();

    const plan: any = (payment as any).plan;
    if (!plan) {
      throw new Error('PayPal checkout requires payment.plan to be eagerly loaded.');
    }
    const isYearly = (plan.billing_period || 'monthly') === 'yearly';
    const planId = isYearly ? planYearly : planMonthly;
    if (!planId) {
      throw new Error(
        `No PayPal billing plan configured for ${isYearly ? 'yearly' : 'monthly'} billing. ` +
          `Set paypal_plan_${isYearly ? 'yearly' : 'monthly'} in admin Settings → Payment ` +
          `(run scripts/paypal-create-plans.ts to create them).`,
      );
    }

    const token = await this.getAccessToken();
    const body = {
      plan_id: planId,
      // Echoed back on the ACTIVATED webhook under resource.custom_id.
      custom_id: payment.reference_code,
      subscriber: options?.user_email
        ? { email_address: options.user_email }
        : undefined,
      application_context: {
        brand_name: appName(),
        shipping_preference: 'NO_SHIPPING',
        user_action: 'SUBSCRIBE_NOW',
        return_url:
          options?.success_url || `${websiteUrl()}/payment/success?ref=${payment.reference_code}`,
        cancel_url:
          options?.cancel_url || `${websiteUrl()}/payment/cancel?ref=${payment.reference_code}`,
      },
    };

    const res = await fetch(`${baseUrl}/v1/billing/subscriptions`, {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${token}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(body),
    });
    if (!res.ok) {
      const text = await res.text();
      this.logger.error(`PayPal subscription create failed: ${res.status} ${text.slice(0, 500)}`);
      throw new Error(`PayPal checkout failed (${res.status})`);
    }
    const json: any = await res.json();
    const approve = Array.isArray(json?.links)
      ? json.links.find((l: any) => l?.rel === 'approve')?.href
      : undefined;
    if (!approve) throw new Error('PayPal checkout returned no approve link');
    return { payment, redirect_url: approve };
  }

  /** PayPal has no hosted customer portal — cancel is handled via cancelSubscription. */
  async getCustomerPortalUrl(_user: User): Promise<string | null> {
    return null;
  }

  /**
   * Cancel a PayPal subscription (POST /v1/billing/subscriptions/:id/cancel).
   * PayPal keeps access billed-through the current period; our
   * `subscriptions.status` flips to 'cancelled' via the CANCELLED webhook.
   */
  async cancelSubscription(paypalSubscriptionId: string): Promise<boolean> {
    const { baseUrl } = await this.getCredentials();
    const token = await this.getAccessToken();
    const res = await fetch(
      `${baseUrl}/v1/billing/subscriptions/${paypalSubscriptionId}/cancel`,
      {
        method: 'POST',
        headers: {
          Authorization: `Bearer ${token}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ reason: `Cancelled from ${appName()}` }),
      },
    );
    // 204 No Content on success. 422 = already cancelled → treat as success.
    if (res.status === 204 || res.status === 422) return true;
    const text = await res.text();
    this.logger.error(`PayPal cancel failed: ${res.status} ${text.slice(0, 300)} for sub=${paypalSubscriptionId}`);
    throw new Error('Could not cancel the subscription');
  }

  /** Fetch a subscription's next_billing_time (unix sec), or 0 if unavailable. */
  private async fetchNextBillingUnix(paypalSubscriptionId: string): Promise<number> {
    try {
      const { baseUrl } = await this.getCredentials();
      const token = await this.getAccessToken();
      const res = await fetch(`${baseUrl}/v1/billing/subscriptions/${paypalSubscriptionId}`, {
        headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
      });
      if (!res.ok) return 0;
      const json: any = await res.json();
      const iso = json?.billing_info?.next_billing_time as string | undefined;
      return iso ? Math.floor(new Date(iso).getTime() / 1000) : 0;
    } catch (err: any) {
      this.logger.warn(`PayPal next_billing_time fetch failed for ${paypalSubscriptionId}: ${err.message}`);
      return 0;
    }
  }

  async verifyWebhook(payload: any, headers: any): Promise<WebhookAction> {
    const { webhookId } = await this.getCredentials();
    if (!webhookId) {
      this.logger.warn('PayPal webhook called but webhook ID is unset.');
      return { type: 'ignored' };
    }

    // Controller passes the RAW body (Buffer) so we control the exact JSON.
    const raw: Buffer = Buffer.isBuffer(payload)
      ? payload
      : Buffer.from(typeof payload === 'string' ? payload : JSON.stringify(payload));
    let event: any;
    try {
      event = JSON.parse(raw.toString('utf8'));
    } catch {
      this.logger.error('PayPal webhook body is not valid JSON.');
      return { type: 'ignored' };
    }

    const h = (k: string) => (headers[k] || headers[k.toLowerCase()] || '').toString();
    const verifyBody = {
      auth_algo: h('paypal-auth-algo'),
      cert_url: h('paypal-cert-url'),
      transmission_id: h('paypal-transmission-id'),
      transmission_sig: h('paypal-transmission-sig'),
      transmission_time: h('paypal-transmission-time'),
      webhook_id: webhookId,
      webhook_event: event,
    };

    const { baseUrl } = await this.getCredentials();
    const token = await this.getAccessToken();
    const verifyRes = await fetch(`${baseUrl}/v1/notifications/verify-webhook-signature`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
      body: JSON.stringify(verifyBody),
    });
    if (!verifyRes.ok) {
      this.logger.error(`PayPal verify-webhook-signature HTTP ${verifyRes.status} — rejecting.`);
      throw new BadRequestException('PayPal webhook verification failed');
    }
    const verifyJson: any = await verifyRes.json();
    if (verifyJson?.verification_status !== 'SUCCESS') {
      this.logger.error(`PayPal webhook signature not verified (status=${verifyJson?.verification_status}).`);
      throw new BadRequestException('Invalid PayPal webhook signature');
    }

    const eventType = event?.event_type as string | undefined;
    const resource = event?.resource || {};
    this.logger.log(`PayPal webhook: ${eventType} (sub=${resource?.id || resource?.billing_agreement_id || 'none'})`);

    switch (eventType) {
      case 'BILLING.SUBSCRIPTION.ACTIVATED': {
        // First-cycle activation. custom_id carries our reference_code.
        const referenceCode = resource?.custom_id;
        const subId = String(resource?.id || '');
        if (!referenceCode || !subId) {
          this.logger.warn('PayPal ACTIVATED missing custom_id or id — cannot match payment.');
          return { type: 'ignored' };
        }
        const nextIso = resource?.billing_info?.next_billing_time as string | undefined;
        return {
          type: 'paypal_payment_success',
          reference_code: referenceCode,
          paypal_subscription_id: subId,
          current_period_end: nextIso ? Math.floor(new Date(nextIso).getTime() / 1000) : 0,
        };
      }

      case 'PAYMENT.SALE.COMPLETED': {
        // A charge cleared — first cycle OR renewal. billing_agreement_id is
        // the subscription id. No reference_code here, so this always takes
        // the renewal branch (extend by store-ref). On the first cycle the
        // ACTIVATED event already completed the pending payment, so extending
        // to the same next_billing_time is a no-op.
        const subId = String(resource?.billing_agreement_id || '');
        if (!subId) return { type: 'ignored' };
        const cpe = await this.fetchNextBillingUnix(subId);
        return {
          type: 'paypal_payment_success',
          reference_code: '',
          paypal_subscription_id: subId,
          current_period_end: cpe,
        };
      }

      case 'BILLING.SUBSCRIPTION.PAYMENT.FAILED': {
        const subId = String(resource?.id || '');
        if (!subId) return { type: 'ignored' };
        return { type: 'paypal_payment_failed', paypal_subscription_id: subId };
      }

      case 'BILLING.SUBSCRIPTION.CANCELLED': {
        const subId = String(resource?.id || '');
        if (!subId) return { type: 'ignored' };
        return { type: 'paypal_subscription_canceled', paypal_subscription_id: subId };
      }

      case 'BILLING.SUBSCRIPTION.EXPIRED':
      case 'BILLING.SUBSCRIPTION.SUSPENDED': {
        const subId = String(resource?.id || '');
        if (!subId) return { type: 'ignored' };
        return { type: 'paypal_subscription_expired', paypal_subscription_id: subId };
      }

      default:
        return { type: 'ignored' };
    }
  }
}
