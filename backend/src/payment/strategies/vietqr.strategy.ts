import { Injectable, Logger, UnauthorizedException } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { PaymentStrategy, CheckoutResult, WebhookAction, CreateCheckoutOptions } from './payment-strategy.interface';
import { Payment } from '../entities/payment.entity';
import { AppSettings } from '../../admin/entities/app-settings.entity';

@Injectable()
export class VietQRStrategy implements PaymentStrategy {
  private readonly logger = new Logger(VietQRStrategy.name);

  constructor(
    @InjectRepository(AppSettings)
    private readonly settingsRepo: Repository<AppSettings>,
  ) {}

  /** Read bank + provider credentials from AppSettings (admin portal), env fallback. */
  private async getCredentials() {
    const s = await this.settingsRepo.findOne({ where: {} });
    return {
      bankId: s?.vietqr_bank_id || process.env.VIETQR_BANK_ID || 'MB',
      accountNumber: s?.vietqr_account_number || process.env.VIETQR_ACCOUNT_NUMBER || '0000000000',
      accountName: s?.vietqr_account_name || process.env.VIETQR_ACCOUNT_NAME || 'DRAFTRIGHT',
      cassoApiKey: s?.casso_api_key || process.env.CASSO_API_KEY || '',
      sepayApiKey: s?.sepay_api_key || process.env.SEPAY_API_KEY || '',
    };
  }

  async createCheckout(payment: Payment, _options?: CreateCheckoutOptions): Promise<CheckoutResult> {
    const { bankId, accountNumber, accountName } = await this.getCredentials();

    // Generate VietQR URL via img.vietqr.io (free, no API key needed)
    const qrUrl = `https://img.vietqr.io/image/${bankId}-${accountNumber}-compact.jpg`
      + `?amount=${payment.amount}`
      + `&addInfo=${encodeURIComponent(payment.reference_code)}`
      + `&accountName=${encodeURIComponent(accountName)}`;

    return {
      payment,
      qr_data: qrUrl,
      bank_info: {
        bank_name: this.getBankDisplayName(bankId),
        account_number: accountNumber,
        account_name: accountName,
        amount: payment.amount,
        currency: 'VND',
        reference: payment.reference_code,
      },
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

    if (!validKeys.includes(provided)) {
      this.logger.warn('VietQR webhook rejected: invalid API key.');
      throw new UnauthorizedException('Invalid webhook authorization');
    }

    // Authenticated. Now match the reference code from any of the supported body shapes.
    // Casso:
    if (payload.data && Array.isArray(payload.data)) {
      for (const tx of payload.data) {
        const desc = (tx.description || '').toUpperCase();
        const match = desc.match(/DR-[A-Z]+-[A-Z0-9]+/);
        if (match && tx.amount > 0) {
          return { type: 'payment_completed', reference_code: match[0] };
        }
      }
    }

    // SePay:
    if (payload.content && payload.transferAmount) {
      const desc = (payload.content || '').toUpperCase();
      const match = desc.match(/DR-[A-Z]+-[A-Z0-9]+/);
      if (match && payload.transferAmount > 0) {
        return { type: 'payment_completed', reference_code: match[0] };
      }
    }

    // MB Bank BaaS:
    if (payload.transactionId && payload.description) {
      const desc = (payload.description || '').toUpperCase();
      const match = desc.match(/DR-[A-Z]+-[A-Z0-9]+/);
      if (match && payload.creditAmount > 0) {
        return { type: 'payment_completed', reference_code: match[0] };
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
