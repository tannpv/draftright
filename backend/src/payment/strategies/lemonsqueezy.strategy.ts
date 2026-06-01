import { Injectable } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { createHmac, timingSafeEqual } from 'crypto';
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

/**
 * Lemon Squeezy strategy — credit-card payments via a Merchant of Record.
 *
 * Why MoR: the business operates from Vietnam, where Stripe isn't available and
 * a local card-acquiring contract needs a registered company. Lemon Squeezy
 * sells the subscription on our behalf (handles global Visa/MC + sales tax) and
 * pays out to a personal or company VN bank, so we can accept cards today.
 *
 * Flow:
 *  - createCheckout → POST /v1/checkouts → hosted checkout URL. We stash our
 *    `reference_code` in checkout `custom` data; LS echoes it back on every
 *    webhook under `meta.custom_data.reference_code`, which is how we re-find
 *    the pending Payment.
 *  - verifyWebhook → HMAC-SHA256 (hex) of the RAW body using the signing secret
 *    must match the `X-Signature` header, then dispatch on `meta.event_name`.
 *
 * Credentials live in AppSettings (admin-editable, no redeploy to rotate).
 *
 * KNOWN LIMITATION (fast-follow): only first-payment activation is wired.
 * Renewals (`subscription_payment_success` on later cycles) and
 * cancel/expire are acknowledged but not yet acted on — see verifyWebhook.
 */
@Injectable()
export class LemonSqueezyStrategy extends BasePaymentStrategy {
  private static readonly API = 'https://api.lemonsqueezy.com/v1';

  constructor(
    @InjectRepository(AppSettings) settingsRepo: Repository<AppSettings>,
    cfg: ConfigService<EnvSchema, true>,
  ) {
    super(settingsRepo, cfg);
  }

  private async getCredentials(): Promise<{
    apiKey: string;
    storeId: string;
    webhookSecret: string;
    variantMonthly: string;
    variantYearly: string;
  }> {
    const s = await this.getSettings();
    // Schema declares LEMONSQUEEZY_STORE_ID as a number for boot
    // validation, but the wire API wants a string — coerce here.
    const storeIdEnv = this.cfg.get('LEMONSQUEEZY_STORE_ID', { infer: true });
    return {
      apiKey: this.resolveCredential(s?.lemonsqueezy_api_key, 'LEMONSQUEEZY_API_KEY'),
      storeId:
        (s?.lemonsqueezy_store_id ||
          (storeIdEnv != null ? String(storeIdEnv) : '')) || '',
      webhookSecret: this.resolveCredential(
        s?.lemonsqueezy_webhook_secret,
        'LEMONSQUEEZY_WEBHOOK_SECRET',
      ),
      variantMonthly: s?.lemonsqueezy_variant_monthly || '',
      variantYearly: s?.lemonsqueezy_variant_yearly || '',
    };
  }

  async createCheckout(payment: Payment, options?: CreateCheckoutOptions): Promise<CheckoutResult> {
    const { apiKey, storeId, variantMonthly, variantYearly } = await this.getCredentials();
    if (!apiKey || !storeId) {
      throw new Error('Lemon Squeezy is not configured. Set the API key + store ID in admin Settings → Payment.');
    }

    const plan: any = (payment as any).plan;
    if (!plan) {
      throw new Error('Lemon Squeezy checkout requires payment.plan to be eagerly loaded.');
    }

    const isYearly = (plan.billing_period || 'monthly') === 'yearly';
    const variantId = isYearly ? variantYearly : variantMonthly;
    if (!variantId) {
      throw new Error(
        `No Lemon Squeezy variant configured for ${isYearly ? 'yearly' : 'monthly'} billing. Set lemonsqueezy_variant_${isYearly ? 'yearly' : 'monthly'} in admin Settings → Payment.`,
      );
    }

    const body = {
      data: {
        type: 'checkouts',
        attributes: {
          checkout_data: {
            email: options?.user_email,
            // Echoed back verbatim on every webhook under meta.custom_data.
            custom: { reference_code: payment.reference_code },
          },
          product_options: {
            redirect_url:
              options?.success_url || `${websiteUrl()}/payment/success?ref=${payment.reference_code}`,
          },
        },
        relationships: {
          store: { data: { type: 'stores', id: String(storeId) } },
          variant: { data: { type: 'variants', id: String(variantId) } },
        },
      },
    };

    const res = await fetch(`${LemonSqueezyStrategy.API}/checkouts`, {
      method: 'POST',
      headers: {
        Accept: 'application/vnd.api+json',
        'Content-Type': 'application/vnd.api+json',
        Authorization: `Bearer ${apiKey}`,
      },
      body: JSON.stringify(body),
    });

    if (!res.ok) {
      const text = await res.text();
      this.logger.error(`Lemon Squeezy checkout failed: ${res.status} ${text.slice(0, 500)}`);
      throw new Error(`Lemon Squeezy checkout failed (${res.status})`);
    }

    const json: any = await res.json();
    const url = json?.data?.attributes?.url;
    if (!url) throw new Error('Lemon Squeezy checkout returned no URL');

    return { payment, redirect_url: url };
  }

  /**
   * One-shot LS Customer Portal URL.  Mirrors the standalone
   * LemonsqueezyService.createCustomerPortalUrl helper so the unified
   * `/payment/portal` endpoint can dispatch through the strategy
   * registry instead of branching on store_type at the controller.
   */
  async getCustomerPortalUrl(user: User): Promise<string | null> {
    if (!user.lemonsqueezy_customer_id) return null;
    const { apiKey } = await this.getCredentials();
    if (!apiKey) throw new Error('Lemon Squeezy is not configured.');

    const res = await fetch(
      `https://api.lemonsqueezy.com/v1/customers/${user.lemonsqueezy_customer_id}`,
      {
        headers: { Accept: 'application/vnd.api+json', Authorization: `Bearer ${apiKey}` },
      },
    );
    if (!res.ok) {
      this.logger.error(`LS customer fetch failed: HTTP ${res.status}`);
      throw new Error('Could not load customer portal');
    }
    const json = await res.json() as { data: { attributes: { urls: { customer_portal: string } } } };
    return json?.data?.attributes?.urls?.customer_portal ?? null;
  }

  async verifyWebhook(payload: any, headers: any): Promise<WebhookAction> {
    const { webhookSecret } = await this.getCredentials();
    if (!webhookSecret) {
      this.logger.warn('Lemon Squeezy webhook called but signing secret is unset.');
      return { type: 'ignored' };
    }

    // The controller passes the RAW request body (Buffer) so the signature is
    // computed over the exact bytes LS signed. A parsed/re-serialized object
    // would not match.
    const raw: Buffer = Buffer.isBuffer(payload)
      ? payload
      : Buffer.from(typeof payload === 'string' ? payload : JSON.stringify(payload));

    const sigHeader = (headers['x-signature'] || headers['X-Signature'] || '').toString();
    const expected = createHmac('sha256', webhookSecret).update(raw).digest('hex');
    if (!this.signatureMatches(expected, sigHeader)) {
      this.logger.error('Lemon Squeezy webhook signature mismatch — rejected.');
      return { type: 'ignored' };
    }

    let event: any;
    try {
      event = JSON.parse(raw.toString('utf8'));
    } catch {
      this.logger.error('Lemon Squeezy webhook body is not valid JSON.');
      return { type: 'ignored' };
    }

    const name = event?.meta?.event_name;
    const referenceCode = event?.meta?.custom_data?.reference_code;
    const subId = String(event?.data?.id || '');
    // LS sends ISO timestamps on subscription events; convert to unix
    // seconds to match the Stripe variant's current_period_end shape.
    const renewsAtIso = event?.data?.attributes?.renews_at as string | undefined;
    const endsAtIso = event?.data?.attributes?.ends_at as string | undefined;
    const customerIdRaw = event?.data?.attributes?.customer_id;
    const customerId = customerIdRaw != null ? String(customerIdRaw) : undefined;
    this.logger.log(`Lemon Squeezy webhook: ${name} (ref=${referenceCode || 'none'}, sub=${subId || 'none'})`);

    const toUnix = (iso?: string): number =>
      iso ? Math.floor(new Date(iso).getTime() / 1000) : 0;

    switch (name) {
      case 'subscription_created':
      case 'subscription_payment_success':
        // Both first activation AND renewal cycles emit
        // subscription_payment_success.  We pass everything through
        // and let payment.service decide: if a PENDING payment with
        // this reference_code exists → complete it (first charge);
        // else → extend expires_at via store_ref (renewal).
        if (!referenceCode) {
          this.logger.warn('Lemon Squeezy payment_success without reference_code — cannot match payment.');
          return { type: 'ignored' };
        }
        if (!subId) {
          this.logger.warn('Lemon Squeezy payment_success without data.id — cannot stamp subscription.');
          return { type: 'ignored' };
        }
        return {
          type: 'lemonsqueezy_payment_success',
          reference_code: referenceCode,
          lemonsqueezy_subscription_id: subId,
          lemonsqueezy_customer_id: customerId,
          current_period_end: toUnix(renewsAtIso),
        };

      case 'subscription_cancelled':
        // User clicked "Cancel" in LS Customer Portal — keep access
        // until current period end.  Status flips to CANCELLED so
        // the expiry cron doesn't auto-renew.
        if (!subId) return { type: 'ignored' };
        return { type: 'lemonsqueezy_subscription_canceled', lemonsqueezy_subscription_id: subId };

      case 'subscription_expired':
        // Final dunning attempt failed OR subscription reached its
        // ends_at after cancellation.  Revoke access immediately.
        if (!subId) return { type: 'ignored' };
        return { type: 'lemonsqueezy_subscription_expired', lemonsqueezy_subscription_id: subId };

      case 'subscription_updated':
      default:
        return { type: 'ignored' };
    }
  }

  /** Constant-time hex signature compare; never throws on length mismatch. */
  private signatureMatches(expectedHex: string, actualHex: string): boolean {
    if (!actualHex || expectedHex.length !== actualHex.length) return false;
    try {
      return timingSafeEqual(Buffer.from(expectedHex, 'hex'), Buffer.from(actualHex, 'hex'));
    } catch {
      return false;
    }
  }
}
