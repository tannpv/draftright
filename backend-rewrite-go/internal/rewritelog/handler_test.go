package rewritelog

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

// fakeService is a test double that satisfies the handler's rewritelogService
// interface. Fields capture args for assertion; configured fields drive
// return values.
type fakeService struct {
	// Stats
	statsResult StatsResult
	statsErr    error

	// ListPending
	listLogs  []RewriteLog
	listTotal int
	listErr   error
	lastPage  int
	lastLimit int

	// Review
	reviewErr    error
	lastReviewID string
	lastQuality  string

	// ExportJSONL
	exportResult string
	exportErr    error
}

func (f *fakeService) Stats(ctx context.Context) (StatsResult, error) {
	return f.statsResult, f.statsErr
}

func (f *fakeService) ListPending(ctx context.Context, page, limit int) ([]RewriteLog, int, error) {
	f.lastPage = page
	f.lastLimit = limit
	return f.listLogs, f.listTotal, f.listErr
}

func (f *fakeService) Review(ctx context.Context, id, quality string) error {
	f.lastReviewID = id
	f.lastQuality = quality
	return f.reviewErr
}

func (f *fakeService) ExportJSONL(ctx context.Context) (string, error) {
	return f.exportResult, f.exportErr
}

// assertKeyOrder fails unless the JSON keys appear in raw in the given order.
// Mirrors the helper in user/admin_handler_test.go.
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

// decodeBody unmarshals into a generic map for value assertions.
func decodeBody(t *testing.T, raw string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("body not JSON: %v (%s)", err, raw)
	}
	return m
}

// routeWithID attaches a chi RouteContext carrying :id so chi.URLParam resolves.
func routeWithID(req *http.Request, id string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

// newH is a convenience wrapper that builds the Handler directly from the fake.
func newH(svc *fakeService) *Handler {
	return &Handler{svc: svc}
}

// ---------------------------------------------------------------------------
// Stats
// ---------------------------------------------------------------------------

// TestStats_Shape: GET /admin/training-data/stats → 200 { total, pending,
// approved, rejected } in that key order.
func TestStats_Shape(t *testing.T) {
	svc := &fakeService{statsResult: StatsResult{
		Total: 100, Pending: 40, Approved: 50, Rejected: 10,
	}}
	h := newH(svc)

	rec := httptest.NewRecorder()
	h.Stats(rec, httptest.NewRequest(http.MethodGet, "/admin/training-data/stats", nil))

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	raw := rec.Body.String()
	assertKeyOrder(t, raw, "total", "pending", "approved", "rejected")

	m := decodeBody(t, raw)
	if m["total"] != float64(100) {
		t.Fatalf("total = %v, want 100", m["total"])
	}
	if m["pending"] != float64(40) {
		t.Fatalf("pending = %v, want 40", m["pending"])
	}
	if m["approved"] != float64(50) {
		t.Fatalf("approved = %v, want 50", m["approved"])
	}
	if m["rejected"] != float64(10) {
		t.Fatalf("rejected = %v, want 10", m["rejected"])
	}
}

// TestStats_ServiceError: Stats service error → 500 internal.
func TestStats_ServiceError(t *testing.T) {
	svc := &fakeService{statsErr: errors.New("db down")}
	h := newH(svc)

	rec := httptest.NewRecorder()
	h.Stats(rec, httptest.NewRequest(http.MethodGet, "/admin/training-data/stats", nil))

	if rec.Code != 500 {
		t.Fatalf("status = %d, want 500; body=%s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

// TestList_Shape: GET /admin/training-data → 200 { logs, total } in that
// key order; logs populated.
func TestList_Shape(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	svc := &fakeService{
		listLogs: []RewriteLog{{
			ID: "l1", Tone: "formal", InputText: "hello", OutputText: "greetings",
			Model: "gpt-4o", ProviderType: "openai", ResponseTimeMs: 123,
			Quality: "pending", CreatedAt: now,
		}},
		listTotal: 1,
	}
	h := newH(svc)

	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequest(http.MethodGet, "/admin/training-data", nil))

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	raw := rec.Body.String()
	// top-level key order: logs first, then total
	assertKeyOrder(t, raw, "logs", "total")

	var body struct {
		Logs  []map[string]any `json:"logs"`
		Total int              `json:"total"`
	}
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatalf("body not JSON: %v (%s)", err, raw)
	}
	if body.Total != 1 {
		t.Fatalf("total = %d, want 1", body.Total)
	}
	if len(body.Logs) != 1 {
		t.Fatalf("len(logs) = %d, want 1", len(body.Logs))
	}
}

// TestList_Defaults: absent page/limit → 1/20 reach the service.
func TestList_Defaults(t *testing.T) {
	svc := &fakeService{listLogs: []RewriteLog{}, listTotal: 0}
	h := newH(svc)

	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequest(http.MethodGet, "/admin/training-data", nil))

	if svc.lastPage != 1 {
		t.Fatalf("page = %d, want 1", svc.lastPage)
	}
	if svc.lastLimit != 20 {
		t.Fatalf("limit = %d, want 20", svc.lastLimit)
	}
}

// TestList_CustomPageLimit: explicit page=2&limit=5 are forwarded.
func TestList_CustomPageLimit(t *testing.T) {
	svc := &fakeService{listLogs: nil, listTotal: 0}
	h := newH(svc)

	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequest(http.MethodGet, "/admin/training-data?page=2&limit=5", nil))

	if svc.lastPage != 2 {
		t.Fatalf("page = %d, want 2", svc.lastPage)
	}
	if svc.lastLimit != 5 {
		t.Fatalf("limit = %d, want 5", svc.lastLimit)
	}
}

// TestList_EmptyIsArray: nil logs from service → "logs":[] not null.
func TestList_EmptyIsArray(t *testing.T) {
	svc := &fakeService{listLogs: nil, listTotal: 0}
	h := newH(svc)

	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequest(http.MethodGet, "/admin/training-data", nil))

	if !strings.Contains(rec.Body.String(), `"logs":[]`) {
		t.Fatalf("expected logs:[] in %s", rec.Body.String())
	}
}

// TestList_ServiceError: list service error → 500 internal.
func TestList_ServiceError(t *testing.T) {
	svc := &fakeService{listErr: errors.New("db down")}
	h := newH(svc)

	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequest(http.MethodGet, "/admin/training-data", nil))

	if rec.Code != 500 {
		t.Fatalf("status = %d, want 500; body=%s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Review (PATCH)
// ---------------------------------------------------------------------------

// TestReview_OK: valid body → 200 { success: true }, args forwarded.
func TestReview_OK(t *testing.T) {
	svc := &fakeService{}
	h := newH(svc)

	rec := httptest.NewRecorder()
	req := routeWithID(
		httptest.NewRequest(http.MethodPatch, "/admin/training-data/abc-123",
			strings.NewReader(`{"quality":"approved"}`)),
		"abc-123",
	)
	h.Review(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	m := decodeBody(t, rec.Body.String())
	if m["success"] != true {
		t.Fatalf("success = %v, want true; body=%s", m["success"], rec.Body.String())
	}
	if svc.lastReviewID != "abc-123" {
		t.Fatalf("id = %q, want abc-123", svc.lastReviewID)
	}
	if svc.lastQuality != "approved" {
		t.Fatalf("quality = %q, want approved", svc.lastQuality)
	}
}

// TestReview_MissingQuality: valid JSON object without "quality" key → service
// called with empty string (Node parity: body.quality = undefined → "").
func TestReview_MissingQuality(t *testing.T) {
	svc := &fakeService{}
	h := newH(svc)

	rec := httptest.NewRecorder()
	req := routeWithID(
		httptest.NewRequest(http.MethodPatch, "/admin/training-data/xyz",
			strings.NewReader(`{}`)),
		"xyz",
	)
	h.Review(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200 (valid object, missing key = Node parity); body=%s", rec.Code, rec.Body.String())
	}
	if svc.lastQuality != "" {
		t.Fatalf("quality = %q, want empty string (absent key → zero value)", svc.lastQuality)
	}
}

// TestReview_MalformedBody: invalid JSON → 400 invalid-input "Invalid request body".
func TestReview_MalformedBody(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"bad json", `{bad`},
		{"array", `[]`},
		{"string", `"approved"`},
		{"number", `42`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &fakeService{}
			h := newH(svc)

			rec := httptest.NewRecorder()
			req := routeWithID(
				httptest.NewRequest(http.MethodPatch, "/admin/training-data/x",
					strings.NewReader(tc.body)),
				"x",
			)
			h.Review(rec, req)

			if rec.Code != 400 {
				t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
			}
			m := decodeBody(t, rec.Body.String())
			if m["code"] != "invalid-input" {
				t.Fatalf("code = %v, want invalid-input", m["code"])
			}
			if m["error"] != "Invalid request body" {
				t.Fatalf("error = %q, want \"Invalid request body\"", m["error"])
			}
		})
	}
}

// TestReview_ServiceError: service.Review returns error (e.g. malformed UUID in
// Postgres) → 500 internal. Mirrors Node where TypeORM throws on bad UUID.
func TestReview_ServiceError(t *testing.T) {
	svc := &fakeService{reviewErr: errors.New("invalid input syntax for type uuid")}
	h := newH(svc)

	rec := httptest.NewRecorder()
	req := routeWithID(
		httptest.NewRequest(http.MethodPatch, "/admin/training-data/not-a-uuid",
			strings.NewReader(`{"quality":"approved"}`)),
		"not-a-uuid",
	)
	h.Review(rec, req)

	if rec.Code != 500 {
		t.Fatalf("status = %d, want 500; body=%s", rec.Code, rec.Body.String())
	}
	m := decodeBody(t, rec.Body.String())
	if m["code"] != "internal" {
		t.Fatalf("code = %v, want internal", m["code"])
	}
}

// ---------------------------------------------------------------------------
// Export
// ---------------------------------------------------------------------------

// TestExport_Headers: GET /admin/training-data/export → 200 with exact
// Content-Type and Content-Disposition headers matching Node's res.setHeader
// calls.
func TestExport_Headers(t *testing.T) {
	svc := &fakeService{exportResult: `{"messages":[{"role":"system","content":"test"}]}`}
	h := newH(svc)

	rec := httptest.NewRecorder()
	h.Export(rec, httptest.NewRequest(http.MethodGet, "/admin/training-data/export", nil))

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/jsonl; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want application/jsonl; charset=utf-8", ct)
	}
	wantCD := "attachment; filename=draftright-training-data.jsonl"
	if cd := rec.Header().Get("Content-Disposition"); cd != wantCD {
		t.Fatalf("Content-Disposition = %q, want %q", cd, wantCD)
	}
}

// TestExport_Body: raw JSONL string is written verbatim (not JSON-encoded).
func TestExport_Body(t *testing.T) {
	jsonl := `{"messages":[{"role":"system","content":"Rewrite the following text in a formal tone. Return only the rewritten text."},{"role":"user","content":"hello"},{"role":"assistant","content":"greetings"}]}`
	svc := &fakeService{exportResult: jsonl}
	h := newH(svc)

	rec := httptest.NewRecorder()
	h.Export(rec, httptest.NewRequest(http.MethodGet, "/admin/training-data/export", nil))

	if rec.Body.String() != jsonl {
		t.Fatalf("body = %q, want %q", rec.Body.String(), jsonl)
	}
}

// TestExport_Empty: empty dataset → 200, empty body.
func TestExport_Empty(t *testing.T) {
	svc := &fakeService{exportResult: ""}
	h := newH(svc)

	rec := httptest.NewRecorder()
	h.Export(rec, httptest.NewRequest(http.MethodGet, "/admin/training-data/export", nil))

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("body = %q, want empty", rec.Body.String())
	}
}

// TestExport_ServiceError: export service error → 500 internal, no jsonl headers.
func TestExport_ServiceError(t *testing.T) {
	svc := &fakeService{exportErr: errors.New("db down")}
	h := newH(svc)

	rec := httptest.NewRecorder()
	h.Export(rec, httptest.NewRequest(http.MethodGet, "/admin/training-data/export", nil))

	if rec.Code != 500 {
		t.Fatalf("status = %d, want 500; body=%s", rec.Code, rec.Body.String())
	}
	// Node sets the jsonl headers only AFTER export() resolves, so an export
	// error carries none. Setting them before the error check would leak
	// Content-Disposition onto the 500 — a parity break.
	if cd := rec.Header().Get("Content-Disposition"); cd != "" {
		t.Fatalf("error response leaked Content-Disposition = %q, want empty", cd)
	}
	if ct := rec.Header().Get("Content-Type"); strings.HasPrefix(ct, "application/jsonl") {
		t.Fatalf("error response leaked Content-Type = %q (jsonl), want JSON envelope", ct)
	}
}
