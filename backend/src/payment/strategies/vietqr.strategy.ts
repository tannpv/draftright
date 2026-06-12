import { Injectable, UnauthorizedException } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { CheckoutResult, WebhookAction, CreateCheckoutOptions } from './payment-strategy.interface';
import { BasePaymentStrategy } from './base-payment.strategy';
import { Payment, PaymentMethod } from '../entities/payment.entity';
import { AppSettings } from '../../admin/entities/app-settings.entity';
import { EnvSchema } from '../../config/env.schema';
import { extractPaymentReference } from '../payment-reference';

@Injectable()
export class VietQRStrategy extends BasePaymentStrategy {
  constructor(
    @InjectRepository(AppSettings) settingsRepo: Repository<AppSettings>,
    cfg: ConfigService<EnvSchema, true>,
  ) {
    super(settingsRepo, cfg);
  }

  /** Read bank + provider credentials from AppSettings (admin portal), typed-env fallback. */
  private async getCredentials() {
    const s = await this.getSettings();
    return {
      // VietQR display fields have sane local-dev defaults (MB bank,
      // dummy account) so the strategy never crashes when nothing is
      // configured.  Real prod values come from the admin portal.
      bankId: this.resolveCredential(s?.vietqr_bank_id, 'VIETQR_BANK_ID') || 'MB',
      accountNumber:
        this.resolveCredential(s?.vietqr_account_number, 'VIETQR_ACCOUNT_NUMBER') ||
        '0000000000',
      accountName:
        this.resolveCredential(s?.vietqr_account_name, 'VIETQR_ACCOUNT_NAME') ||
        'DRAFTRIGHT',
      // Webhook keys: empty when nothing configured.  The two
      // providers (Casso / SePay) are mutually exclusive at the
      // verifyWebhook layer.
      cassoApiKey: this.resolveCredential(s?.casso_api_key, 'CASSO_API_KEY'),
      sepayApiKey: this.resolveCredential(s?.sepay_api_key, 'SEPAY_API_KEY'),
    };
  }

  async createCheckout(payment: Payment, _options?: CreateCheckoutOptions): Promise<CheckoutResult> {
    const { bankId, accountNumber, accountName } = await this.getCredentials();

    // Generate VietQR URL via img.vietqr.io (free, no API key needed).
    // Template choice:
    //   - `compact`  = QR + bank logo only.  Amount is embedded in the
    //     scannable payload but NOT rendered as text on the image, so
    //     users staring at the rendered image saw no "124,000 VND"
    //     number and got confused (bug 2026-06-01).
    //   - `compact2` = same QR + amount + addInfo overlaid as text.
    //     Users can verify the amount visually before scanning, and
    //     the scanned payload is identical to compact.
    const bankInfo = {
      bank_name: this.getBankDisplayName(bankId),
      account_number: accountNumber,
      account_name: accountName,
      amount: payment.amount,
      currency: 'VND',
      reference: payment.reference_code,
    };

    // BANK_TRANSFER and VIETQR share this strategy but have different
    // UX shapes on the client.  Returning both qr_data + bank_info
    // for either method made the Flutter sealed CheckoutResult
    // dispatcher (priority: redirect > qr > bank) always pick
    // QrCheckout, so the Bank-transfer dialog never showed.
    //   - VIETQR        → qr_data + bank_info (manual fallback table)
    //   - BANK_TRANSFER → bank_info only      (no QR — manual transfer flow)
    if (payment.method === PaymentMethod.BANK_TRANSFER) {
      return { payment, bank_info: bankInfo };
    }

    const qrUrl = `https://img.vietqr.io/image/${bankId}-${accountNumber}-compact2.jpg`
      + `?amount=${payment.amount}`
      + `&addInfo=${encodeURIComponent(payment.reference_code)}`
      + `&accountName=${encodeURIComponent(accountName)}`;

    return {
      payment,
      qr_data: qrUrl,
      bank_info: bankInfo,
    };
  }

  /**
   * Verify VietQR/Casso/SePay webhook. Both providers send a header bearing
   * their secret API key — we MUST verify the header matches the configured
   * key. The previous implementation accepted ANY POST and matched on a
   * reference-code regex in the body, which let anyone forge a payment.
   */
  async verifyWebhook(payload: any, headers: any): Promise<WebhookAction> {
    const { cassoApiKey, sepayApiKey } = await this.getCredentials();

    // Header conventions:
    //   Casso:  `Authorization: Apikey <CASSO_API_KEY>` or `Secure-Token: <key>` (legacy)
    //   SePay:  `Authorization: Apikey <SEPAY_API_KEY>` (matches Casso convention)
    //   MB BaaS: bank-side push; signed differently — not supported here.
    const authHeader = (headers['authorization'] || headers['Authorization'] || '').toString();
    const secureToken = (headers['secure-token'] || headers['Secure-Token'] || '').toString();
    const provided = authHeader.replace(/^Apikey\s+/i, '').trim() || secureToken.trim();

    if (!provided) {
      this.logger.warn('VietQR webhook rejected: no Authorization or Secure-Token header.');
      throw new UnauthorizedException('Missing webhook authorization');
    }

    const validKeys = [cassoApiKey, sepayApiKey].filter(Boolean);
    if (validKeys.length === 0) {
      this.logger.warn('VietQR webhook called but no Casso/SePay key configured. Rejecting.');
      throw new UnauthorizedException('VietQR webhooks not configured');
    }

    if (!validKeys.some((key) => this.timingSafeStrEqual(key, provided))) {
      this.logger.warn('VietQR webhook rejected: invalid API key.');
      throw new UnauthorizedException('Invalid webhook authorization');
    }

    // Authenticated. Now match the reference code from any of the supported body shapes.
    // Casso:
    if (payload.data && Array.isArray(payload.data)) {
      for (const tx of payload.data) {
        const ref = extractPaymentReference(tx.description);
        if (ref && tx.amount > 0) {
          return { type: 'payment_completed', reference_code: ref };
        }
      }
    }

    // SePay:
    if (payload.content && payload.transferAmount) {
      const ref = extractPaymentReference(payload.content);
      if (ref && payload.transferAmount > 0) {
        return { type: 'payment_completed', reference_code: ref };
      }
    }

    // MB Bank BaaS:
    if (payload.transactionId && payload.description) {
      const ref = extractPaymentReference(payload.description);
      if (ref && payload.creditAmount > 0) {
        return { type: 'payment_completed', reference_code: ref };
      }
    }

    return { type: 'ignored' };
  }

  private getBankDisplayName(bankId: string): string {
    const banks: Record<string, string> = {
      MB: 'MB Bank (Quân Đội)',
      VCB: 'Vietcombank',
      ACB: 'ACB',
      TCB: 'Techcombank',
      VPB: 'VPBank',
      TPB: 'TPBank',
      BIDV: 'BIDV',
      VTB: 'VietinBank',
      SCB: 'Sacombank',
    };
    return banks[bankId] || bankId;
  }
}
