package subscription_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/shared"
	"github.com/tannpv/draftright-rewrite/internal/subscription"
)

// withClaims stamps a user id into the request context the way RequireAuth
// would, so the handler can read it via shared.ClaimsFromContext.
func withClaims(r *http.Request, sub string) *http.Request {
	ctx := shared.ContextWithClaims(r.Context(), &auth.Claims{Sub: sub})
	return r.WithContext(ctx)
}

func TestHandler_Get_200(t *testing.T) {
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	exp := now.Add(2 * 24 * time.Hour)
	stub := &stubReader{
		sub: &subscription.SubView{
			Status:        "active",
			ExpiresAt:     &exp,
			PlanName:      "Pro",
			DailyLimit:    1000,
			BillingPeriod: "monthly",
		},
		usageToday: 7,
	}
	h := subscription.NewHandler(newSubSvc(t, stub))

	req := withClaims(httptest.NewRequest(http.MethodGet, "/subscription", nil), "u1")
	rec := httptest.NewRecorder()
	h.Get(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "\"nudge\"") {
		t.Fatalf("body missing nudge key: %s", rec.Body.String())
	}
}

func TestHandler_VerifyReceipt_201(t *testing.T) {
	exp := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	stub := &stubReader{
		sub: &subscription.SubView{
			Status:        "active",
			ExpiresAt:     &exp,
			PlanName:      "Pro",
			DailyLimit:    1000,
			BillingPeriod: "monthly",
		},
	}
	h := subscription.NewHandler(newSubSvc(t, stub))

	body := strings.NewReader(`{"receipt":"anything"}`)
	req := withClaims(httptest.NewRequest(http.MethodPost, "/subscription/verify-receipt", body), "u1")
	rec := httptest.NewRecorder()
	h.VerifyReceipt(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "\"subscription\"") {
		t.Fatalf("body missing subscription key: %s", rec.Body.String())
	}
}
