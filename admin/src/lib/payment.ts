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

// PaymentStatusKey mirrors the Go backend PaymentStatus enum (payment/domain.go).
export const PaymentStatusKey = {
  PENDING:   'pending',
  COMPLETED: 'completed',
  FAILED:    'failed',
  EXPIRED:   'expired',
  REFUNDED:  'refunded',
} as const;
export type PaymentStatusKey = (typeof PaymentStatusKey)[keyof typeof PaymentStatusKey];

// SubscriptionStatusKey identifies a subscription's lifecycle state (sub.status).
export const SubscriptionStatusKey = {
  ACTIVE: 'active',
} as const;
export type SubscriptionStatusKey = (typeof SubscriptionStatusKey)[keyof typeof SubscriptionStatusKey];
