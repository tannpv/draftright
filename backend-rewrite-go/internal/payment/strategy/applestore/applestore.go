package applestore

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/tannpv/draftright-rewrite/internal/payment/strategy"
)

const manageSubscriptionsURL = "itms-apps://apps.apple.com/account/subscriptions"

// notificationType → strategy action.
var notifAction = map[string]string{
	"SUBSCRIBED":                strategy.ActionAppleSubscribed,
	"DID_RENEW":                 strategy.ActionAppleRenewed,
	"EXPIRED":                   strategy.ActionAppleExpired,
	"GRACE_PERIOD_EXPIRED":      strategy.ActionAppleExpired,
	"REFUND":                    strategy.ActionAppleRefunded,
	"DID_CHANGE_RENEWAL_STATUS": strategy.ActionAppleRenewed, // status flips don't grant; mapped, handler decides
}

type Strategy struct{ v *Verifier }

var _ strategy.Strategy = (*Strategy)(nil)

func New(v *Verifier) *Strategy { return &Strategy{v: v} }

// CreateCheckout: IAP is redeemed, not checked out. Never routed (absent from
// registeredMethods); returns the honest sentinel if it ever is.
func (s *Strategy) CreateCheckout(ctx context.Context, p strategy.Payment, plan strategy.Plan, opts strategy.Options) (strategy.Result, error) {
	return strategy.Result{}, strategy.ErrNotCheckoutMethod
}

// CustomerPortalURL: Apple subscriptions are managed in Settings.
func (s *Strategy) CustomerPortalURL(ctx context.Context, u strategy.PortalUser) (string, error) {
	return manageSubscriptionsURL, nil
}

// CancelSubscription: Apple forbids server-side cancel.
func (s *Strategy) CancelSubscription(ctx context.Context, subscriptionID string) (bool, error) {
	return false, nil
}

// VerifyWebhook: App Store Server Notifications V2 — the body is
// {"signedPayload": "<JWS>"}; the JWS decodes to {notificationType, data:{signedTransactionInfo}}.
func (s *Strategy) VerifyWebhook(ctx context.Context, payload []byte, headers http.Header) (strategy.WebhookAction, error) {
	var body struct {
		SignedPayload string `json:"signedPayload"`
	}
	if err := json.Unmarshal(payload, &body); err != nil || body.SignedPayload == "" {
		return strategy.WebhookAction{}, &strategy.WebhookError{Status: 400, Message: "missing signedPayload"}
	}
	return s.VerifyNotification(body.SignedPayload)
}

// VerifyNotification verifies an ASSN V2 signed payload and its inner
// transaction, returning the mapped action.
//
// The OUTER envelope is signature-verified only (VerifyEnvelope): it carries
// notificationType + data.signedTransactionInfo and has NO top-level
// bundleId/environment, so running the claim-checking Verify on it would always
// fail (review C3). bundleId/environment are checked on the INNER transaction.
func (s *Strategy) VerifyNotification(signedPayload string) (strategy.WebhookAction, error) {
	if s.v == nil {
		return strategy.WebhookAction{}, errors.New("applestore: verifier not configured")
	}
	envBytes, err := s.v.VerifyEnvelope(signedPayload)
	if err != nil {
		return strategy.WebhookAction{}, &strategy.WebhookError{Status: 401, Message: "notification signature: " + err.Error()}
	}
	var env struct {
		NotificationType string `json:"notificationType"`
		Data             struct {
			SignedTransactionInfo string `json:"signedTransactionInfo"`
		} `json:"data"`
	}
	if err := json.Unmarshal(envBytes, &env); err != nil || env.Data.SignedTransactionInfo == "" {
		return strategy.WebhookAction{}, &strategy.WebhookError{Status: 400, Message: "notification: no signedTransactionInfo"}
	}
	tx, err := s.v.Verify(env.Data.SignedTransactionInfo) // inner: full claim check
	if err != nil {
		return strategy.WebhookAction{}, &strategy.WebhookError{Status: 401, Message: "transaction signature: " + err.Error()}
	}
	act, ok := notifAction[env.NotificationType]
	if !ok {
		return strategy.Ignored(), nil
	}
	return strategy.WebhookAction{
		Type:                       act,
		AppleTransactionID:         tx.TransactionID,
		AppleOriginalTransactionID: tx.OriginalTransactionID,
		AppleProductID:             tx.ProductID,
		CurrentPeriodEnd:           tx.ExpiresDate / 1000, // ms → unix seconds
	}, nil
}
