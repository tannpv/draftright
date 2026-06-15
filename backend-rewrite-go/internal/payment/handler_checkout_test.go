package payment

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/payment/strategy"
	auth "github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/shared"
	"github.com/tannpv/draftright-rewrite/internal/subscription"
)

// withClaims stamps a user id onto the request context exactly as the
// RequireAuth middleware does, so the handler's ClaimsFromContext succeeds.
func withClaims(req *http.Request, sub string) *http.Request {
	return req.WithContext(shared.ContextWithClaims(req.Context(), &auth.Claims{Sub: sub}))
}

func TestCheckoutHandler_ValidationError(t *testing.T) {
	h := NewHandler(&Service{})
	rec := httptest.NewRecorder()
	req := withClaims(httptest.NewRequest(http.MethodPost, "/payment/checkout",
		strings.NewReader(`{"method":"banana"}`)), "u1")
	h.Checkout(rec, req)
	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var env map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env["code"] != "invalid-input" {
		t.Fatalf("code = %v, want invalid-input", env["code"])
	}
	want := "plan_id must be a string. method must be one of the following values: stripe, paypal, momo, vietqr, bank_transfer, lemonsqueezy, apple_pay, google_pay"
	if env["error"] != want {
		t.Fatalf("error =\n%v\nwant\n%v", env["error"], want)
	}
}

func TestCheckoutHandler_Success201(t *testing.T) {
	repo := &fakeCheckoutRepo{
		plan:    &CheckoutPlan{Name: "Pro", PriceCents: 124000, Currency: "VND"},
		user:    &CheckoutUser{ID: "u1", Email: "u@x.com"},
		created: &CreatedPayment{ID: "pay1"},
	}
	strat := fakeStrategy{res: strategy.Result{QRData: "https://img.vietqr.io/x"}}
	svc := svcWith(repo, strat, []string{"vietqr"})
	h := NewHandler(svc)
	rec := httptest.NewRecorder()
	req := withClaims(httptest.NewRequest(http.MethodPost, "/payment/checkout",
		strings.NewReader(`{"plan_id":"p1","method":"vietqr"}`)), "u1")
	h.Checkout(rec, req)
	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201 (body=%s)", rec.Code, rec.Body.String())
	}
}

func TestPortalHandler_200(t *testing.T) {
	svc := portalSvc(&fakeCheckoutRepo{user: &CheckoutUser{ID: "u1", StripeCustomerID: "cus_1"}},
		fakeSubs{sub: &subscription.AccountSub{StoreType: "stripe"}},
		map[string]strategy.Strategy{"stripe": fakeStrategyPortal{url: "https://portal.stripe.com/session/abc"}})
	h := NewHandler(svc)
	rec := httptest.NewRecorder()
	req := withClaims(httptest.NewRequest(http.MethodGet, "/payment/portal", nil), "u1")
	h.Portal(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var env map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env["url"] != "https://portal.stripe.com/session/abc" {
		t.Fatalf("url = %v", env["url"])
	}
}

func TestCancelHandler_200(t *testing.T) {
	exp := time.Now()
	svc := portalSvc(&fakeCheckoutRepo{user: &CheckoutUser{ID: "u1"}},
		fakeSubs{sub: &subscription.AccountSub{StoreType: "stripe", StoreTransactionID: "sub_1", ExpiresAt: &exp}},
		map[string]strategy.Strategy{"stripe": fakeStrategyCancel{ok: true}})
	h := NewHandler(svc)
	rec := httptest.NewRecorder()
	req := withClaims(httptest.NewRequest(http.MethodDelete, "/payment/subscription", nil), "u1")
	h.CancelSubscription(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var env map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env["cancelled"] != true {
		t.Fatalf("cancelled = %v", env["cancelled"])
	}
}
