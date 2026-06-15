// Package strategy is the payment-provider port: the Service depends on the
// Strategy interface; concrete providers live in sub-packages (stripe, vietqr,
// lemonsqueezy). All types here are plain values with zero infra deps so the
// use case and tests never touch pgx/http.
package strategy

import (
	"context"
	"crypto/subtle"
)

// Payment is the just-created payment row a strategy reads to build checkout.
type Payment struct {
	ID            string
	UserID        string
	PlanID        string
	Amount        int
	Currency      string
	Method        string
	ReferenceCode string
}

// Plan is the plan context a strategy needs (Stripe price, trial, cadence).
type Plan struct {
	Name          string
	StripePriceID string // "" when unset
	TrialDays     int
	BillingPeriod string // "monthly" | "yearly" | "none" | ...
	PriceCents    int
}

// Options ports CreateCheckoutOptions + the service-injected user fields.
type Options struct {
	SuccessURL       string
	CancelURL        string
	StripeCustomerID string // user.stripe_customer_id ("" = none)
	UserEmail        string
}

// PortalUser is the subset getCustomerPortalUrl needs.
type PortalUser struct {
	StripeCustomerID       string
	LemonSqueezyCustomerID string
}

// BankInfo is the VietQR/bank-transfer manual-payment block.
type BankInfo struct {
	BankName      string `json:"bank_name"`
	AccountNumber string `json:"account_number"`
	AccountName   string `json:"account_name"`
	Amount        int    `json:"amount"`
	Currency      string `json:"currency"`
	Reference     string `json:"reference"`
}

// WalletIntent is the Apple/Google Pay PaymentIntent block. MerchantIdentifier
// is omitted when empty (Node sends `undefined`).
type WalletIntent struct {
	ClientSecret       string `json:"client_secret"`
	PublishableKey     string `json:"publishable_key"`
	MerchantIdentifier string `json:"merchant_identifier,omitempty"`
	CountryCode        string `json:"country_code"`
	CurrencyCode       string `json:"currency_code"`
	DisplayAmount      string `json:"display_amount"`
	DisplayLabel       string `json:"display_label"`
}

// Result is what a strategy returns from CreateCheckout. The Service composes
// the HTTP response from this + the persisted payment row. Empty optional
// fields are omitted from JSON by the response marshaler.
type Result struct {
	RedirectURL  string        // "" → omit
	QRData       string        // "" → omit (also persisted to payments.qr_data)
	BankInfo     *BankInfo     // nil → omit
	WalletIntent *WalletIntent // nil → omit
}

// WebhookAction Type discriminators. Match the Node WebhookAction union tags
// (payment-strategy.interface.ts) byte-for-byte — Service.HandleWebhook
// switches on them.
const (
	ActionPaymentCompleted       = "payment_completed"
	ActionPaymentFailed          = "payment_failed"
	ActionSubscriptionRenewed    = "subscription_renewed"
	ActionSubscriptionCanceled   = "subscription_canceled"
	ActionDisputeCreated         = "dispute_created"
	ActionLSPaymentSuccess       = "lemonsqueezy_payment_success"
	ActionLSPaymentFailed        = "lemonsqueezy_payment_failed"
	ActionLSSubscriptionCanceled = "lemonsqueezy_subscription_canceled"
	ActionLSSubscriptionExpired  = "lemonsqueezy_subscription_expired"
	ActionIgnored                = "ignored"
)

// WebhookAction is the verified-webhook outcome the Service dispatches on. It
// flattens Node's discriminated union: only the fields relevant to Type are
// populated; the rest stay zero. CurrentPeriodEnd is unix seconds.
type WebhookAction struct {
	Type                 string
	ReferenceCode        string
	StripeSubscriptionID string
	StripeCustomerID     string
	StripeChargeID       string
	Amount               int
	CurrentPeriodEnd     int64
	LSSubscriptionID     string
	LSCustomerID         string
	LSVariantID          string
}

// Ignored is the no-op action (returned for events we don't act on).
func Ignored() WebhookAction { return WebhookAction{Type: ActionIgnored} }

// Strategy is the provider port. CreateCheckout may hit an external API and
// return an error (the Service marks the payment failed + 400s). Portal/Cancel
// default to "" / false for providers that don't support them.
type Strategy interface {
	CreateCheckout(ctx context.Context, p Payment, plan Plan, opts Options) (Result, error)
	// CustomerPortalURL returns "" when there is no portal for this user.
	CustomerPortalURL(ctx context.Context, u PortalUser) (string, error)
	// CancelSubscription returns false when the provider can't cancel.
	CancelSubscription(ctx context.Context, subscriptionID string) (bool, error)
}

// WebhookError carries an HTTP status + exact Node message out of a strategy's
// VerifyWebhook. Status 400 → BadRequestException parity; 401 →
// UnauthorizedException parity. The Service converts it to its DomainError.
type WebhookError struct {
	Status  int
	Message string
}

func (e *WebhookError) Error() string { return e.Message }

// ResolveCredential ports BasePaymentStrategy.resolveCredential: DB value wins
// when non-empty, else the env fallback, else "".
func ResolveCredential(fromDB, env string) string {
	if fromDB != "" {
		return fromDB
	}
	return env
}

// TimingSafeStrEqual ports timingSafeStrEqual: constant-time, length-checked.
func TimingSafeStrEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
