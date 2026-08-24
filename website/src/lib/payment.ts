// Payment-method + store-type identifiers — the website's single source of
// truth, mirroring the Go backend enums (backend-rewrite-go/internal/payment/
// {methods.go, domain.go}). Keeps raw literals like `methodKey === 'lemonsqueezy'`
// out of the components (DraftRight RULE #1 — no hardcoding, one source of truth).

export const PaymentMethodKey = {
  STRIPE:        'stripe',
  VIETQR:        'vietqr',
  BANK_TRANSFER: 'bank_transfer',
  LEMONSQUEEZY:  'lemonsqueezy',
} as const;
export type PaymentMethodKey = (typeof PaymentMethodKey)[keyof typeof PaymentMethodKey];

// StoreTypeKey identifies which provider billed a subscription (sub.store_type).
// The backend models this as a separate StoreType enum from PaymentMethod (the
// wire values coincide); the in-app cancel/manage flow gates on these.
export const StoreTypeKey = {
  STRIPE:       'stripe',
  LEMONSQUEEZY: 'lemonsqueezy',
} as const;
export type StoreTypeKey = (typeof StoreTypeKey)[keyof typeof StoreTypeKey];

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
