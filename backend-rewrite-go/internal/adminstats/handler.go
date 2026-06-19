// handler.go — HTTP edge for GET /admin/stats and GET /admin/analytics.
// Mounts inside the admin group (jwtMW → RequireAdmin) wired in the router
// task; the handler itself does no auth.
//
//	GET /admin/stats     → 200 { total_users, active_subscriptions, rewrites_today, rewrites_this_month }
//	GET /admin/analytics → 200 { mrr, total_revenue, plans_breakdown, monthly_stats }
//
// Node parity notes:
//   - Both routes are plain @Get with no @HttpCode override → default 200.
//   - No request body, no path params, no query params.
//   - Service error → Node AllExceptionsFilter 500; Go shared.WriteError("internal",...) → 500.
//   - plans_breakdown and monthly_stats are [] not null (use case already coerces
//     nil → [] before returning, so the handler writes through without extra guarding).
package adminstats

import (
	"context"
	"net/http"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// adminStatsService is the handler's consumer-side port; *Service satisfies it.
// Kept on the consumer side (CLAUDE.md guardrail) so tests inject a fake without a DB.
type adminStatsService interface {
	Stats(ctx context.Context) (StatsResult, error)
	Analytics(ctx context.Context) (AnalyticsResult, error)
}

// Handler serves the admin stats + analytics routes.
type Handler struct {
	svc adminStatsService
}

// NewHandler wires the service.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// GetStats handles GET /admin/stats → 200 { total_users, active_subscriptions,
// rewrites_today, rewrites_this_month }.
//
// Parity: Node getStats() returns the four counts directly from the controller
// (no wrapping object). StatsResult json tags are the exact keys Node returns.
func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.Stats(r.Context())
	if err != nil {
		shared.WriteError(w, r, "internal", "stats failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, res)
}

// GetAnalytics handles GET /admin/analytics → 200 { mrr, total_revenue,
// plans_breakdown, monthly_stats }.
//
// Parity: Node getAnalytics() returns the four fields directly. The use case
// (usecase.go Analytics()) already coerces nil slices to [] before returning,
// so plans_breakdown and monthly_stats always marshal as [] not null.
func (h *Handler) GetAnalytics(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.Analytics(r.Context())
	if err != nil {
		shared.WriteError(w, r, "internal", "analytics failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, res)
}
