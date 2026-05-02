import { Injectable } from '@nestjs/common';
import { PaymentStrategy, CheckoutResult } from './payment-strategy.interface';
import { Payment } from '../entities/payment.entity';

@Injectable()
export class VietQRStrategy implements PaymentStrategy {
  private get bankId() { return process.env.VIETQR_BANK_ID || 'MB'; } // MB Bank
  private get accountNumber() { return process.env.VIETQR_ACCOUNT_NUMBER || '0000000000'; }
  private get accountName() { return process.env.VIETQR_ACCOUNT_NAME || 'DRAFTRIGHT'; }
  private get cassoApiKey() { return process.env.CASSO_API_KEY || ''; }
  private get sepayApiKey() { return process.env.SEPAY_API_KEY || ''; }

  async createCheckout(payment: Payment): Promise<CheckoutResult> {

    // Generate VietQR URL via img.vietqr.io (free, no API key needed)
    const qrUrl = `https://img.vietqr.io/image/${this.bankId}-${this.accountNumber}-compact.jpg`
      + `?amount=${payment.amount}`
      + `&addInfo=${encodeURIComponent(payment.reference_code)}`
      + `&accountName=${encodeURIComponent(this.accountName)}`;

    return {
      payment,
      qr_data: qrUrl,
      bank_info: {
        bank_name: this.getBankDisplayName(this.bankId),
        account_number: this.accountNumber,
        account_name: this.accountName,
        amount: payment.amount,
        currency: 'VND',
        reference: payment.reference_code,
      },
    };
  }

  async verifyWebhook(payload: any, headers: any): Promise<{ reference_code: string; status: 'completed' | 'failed' } | null> {
    // Try Casso webhook format
    if (payload.data && Array.isArray(payload.data)) {
      for (const tx of payload.data) {
        const desc = (tx.description || '').toUpperCase();
        // Look for our reference code pattern: DR-PRO-xxxxx
        const match = desc.match(/DR-[A-Z]+-[A-Z0-9]+/);
        if (match && tx.amount > 0) {
          return { reference_code: match[0], status: 'completed' };
        }
      }
    }

    // Try SePay webhook format
    if (payload.content && payload.transferAmount) {
      const desc = (payload.content || '').toUpperCase();
      const match = desc.match(/DR-[A-Z]+-[A-Z0-9]+/);
      if (match && payload.transferAmount > 0) {
        return { reference_code: match[0], status: 'completed' };
      }
    }

    // Try MB Bank BaaS webhook format
    if (payload.transactionId && payload.description) {
      const desc = (payload.description || '').toUpperCase();
      const match = desc.match(/DR-[A-Z]+-[A-Z0-9]+/);
      if (match && payload.creditAmount > 0) {
        return { reference_code: match[0], status: 'completed' };
      }
    }

    return null;
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
