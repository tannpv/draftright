package payment

import (
	"context"
	"testing"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/payment/strategy"
	"github.com/tannpv/draftright-rewrite/internal/subscription"
)

type fakeSubs struct{ sub *subscription.AccountSub }

func (f fakeSubs) ActiveByUser(context.Context, string) (*subscription.AccountSub, error) {
	return f.sub, nil
}

type fakeStrategyCancel struct{ ok bool }

func (f fakeStrategyCancel) CreateCheckout(context.Context, strategy.Payment, strategy.Plan, strategy.Options) (strategy.Result, error) {
	return strategy.Result{}, nil
}
func (f fakeStrategyCancel) CustomerPortalURL(context.Context, strategy.PortalUser) (string, error) {
	return "", nil
}
func (f fakeStrategyCancel) CancelSubscription(context.Context, string) (bool, error) {
	return f.ok, nil
}

func portalSvc(repo CheckoutRepo, subs SubsPort, strategies map[string]strategy.Strategy) *Service {
	return &Service{
		checkoutRepo: repo,
		subs:         subs,
		strategies:   strategies,
		now:          func() time.Time { return time.Unix(1700000000, 0).UTC() },
		genRef:       func() string { return "DR-PRO-TESTREF" },
	}
}

func TestPortal_UserNotFound(t *testing.T) {
	s := portalSvc(&fakeCheckoutRepo{user: nil}, nil, nil)
	_, err := s.CustomerPortalURL(context.Background(), "u1")
	assertDomainErr(t, err, 404, "User not found")
}

func TestPortal_NoActiveSub(t *testing.T) {
	s := portalSvc(&fakeCheckoutRepo{user: &CheckoutUser{ID: "u1"}}, fakeSubs{sub: nil}, nil)
	_, err := s.CustomerPortalURL(context.Background(), "u1")
	assertDomainErr(t, err, 404, "No active subscription to manage")
}

func TestPortal_UnsupportedStore(t *testing.T) {
	s := portalSvc(&fakeCheckoutRepo{user: &CheckoutUser{ID: "u1"}},
		fakeSubs{sub: &subscription.AccountSub{StoreType: "vietqr"}}, nil)
	_, err := s.CustomerPortalURL(context.Background(), "u1")
	assertDomainErr(t, err, 404, "Subscriptions sourced from 'vietqr' have no self-service portal")
}

func TestPortal_StripeURLEmpty(t *testing.T) {
	s := portalSvc(&fakeCheckoutRepo{user: &CheckoutUser{ID: "u1"}},
		fakeSubs{sub: &subscription.AccountSub{StoreType: "stripe"}},
		map[string]strategy.Strategy{"stripe": fakeStrategy{}})
	_, err := s.CustomerPortalURL(context.Background(), "u1")
	assertDomainErr(t, err, 404, "Customer portal is not available for this subscription")
}

func TestCancel_NoProviderRef(t *testing.T) {
	s := portalSvc(&fakeCheckoutRepo{user: &CheckoutUser{ID: "u1"}},
		fakeSubs{sub: &subscription.AccountSub{StoreType: "stripe", StoreTransactionID: ""}}, nil)
	_, err := s.CancelActiveSubscription(context.Background(), "u1")
	assertDomainErr(t, err, 404, "Subscription has no provider reference to cancel")
}

func TestCancel_ProviderDeclined(t *testing.T) {
	exp := time.Now()
	s := portalSvc(&fakeCheckoutRepo{user: &CheckoutUser{ID: "u1"}},
		fakeSubs{sub: &subscription.AccountSub{StoreType: "stripe", StoreTransactionID: "sub_1", ExpiresAt: &exp}},
		map[string]strategy.Strategy{"stripe": fakeStrategy{}})
	_, err := s.CancelActiveSubscription(context.Background(), "u1")
	assertDomainErr(t, err, 404, "Provider declined to cancel the subscription")
}

func TestCancel_Success(t *testing.T) {
	exp := time.Now()
	s := portalSvc(&fakeCheckoutRepo{user: &CheckoutUser{ID: "u1"}},
		fakeSubs{sub: &subscription.AccountSub{StoreType: "stripe", StoreTransactionID: "sub_1", ExpiresAt: &exp}},
		map[string]strategy.Strategy{"stripe": fakeStrategyCancel{ok: true}})
	out, err := s.CancelActiveSubscription(context.Background(), "u1")
	if err != nil {
		t.Fatal(err)
	}
	if !out.Cancelled || out.ExpiresAt == nil {
		t.Fatalf("expected cancelled+expires_at, got %+v", out)
	}
}
