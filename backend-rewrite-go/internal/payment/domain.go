// Package payment ports the NestJS payment subsystem. Phase 3a is the
// byte-deterministic foundation: enums, reference codes, store-type
// mapping, enabled-methods registry, and the read endpoints. Concrete
// strategies, checkout, and webhooks land in Phases 3b/3c.
package payment

// PaymentMethod mirrors backend/src/payment/entities/payment.entity.ts
// PaymentMethod. The string values are the DB/wire names — keep them
// byte-identical to Node (they appear in payments.method and in
// GET /payment/methods).
type PaymentMethod string

const (
	MethodStripe       PaymentMethod = "stripe"
	MethodPayPal       PaymentMethod = "paypal"
	MethodMoMo         PaymentMethod = "momo"
	MethodVietQR       PaymentMethod = "vietqr"
	MethodBankTransfer PaymentMethod = "bank_transfer"
	MethodLemonSqueezy PaymentMethod = "lemonsqueezy"
	MethodApplePay     PaymentMethod = "apple_pay"
	MethodGooglePay    PaymentMethod = "google_pay"
)

// PaymentStatus mirrors the Node PaymentStatus enum.
type PaymentStatus string

const (
	StatusPending   PaymentStatus = "pending"
	StatusCompleted PaymentStatus = "completed"
	StatusFailed    PaymentStatus = "failed"
	StatusExpired   PaymentStatus = "expired"
	StatusRefunded  PaymentStatus = "refunded"
)

// StoreType mirrors backend/src/subscriptions/entities/subscription.entity.ts
// StoreType — the audit field stamped on the subscription a payment grants.
type StoreType string

const (
	StoreGooglePlay   StoreType = "google_play"
	StoreAppleIAP     StoreType = "apple_iap"
	StoreAdminGranted StoreType = "admin_granted"
	StoreLemonSqueezy StoreType = "lemonsqueezy"
	StoreStripe       StoreType = "stripe"
	StoreVietQR       StoreType = "vietqr"
	StoreBankTransfer StoreType = "bank_transfer"
	StorePayPal       StoreType = "paypal"
)

// StoreTypeForMethod maps a payment method to the StoreType its resulting
// subscription row carries. Ports store-type-mapping.ts verbatim. Apple/Google
// Pay run on Stripe → StoreStripe; MoMo + any unknown → StoreAdminGranted.
func StoreTypeForMethod(m PaymentMethod) StoreType {
	switch m {
	case MethodStripe:
		return StoreStripe
	case MethodLemonSqueezy:
		return StoreLemonSqueezy
	case MethodVietQR:
		return StoreVietQR
	case MethodBankTransfer:
		return StoreBankTransfer
	case MethodPayPal:
		return StorePayPal
	case MethodApplePay, MethodGooglePay:
		return StoreStripe
	case MethodMoMo:
		return StoreAdminGranted
	default:
		return StoreAdminGranted
	}
}
