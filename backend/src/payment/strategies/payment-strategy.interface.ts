import { Payment } from '../entities/payment.entity';

export interface CheckoutResult {
  payment: Payment;
  redirect_url?: string; // Stripe/PayPal checkout URL
  qr_data?: string;      // VietQR base64 or data URL
  bank_info?: {           // For bank transfer display
    bank_name: string;
    account_number: string;
    account_name: string;
    amount: number;
    currency: string;
    reference: string;
  };
}

export interface PaymentStrategy {
  createCheckout(payment: Payment, options?: { success_url?: string; cancel_url?: string }): Promise<CheckoutResult>;
  verifyWebhook(payload: any, headers: any): Promise<{ reference_code: string; status: 'completed' | 'failed' } | null>;
}
