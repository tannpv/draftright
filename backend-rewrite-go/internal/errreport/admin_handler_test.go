package errreport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/tannpv/draftright-rewrite/internal/aiprovider"
)

// fakeAdminSvc satisfies adminHandlerService. Fields capture args; configured
// fields drive return values.
type fakeAdminSvc struct {
	// List
	listItems  []ErrorReportEntity
	listTotal  int
	listErr    error
	lastFilter AdminListFilter

	// Get
	getEntity ErrorReportEntity
	getErr    error
	lastGetID string

	// Delete
	deleteOK     bool
	deleteErr    error
	lastDeleteID string

	// SetStatus
	statusEntity   ErrorReportEntity
	statusErr      error
	lastStatusID   string
	lastStatus     int
	lastResolvedBy string

	// SuggestFix
	fixEntity ErrorReportEntity
	fixErr    error
	lastFixID string
}

func (f *fakeAdminSvc) List(ctx context.Context, filter AdminListFilter) ([]ErrorReportEntity, int, error) {
	f.lastFilter = filter
	return f.listItems, f.listTotal, f.listErr
}

func (f *fakeAdminSvc) Get(ctx context.Context, id string) (ErrorReportEntity, error) {
	f.lastGetID = id
	return f.getEntity, f.getErr
}

func (f *fakeAdminSvc) Delete(ctx context.Context, id string) (bool, error) {
	f.lastDeleteID = id
	return f.deleteOK, f.deleteErr
}

func (f *fakeAdminSvc) SetStatus(ctx context.Context, id string, status int, resolvedBy string) (ErrorReportEntity, error) {
	f.lastStatusID = id
	f.lastStatus = status
	f.lastResolvedBy = resolvedBy
	return f.statusEntity, f.statusErr
}

func (f *fakeAdminSvc) SuggestFix(ctx context.Context, id string) (ErrorReportEntity, error) {
	f.lastFixID = id
	return f.fixEntity, f.fixErr
}

// fakeCron satisfies adminHandlerCron — captures whether RunOnce ran.
type fakeCron struct {
	ran bool
	res CronResult
}

func (c *fakeCron) RunOnce(ctx context.Context) CronResult {
	c.ran = true
	return c.res
}

// routeWithID attaches a chi RouteContext carrying :id so chi.URLParam resolves.
func routeWithID(req *http.Request, id string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func newAdminH(svc *fakeAdminSvc, cron *fakeCron) *AdminHandler {
	return &AdminHandler{svc: svc, cron: cron}
}

// ---------------------------------------------------------------------------
// E1 — GET /admin/errors
// ---------------------------------------------------------------------------

func TestAdminList_Shape(t *testing.T) {
	svc := &fakeAdminSvc{
		listItems: []ErrorReportEntity{{ID: "a"}},
		listTotal: 7,
	}
	h := newAdminH(svc, &fakeCron{})

	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequest(http.MethodGet, "/admin/errors", nil))

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	raw := rec.Body.String()
	// { items, total } in order.
	itemsIdx := strings.Index(raw, `"items"`)
	totalIdx := strings.Index(raw, `"total"`)
	if itemsIdx < 0 || totalIdx < 0 || itemsIdx >= totalIdx {
		t.Fatalf("items/total key order wrong: %s", raw)
	}
}

func TestAdminList_EmptyItemsIsArray(t *testing.T) {
	svc := &fakeAdminSvc{listItems: nil, listTotal: 0}
	h := newAdminH(svc, &fakeCron{})

	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequest(http.MethodGet, "/admin/errors", nil))

	if !strings.Contains(rec.Body.String(), `"items":[]`) {
		t.Fatalf("empty items must marshal as []; got %s", rec.Body.String())
	}
}

func TestAdminList_Defaults(t *testing.T) {
	svc := &fakeAdminSvc{}
	h := newAdminH(svc, &fakeCron{})

	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequest(http.MethodGet, "/admin/errors", nil))

	if svc.lastFilter.Limit != 50 {
		t.Fatalf("default limit = %d, want 50", svc.lastFilter.Limit)
	}
	if svc.lastFilter.Offset != 0 {
		t.Fatalf("default offset = %d, want 0", svc.lastFilter.Offset)
	}
	if svc.lastFilter.Platform != nil || svc.lastFilter.Severity != nil || svc.lastFilter.Status != nil {
		t.Fatalf("no filters expected: %+v", svc.lastFilter)
	}
}

func TestAdminList_LimitCap(t *testing.T) {
	svc := &fakeAdminSvc{}
	h := newAdminH(svc, &fakeCron{})

	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequest(http.MethodGet, "/admin/errors?limit=500", nil))

	if svc.lastFilter.Limit != 200 {
		t.Fatalf("limit cap = %d, want 200", svc.lastFilter.Limit)
	}
}

func TestAdminList_LimitZeroFallsToDefault(t *testing.T) {
	svc := &fakeAdminSvc{}
	h := newAdminH(svc, &fakeCron{})

	rec := httptest.NewRecorder()
	// Number("0") = 0; 0 || 50 = 50 (Node falsy).
	h.List(rec, httptest.NewRequest(http.MethodGet, "/admin/errors?limit=0", nil))

	if svc.lastFilter.Limit != 50 {
		t.Fatalf("limit=0 should fall to 50, got %d", svc.lastFilter.Limit)
	}
}

func TestAdminList_BadLimitFallsToDefault(t *testing.T) {
	svc := &fakeAdminSvc{}
	h := newAdminH(svc, &fakeCron{})

	rec := httptest.NewRecorder()
	// Number("abc") = NaN; NaN || 50 = 50.
	h.List(rec, httptest.NewRequest(http.MethodGet, "/admin/errors?limit=abc", nil))

	if svc.lastFilter.Limit != 50 {
		t.Fatalf("limit=abc should fall to 50, got %d", svc.lastFilter.Limit)
	}
}

func TestAdminList_Filters(t *testing.T) {
	svc := &fakeAdminSvc{}
	h := newAdminH(svc, &fakeCron{})

	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequest(http.MethodGet, "/admin/errors?platform=ios&severity=fatal&status=0&offset=10", nil))

	if svc.lastFilter.Platform == nil || *svc.lastFilter.Platform != "ios" {
		t.Fatalf("platform filter not set: %+v", svc.lastFilter.Platform)
	}
	if svc.lastFilter.Severity == nil || *svc.lastFilter.Severity != "fatal" {
		t.Fatalf("severity filter not set: %+v", svc.lastFilter.Severity)
	}
	if svc.lastFilter.Status == nil || *svc.lastFilter.Status != 0 {
		t.Fatalf("status filter not set: %+v", svc.lastFilter.Status)
	}
	if svc.lastFilter.Offset != 10 {
		t.Fatalf("offset = %d, want 10", svc.lastFilter.Offset)
	}
}

func TestAdminList_Error500(t *testing.T) {
	svc := &fakeAdminSvc{listErr: context.DeadlineExceeded}
	h := newAdminH(svc, &fakeCron{})

	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequest(http.MethodGet, "/admin/errors", nil))

	if rec.Code != 500 {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"internal"`) {
		t.Fatalf("want internal code: %s", rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// E2 — GET /admin/errors/:id
// ---------------------------------------------------------------------------

func TestAdminGet_OK(t *testing.T) {
	svc := &fakeAdminSvc{getEntity: ErrorReportEntity{ID: "x", Platform: "ios"}}
	h := newAdminH(svc, &fakeCron{})

	rec := httptest.NewRecorder()
	req := routeWithID(httptest.NewRequest(http.MethodGet, "/admin/errors/x", nil), "x")
	h.Get(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if svc.lastGetID != "x" {
		t.Fatalf("id = %q, want x", svc.lastGetID)
	}
}

func TestAdminGet_NotFound400(t *testing.T) {
	svc := &fakeAdminSvc{getErr: ErrNotFound}
	h := newAdminH(svc, &fakeCron{})

	rec := httptest.NewRecorder()
	req := routeWithID(httptest.NewRequest(http.MethodGet, "/admin/errors/x", nil), "x")
	h.Get(rec, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	raw := rec.Body.String()
	if !strings.Contains(raw, `"error":"not found"`) || !strings.Contains(raw, `"code":"invalid-input"`) {
		t.Fatalf("envelope mismatch: %s", raw)
	}
}

func TestAdminGet_Error500(t *testing.T) {
	svc := &fakeAdminSvc{getErr: context.DeadlineExceeded}
	h := newAdminH(svc, &fakeCron{})

	rec := httptest.NewRecorder()
	req := routeWithID(httptest.NewRequest(http.MethodGet, "/admin/errors/x", nil), "x")
	h.Get(rec, req)

	if rec.Code != 500 {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// E3 — PATCH /admin/errors/:id
// ---------------------------------------------------------------------------

func TestAdminPatch_MissingStatus400(t *testing.T) {
	svc := &fakeAdminSvc{}
	h := newAdminH(svc, &fakeCron{})

	rec := httptest.NewRecorder()
	req := routeWithID(httptest.NewRequest(http.MethodPatch, "/admin/errors/x", strings.NewReader(`{"resolved_by":"admin"}`)), "x")
	h.Patch(rec, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	raw := rec.Body.String()
	if !strings.Contains(raw, `"error":"status required"`) || !strings.Contains(raw, `"code":"invalid-input"`) {
		t.Fatalf("envelope mismatch: %s", raw)
	}
}

func TestAdminPatch_EmptyBody400(t *testing.T) {
	// Node body-parser treats an empty body as {} → admin.controller.ts
	// `body.status === undefined` fires → 400 "status required" (NOT a
	// JSON-decode error). io.EOF must be tolerated so we reach that check.
	svc := &fakeAdminSvc{}
	h := newAdminH(svc, &fakeCron{})

	rec := httptest.NewRecorder()
	req := routeWithID(httptest.NewRequest(http.MethodPatch, "/admin/errors/x", strings.NewReader(``)), "x")
	h.Patch(rec, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	raw := rec.Body.String()
	if !strings.Contains(raw, `"error":"status required"`) || !strings.Contains(raw, `"code":"invalid-input"`) {
		t.Fatalf("envelope mismatch: %s", raw)
	}
}

func TestAdminPatch_OK(t *testing.T) {
	svc := &fakeAdminSvc{statusEntity: ErrorReportEntity{ID: "x", Status: 4}}
	h := newAdminH(svc, &fakeCron{})

	rec := httptest.NewRecorder()
	req := routeWithID(httptest.NewRequest(http.MethodPatch, "/admin/errors/x", strings.NewReader(`{"status":4,"resolved_by":"bob"}`)), "x")
	h.Patch(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if svc.lastStatus != 4 || svc.lastResolvedBy != "bob" || svc.lastStatusID != "x" {
		t.Fatalf("args wrong: id=%q status=%d rb=%q", svc.lastStatusID, svc.lastStatus, svc.lastResolvedBy)
	}
}

func TestAdminPatch_StatusZeroIsValid(t *testing.T) {
	// status:0 is present (not undefined) → must pass the required check.
	svc := &fakeAdminSvc{statusEntity: ErrorReportEntity{ID: "x"}}
	h := newAdminH(svc, &fakeCron{})

	rec := httptest.NewRecorder()
	req := routeWithID(httptest.NewRequest(http.MethodPatch, "/admin/errors/x", strings.NewReader(`{"status":0}`)), "x")
	h.Patch(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status:0 should be valid, got %d; body=%s", rec.Code, rec.Body.String())
	}
	if svc.lastStatus != 0 {
		t.Fatalf("status forwarded = %d, want 0", svc.lastStatus)
	}
}

func TestAdminPatch_BadBody400(t *testing.T) {
	svc := &fakeAdminSvc{}
	h := newAdminH(svc, &fakeCron{})

	rec := httptest.NewRecorder()
	req := routeWithID(httptest.NewRequest(http.MethodPatch, "/admin/errors/x", strings.NewReader(`not json`)), "x")
	h.Patch(rec, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminPatch_NotFound400(t *testing.T) {
	svc := &fakeAdminSvc{statusErr: ErrNotFound}
	h := newAdminH(svc, &fakeCron{})

	rec := httptest.NewRecorder()
	req := routeWithID(httptest.NewRequest(http.MethodPatch, "/admin/errors/x", strings.NewReader(`{"status":4}`)), "x")
	h.Patch(rec, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"error":"not found"`) {
		t.Fatalf("want not found: %s", rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// E4 — DELETE /admin/errors/:id
// ---------------------------------------------------------------------------

func TestAdminDelete_Deleted(t *testing.T) {
	svc := &fakeAdminSvc{deleteOK: true}
	h := newAdminH(svc, &fakeCron{})

	rec := httptest.NewRecorder()
	req := routeWithID(httptest.NewRequest(http.MethodDelete, "/admin/errors/x", nil), "x")
	h.Delete(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	raw := rec.Body.String()
	idIdx := strings.Index(raw, `"id"`)
	delIdx := strings.Index(raw, `"deleted"`)
	if idIdx < 0 || delIdx < 0 || idIdx >= delIdx {
		t.Fatalf("id/deleted key order wrong: %s", raw)
	}
	var m map[string]any
	_ = json.Unmarshal([]byte(raw), &m)
	if m["id"] != "x" || m["deleted"] != true {
		t.Fatalf("body wrong: %s", raw)
	}
}

func TestAdminDelete_Idempotent(t *testing.T) {
	svc := &fakeAdminSvc{deleteOK: false}
	h := newAdminH(svc, &fakeCron{})

	rec := httptest.NewRecorder()
	req := routeWithID(httptest.NewRequest(http.MethodDelete, "/admin/errors/gone", nil), "gone")
	h.Delete(rec, req)

	if rec.Code != 200 {
		t.Fatalf("idempotent delete must be 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"deleted":false`) {
		t.Fatalf("want deleted:false: %s", rec.Body.String())
	}
}

func TestAdminDelete_Error500(t *testing.T) {
	svc := &fakeAdminSvc{deleteErr: context.DeadlineExceeded}
	h := newAdminH(svc, &fakeCron{})

	rec := httptest.NewRecorder()
	req := routeWithID(httptest.NewRequest(http.MethodDelete, "/admin/errors/x", nil), "x")
	h.Delete(rec, req)

	if rec.Code != 500 {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// E5 — POST /admin/errors/:id/suggest-fix
// ---------------------------------------------------------------------------

func TestAdminSuggestFix_Created201(t *testing.T) {
	svc := &fakeAdminSvc{fixEntity: ErrorReportEntity{ID: "x", Status: 3}}
	h := newAdminH(svc, &fakeCron{})

	rec := httptest.NewRecorder()
	req := routeWithID(httptest.NewRequest(http.MethodPost, "/admin/errors/x/suggest-fix", nil), "x")
	h.SuggestFix(rec, req)

	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	if svc.lastFixID != "x" {
		t.Fatalf("id = %q, want x", svc.lastFixID)
	}
}

func TestAdminSuggestFix_NoDefaultProvider400(t *testing.T) {
	svc := &fakeAdminSvc{fixErr: aiprovider.ErrNoDefaultProvider}
	h := newAdminH(svc, &fakeCron{})

	rec := httptest.NewRecorder()
	req := routeWithID(httptest.NewRequest(http.MethodPost, "/admin/errors/x/suggest-fix", nil), "x")
	h.SuggestFix(rec, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	raw := rec.Body.String()
	if !strings.Contains(raw, `"error":"No default AI provider configured"`) || !strings.Contains(raw, `"code":"invalid-input"`) {
		t.Fatalf("envelope mismatch: %s", raw)
	}
}

func TestAdminSuggestFix_NotFound400(t *testing.T) {
	svc := &fakeAdminSvc{fixErr: ErrNotFound}
	h := newAdminH(svc, &fakeCron{})

	rec := httptest.NewRecorder()
	req := routeWithID(httptest.NewRequest(http.MethodPost, "/admin/errors/x/suggest-fix", nil), "x")
	h.SuggestFix(rec, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"error":"not found"`) {
		t.Fatalf("want not found: %s", rec.Body.String())
	}
}

func TestAdminSuggestFix_Error500(t *testing.T) {
	svc := &fakeAdminSvc{fixErr: context.DeadlineExceeded}
	h := newAdminH(svc, &fakeCron{})

	rec := httptest.NewRecorder()
	req := routeWithID(httptest.NewRequest(http.MethodPost, "/admin/errors/x/suggest-fix", nil), "x")
	h.SuggestFix(rec, req)

	if rec.Code != 500 {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// E6 — POST /admin/errors/run-ai-cron
// ---------------------------------------------------------------------------

func TestAdminRunCron_Created201(t *testing.T) {
	cron := &fakeCron{}
	h := newAdminH(&fakeAdminSvc{}, cron)

	rec := httptest.NewRecorder()
	h.RunCron(rec, httptest.NewRequest(http.MethodPost, "/admin/errors/run-ai-cron", nil))

	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	if !cron.ran {
		t.Fatalf("cron should have run")
	}
	if !strings.Contains(rec.Body.String(), `"ok":true`) {
		t.Fatalf("want ok:true: %s", rec.Body.String())
	}
}
