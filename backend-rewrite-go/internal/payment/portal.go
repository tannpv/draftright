package payment

import (
	"context"
	"encoding/json"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/payment/strategy"
	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// storeTypePortalKey maps a subscription's store_type to the strategy key for
// the customer-portal path. Mirrors getCustomerPortalUrl's switch: only
// stripe + lemonsqueezy have a hosted portal (PayPal deliberately absent —
// it has no portal concept).
func storeTypePortalKey(storeType string) (string, bool) {
	switch storeType {
	case "stripe":
		return "stripe", true
	case "lemonsqueezy":
		return "lemonsqueezy", true
	default:
		return "", false
	}
}

// storeTypeCancelKey maps a subscription's store_type to the strategy key for
// the in-app cancel path. Mirrors cancelActiveSubscription's switch: PayPal
// has no hosted portal but DOES support programmatic cancel.
func storeTypeCancelKey(storeType string) (string, bool) {
	switch storeType {
	case "stripe":
		return "stripe", true
	case "lemonsqueezy":
		return "lemonsqueezy", true
	case "paypal":
		return "paypal", true
	default:
		return "", false
	}
}

// PortalUserFrom adapts the checkout user to the strategy's portal input.
func PortalUserFrom(u *CheckoutUser) strategy.PortalUser {
	return strategy.PortalUser{StripeCustomerID: u.StripeCustomerID, LemonSqueezyCustomerID: u.LemonSqueezyCustomerID}
}

// CustomerPortalURL ports getCustomerPortalUrl. Every guard is a 404 (NotFound).
func (s *Service) CustomerPortalURL(ctx context.Context, userID string) (string, error) {
	user, err := s.checkoutRepo.UserForCheckout(ctx, userID)
	if err != nil {
		return "", err
	}
	if user == nil {
		return "", notFound("User not found")
	}
	sub, err := s.subs.ActiveByUser(ctx, userID)
	if err != nil {
		return "", err
	}
	if sub == nil {
		return "", notFound("No active subscription to manage")
	}
	key, ok := storeTypePortalKey(sub.StoreType)
	if !ok {
		return "", notFound("Subscriptions sourced from '" + sub.StoreType + "' have no self-service portal")
	}
	strat, ok := s.strategies[key]
	if !ok {
		return "", notFound("Subscriptions sourced from '" + sub.StoreType + "' have no self-service portal")
	}
	url, err := strat.CustomerPortalURL(ctx, PortalUserFrom(user))
	if err != nil {
		return "", internalError(err.Error())
	}
	if url == "" {
		return "", notFound("Customer portal is not available for this subscription")
	}
	return url, nil
}

// CancelResult ports the cancel response body. expires_at is an ISO-ms string
// (or null), matching Node's Date serialization — NOT Go's default RFC3339-nano.
type CancelResult struct {
	Cancelled bool
	ExpiresAt *time.Time
}

// MarshalJSON pins {"cancelled":bool,"expires_at":<ISOMillis|null>}.
func (c CancelResult) MarshalJSON() ([]byte, error) {
	var exp *string
	if c.ExpiresAt != nil {
		iso := shared.ISOMillis(*c.ExpiresAt)
		exp = &iso
	}
	return json.Marshal(struct {
		Cancelled bool    `json:"cancelled"`
		ExpiresAt *string `json:"expires_at"`
	}{Cancelled: c.Cancelled, ExpiresAt: exp})
}

// CancelActiveSubscription ports cancelActiveSubscription. Every guard is 404.
func (s *Service) CancelActiveSubscription(ctx context.Context, userID string) (*CancelResult, error) {
	user, err := s.checkoutRepo.UserForCheckout(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, notFound("User not found")
	}
	sub, err := s.subs.ActiveByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if sub == nil {
		return nil, notFound("No active subscription to cancel")
	}
	if sub.StoreTransactionID == "" {
		return nil, notFound("Subscription has no provider reference to cancel")
	}
	key, ok := storeTypeCancelKey(sub.StoreType)
	if !ok {
		return nil, notFound("Subscriptions sourced from '" + sub.StoreType + "' can't be cancelled in-app")
	}
	strat, ok := s.strategies[key]
	if !ok {
		return nil, notFound("Subscriptions sourced from '" + sub.StoreType + "' can't be cancelled in-app")
	}
	cancelled, err := strat.CancelSubscription(ctx, sub.StoreTransactionID)
	if err != nil {
		return nil, internalError(err.Error())
	}
	if !cancelled {
		return nil, notFound("Provider declined to cancel the subscription")
	}
	return &CancelResult{Cancelled: true, ExpiresAt: sub.ExpiresAt}, nil
}
