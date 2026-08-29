package payment

import (
	"context"
	"testing"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/payment/strategy/applestore"
)

// newTestServiceForRedeem wires a Service for RedeemAppleTransaction tests:
// the fake WebhookRepo/VariantResolver pair from newTestServiceWithAppleProducts
// (so resolvePlanIDFromAppleProduct resolves the monthly/yearly product ids),
// plus a stub appleVerify seam. The stub always returns the monthly product,
// a fixed original transaction id ("o1"), and a future expiry — the seam
// under test doesn't need to parse the signedTransaction argument, only hand
// back a known JWSPayload shape.
func newTestServiceForRedeem(t *testing.T, subs SubsWriter, emailer WebhookEmailer, monthly, yearly string) *Service {
	t.Helper()
	repo := &fakeWebhookRepo{planID: "pl_apple_stub"}
	svc := &Service{}
	svc.WithWebhook(repo, subs, emailer, fakeVariants{appleMonthly: monthly, appleYearly: yearly})
	svc.WithAppleVerify(func(signedTransaction string) (applestore.JWSPayload, error) {
		return applestore.JWSPayload{
			ProductID:             monthly,
			OriginalTransactionID: "o1",
			TransactionID:         "t1",
			ExpiresDate:           time.Now().Add(30 * 24 * time.Hour).UnixMilli(),
		}, nil
	})
	return svc
}

func TestRedeemAppleTransaction_GrantsOnceAndStamps(t *testing.T) {
	f := &fakeSubsWriter{}
	em := &noopEmailer{}
	s := newTestServiceForRedeem(t, f, em, "com.draftright.pro.monthly", "com.draftright.pro.yearly")
	if err := s.RedeemAppleTransaction(context.Background(), "user-1", "stub-jws-monthly"); err != nil {
		t.Fatal(err)
	}
	if !f.granted || f.grantStore != string(StoreAppleIAP) {
		t.Fatalf("grant store type = %q, want apple_iap (granted=%v)", f.grantStore, f.granted)
	}
	if f.stampByUserRef != "o1" {
		t.Fatalf("store ref = %q, want original transaction id o1", f.stampByUserRef)
	}
	if em.activatedCalls != 1 {
		t.Fatalf("first IAP purchase should send one activation email, got %d", em.activatedCalls)
	}
}
