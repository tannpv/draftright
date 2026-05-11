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

/**
 * Outcome of verifying a webhook payload. The PaymentService dispatches on
 * `type` and routes to the appropriate handler. `ignored` is returned for
 * webhook events we don't act on (e.g. customer.created).
 */
export type WebhookAction =
  | { type: 'payment_completed'; reference_code: string; stripe_subscription_id?: string; stripe_customer_id?: string }
  | { type: 'payment_failed';    reference_code: string }
  | { type: 'subscription_renewed';  stripe_subscription_id: string; current_period_end: number /* unix sec */ }
  | { type: 'subscription_canceled'; stripe_subscription_id: string }
  | { type: 'dispute_created';   stripe_charge_id: string; amount: number }
  | { type: 'ignored' };

export interface PaymentStrategy {
  createCheckout(payment: Payment, options?: CreateCheckoutOptions): Promise<CheckoutResult>;
  verifyWebhook(payload: any, headers: any): Promise<WebhookAction>;
}

export interface CreateCheckoutOptions {
  success_url?: string;
  cancel_url?: string;
  /** Pre-existing Stripe Customer ID (cus_XXXX) to reuse, or null if new. */
  stripe_customer_id?: string | null;
  /** User email (always available; required if stripe_customer_id is null). */
  user_email?: string;
}
