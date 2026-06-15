package stripe

import (
	"context"
	"testing"

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
