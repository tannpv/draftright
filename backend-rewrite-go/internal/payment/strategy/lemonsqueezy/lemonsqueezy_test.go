package lemonsqueezy

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tannpv/draftright-rewrite/internal/payment/strategy"
)

func lsSign(payload, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestLSVerifyWebhook_PaymentSuccess(t *testing.T) {
	s := New(Creds{WebhookSecret: "shh"}, "")
	payload := `{"meta":{"event_name":"subscription_payment_success","custom_data":{"reference_code":"DR-PRO-XY"}},"data":{"id":"99","attributes":{"renews_at":"2027-01-01T00:00:00.000000Z","customer_id":555,"variant_id":42}}}`
	h := http.Header{}
	h.Set("X-Signature", lsSign(payload, "shh"))
	act, err := s.VerifyWebhook(context.Background(), []byte(payload), h)
	if err != nil {
		t.Fatal(err)
	}
	if act.Type != strategy.ActionLSPaymentSuccess || act.ReferenceCode != "DR-PRO-XY" ||
		act.LSSubscriptionID != "99" || act.LSCustomerID != "555" || act.LSVariantID != "42" ||
		act.CurrentPeriodEnd != 1798761600 {
		t.Fatalf("bad action: %+v", act)
	}
}

func TestLSVerifyWebhook_UnsetSecretIgnored(t *testing.T) {
	s := New(Creds{WebhookSecret: ""}, "")
	act, err := s.VerifyWebhook(context.Background(), []byte(`{}`), http.Header{})
	if err != nil || act.Type != strategy.ActionIgnored {
		t.Fatalf("want ignored,nil; got %+v,%v", act, err)
	}
}

func TestLSVerifyWebhook_BadSignature400(t *testing.T) {
	s := New(Creds{WebhookSecret: "shh"}, "")
	h := http.Header{}
	h.Set("X-Signature", lsSign(`{}`, "wrong"))
	_, err := s.VerifyWebhook(context.Background(), []byte(`{}`), h)
	var we *strategy.WebhookError
	if !errors.As(err, &we) || we.Status != 400 || we.Message != "Invalid Lemon Squeezy webhook signature" {
		t.Fatalf("want 400 invalid-sig, got %v", err)
	}
}

func TestLSVerifyWebhook_CancelledExpiredFailed(t *testing.T) {
	s := New(Creds{WebhookSecret: "shh"}, "")
	for name, want := range map[string]string{
		"subscription_cancelled":      strategy.ActionLSSubscriptionCanceled,
		"subscription_expired":        strategy.ActionLSSubscriptionExpired,
		"subscription_payment_failed": strategy.ActionLSPaymentFailed,
	} {
		payload := `{"meta":{"event_name":"` + name + `"},"data":{"id":"7"}}`
		h := http.Header{}
		h.Set("X-Signature", lsSign(payload, "shh"))
		act, err := s.VerifyWebhook(context.Background(), []byte(payload), h)
		if err != nil || act.Type != want || act.LSSubscriptionID != "7" {
			t.Fatalf("%s → %+v %v", name, act, err)
		}
	}
}

func TestLSVerifyWebhook_UpdatedIgnored(t *testing.T) {
	s := New(Creds{WebhookSecret: "shh"}, "")
	payload := `{"meta":{"event_name":"subscription_updated"},"data":{"id":"7"}}`
	h := http.Header{}
	h.Set("X-Signature", lsSign(payload, "shh"))
	act, err := s.VerifyWebhook(context.Background(), []byte(payload), h)
	if err != nil || act.Type != strategy.ActionIgnored {
		t.Fatalf("want ignored, got %+v %v", act, err)
	}
}

func TestLS_NotConfigured(t *testing.T) {
	s := New(Creds{}, "http://localhost:4000")
	_, err := s.CreateCheckout(context.Background(), strategy.Payment{ReferenceCode: "DR-PRO-1"}, strategy.Plan{}, strategy.Options{})
	if err == nil {
		t.Fatal("missing apiKey/storeId must error")
	}
}

func TestLS_CheckoutBodyAndVariant(t *testing.T) {
	var gotBody map[string]any
	var gotAuth, gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(200)
		w.Write([]byte(`{"data":{"attributes":{"url":"https://ls.example/checkout/xyz"}}}`))
	}))
	defer srv.Close()

	s := New(Creds{APIKey: "key", StoreID: "55", VariantMonthly: "111", VariantYearly: "222"}, "http://localhost:4000")
	s.apiBase = srv.URL
	res, err := s.CreateCheckout(context.Background(),
		strategy.Payment{ReferenceCode: "DR-PRO-ABCD1234"},
		strategy.Plan{BillingPeriod: "yearly"},
		strategy.Options{UserEmail: "u@x.com"})
	if err != nil {
		t.Fatal(err)
	}
	if res.RedirectURL != "https://ls.example/checkout/xyz" {
		t.Fatalf("redirect=%q", res.RedirectURL)
	}
	if gotAuth != "Bearer key" || gotAccept != "application/vnd.api+json" {
		t.Fatalf("headers: auth=%q accept=%q", gotAuth, gotAccept)
	}
	attrs := gotBody["data"].(map[string]any)["attributes"].(map[string]any)
	po := attrs["product_options"].(map[string]any)
	ev := po["enabled_variants"].([]any)
	if len(ev) != 1 || ev[0].(float64) != 222 {
		t.Fatalf("yearly must select variant 222, got %v", ev)
	}
}

func TestLS_CheckoutErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(422)
		w.Write([]byte(`{"errors":[{"detail":"bad variant"}]}`))
	}))
	defer srv.Close()
	s := New(Creds{APIKey: "k", StoreID: "1", VariantMonthly: "1"}, "http://localhost:4000")
	s.apiBase = srv.URL
	_, err := s.CreateCheckout(context.Background(), strategy.Payment{ReferenceCode: "DR-PRO-1"}, strategy.Plan{}, strategy.Options{})
	if err == nil {
		t.Fatal("non-2xx LS response must error")
	}
	// Byte-exact Node parity: payment.service.ts re-throws err.message to the client.
	if got, want := err.Error(), "Lemon Squeezy checkout failed (422)"; got != want {
		t.Fatalf("checkout error = %q, want %q", got, want)
	}
}

func TestLS_PortalErrorMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`{"errors":[{"detail":"boom"}]}`))
	}))
	defer srv.Close()
	s := New(Creds{APIKey: "k"}, "http://localhost:4000")
	s.apiBase = srv.URL
	_, err := s.CustomerPortalURL(context.Background(), strategy.PortalUser{LemonSqueezyCustomerID: "cust-1"})
	if err == nil {
		t.Fatal("non-2xx LS response must error")
	}
	if got, want := err.Error(), "Could not load customer portal"; got != want {
		t.Fatalf("portal error = %q, want %q", got, want)
	}
}

func TestLS_CancelErrorMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`{"errors":[{"detail":"no sub"}]}`))
	}))
	defer srv.Close()
	s := New(Creds{APIKey: "k"}, "http://localhost:4000")
	s.apiBase = srv.URL
	_, err := s.CancelSubscription(context.Background(), "sub-1")
	if err == nil {
		t.Fatal("non-2xx LS response must error")
	}
	if got, want := err.Error(), "Could not cancel the subscription"; got != want {
		t.Fatalf("cancel error = %q, want %q", got, want)
	}
}

func TestLS_CheckoutEmailKeyOmission(t *testing.T) {
	capture := func(email string) map[string]any {
		var gotBody map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &gotBody)
			w.WriteHeader(200)
			w.Write([]byte(`{"data":{"attributes":{"url":"https://ls.example/c"}}}`))
		}))
		defer srv.Close()
		s := New(Creds{APIKey: "k", StoreID: "1", VariantMonthly: "111"}, "http://localhost:4000")
		s.apiBase = srv.URL
		if _, err := s.CreateCheckout(context.Background(),
			strategy.Payment{ReferenceCode: "DR-PRO-1"},
			strategy.Plan{},
			strategy.Options{UserEmail: email}); err != nil {
			t.Fatal(err)
		}
		attrs := gotBody["data"].(map[string]any)["attributes"].(map[string]any)
		return attrs["checkout_data"].(map[string]any)
	}

	// Empty email → Node's JSON.stringify drops the key; Go must too.
	if cd := capture(""); func() bool { _, ok := cd["email"]; return ok }() {
		t.Fatalf("empty UserEmail must omit email key, got %v", cd)
	}
	// Non-empty email → key present with value.
	if cd := capture("u@x.com"); cd["email"] != "u@x.com" {
		t.Fatalf("non-empty UserEmail must set email=u@x.com, got %v", cd["email"])
	}
}
