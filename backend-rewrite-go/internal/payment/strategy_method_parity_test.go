package payment

import (
	"testing"

	"github.com/tannpv/draftright-rewrite/internal/payment/strategy"
)

// TestStrategyMethodConstsMatchPayment is the Rule #1 can't-merge guard (#204
// finding #5): strategy.Method* are duplicated from payment.Method* because the
// low-level strategy package can't import payment back (import cycle). Assert the
// two stay in sync — a rename on either side fails here instead of silently
// breaking method dispatch in the vietqr/stripe strategies.
func TestStrategyMethodConstsMatchPayment(t *testing.T) {
	cases := []struct {
		strat     string
		canonical PaymentMethod
	}{
		{strategy.MethodBankTransfer, MethodBankTransfer},
		{strategy.MethodApplePay, MethodApplePay},
		{strategy.MethodGooglePay, MethodGooglePay},
		{strategy.MethodAppleIAP, MethodAppleIAP},
	}
	for _, c := range cases {
		if c.strat != string(c.canonical) {
			t.Errorf("strategy method %q != payment enum %q", c.strat, c.canonical)
		}
	}
}
