import { Injectable } from '@nestjs/common';
import { PaymentStrategy, CheckoutResult, WebhookAction, CreateCheckoutOptions } from './payment-strategy.interface';
import { Payment } from '../entities/payment.entity';
import { websiteUrl } from '../../common/app-config';

@Injectable()
export class PayPalStrategy implements PaymentStrategy {
  private get clientId() { return process.env.PAYPAL_CLIENT_ID || ''; }
  private get clientSecret() { return process.env.PAYPAL_CLIENT_SECRET || ''; }
  private get baseUrl() {
    return process.env.PAYPAL_MODE === 'live'
      ? 'https://api-m.paypal.com'
      : 'https://api-m.sandbox.paypal.com';
  }

  private async getAccessToken(): Promise<string> {
    const response = await fetch(`${this.baseUrl}/v1/oauth2/token`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/x-www-form-urlencoded',
        'Authorization': `Basic ${Buffer.from(`${this.clientId}:${this.clientSecret}`).toString('base64')}`,
      },
      body: 'grant_type=client_credentials',
    });
    const data = await response.json();
    return data.access_token;
  }

  async createCheckout(payment: Payment, options?: CreateCheckoutOptions): Promise<CheckoutResult> {
    if (!this.clientId) throw new Error('PayPal payments are not available yet. Please use VietQR or Bank Transfer.');

    const token = await this.getAccessToken();
    // Convert VND to USD for PayPal (VND not supported directly)
    const amountUsd = payment.currency === 'VND'
      ? (payment.amount / 25000).toFixed(2)
      : (payment.amount / 100).toFixed(2);
    const currency = payment.currency === 'VND' ? 'USD' : payment.currency;

    const response = await fetch(`${this.baseUrl}/v2/checkout/orders`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${token}`,
      },
      body: JSON.stringify({
        intent: 'CAPTURE',
        purchase_units: [{
          reference_id: payment.reference_code,
          amount: { currency_code: currency, value: amountUsd },
          description: 'DraftRight Pro Subscription',
        }],
        application_context: {
          return_url: options?.success_url || `${websiteUrl()}/payment/success?ref=${payment.reference_code}`,
          cancel_url: options?.cancel_url || `${websiteUrl()}/payment/cancel`,
          brand_name: 'DraftRight',
        },
      }),
    });

    const order = await response.json();
    const approveLink = order.links?.find((l: any) => l.rel === 'approve');

    return {
      payment,
      redirect_url: approveLink?.href,
    };
  }

  async verifyWebhook(payload: any, _headers: any): Promise<WebhookAction> {
    // ⚠️ Phase 3a: this strategy is gated off via PAYMENT_ENABLED_METHODS.
    // The signature verification (calling PayPal's verify-webhook-signature endpoint)
    // is not implemented — accepting unsigned webhooks would let anyone fake a
    // payment. Enabling PayPal in 3b requires implementing that verify call first.
    if (payload.event_type === 'CHECKOUT.ORDER.APPROVED' || payload.event_type === 'PAYMENT.CAPTURE.COMPLETED') {
      const referenceCode = payload.resource?.purchase_units?.[0]?.reference_id;
      if (referenceCode) {
        return { type: 'payment_completed', reference_code: referenceCode };
      }
    }
    return { type: 'ignored' };
  }
}
