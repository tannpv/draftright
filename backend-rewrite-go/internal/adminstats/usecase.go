// Package adminstats implements the GET /admin/stats use case.
//
// Parity authority: src/admin/admin.controller.ts getStats() + the services it
// calls. Node drives four reads in parallel:
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
// Clean-arch invariants:
//   - This package imports NO pgx / chi / net/http.
//   - Consumer-side port interfaces are declared here; main.go injects concretes.
//   - Sibling data types (subscription.PlanBreakdown, subscription.MonthStat,
//     plans.PlanEntity) are imported as plain data — their constructors are not.
package adminstats

import (
	"context"
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
