package payment

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tannpv/draftright-rewrite/internal/payment/strategy"
)

// TestAppleRedeemHandler_GrantsForAuthedUser is the HTTP-layer companion to
// TestRedeemAppleTransaction_GrantsOnceAndStamps (apple_redeem_test.go): same
// Service wiring (newTestServiceForRedeem), driven through the actual
// AppleRedeem handler + the real claims accessor/setter (withClaims,
// handler_checkout_test.go) instead of calling the Service directly.
func TestAppleRedeemHandler_GrantsForAuthedUser(t *testing.T) {
	f := &fakeSubsWriter{}
	svc := newTestServiceForRedeem(t, f, &noopEmailer{}, "com.draftright.pro.monthly", "com.draftright.pro.yearly")
	h := NewHandler(svc)

	req := withClaims(httptest.NewRequest(http.MethodPost, "/payment/apple/redeem",
		strings.NewReader(`{"signedTransaction":"stub-jws-monthly"}`)), "user-9")
	rec := httptest.NewRecorder()
	h.AppleRedeem(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body)
	}
	if !f.granted || f.grantStore != string(StoreAppleIAP) {
		t.Fatalf("handler did not grant apple_iap for the authed user: %+v", f)
	}
}

// TestAppleRedeemHandler_Unauthorized covers the no-claims path (router
// misconfiguration or a route the auth middleware never wrapped) — mirrors
// the "auth context missing" branch every other JWT handler in this package
// has a test for.
func TestAppleRedeemHandler_Unauthorized(t *testing.T) {
	svc := newTestServiceForRedeem(t, &fakeSubsWriter{}, &noopEmailer{}, "com.draftright.pro.monthly", "com.draftright.pro.yearly")
	h := NewHandler(svc)

	req := httptest.NewRequest(http.MethodPost, "/payment/apple/redeem", strings.NewReader(`{"signedTransaction":"x"}`))
	rec := httptest.NewRecorder()
	h.AppleRedeem(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status %d, want 500 (auth context missing — router misconfiguration, not a client error)", rec.Code)
	}
}

// TestAppleRedeemHandler_MissingSignedTransaction covers the validation path.
func TestAppleRedeemHandler_MissingSignedTransaction(t *testing.T) {
	svc := newTestServiceForRedeem(t, &fakeSubsWriter{}, &noopEmailer{}, "com.draftright.pro.monthly", "com.draftright.pro.yearly")
	h := NewHandler(svc)

	req := withClaims(httptest.NewRequest(http.MethodPost, "/payment/apple/redeem", strings.NewReader(`{}`)), "user-9")
	rec := httptest.NewRecorder()
	h.AppleRedeem(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", rec.Code)
	}
}

// TestAppleWebhook_UngatedReachesStrategy proves the webhook path bypasses
// EnabledMethods: apple_iap is absent from fakeSettings' CSV (found:false →
// falls back to DefaultPaymentMethod, which never lists apple_iap either),
// yet a webhook still reaches the strategy and dispatches — because
// AppleWebhook calls the service's HandleProviderNotification, not the
// gated HandleWebhook every other provider's route uses.
func TestAppleWebhook_UngatedReachesStrategy(t *testing.T) {
	subs := &fakeSubsWriter{}
	svc := webhookSvc(fakeSettings{found: false},
		fakeVerifier{action: strategy.WebhookAction{
			Type: strategy.ActionAppleRenewed, AppleOriginalTransactionID: "o-ungated",
			CurrentPeriodEnd: 4102444800, // 2100-01-01
		}},
		&fakeWebhookRepo{}, subs, &noopEmailer{}, fakeVariants{})
	h := NewHandler(svc)

	req := httptest.NewRequest(http.MethodPost, "/payment/webhook/apple", strings.NewReader(`{"signedPayload":"stub"}`))
	rec := httptest.NewRecorder()
	h.AppleWebhook(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d, body %s (apple_iap must be reachable though it's absent from EnabledMethods)", rec.Code, rec.Body)
	}
	if subs.extended != "apple_iap:o-ungated" {
		t.Fatalf("webhook did not reach the strategy/dispatch: extended=%q", subs.extended)
	}
}
