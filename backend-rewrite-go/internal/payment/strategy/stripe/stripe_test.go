package stripe

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/payment/strategy"
)

func paymentFixture() strategy.Payment { return strategy.Payment{Method: "stripe"} }
func planFixture() strategy.Plan       { return strategy.Plan{} }
func optsFixture() strategy.Options    { return strategy.Options{} }

func TestDisplayAmount(t *testing.T) {
	cases := []struct {
		currency string
		amount   int
		want     string
	}{
		{"VND", 124000, "124000"},
		{"JPY", 500, "500"},
		{"KRW", 9900, "9900"},
		{"USD", 999, "9.99"},
		{"usd", 1000, "10.00"},
	}
	for _, c := range cases {
		if got := displayAmount(c.currency, c.amount); got != c.want {
			t.Fatalf("displayAmount(%q,%d)=%q want %q", c.currency, c.amount, got, c.want)
		}
	}
}

func TestDisplayLabel(t *testing.T) {
	if got := displayLabel("Pro", "monthly"); got != "Pro · Monthly" {
		t.Fatalf("got %q", got)
	}
	if got := displayLabel("Pro", "yearly"); got != "Pro · Yearly" {
		t.Fatalf("got %q", got)
	}
	if got := displayLabel("", "monthly"); got != "Pro · Monthly" {
		t.Fatalf("empty name → Pro, got %q", got)
	}
	if got := displayLabel("Pro", "none"); got != "Pro" {
		t.Fatalf("no cadence → name only, got %q", got)
	}
}

func signStripe(t *testing.T, payload, secret string, ts int64) string {
	t.Helper()
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%d.%s", ts, payload)
	return fmt.Sprintf("t=%d,v1=%s", ts, hex.EncodeToString(mac.Sum(nil)))
}

func TestStripeVerifyWebhook_CheckoutCompleted(t *testing.T) {
	s := New(Creds{SecretKey: "sk_test", WebhookSecret: "whsec_test"}, Env{})
	payload := `{"id":"evt_1","type":"checkout.session.completed","api_version":"2025-08-27.basil","data":{"object":{"metadata":{"reference_code":"DR-PRO-ABCD"},"subscription":"sub_9","customer":"cus_9"}}}`
	ts := time.Now().Unix()
	h := http.Header{}
	h.Set("Stripe-Signature", signStripe(t, payload, "whsec_test", ts))
	act, err := s.VerifyWebhook(context.Background(), []byte(payload), h)
	if err != nil {
		t.Fatal(err)
	}
	if act.Type != strategy.ActionPaymentCompleted || act.ReferenceCode != "DR-PRO-ABCD" ||
		act.StripeSubscriptionID != "sub_9" || act.StripeCustomerID != "cus_9" {
		t.Fatalf("bad action: %+v", act)
	}
}

func TestStripeVerifyWebhook_UnsetSecretIsIgnored(t *testing.T) {
	s := New(Creds{SecretKey: "sk_test", WebhookSecret: ""}, Env{})
	act, err := s.VerifyWebhook(context.Background(), []byte(`{}`), http.Header{})
	if err != nil || act.Type != strategy.ActionIgnored {
		t.Fatalf("want ignored,nil; got %+v,%v", act, err)
	}
}

func TestStripeVerifyWebhook_BadSignature400(t *testing.T) {
	s := New(Creds{SecretKey: "sk_test", WebhookSecret: "whsec_test"}, Env{})
	h := http.Header{}
	h.Set("Stripe-Signature", "t=1,v1=deadbeef")
	_, err := s.VerifyWebhook(context.Background(), []byte(`{}`), h)
	var we *strategy.WebhookError
	if !errors.As(err, &we) || we.Status != 400 || we.Message != "Invalid Stripe webhook signature" {
		t.Fatalf("want 400 invalid-sig WebhookError, got %v", err)
	}
}

func TestStripeVerifyWebhook_InvoiceRenewed(t *testing.T) {
	s := New(Creds{SecretKey: "sk_test", WebhookSecret: "whsec_test"}, Env{})
	payload := `{"id":"evt_2","type":"invoice.payment_succeeded","api_version":"2025-08-27.basil","data":{"object":{"subscription":"sub_7","lines":{"data":[{"period":{"end":1893456000}}]}}}}`
	ts := time.Now().Unix()
	h := http.Header{}
	h.Set("Stripe-Signature", signStripe(t, payload, "whsec_test", ts))
	act, err := s.VerifyWebhook(context.Background(), []byte(payload), h)
	if err != nil {
		t.Fatal(err)
	}
	if act.Type != strategy.ActionSubscriptionRenewed || act.StripeSubscriptionID != "sub_7" || act.CurrentPeriodEnd != 1893456000 {
		t.Fatalf("bad renew action: %+v", act)
	}
}

func TestStripeVerifyWebhook_SubscriptionDeleted(t *testing.T) {
	s := New(Creds{SecretKey: "sk_test", WebhookSecret: "whsec_test"}, Env{})
	payload := `{"id":"evt_3","type":"customer.subscription.deleted","api_version":"2025-08-27.basil","data":{"object":{"id":"sub_5"}}}`
	ts := time.Now().Unix()
	h := http.Header{}
	h.Set("Stripe-Signature", signStripe(t, payload, "whsec_test", ts))
	act, err := s.VerifyWebhook(context.Background(), []byte(payload), h)
	if err != nil {
		t.Fatal(err)
	}
	if act.Type != strategy.ActionSubscriptionCanceled || act.StripeSubscriptionID != "sub_5" {
		t.Fatalf("bad cancel action: %+v", act)
	}
}

func TestStripeVerifyWebhook_UnhandledIsIgnored(t *testing.T) {
	s := New(Creds{SecretKey: "sk_test", WebhookSecret: "whsec_test"}, Env{})
	payload := `{"id":"evt_4","type":"customer.created","api_version":"2025-08-27.basil","data":{"object":{}}}`
	ts := time.Now().Unix()
	h := http.Header{}
	h.Set("Stripe-Signature", signStripe(t, payload, "whsec_test", ts))
	act, err := s.VerifyWebhook(context.Background(), []byte(payload), h)
	if err != nil || act.Type != strategy.ActionIgnored {
		t.Fatalf("want ignored, got %+v %v", act, err)
	}
}

func TestCheckout_NotConfigured(t *testing.T) {
	s := New(Creds{}, Env{})
	_, err := s.CreateCheckout(context.Background(),
		paymentFixture(), planFixture(), optsFixture())
	// Node string (stripe.strategy.ts line 56): authoritative.
	const want = "Stripe is not configured. Set STRIPE_SECRET_KEY in admin Settings → Payment."
	if err == nil || err.Error() != want {
		t.Fatalf("empty key must error with the Node message, got %v", err)
	}
}
