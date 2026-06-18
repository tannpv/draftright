// Package adminstats_test verifies the Stats usecase in isolation using fakes.
// Parity authority: src/admin/admin.controller.ts getStats() which drives
// Promise.all([usersService.count(), subscriptionsService.countActive(),
// usageService.countToday(), usageService.countThisMonth()]) and returns
// { total_users, active_subscriptions, rewrites_today, rewrites_this_month }.
package adminstats_test

import (
	"context"
	"testing"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/adminstats"
	"github.com/tannpv/draftright-rewrite/internal/plans"
	"github.com/tannpv/draftright-rewrite/internal/subscription"
)

// ─── fakes ───────────────────────────────────────────────────────────────────

type fakeUserCounter struct{ n int }

func (f *fakeUserCounter) Count(_ context.Context) (int, error) { return f.n, nil }

// fakeSub satisfies subStatsPort. Only CountActive is exercised by Stats();
// GetPlansBreakdown and GetMonthlyStatsAt are for Task 10 (analytics).
type fakeSub struct {
	active         int
	plansBreakdown []subscription.PlanBreakdown
	monthlyStats   []subscription.MonthStat
	// lastNow captures the `now` arg passed to GetMonthlyStatsAt for assertion.
	lastNow time.Time
}

func (f *fakeSub) CountActive(_ context.Context) (int, error) { return f.active, nil }
func (f *fakeSub) GetPlansBreakdown(_ context.Context) ([]subscription.PlanBreakdown, error) {
	return f.plansBreakdown, nil
}
func (f *fakeSub) GetMonthlyStatsAt(_ context.Context, now time.Time, _ int) ([]subscription.MonthStat, error) {
	f.lastNow = now
	return f.monthlyStats, nil
}

// fakeUsage satisfies usageGlobal. Records the `now` arg to EACH call
// separately so a test can assert both forwarded clock values independently
// (a single shared field would mask one call receiving the wrong value).
type fakeUsage struct {
	today        int
	month        int
	todayErr     error
	monthErr     error
	lastNowToday time.Time
	lastNowMonth time.Time
}

func (f *fakeUsage) CountTodayAllAt(_ context.Context, now time.Time) (int, error) {
	f.lastNowToday = now
	return f.today, f.todayErr
}
func (f *fakeUsage) CountThisMonthAllAt(_ context.Context, now time.Time) (int, error) {
	f.lastNowMonth = now
	return f.month, f.monthErr
}

// fakePlanLister satisfies planLister. Not exercised by Stats() but required
// to construct the Service (Task 10 will use it).
type fakePlanLister struct{ plans []plans.PlanEntity }

func (f *fakePlanLister) ListAll(_ context.Context) ([]plans.PlanEntity, error) {
	return f.plans, nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func newService(users *fakeUserCounter, subs *fakeSub, usage *fakeUsage, pl *fakePlanLister, now func() time.Time) *adminstats.Service {
	return adminstats.NewService(users, subs, usage, pl, now)
}

// ─── tests ───────────────────────────────────────────────────────────────────

// TestStats_AllCounts verifies that Stats returns the 4 counts from the
// correct ports and passes the injected `now()` to both usage calls.
func TestStats_AllCounts(t *testing.T) {
	fixedNow := time.Date(2026, 6, 18, 9, 30, 0, 0, time.UTC)

	users := &fakeUserCounter{n: 42}
	subs := &fakeSub{active: 7}
	usage := &fakeUsage{today: 15, month: 300}
	pl := &fakePlanLister{}

	svc := newService(users, subs, usage, pl, func() time.Time { return fixedNow })

	result, err := svc.Stats(context.Background())
	if err != nil {
		t.Fatalf("Stats() error: %v", err)
	}

	if result.TotalUsers != 42 {
		t.Errorf("TotalUsers = %d, want 42", result.TotalUsers)
	}
	if result.ActiveSubscriptions != 7 {
		t.Errorf("ActiveSubscriptions = %d, want 7", result.ActiveSubscriptions)
	}
	if result.RewritesToday != 15 {
		t.Errorf("RewritesToday = %d, want 15", result.RewritesToday)
	}
	if result.RewritesThisMonth != 300 {
		t.Errorf("RewritesThisMonth = %d, want 300", result.RewritesThisMonth)
	}

	// The injected clock value must flow through to BOTH usage calls.
	if !usage.lastNowToday.Equal(fixedNow) {
		t.Errorf("CountTodayAllAt now = %v, want %v (injected clock not forwarded)", usage.lastNowToday, fixedNow)
	}
	if !usage.lastNowMonth.Equal(fixedNow) {
		t.Errorf("CountThisMonthAllAt now = %v, want %v (injected clock not forwarded)", usage.lastNowMonth, fixedNow)
	}
}

// fakeUserCounterErr / fakeSubErr let a single port fail so we can assert
// Stats propagates the error and returns a zero-value result.
type fakeUserCounterErr struct{ err error }

func (f *fakeUserCounterErr) Count(_ context.Context) (int, error) { return 0, f.err }

// TestStats_ErrorPropagation verifies that an error from any port aborts Stats
// with a zero-value result and the underlying error (no partial result).
func TestStats_ErrorPropagation(t *testing.T) {
	now := func() time.Time { return time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC) }

	t.Run("user count error", func(t *testing.T) {
		boom := errTest("user count down")
		svc := adminstats.NewService(&fakeUserCounterErr{err: boom}, &fakeSub{}, &fakeUsage{}, &fakePlanLister{}, now)
		result, err := svc.Stats(context.Background())
		if err == nil {
			t.Fatal("Stats() error = nil, want propagated error")
		}
		if (result != adminstats.StatsResult{}) {
			t.Errorf("result = %+v, want zero value on error", result)
		}
	})

	t.Run("usage today error", func(t *testing.T) {
		boom := errTest("usage today down")
		svc := adminstats.NewService(&fakeUserCounter{n: 1}, &fakeSub{active: 1}, &fakeUsage{todayErr: boom}, &fakePlanLister{}, now)
		result, err := svc.Stats(context.Background())
		if err == nil {
			t.Fatal("Stats() error = nil, want propagated error")
		}
		if (result != adminstats.StatsResult{}) {
			t.Errorf("result = %+v, want zero value on error", result)
		}
	})
}

// errTest is a tiny sentinel error type so the test needs no extra import.
type errTest string

func (e errTest) Error() string { return string(e) }

// ─── Analytics tests ──────────────────────────────────────────────────────────

// TestAnalytics_MRR is the canonical mixed-billing-period case from the plan:
//
//	Pro monthly  2 active × 999  cents         → 1998
//	ProYear yearly 1 active × round(9990/12)   → 833   (Math.round parity)
//	mrr = 2831, total_revenue = sum of monthly stats revenue
//
// Node authority: admin.controller.ts getAnalytics() lines 592-618.
func TestAnalytics_MRR(t *testing.T) {
	fixedNow := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)

	// NOTE: the breakdown rows' PriceCents is IGNORED by the MRR algorithm —
	// price + billing_period are read from the matched plans.PlanEntity, not
	// from the breakdown row (Node matches plan by name then reads plan.price).
	// Values here are only realistic filler, not the source of the MRR math.
	subs := &fakeSub{
		plansBreakdown: []subscription.PlanBreakdown{
			{PlanName: "Pro", ActiveCount: 2, PriceCents: 999},
			{PlanName: "ProYear", ActiveCount: 1, PriceCents: 9990},
		},
		monthlyStats: []subscription.MonthStat{
			{Month: "2026-06", NewSubscriptions: 3, RevenueCents: 5000, Churned: 0},
		},
	}
	pl := &fakePlanLister{plans: []plans.PlanEntity{
		{Name: "Pro", PriceCents: 999, BillingPeriod: "monthly"},
		{Name: "ProYear", PriceCents: 9990, BillingPeriod: "yearly"},
	}}

	svc := newService(&fakeUserCounter{}, subs, &fakeUsage{}, pl, func() time.Time { return fixedNow })
	result, err := svc.Analytics(context.Background())
	if err != nil {
		t.Fatalf("Analytics() error: %v", err)
	}

	// Pro monthly: 2 * 999 = 1998
	// ProYear yearly: 1 * round(9990/12) = 1 * round(832.5) = 1 * 833 = 833
	// mrr = 1998 + 833 = 2831
	if result.MRR != 2831 {
		t.Errorf("MRR = %d, want 2831", result.MRR)
	}

	// total_revenue = sum of monthly_stats.revenue_cents = 5000
	if result.TotalRevenue != 5000 {
		t.Errorf("TotalRevenue = %d, want 5000", result.TotalRevenue)
	}
}

// TestAnalytics_SkipsZeroPrice confirms that plans with price_cents == 0 are
// excluded from MRR (parity: Node `if plan && plan.price_cents > 0`).
func TestAnalytics_SkipsZeroPrice(t *testing.T) {
	subs := &fakeSub{
		plansBreakdown: []subscription.PlanBreakdown{
			{PlanName: "Free", ActiveCount: 10, PriceCents: 0},
			{PlanName: "Pro", ActiveCount: 1, PriceCents: 999},
		},
		monthlyStats: []subscription.MonthStat{},
	}
	pl := &fakePlanLister{plans: []plans.PlanEntity{
		{Name: "Free", PriceCents: 0, BillingPeriod: "monthly"},
		{Name: "Pro", PriceCents: 999, BillingPeriod: "monthly"},
	}}

	svc := newService(&fakeUserCounter{}, subs, &fakeUsage{}, pl, time.Now)
	result, err := svc.Analytics(context.Background())
	if err != nil {
		t.Fatalf("Analytics() error: %v", err)
	}

	if result.MRR != 999 {
		t.Errorf("MRR = %d, want 999 (free plan must be skipped)", result.MRR)
	}
}

// TestAnalytics_UnknownPlanSkipped confirms that a breakdown row whose
// plan_name doesn't match any plan in ListAll is silently skipped (no crash).
func TestAnalytics_UnknownPlanSkipped(t *testing.T) {
	subs := &fakeSub{
		plansBreakdown: []subscription.PlanBreakdown{
			{PlanName: "Orphan", ActiveCount: 5, PriceCents: 500},
			{PlanName: "Pro", ActiveCount: 1, PriceCents: 999},
		},
		monthlyStats: []subscription.MonthStat{},
	}
	pl := &fakePlanLister{plans: []plans.PlanEntity{
		{Name: "Pro", PriceCents: 999, BillingPeriod: "monthly"},
	}}

	svc := newService(&fakeUserCounter{}, subs, &fakeUsage{}, pl, time.Now)
	result, err := svc.Analytics(context.Background())
	if err != nil {
		t.Fatalf("Analytics() error: %v", err)
	}

	// Orphan has no matching plan → skipped; only Pro contributes.
	if result.MRR != 999 {
		t.Errorf("MRR = %d, want 999 (orphan breakdown must be skipped)", result.MRR)
	}
}

// TestAnalytics_ForwardsNowToMonthlyStats confirms that the injected clock
// value is forwarded to GetMonthlyStatsAt (not time.Now() called directly).
func TestAnalytics_ForwardsNowToMonthlyStats(t *testing.T) {
	fixedNow := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	subs := &fakeSub{monthlyStats: []subscription.MonthStat{}}
	pl := &fakePlanLister{}

	svc := newService(&fakeUserCounter{}, subs, &fakeUsage{}, pl, func() time.Time { return fixedNow })
	_, err := svc.Analytics(context.Background())
	if err != nil {
		t.Fatalf("Analytics() error: %v", err)
	}

	if !subs.lastNow.Equal(fixedNow) {
		t.Errorf("GetMonthlyStatsAt now = %v, want %v (injected clock not forwarded)", subs.lastNow, fixedNow)
	}
}

// TestAnalytics_TotalRevenueMultipleMonths verifies TotalRevenue sums all
// monthly_stats entries (not just one).
func TestAnalytics_TotalRevenueMultipleMonths(t *testing.T) {
	subs := &fakeSub{
		plansBreakdown: []subscription.PlanBreakdown{},
		monthlyStats: []subscription.MonthStat{
			{Month: "2026-04", RevenueCents: 1000},
			{Month: "2026-05", RevenueCents: 2000},
			{Month: "2026-06", RevenueCents: 3000},
		},
	}
	pl := &fakePlanLister{}

	svc := newService(&fakeUserCounter{}, subs, &fakeUsage{}, pl, time.Now)
	result, err := svc.Analytics(context.Background())
	if err != nil {
		t.Fatalf("Analytics() error: %v", err)
	}

	if result.TotalRevenue != 6000 {
		t.Errorf("TotalRevenue = %d, want 6000", result.TotalRevenue)
	}
}

// errSub / errPlanLister fail a single Analytics port so we can assert the
// error is propagated with a zero-value result (symmetry with the Stats
// error-path tests above).
type errSub struct {
	breakdownErr error
	monthlyErr   error
}

func (f *errSub) CountActive(_ context.Context) (int, error) { return 0, nil }
func (f *errSub) GetPlansBreakdown(_ context.Context) ([]subscription.PlanBreakdown, error) {
	return nil, f.breakdownErr
}
func (f *errSub) GetMonthlyStatsAt(_ context.Context, _ time.Time, _ int) ([]subscription.MonthStat, error) {
	if f.monthlyErr != nil {
		return nil, f.monthlyErr
	}
	return []subscription.MonthStat{}, nil
}

type errPlanLister struct{ err error }

func (f *errPlanLister) ListAll(_ context.Context) ([]plans.PlanEntity, error) { return nil, f.err }

// TestAnalytics_ErrorPropagation verifies that an error from any Analytics port
// aborts with a zero-value result and the underlying error.
func TestAnalytics_ErrorPropagation(t *testing.T) {
	now := func() time.Time { return time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC) }

	t.Run("plans breakdown error", func(t *testing.T) {
		svc := adminstats.NewService(&fakeUserCounter{}, &errSub{breakdownErr: errTest("breakdown down")}, &fakeUsage{}, &fakePlanLister{}, now)
		result, err := svc.Analytics(context.Background())
		if err == nil {
			t.Fatal("Analytics() error = nil, want propagated error")
		}
		if result.MRR != 0 || result.TotalRevenue != 0 || result.PlansBreakdown != nil || result.MonthlyStats != nil {
			t.Errorf("result = %+v, want zero value on error", result)
		}
	})

	t.Run("monthly stats error", func(t *testing.T) {
		svc := adminstats.NewService(&fakeUserCounter{}, &errSub{monthlyErr: errTest("monthly down")}, &fakeUsage{}, &fakePlanLister{}, now)
		result, err := svc.Analytics(context.Background())
		if err == nil {
			t.Fatal("Analytics() error = nil, want propagated error")
		}
		if result.MRR != 0 || result.TotalRevenue != 0 || result.PlansBreakdown != nil || result.MonthlyStats != nil {
			t.Errorf("result = %+v, want zero value on error", result)
		}
	})

	t.Run("plan list error", func(t *testing.T) {
		svc := adminstats.NewService(&fakeUserCounter{}, &fakeSub{}, &fakeUsage{}, &errPlanLister{err: errTest("list down")}, now)
		result, err := svc.Analytics(context.Background())
		if err == nil {
			t.Fatal("Analytics() error = nil, want propagated error")
		}
		if result.MRR != 0 || result.TotalRevenue != 0 || result.PlansBreakdown != nil || result.MonthlyStats != nil {
			t.Errorf("result = %+v, want zero value on error", result)
		}
	})
}

// TestStats_ZeroValues confirms zero values marshal cleanly (no omitempty).
func TestStats_ZeroValues(t *testing.T) {
	svc := newService(
		&fakeUserCounter{n: 0},
		&fakeSub{active: 0},
		&fakeUsage{today: 0, month: 0},
		&fakePlanLister{},
		time.Now,
	)
	result, err := svc.Stats(context.Background())
	if err != nil {
		t.Fatalf("Stats() error: %v", err)
	}
	if result.TotalUsers != 0 || result.ActiveSubscriptions != 0 ||
		result.RewritesToday != 0 || result.RewritesThisMonth != 0 {
		t.Errorf("all zero expected, got %+v", result)
	}
}
