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
