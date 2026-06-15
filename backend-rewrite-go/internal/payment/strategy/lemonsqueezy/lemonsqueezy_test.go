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
}
