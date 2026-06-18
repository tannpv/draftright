package adminstats

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tannpv/draftright-rewrite/internal/subscription"
)

// fakeAdminStatsService is a test double that satisfies the handler's
// adminStatsService interface. Fields drive return values.
type fakeAdminStatsService struct {
	statsResult     StatsResult
	statsErr        error
	analyticsResult AnalyticsResult
	analyticsErr    error
}

func (f *fakeAdminStatsService) Stats(ctx context.Context) (StatsResult, error) {
	return f.statsResult, f.statsErr
}

func (f *fakeAdminStatsService) Analytics(ctx context.Context) (AnalyticsResult, error) {
	return f.analyticsResult, f.analyticsErr
}

// assertKeyOrder fails unless the JSON keys appear in raw in the given order.
// Mirrors the helper in rewritelog/handler_test.go.
func assertKeyOrder(t *testing.T, raw string, keys ...string) {
	t.Helper()
	prev := -1
	for _, k := range keys {
		idx := strings.Index(raw, `"`+k+`"`)
		if idx < 0 {
			t.Fatalf("key %q missing from body: %s", k, raw)
		}
		if idx <= prev {
			t.Fatalf("key %q out of order in body: %s", k, raw)
		}
		prev = idx
	}
}

// newAdminStatsH builds the Handler directly from the fake (bypasses NewHandler
// which requires *Service — the fake satisfies the interface directly).
func newAdminStatsH(svc *fakeAdminStatsService) *Handler {
	return &Handler{svc: svc}
}

// ---------------------------------------------------------------------------
// Stats — GET /admin/stats
// ---------------------------------------------------------------------------

// TestAdminStats_Shape: 200 { total_users, active_subscriptions, rewrites_today,
// rewrites_this_month } in that key order.
func TestAdminStats_Shape(t *testing.T) {
	svc := &fakeAdminStatsService{statsResult: StatsResult{
		TotalUsers: 42, ActiveSubscriptions: 7, RewritesToday: 5, RewritesThisMonth: 120,
	}}
	h := newAdminStatsH(svc)

	rec := httptest.NewRecorder()
	h.GetStats(rec, httptest.NewRequest(http.MethodGet, "/admin/stats", nil))

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	raw := rec.Body.String()
	assertKeyOrder(t, raw, "total_users", "active_subscriptions", "rewrites_today", "rewrites_this_month")

	if !strings.Contains(raw, `"total_users":42`) {
		t.Fatalf("total_users not 42 in body: %s", raw)
	}
	if !strings.Contains(raw, `"active_subscriptions":7`) {
		t.Fatalf("active_subscriptions not 7 in body: %s", raw)
	}
	if !strings.Contains(raw, `"rewrites_today":5`) {
		t.Fatalf("rewrites_today not 5 in body: %s", raw)
	}
	if !strings.Contains(raw, `"rewrites_this_month":120`) {
		t.Fatalf("rewrites_this_month not 120 in body: %s", raw)
	}
}

// TestAdminStats_ServiceError: service error → 500 { error, code, request_id }.
func TestAdminStats_ServiceError(t *testing.T) {
	svc := &fakeAdminStatsService{statsErr: errors.New("db down")}
	h := newAdminStatsH(svc)

	rec := httptest.NewRecorder()
	h.GetStats(rec, httptest.NewRequest(http.MethodGet, "/admin/stats", nil))

	if rec.Code != 500 {
		t.Fatalf("status = %d, want 500; body=%s", rec.Code, rec.Body.String())
	}
	raw := rec.Body.String()
	if !strings.Contains(raw, `"code":"internal"`) {
		t.Fatalf("expected code=internal in body: %s", raw)
	}
}

// ---------------------------------------------------------------------------
// Analytics — GET /admin/analytics
// ---------------------------------------------------------------------------

// TestAdminAnalytics_Shape: 200 { mrr, total_revenue, plans_breakdown,
// monthly_stats } in that key order; populated slices.
func TestAdminAnalytics_Shape(t *testing.T) {
	svc := &fakeAdminStatsService{analyticsResult: AnalyticsResult{
		MRR:          9900,
		TotalRevenue: 29700,
		PlansBreakdown: []subscription.PlanBreakdown{
			{PlanName: "Pro Monthly", ActiveCount: 3, PriceCents: 3300},
		},
		MonthlyStats: []subscription.MonthStat{
			{Month: "2026-01", NewSubscriptions: 2, RevenueCents: 6600, Churned: 0},
		},
	}}
	h := newAdminStatsH(svc)

	rec := httptest.NewRecorder()
	h.GetAnalytics(rec, httptest.NewRequest(http.MethodGet, "/admin/analytics", nil))

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	raw := rec.Body.String()
	assertKeyOrder(t, raw, "mrr", "total_revenue", "plans_breakdown", "monthly_stats")

	if !strings.Contains(raw, `"mrr":9900`) {
		t.Fatalf("mrr not 9900 in body: %s", raw)
	}
	if !strings.Contains(raw, `"total_revenue":29700`) {
		t.Fatalf("total_revenue not 29700 in body: %s", raw)
	}
	if !strings.Contains(raw, `"plan_name":"Pro Monthly"`) {
		t.Fatalf("plans_breakdown not populated in body: %s", raw)
	}
	if !strings.Contains(raw, `"month":"2026-01"`) {
		t.Fatalf("monthly_stats not populated in body: %s", raw)
	}
}

// TestAdminAnalytics_EmptySlicesAreArrayNotNull: when the use case returns
// non-nil empty slices (use case already guarantees this — its nil-guard runs
// before returning), the handler emits [] not null. This confirms the use case
// guarantee holds through the handler.
func TestAdminAnalytics_EmptySlicesAreArrayNotNull(t *testing.T) {
	svc := &fakeAdminStatsService{analyticsResult: AnalyticsResult{
		MRR:            0,
		TotalRevenue:   0,
		PlansBreakdown: []subscription.PlanBreakdown{},
		MonthlyStats:   []subscription.MonthStat{},
	}}
	h := newAdminStatsH(svc)

	rec := httptest.NewRecorder()
	h.GetAnalytics(rec, httptest.NewRequest(http.MethodGet, "/admin/analytics", nil))

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	raw := rec.Body.String()
	if !strings.Contains(raw, `"plans_breakdown":[]`) {
		t.Fatalf("expected plans_breakdown:[] in body: %s", raw)
	}
	if !strings.Contains(raw, `"monthly_stats":[]`) {
		t.Fatalf("expected monthly_stats:[] in body: %s", raw)
	}
}

// TestAdminAnalytics_ServiceError: service error → 500 { error, code, request_id }.
func TestAdminAnalytics_ServiceError(t *testing.T) {
	svc := &fakeAdminStatsService{analyticsErr: errors.New("db down")}
	h := newAdminStatsH(svc)

	rec := httptest.NewRecorder()
	h.GetAnalytics(rec, httptest.NewRequest(http.MethodGet, "/admin/analytics", nil))

	if rec.Code != 500 {
		t.Fatalf("status = %d, want 500; body=%s", rec.Code, rec.Body.String())
	}
	raw := rec.Body.String()
	if !strings.Contains(raw, `"code":"internal"`) {
		t.Fatalf("expected code=internal in body: %s", raw)
	}
}
