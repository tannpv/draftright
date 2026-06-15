package strategy

import "testing"

func TestResolveCredential(t *testing.T) {
	cases := []struct{ db, env, want string }{
		{"from-db", "from-env", "from-db"}, // DB priority
		{"", "from-env", "from-env"},       // empty DB → env
		{"", "", ""},                       // both empty
	}
	for _, c := range cases {
		if got := ResolveCredential(c.db, c.env); got != c.want {
			t.Fatalf("ResolveCredential(%q,%q)=%q want %q", c.db, c.env, got, c.want)
		}
	}
}

func TestTimingSafeStrEqual(t *testing.T) {
	if !TimingSafeStrEqual("secret", "secret") {
		t.Fatal("equal strings must compare true")
	}
	if TimingSafeStrEqual("secret", "secre") {
		t.Fatal("length mismatch must be false")
	}
	if TimingSafeStrEqual("secret", "secreT") {
		t.Fatal("differing strings must be false")
	}
}

func TestWebhookActionConstants(t *testing.T) {
	// Type strings must match Node's WebhookAction union tags byte-for-byte —
	// the Service switches on these.
	cases := map[string]string{
		ActionPaymentCompleted:       "payment_completed",
		ActionPaymentFailed:          "payment_failed",
		ActionSubscriptionRenewed:    "subscription_renewed",
		ActionSubscriptionCanceled:   "subscription_canceled",
		ActionDisputeCreated:         "dispute_created",
		ActionLSPaymentSuccess:       "lemonsqueezy_payment_success",
		ActionLSPaymentFailed:        "lemonsqueezy_payment_failed",
		ActionLSSubscriptionCanceled: "lemonsqueezy_subscription_canceled",
		ActionLSSubscriptionExpired:  "lemonsqueezy_subscription_expired",
		ActionIgnored:                "ignored",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("constant = %q, want %q", got, want)
		}
	}
}
