package strategy

import "testing"

func TestAppleWebhookActionFields(t *testing.T) {
	a := WebhookAction{
		Type:                       ActionAppleRenewed,
		AppleTransactionID:         "t1",
		AppleOriginalTransactionID: "o1",
		AppleProductID:             "p1",
		CurrentPeriodEnd:           123, // reused existing field, not a new one
	}
	if a.Type != ActionAppleRenewed || a.AppleOriginalTransactionID != "o1" || a.CurrentPeriodEnd != 123 {
		t.Fatal("apple webhook action fields not wired")
	}
	if ErrNotCheckoutMethod == nil {
		t.Fatal("ErrNotCheckoutMethod must be defined")
	}
}
