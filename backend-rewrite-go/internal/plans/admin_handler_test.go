package plans

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/tannpv/draftright-rewrite/internal/shared/listquery"
)

// fakeAdminRepo satisfies the admin usecase's plansStore port. It captures the
// NewPlan passed to Create so the create-default tests can assert on it, and
// records which list branch ran (ListAll vs ListPaginated) for the dual-mode
// tests.
type fakeAdminRepo struct {
	listAll       []PlanEntity
	paginated     []PlanEntity
	total         int
	created       NewPlan // captured Create arg
	calledListAll bool
	calledPaginat bool
}

func (f *fakeAdminRepo) ListAll(ctx context.Context) ([]PlanEntity, error) {
	f.calledListAll = true
	return f.listAll, nil
}

func (f *fakeAdminRepo) ListPaginated(ctx context.Context, b listquery.Built) ([]PlanEntity, int, error) {
	f.calledPaginat = true
	return f.paginated, f.total, nil
}

func (f *fakeAdminRepo) Create(ctx context.Context, in NewPlan) (PlanEntity, error) {
	f.created = in
	return PlanEntity{Name: in.Name}, nil
}

func (f *fakeAdminRepo) Update(ctx context.Context, id string, p PlanPatch) (PlanEntity, error) {
	return PlanEntity{ID: id}, nil
}

func (f *fakeAdminRepo) SoftDelete(ctx context.Context, id string) error { return nil }

// assertAdminKeyOrder fails unless the JSON keys appear in raw in the given
// order. Local to this file (no such helper exists in the plans package).
func assertAdminKeyOrder(t *testing.T, raw string, keys ...string) {
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

func newAdminHandler(repo *fakeAdminRepo) *AdminHandler {
	return NewAdminHandler(NewAdminService(repo))
}

// TestList_UnpaginatedWhenNoParams: GET /admin/plans with no query params →
// legacy unpaginated branch → 200 bare JSON array (empty → "[]").
func TestList_UnpaginatedWhenNoParams(t *testing.T) {
	repo := &fakeAdminRepo{listAll: []PlanEntity{}}
	h := newAdminHandler(repo)
	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequest(http.MethodGet, "/admin/plans", nil))

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !repo.calledListAll || repo.calledPaginat {
		t.Fatalf("expected ListAll branch; listAll=%v paginated=%v", repo.calledListAll, repo.calledPaginat)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "[]" {
		t.Fatalf("body = %q, want %q", got, "[]")
	}
}

// TestList_PaginatedWhenParam: any of page/search/status/sort_by present →
// paginated branch → 200 { rows, total } with key order rows,total.
func TestList_PaginatedWhenParam(t *testing.T) {
	repo := &fakeAdminRepo{paginated: []PlanEntity{}, total: 7}
	h := newAdminHandler(repo)
	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequest(http.MethodGet, "/admin/plans?page=2", nil))

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !repo.calledPaginat || repo.calledListAll {
		t.Fatalf("expected ListPaginated branch; listAll=%v paginated=%v", repo.calledListAll, repo.calledPaginat)
	}
	raw := rec.Body.String()
	assertAdminKeyOrder(t, raw, "rows", "total")
	var body struct {
		Rows  []map[string]any `json:"rows"`
		Total int              `json:"total"`
	}
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatalf("body not JSON: %v (%s)", err, raw)
	}
	if body.Total != 7 {
		t.Fatalf("total = %d, want 7", body.Total)
	}
}

// TestCreate_Returns201: POST /admin/plans → 201.
func TestCreate_Returns201(t *testing.T) {
	repo := &fakeAdminRepo{}
	h := newAdminHandler(repo)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/plans",
		strings.NewReader(`{"name":"Pro","daily_limit":100,"price_cents":900,"billing_period":"monthly"}`))
	h.Create(rec, req)

	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	if repo.created.Name != "Pro" || repo.created.BillingPeriod != "monthly" {
		t.Fatalf("created = %+v", repo.created)
	}
}

// TestPlanConfig_PriceAlias: sort_by=price must map to price_cents in the Order
// clause (Node's plans sort map alias).
func TestPlanConfig_PriceAlias(t *testing.T) {
	q := listquery.Parse(map[string][]string{"sort_by": {"price"}})
	b := listquery.Build(q, planSearchCols, planSortAllow, "created_at", "is_active")
	if !strings.Contains(b.Order, "price_cents") {
		t.Fatalf("order %q does not contain 'price_cents'", b.Order)
	}
}

// TestCreate_AppliesNodeDefaults: absent trial_days → 30, absent billing_period
// → "none" must reach the repo (TypeORM omit-absent-column parity). price_cents
// absent → 0; currency/stripe_price_id absent → nil.
func TestCreate_AppliesNodeDefaults(t *testing.T) {
	repo := &fakeAdminRepo{}
	h := newAdminHandler(repo)
	rec := httptest.NewRecorder()
	// Only the required fields present; all optionals absent.
	req := httptest.NewRequest(http.MethodPost, "/admin/plans",
		strings.NewReader(`{"name":"Free","daily_limit":10}`))
	h.Create(rec, req)

	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	if repo.created.TrialDays != 30 {
		t.Fatalf("trial_days = %d, want 30 (DB default)", repo.created.TrialDays)
	}
	if repo.created.BillingPeriod != "none" {
		t.Fatalf("billing_period = %q, want \"none\" (DB default)", repo.created.BillingPeriod)
	}
	if repo.created.PriceCents != 0 {
		t.Fatalf("price_cents = %d, want 0 (DB default)", repo.created.PriceCents)
	}
	if repo.created.Currency != nil {
		t.Fatalf("currency = %v, want nil", repo.created.Currency)
	}
	if repo.created.StripePriceID != nil {
		t.Fatalf("stripe_price_id = %v, want nil", repo.created.StripePriceID)
	}
	if repo.created.Name != "Free" || repo.created.DailyLimit != 10 {
		t.Fatalf("required fields not passed through: %+v", repo.created)
	}
}

// TestDelete_SuccessBody: DELETE /admin/plans/:id → 200 { "success": true }.
func TestDelete_SuccessBody(t *testing.T) {
	h := newAdminHandler(&fakeAdminRepo{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/admin/plans/x", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "x")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	h.Delete(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `{"success":true}` {
		t.Fatalf("body = %q, want %q", got, `{"success":true}`)
	}
}
