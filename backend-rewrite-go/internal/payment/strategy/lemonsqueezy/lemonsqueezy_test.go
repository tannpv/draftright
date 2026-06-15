package lemonsqueezy

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tannpv/draftright-rewrite/internal/payment/strategy"
)

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
