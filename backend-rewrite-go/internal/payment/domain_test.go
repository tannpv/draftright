package payment

import "testing"

func TestStoreTypeForMethod(t *testing.T) {
	cases := map[PaymentMethod]StoreType{
		MethodStripe:         StoreStripe,
		MethodLemonSqueezy:   StoreLemonSqueezy,
		MethodVietQR:         StoreVietQR,
		MethodBankTransfer:   StoreBankTransfer,
		MethodPayPal:         StorePayPal,
		MethodApplePay:       StoreStripe,
		MethodGooglePay:      StoreStripe,
		MethodMoMo:           StoreAdminGranted,
		PaymentMethod("???"): StoreAdminGranted,
	}
	for in, want := range cases {
		if got := StoreTypeForMethod(in); got != want {
			t.Errorf("StoreTypeForMethod(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEnumWireValues(t *testing.T) {
	pairs := []struct{ got, want string }{
		{string(MethodStripe), "stripe"},
		{string(MethodPayPal), "paypal"},
		{string(MethodMoMo), "momo"},
		{string(MethodVietQR), "vietqr"},
		{string(MethodBankTransfer), "bank_transfer"},
		{string(MethodLemonSqueezy), "lemonsqueezy"},
		{string(MethodApplePay), "apple_pay"},
		{string(MethodGooglePay), "google_pay"},
		{string(StatusPending), "pending"},
		{string(StatusCompleted), "completed"},
		{string(StatusFailed), "failed"},
		{string(StatusExpired), "expired"},
		{string(StatusRefunded), "refunded"},
		{string(StoreGooglePlay), "google_play"},
		{string(StoreAppleIAP), "apple_iap"},
		{string(StoreAdminGranted), "admin_granted"},
		{string(StoreLemonSqueezy), "lemonsqueezy"},
		{string(StoreStripe), "stripe"},
		{string(StoreVietQR), "vietqr"},
		{string(StoreBankTransfer), "bank_transfer"},
		{string(StorePayPal), "paypal"},
	}
	for _, p := range pairs {
		if p.got != p.want {
			t.Errorf("wire value = %q, want %q", p.got, p.want)
		}
	}
}
