package applestore

import (
	"context"
	"net/http"
	"testing"

	"github.com/tannpv/draftright-rewrite/internal/payment/strategy"
)

func TestStrategy_ContractShape(t *testing.T) {
	var _ strategy.Strategy = New(nil) // compile-time interface assertion

	s := New(nil)
	if _, err := s.CreateCheckout(context.Background(), strategy.Payment{}, strategy.Plan{}, strategy.Options{}); err != strategy.ErrNotCheckoutMethod {
		t.Fatalf("CreateCheckout = %v, want ErrNotCheckoutMethod", err)
	}
	url, _ := s.CustomerPortalURL(context.Background(), strategy.PortalUser{})
	if url == "" {
		t.Fatal("CustomerPortalURL should return the manage-subscriptions deep link")
	}
	if ok, _ := s.CancelSubscription(context.Background(), "x"); ok {
		t.Fatal("CancelSubscription must return false (Apple forbids server cancel)")
	}
	if _, err := s.VerifyWebhook(context.Background(), []byte("{}"), http.Header{}); err == nil {
		t.Fatal("VerifyWebhook should reject a body with no signedPayload")
	}
}

// The notifAction map IS the type→action logic (VerifyNotification is thin over
// it, gated only by real crypto). #217's webhook_test already proves
// ActionAppleExpired → ExpireByStoreRef (revoke, no email, idempotent), so
// asserting the map entry proves REVOKE→revoke end to end.
func TestNotifAction_RevokeAndFailToRenew(t *testing.T) {
	// REVOKE (family-sharing access pulled) must revoke now — same action as EXPIRED.
	if got := notifAction["REVOKE"]; got != strategy.ActionAppleExpired {
		t.Fatalf("REVOKE mapped to %q, want %q (revoke via ExpireByStoreRef)", got, strategy.ActionAppleExpired)
	}
	// DID_FAIL_TO_RENEW is billing-retry/grace, NOT expiry — deliberately
	// unmapped so it falls through to Ignored (no entitlement change);
	// GRACE_PERIOD_EXPIRED owns the eventual revoke.
	if _, ok := notifAction["DID_FAIL_TO_RENEW"]; ok {
		t.Fatal("DID_FAIL_TO_RENEW must stay unmapped (→ Ignored); mapping it would change entitlement during grace")
	}
}
