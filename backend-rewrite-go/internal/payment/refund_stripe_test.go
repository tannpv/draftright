package payment

import "testing"

// Compile-time assertion: the real impl satisfies the consumer-side port.
var _ stripeRefunder = (*stripeSDKRefunder)(nil)

// TestNewStripeSDKRefunder is a build-only smoke test. The real impl hits the
// Stripe network, so the refund LOGIC is exercised via a fake refunder in the
// consuming usecase's tests (next task) — never against live Stripe here.
func TestNewStripeSDKRefunder(t *testing.T) {
	r := newStripeSDKRefunder()
	if r == nil {
		t.Fatal("newStripeSDKRefunder() returned nil")
	}
	if r.newClient == nil {
		t.Fatal("newStripeSDKRefunder() left newClient seam nil")
	}
}
