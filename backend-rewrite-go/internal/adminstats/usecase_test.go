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

// fakeUsage satisfies usageGlobal. Records the `now` arg to each call so
// tests can assert the injected clock value was forwarded.
type fakeUsage struct {
	today   int
	month   int
	lastNow time.Time // last `now` forwarded to either call
}

func (f *fakeUsage) CountTodayAllAt(_ context.Context, now time.Time) (int, error) {
	f.lastNow = now
	return f.today, nil
}
func (f *fakeUsage) CountThisMonthAllAt(_ context.Context, now time.Time) (int, error) {
	f.lastNow = now
	return f.month, nil
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

	// The injected clock value must flow through to usage calls.
	if !usage.lastNow.Equal(fixedNow) {
		t.Errorf("usage lastNow = %v, want %v (injected clock not forwarded)", usage.lastNow, fixedNow)
	}
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
