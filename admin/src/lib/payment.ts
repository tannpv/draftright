// Payment-method identifiers — the admin portal's single source of truth,
// mirroring the Go backend PaymentMethod enum (backend-rewrite-go/internal/
// payment/methods.go). RULE #1: no raw method literals scattered across pages.
export const PaymentMethodKey = {
  STRIPE:        'stripe',
  PAYPAL:        'paypal',
  VIETQR:        'vietqr',
  BANK_TRANSFER: 'bank_transfer',
} as const;
export type PaymentMethodKey = (typeof PaymentMethodKey)[keyof typeof PaymentMethodKey];
