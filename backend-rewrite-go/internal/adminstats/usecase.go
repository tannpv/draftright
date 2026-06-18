// Package adminstats implements the GET /admin/stats and GET /admin/analytics
// use cases.
//
// Parity authority: src/admin/admin.controller.ts getStats() + getAnalytics()
// and the services they call. Node drives four reads in parallel for getStats:
//
//	[total_users, active_subscriptions, rewrites_today, rewrites_this_month] =
//	    await Promise.all([
//	        usersService.count(),
//	        subscriptionsService.countActive(),
//	        usageService.countToday(),   // all users, global
//	        usageService.countThisMonth(), // all users, global
//	    ])
//
// Go runs them sequentially (same net result; any parallelism is a future
// optimisation if needed). The four counts feed StatsResult which the handler
// serialises to { total_users, active_subscriptions, rewrites_today,
// rewrites_this_month }.
//
// getAnalytics() MRR algorithm (Node lines 598-609):
//
//	for b in breakdown:
//	    plan = plans.find(p => p.name === b.plan_name)
//	    if plan && plan.price_cents > 0:
//	        if plan.billing_period === 'monthly': mrr += b.active_count * plan.price_cents
//	        if plan.billing_period === 'yearly':  mrr += b.active_count * Math.round(plan.price_cents / 12)
//	total_revenue = monthlyStats.reduce((sum, m) => sum + m.revenue_cents, 0)
//
// Clean-arch invariants:
//   - This package imports NO pgx / chi / net/http.
//   - Consumer-side port interfaces are declared here; main.go injects concretes.
//   - Sibling data types (subscription.PlanBreakdown, subscription.MonthStat,
//     plans.PlanEntity) are imported as plain data — their constructors are not.
package adminstats

import (
	"context"
	"math"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/plans"
	"github.com/tannpv/draftright-rewrite/internal/subscription"
)

// ─── Consumer-side port interfaces ───────────────────────────────────────────

// userCounter counts all users in the system.
// Satisfied by *user.AdminRepo (its Count method wraps SELECT COUNT(*) FROM users).
type userCounter interface {
	Count(ctx context.Context) (int, error)
}

// subStatsPort provides subscription aggregates needed by stats + analytics.
// Satisfied by *subscription.Reader. Stats() only calls CountActive; the other
// two methods are here so Task 10 (analytics) can extend this same Service
// without changing the interface or the constructor.
type subStatsPort interface {
	CountActive(ctx context.Context) (int, error)
	GetPlansBreakdown(ctx context.Context) ([]subscription.PlanBreakdown, error)
	GetMonthlyStatsAt(ctx context.Context, now time.Time, months int) ([]subscription.MonthStat, error)
}

// usageGlobal provides system-wide usage aggregates (all users, no user_id filter).
// Satisfied by *usage.Counter (CountTodayAllAt / CountThisMonthAllAt, added Task 1).
type usageGlobal interface {
	CountTodayAllAt(ctx context.Context, now time.Time) (int, error)
	CountThisMonthAllAt(ctx context.Context, now time.Time) (int, error)
}

// planLister lists every plan (used by Task 10 analytics breakdown).
// Satisfied by *plans.AdminService (ListAll). Not called by Stats() itself.
type planLister interface {
	ListAll(ctx context.Context) ([]plans.PlanEntity, error)
}

// ─── Domain ──────────────────────────────────────────────────────────────────

// StatsResult is the GET /admin/stats response body. JSON tags mirror Node's
// returned object keys exactly: total_users, active_subscriptions,
// rewrites_today, rewrites_this_month.
type StatsResult struct {
	TotalUsers          int `json:"total_users"`
	ActiveSubscriptions int `json:"active_subscriptions"`
	RewritesToday       int `json:"rewrites_today"`
	RewritesThisMonth   int `json:"rewrites_this_month"`
}

// AnalyticsResult is the GET /admin/analytics response body. JSON keys and
// field order mirror Node's getAnalytics() returned literal exactly:
// mrr, total_revenue, plans_breakdown, monthly_stats.
// PlansBreakdown and MonthlyStats reuse the sibling types declared in
// subscription/reader.go so the JSON tags (plan_name/active_count/price_cents
// and month/new_subscriptions/revenue_cents/churned) are inherited verbatim.
type AnalyticsResult struct {
	MRR            int                          `json:"mrr"`
	TotalRevenue   int                          `json:"total_revenue"`
	PlansBreakdown []subscription.PlanBreakdown `json:"plans_breakdown"`
	MonthlyStats   []subscription.MonthStat     `json:"monthly_stats"`
}

// ─── Service ─────────────────────────────────────────────────────────────────

// Service is the adminstats use case. It composes four sibling ports; main.go
// injects concrete adapters at startup.
type Service struct {
	users userCounter
	subs  subStatsPort
	usage usageGlobal
	plans planLister
	now   func() time.Time
}

// NewService wires the four ports and the clock. Pass time.Now in production;
// pass a fixed-time func in tests.
func NewService(
	users userCounter,
	subs subStatsPort,
	usage usageGlobal,
	plans planLister,
	now func() time.Time,
) *Service {
	return &Service{users: users, subs: subs, usage: usage, plans: plans, now: now}
}

// Stats mirrors Node getStats(): collects the four global counts and returns
// them as StatsResult. The injected clock (s.now()) is forwarded to both
// usage calls so the "today" and "this month" windows are consistent with the
// time the request landed, and so tests can inject a deterministic clock.
func (s *Service) Stats(ctx context.Context) (StatsResult, error) {
	now := s.now()

	totalUsers, err := s.users.Count(ctx)
	if err != nil {
		return StatsResult{}, err
	}

	activeSubs, err := s.subs.CountActive(ctx)
	if err != nil {
		return StatsResult{}, err
	}

	rewritesToday, err := s.usage.CountTodayAllAt(ctx, now)
	if err != nil {
		return StatsResult{}, err
	}

	rewritesThisMonth, err := s.usage.CountThisMonthAllAt(ctx, now)
	if err != nil {
		return StatsResult{}, err
	}

	return StatsResult{
		TotalUsers:          totalUsers,
		ActiveSubscriptions: activeSubs,
		RewritesToday:       rewritesToday,
		RewritesThisMonth:   rewritesThisMonth,
	}, nil
}

// Analytics mirrors Node getAnalytics(): computes MRR from the active
// subscriptions breakdown + plan catalog, and returns the 12-month revenue
// trend. Parity authority: src/admin/admin.controller.ts lines 592-618.
//
// MRR algorithm (exact match to Node Math.round):
//
//	monthly plan → active_count * price_cents
//	yearly  plan → active_count * round(price_cents / 12)
//	price_cents <= 0 → skip (free/unpriced plans don't contribute)
//	plan matched by Name == b.PlanName (Node: p.name === b.plan_name)
func (s *Service) Analytics(ctx context.Context) (AnalyticsResult, error) {
	now := s.now()

	breakdown, err := s.subs.GetPlansBreakdown(ctx)
	if err != nil {
		return AnalyticsResult{}, err
	}

	monthlyStats, err := s.subs.GetMonthlyStatsAt(ctx, now, 12)
	if err != nil {
		return AnalyticsResult{}, err
	}

	allPlans, err := s.plans.ListAll(ctx)
	if err != nil {
		return AnalyticsResult{}, err
	}

	// Build a name→plan lookup to mirror Node's Array.find.
	planByName := make(map[string]plans.PlanEntity, len(allPlans))
	for _, p := range allPlans {
		planByName[p.Name] = p
	}

	var mrr int
	for _, b := range breakdown {
		p, ok := planByName[b.PlanName]
		if !ok {
			// No matching plan — skip (Node: plan is undefined → condition fails).
			continue
		}
		if p.PriceCents <= 0 {
			// Free or unpriced plan — skip (Node: plan.price_cents > 0).
			continue
		}
		switch p.BillingPeriod {
		case "monthly":
			mrr += b.ActiveCount * p.PriceCents
		case "yearly":
			// Node: Math.round(plan.price_cents / 12)
			mrr += b.ActiveCount * int(math.Round(float64(p.PriceCents)/12.0))
		}
		// Any other billing_period is silently ignored (parity: Node else-if chain).
	}

	var totalRevenue int
	for _, m := range monthlyStats {
		totalRevenue += m.RevenueCents
	}

	// Ensure non-nil slices so JSON marshals [] not null (parity: Node returns
	// the DB result directly, which is always an array).
	if breakdown == nil {
		breakdown = []subscription.PlanBreakdown{}
	}
	if monthlyStats == nil {
		monthlyStats = []subscription.MonthStat{}
	}

	return AnalyticsResult{
		MRR:            mrr,
		TotalRevenue:   totalRevenue,
		PlansBreakdown: breakdown,
		MonthlyStats:   monthlyStats,
	}, nil
}
