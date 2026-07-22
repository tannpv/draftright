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
  /**
   * Native wallet (Apple Pay / Google Pay) checkout payload — the
   * mobile SDK confirms a PaymentIntent client-side without
   * redirecting the user.  Sent instead of `redirect_url` when the
   * inbound payment.method is `apple_pay` or `google_pay`.
   *
   * - `client_secret`: Stripe PaymentIntent.client_secret used by
   *   `flutter_stripe` to present the platform sheet.
   * - `publishable_key`: Stripe publishable key the mobile SDK needs
   *   to talk to Stripe directly.
   * - `merchant_identifier` (Apple Pay only): the Apple Pay merchant
   *   ID configured in the Stripe + Apple dashboards.
   * - `country_code`: ISO-3166 two-letter code for the merchant
   *   country (Stripe checks this against the wallet's currency).
   * - `currency_code`: ISO-4217 three-letter code; mirrors
   *   payment.currency so the sheet shows the correct amount.
   */
  wallet_intent?: {
    client_secret: string;
    publishable_key: string;
    merchant_identifier?: string;
    country_code: string;
    currency_code: string;
    /**
     * Display amount for the platform sheet's line-item.  Apple Pay
     * shows this string verbatim above the "Pay" button; Google Pay
     * uses it as the order summary line.  Format is a positive decimal
     * string ("4.99") — Stripe SDK does NOT convert from cents.
     */
    display_amount: string;
    /**
     * Human label for the line-item ("DraftRight Pro · Monthly").
     */
    display_label: string;
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
  // Lemon Squeezy webhook events.  Distinct from the Stripe-prefixed
  // variants above because the subscription ID lives under
  // `data.id` rather than `data.object.id`, and because LS reports
  // a "renews_at" date rather than `current_period_end` (unix).
  // PaymentService.handleWebhook dispatches these to the generic
  // SubscriptionsService.*ByStoreRef helpers with StoreType.LEMONSQUEEZY.
  | { type: 'lemonsqueezy_payment_success'; reference_code: string; lemonsqueezy_subscription_id: string; lemonsqueezy_customer_id?: string; lemonsqueezy_variant_id?: string; current_period_end: number /* unix sec */ }
  | { type: 'lemonsqueezy_payment_failed';   lemonsqueezy_subscription_id: string }
  | { type: 'lemonsqueezy_subscription_canceled'; lemonsqueezy_subscription_id: string }
  | { type: 'lemonsqueezy_subscription_expired';  lemonsqueezy_subscription_id: string }
  // PayPal Subscriptions webhook events. Same generic shape as LS —
  // the subscription id is PayPal's `resource.id` (subscription events)
  // or `resource.billing_agreement_id` (PAYMENT.SALE.COMPLETED), and
  // `reference_code` echoes back from the subscription `custom_id`.
  // PaymentService.handleWebhook dispatches these to the generic
  // SubscriptionsService.*ByStoreRef helpers with StoreType.PAYPAL.
  | { type: 'paypal_payment_success'; reference_code: string; paypal_subscription_id: string; current_period_end: number /* unix sec */ }
  | { type: 'paypal_payment_failed';   paypal_subscription_id: string }
  | { type: 'paypal_subscription_canceled'; paypal_subscription_id: string }
  | { type: 'paypal_subscription_expired';  paypal_subscription_id: string }
  | { type: 'ignored' };

// The strategy contract is BasePaymentStrategy (abstract class). These are the
// shared data shapes its methods use.

export interface CreateCheckoutOptions {
  success_url?: string;
  cancel_url?: string;
  /** Pre-existing Stripe Customer ID (cus_XXXX) to reuse, or null if new. */
  stripe_customer_id?: string | null;
  /** User email (always available; required if stripe_customer_id is null). */
  user_email?: string;
}
