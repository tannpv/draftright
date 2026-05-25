import { Injectable } from '@nestjs/common';
import { createHmac } from 'crypto';
import { PaymentStrategy, CheckoutResult, WebhookAction, CreateCheckoutOptions } from './payment-strategy.interface';
import { Payment } from '../entities/payment.entity';
import { websiteUrl, backendUrl } from '../../common/app-config';

@Injectable()
export class MomoStrategy implements PaymentStrategy {
  private get partnerCode() { return process.env.MOMO_PARTNER_CODE || ''; }
  private get accessKey() { return process.env.MOMO_ACCESS_KEY || ''; }
  private get secretKey() { return process.env.MOMO_SECRET_KEY || ''; }
  private get baseUrl() {
    return process.env.MOMO_MODE === 'live'
      ? 'https://payment.momo.vn'
      : 'https://test-payment.momo.vn';
  }

  async createCheckout(payment: Payment, options?: CreateCheckoutOptions): Promise<CheckoutResult> {
    if (!this.partnerCode || !this.secretKey) {
      throw new Error('Momo payments are not available yet. Please use VietQR or Bank Transfer.');
    }

    const redirectUrl = options?.success_url || `${websiteUrl()}/payment/success?ref=${payment.reference_code}`;
    const ipnUrl = `${backendUrl()}/payment/webhook/momo`;
    const requestId = payment.id;
    const orderId = payment.reference_code;
    const orderInfo = `DraftRight Pro - ${payment.reference_code}`;
    const amount = payment.amount.toString();
    const requestType = 'payWithMethod';
    const extraData = '';
    const autoCapture = true;
    const lang = 'vi';

    // Create HMAC SHA256 signature
    const rawSignature = `accessKey=${this.accessKey}&amount=${amount}&extraData=${extraData}&ipnUrl=${ipnUrl}&orderId=${orderId}&orderInfo=${orderInfo}&partnerCode=${this.partnerCode}&redirectUrl=${redirectUrl}&requestId=${requestId}&requestType=${requestType}`;
    const signature = createHmac('sha256', this.secretKey).update(rawSignature).digest('hex');

    const response = await fetch(`${this.baseUrl}/v2/gateway/api/create`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        partnerCode: this.partnerCode,
        partnerName: 'DraftRight',
        requestId,
        amount: parseInt(amount),
        orderId,
        orderInfo,
        redirectUrl,
        ipnUrl,
        requestType,
        extraData,
        autoCapture,
        lang,
        signature,
      }),
    });

    const data = await response.json();

    if (data.resultCode !== 0) {
      throw new Error(data.message || 'Momo payment creation failed');
    }

    return {
      payment,
      redirect_url: data.payUrl,
      qr_data: data.qrCodeUrl,
    };
  }

  async verifyWebhook(payload: any, _headers: any): Promise<WebhookAction> {
    // ⚠️ Phase 3a: Momo strategy is gated off via PAYMENT_ENABLED_METHODS.
    // The IPN signature verification (HMAC SHA256 over a fixed-order param string
    // per Momo spec) is not implemented — accepting unsigned IPN would let anyone
    // fake a payment by POSTing { resultCode: 0, orderId: anything } to the
    // webhook URL. Enabling Momo in 3b requires implementing that HMAC verify.
    if (payload.resultCode === 0 && payload.orderId) {
      return { type: 'payment_completed', reference_code: payload.orderId };
    }
    if (payload.orderId) {
      return { type: 'payment_failed', reference_code: payload.orderId };
    }
    return { type: 'ignored' };
  }
}
