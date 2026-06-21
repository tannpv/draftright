package plans

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/tannpv/draftright-rewrite/internal/shared"
	"github.com/tannpv/draftright-rewrite/internal/shared/listquery"
)

// planSearchCols / planSortAllow are this consumer's listquery config for the
// paginated branch of GET /admin/plans (plans.service.ts findAllPaginated).
// searchCols are ILIKE-matched against `search`; sortAllow is the
// injection-safe allow-list mapping public sort keys → SQL columns. The
// `price → price_cents` alias mirrors Node exactly. Default sort created_at,
// status column is_active.
var planSearchCols = []string{"name", "currency", "billing_period"}
var planSortAllow = map[string]string{
	"name": "name", "price": "price_cents", "currency": "currency",
	"billing_period": "billing_period", "trial_days": "trial_days",
	"is_active": "is_active", "created_at": "created_at",
}

// adminPlansService is the handler's consumer-side port; *AdminService
// satisfies it. Kept on the consumer side (CLAUDE.md guardrail) so tests inject
// a fake.
type adminPlansService interface {
	ListAll(ctx context.Context) ([]PlanEntity, error)
	ListPaginated(ctx context.Context, b listquery.Built) ([]PlanEntity, int, error)
	Create(ctx context.Context, in NewPlan) (PlanEntity, error)
	Update(ctx context.Context, id string, p PlanPatch) (PlanEntity, error)
	SoftDelete(ctx context.Context, id string) error
}

// AdminHandler serves the admin plans routes. All routes mount inside the admin
// group (jwtMW → RequireAdmin) wired in the router task; the handler itself does
// no auth.
//
//	GET    /admin/plans       dual-mode: bare array (no params) | { rows, total }
//	POST   /admin/plans       create → 201
//	PATCH  /admin/plans/:id    partial update → 200
//	DELETE /admin/plans/:id    soft delete → 200 { success:true }
type AdminHandler struct {
	svc adminPlansService
}

// NewAdminHandler wires the service.
func NewAdminHandler(svc *AdminService) *AdminHandler { return &AdminHandler{svc: svc} }

// adminPaginatedResponse is the { rows, total } body of the paginated branch.
// Field order (rows, total) matches Node's findAllPaginated / ListResult return
// exactly. Same ordered-struct approach aiprovider's paginatedResponse uses.
type adminPaginatedResponse struct {
	Rows  []PlanEntity `json:"rows"`
	Total int          `json:"total"`
}

// createPlanBody decodes the POST body. Pointers distinguish an absent key
// (nil) from an explicit value so the handler can apply the DB-column defaults
// TypeORM relies on (Node's create omits absent fields from the INSERT):
// trial_days absent → 30, billing_period absent → "none", price_cents absent →
// 0; currency/stripe_price_id absent → NULL. name + daily_limit are required.
type createPlanBody struct {
	Name          *string `json:"name"`
	DailyLimit    *int    `json:"daily_limit"`
	PriceCents    *int    `json:"price_cents"`
	BillingPeriod *string `json:"billing_period"`
	Currency      *string `json:"currency"`
	TrialDays     *int    `json:"trial_days"`
	StripePriceID *string `json:"stripe_price_id"`
}

// patchPlanBody decodes the PATCH body — every field optional (nil = unchanged).
// PlanPatch carries no json tags (it's a domain type), so a snake_case decode
// target is needed to bind the admin UI's snake_case keys correctly before
// building the PlanPatch (TypeORM partial update parity).
type patchPlanBody struct {
	Name          *string `json:"name"`
	DailyLimit    *int    `json:"daily_limit"`
	PriceCents    *int    `json:"price_cents"`
	BillingPeriod *string `json:"billing_period"`
	Currency      *string `json:"currency"`
	TrialDays     *int    `json:"trial_days"`
	StripePriceID *string `json:"stripe_price_id"`
	IsActive      *bool   `json:"is_active"`
}

// List handles GET /admin/plans. Dual-mode (admin.controller.ts listPlans):
// when page, search, status and sort_by are ALL absent → legacy unpaginated
// findAll as a bare array; otherwise the paginated { rows, total } branch.
func (h *AdminHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if !q.Has("page") && !q.Has("search") && !q.Has("status") && !q.Has("sort_by") {
		plans, err := h.svc.ListAll(r.Context())
		if err != nil {
			shared.WriteError(w, r, "internal", "plans failed")
			return
		}
		if plans == nil {
			plans = []PlanEntity{}
		}
		shared.WriteJSON(w, http.StatusOK, plans)
		return
	}

	b := listquery.Build(listquery.Parse(q), planSearchCols, planSortAllow, "created_at", "is_active")
	rows, total, err := h.svc.ListPaginated(r.Context(), b)
	if err != nil {
		shared.WriteError(w, r, "internal", "plans failed")
		return
	}
	if rows == nil {
		rows = []PlanEntity{}
	}
	shared.WriteJSON(w, http.StatusOK, adminPaginatedResponse{Rows: rows, Total: total})
}

// Create handles POST /admin/plans → 201. Node's @Post() has no @HttpCode
// override, so the success status is 201. The body is an inline `@Body() {…}`
// (NOT a class-validator DTO), so the decode is lenient — unknown fields are
// ignored; only a malformed body 400s.
func (h *AdminHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body createPlanBody
	if !shared.DecodeJSON(w, r, &body, shared.DecodeStrict) {
		return
	}

	// Resolve Node's create defaults BEFORE building NewPlan. Task 9's repo
	// always inserts trial_days + billing_period as columns, so an absent
	// value must be filled here (not left as the Go zero) to match TypeORM's
	// omit-absent-column behaviour where the DB default applies.
	trialDays := 30
	if body.TrialDays != nil {
		trialDays = *body.TrialDays
	}
	billingPeriod := "none"
	if body.BillingPeriod != nil {
		billingPeriod = *body.BillingPeriod
	}
	priceCents := 0
	if body.PriceCents != nil {
		priceCents = *body.PriceCents
	}

	in := NewPlan{
		Name:          derefStr(body.Name),
		DailyLimit:    derefInt(body.DailyLimit),
		PriceCents:    priceCents,
		BillingPeriod: billingPeriod,
		Currency:      body.Currency,
		TrialDays:     trialDays,
		StripePriceID: body.StripePriceID,
	}

	plan, err := h.svc.Create(r.Context(), in)
	if err != nil {
		shared.WriteError(w, r, "internal", "plans failed")
		return
	}
	shared.WriteJSON(w, http.StatusCreated, plan)
}

// Update handles PATCH /admin/plans/:id → 200 full plan. Missing keys = nil =
// column untouched (TypeORM partial update parity). ANY repo error maps to 500
// internal — a missing id surfaces as pgx.ErrNoRows, which mirrors Node's
// EntityNotFoundError → AllExceptionsFilter (code "internal"), NOT a 404.
func (h *AdminHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body patchPlanBody
	if !shared.DecodeJSON(w, r, &body, shared.DecodeStrict) {
		return
	}

	patch := PlanPatch{
		Name:          body.Name,
		DailyLimit:    body.DailyLimit,
		PriceCents:    body.PriceCents,
		BillingPeriod: body.BillingPeriod,
		Currency:      body.Currency,
		TrialDays:     body.TrialDays,
		StripePriceID: body.StripePriceID,
		IsActive:      body.IsActive,
	}

	plan, err := h.svc.Update(r.Context(), id, patch)
	if err != nil {
		shared.WriteError(w, r, "internal", "plans failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, plan)
}

// Delete handles DELETE /admin/plans/:id → 200 { success:true }.
func (h *AdminHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.SoftDelete(r.Context(), id); err != nil {
		shared.WriteError(w, r, "internal", "plans failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, struct {
		Success bool `json:"success"`
	}{true})
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}
