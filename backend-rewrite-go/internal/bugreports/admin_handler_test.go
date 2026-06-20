package bugreports

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/tannpv/draftright-rewrite/internal/aiprovider"
)

// fakeHandlerSvc satisfies adminHandlerService; fields capture args and drive
// return values.
type fakeHandlerSvc struct {
	listItems  []BugReportEntity
	listTotal  int
	listErr    error
	lastFilter AdminListFilter

	getEntity BugReportEntity
	getErr    error

	delErr error

	updEntity BugReportEntity
	updErr    error
	lastPatch BugPatch

	fixEntity BugReportEntity
	fixErr    error

	ss    *Screenshot
	ssErr error
}

func (f *fakeHandlerSvc) List(_ context.Context, filter AdminListFilter) ([]BugReportEntity, int, error) {
	f.lastFilter = filter
	return f.listItems, f.listTotal, f.listErr
}
func (f *fakeHandlerSvc) Get(context.Context, string) (BugReportEntity, error) {
	return f.getEntity, f.getErr
}
func (f *fakeHandlerSvc) Delete(context.Context, string) error { return f.delErr }
func (f *fakeHandlerSvc) Update(_ context.Context, _ string, p BugPatch) (BugReportEntity, error) {
	f.lastPatch = p
	return f.updEntity, f.updErr
}
func (f *fakeHandlerSvc) SuggestFix(context.Context, string) (BugReportEntity, error) {
	return f.fixEntity, f.fixErr
}
func (f *fakeHandlerSvc) GetScreenshot(context.Context, string) (*Screenshot, error) {
	return f.ss, f.ssErr
}

func newRouter(h *AdminHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/admin/bug-reports", h.List)
	r.Get("/admin/bug-reports/{id}", h.Get)
	r.Get("/admin/bug-reports/{id}/screenshot", h.Screenshot)
	r.Patch("/admin/bug-reports/{id}", h.Patch)
	r.Delete("/admin/bug-reports/{id}", h.Delete)
	r.Post("/admin/bug-reports/{id}/fix-proposal", h.FixProposal)
	return r
}

func do(t *testing.T, h http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rd *strings.Reader
	if body != "" {
		rd = strings.NewReader(body)
	} else {
		rd = strings.NewReader("")
	}
	req := httptest.NewRequest(method, target, rd)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestList_OK(t *testing.T) {
	svc := &fakeHandlerSvc{listItems: []BugReportEntity{{ID: "a"}}, listTotal: 1}
	rec := do(t, newRouter(NewAdminHandler(nil).withSvc(svc)), "GET", "/admin/bug-reports?search=x&kind=bug&status=new&target_platform=mobile", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"rows"`) {
		t.Fatalf("list body must use key \"rows\" not \"items\": %s", rec.Body.String())
	}
	var got listResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Total != 1 || len(got.Rows) != 1 {
		t.Fatalf("body=%s", rec.Body.String())
	}
	if svc.lastFilter.Status != "new" || svc.lastFilter.Kind != "bug" || svc.lastFilter.TargetPlatform != "mobile" {
		t.Fatalf("filters not applied: %+v", svc.lastFilter)
	}
}

func TestList_IgnoresInvalidFilters(t *testing.T) {
	svc := &fakeHandlerSvc{}
	do(t, newRouter(NewAdminHandler(nil).withSvc(svc)), "GET", "/admin/bug-reports?kind=junk&status=all&target_platform=toaster", "")
	if svc.lastFilter.Status != "" || svc.lastFilter.Kind != "" || svc.lastFilter.TargetPlatform != "" {
		t.Fatalf("invalid filters should be dropped: %+v", svc.lastFilter)
	}
}

func TestGet_404(t *testing.T) {
	svc := &fakeHandlerSvc{getErr: ErrNotFound}
	rec := do(t, newRouter(NewAdminHandler(nil).withSvc(svc)), "GET", "/admin/bug-reports/x", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"bug report not found"`) || !strings.Contains(rec.Body.String(), `"not-found"`) {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestPatch_ValidationError(t *testing.T) {
	svc := &fakeHandlerSvc{updErr: &BugValidationError{Msg: "status must be one of: new, reviewing, fix_proposed, resolved, wont_fix"}}
	rec := do(t, newRouter(NewAdminHandler(nil).withSvc(svc)), "PATCH", "/admin/bug-reports/x", `{"status":"nope"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"invalid-input"`) {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestPatch_OK(t *testing.T) {
	svc := &fakeHandlerSvc{updEntity: BugReportEntity{ID: "x", Status: "reviewing"}}
	rec := do(t, newRouter(NewAdminHandler(nil).withSvc(svc)), "PATCH", "/admin/bug-reports/x", `{"status":"reviewing","is_public":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.lastPatch.Status == nil || *svc.lastPatch.Status != "reviewing" {
		t.Fatalf("status not decoded")
	}
	if svc.lastPatch.IsPublic == nil || !*svc.lastPatch.IsPublic {
		t.Fatalf("is_public not decoded")
	}
}

// TestPatch_ExplicitNullSetsPresenceFlag pins issue #39 at the HTTP edge: an
// explicit JSON null is PRESENT (Set flag true, value pointer nil), distinct
// from an absent key. Node's `!== undefined` enters the branch for null.
func TestPatch_ExplicitNullSetsPresenceFlag(t *testing.T) {
	svc := &fakeHandlerSvc{updEntity: BugReportEntity{ID: "x"}}
	do(t, newRouter(NewAdminHandler(nil).withSvc(svc)), "PATCH", "/admin/bug-reports/x", `{"admin_notes":null}`)
	if !svc.lastPatch.AdminNotesSet {
		t.Fatalf("explicit null admin_notes should set presence flag")
	}
	if svc.lastPatch.AdminNotes != nil {
		t.Fatalf("explicit null admin_notes value should be nil pointer")
	}
}

// TestPatch_AbsentKeyLeavesFlagUnset: an absent key must NOT set the presence
// flag (the other half of the #39 distinction).
func TestPatch_AbsentKeyLeavesFlagUnset(t *testing.T) {
	svc := &fakeHandlerSvc{updEntity: BugReportEntity{ID: "x"}}
	do(t, newRouter(NewAdminHandler(nil).withSvc(svc)), "PATCH", "/admin/bug-reports/x", `{"status":"reviewing"}`)
	if svc.lastPatch.AdminNotesSet {
		t.Fatalf("absent admin_notes must leave presence flag unset")
	}
}

func TestDelete_OK(t *testing.T) {
	svc := &fakeHandlerSvc{}
	rec := do(t, newRouter(NewAdminHandler(nil).withSvc(svc)), "DELETE", "/admin/bug-reports/x", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"success":true}` {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestDelete_404(t *testing.T) {
	svc := &fakeHandlerSvc{delErr: ErrNotFound}
	rec := do(t, newRouter(NewAdminHandler(nil).withSvc(svc)), "DELETE", "/admin/bug-reports/x", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestFixProposal_201(t *testing.T) {
	svc := &fakeHandlerSvc{fixEntity: BugReportEntity{ID: "x", Status: "fix_proposed"}}
	rec := do(t, newRouter(NewAdminHandler(nil).withSvc(svc)), "POST", "/admin/bug-reports/x/fix-proposal", "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestFixProposal_NoProvider400(t *testing.T) {
	svc := &fakeHandlerSvc{fixErr: aiprovider.ErrNoDefaultProvider}
	rec := do(t, newRouter(NewAdminHandler(nil).withSvc(svc)), "POST", "/admin/bug-reports/x/fix-proposal", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "No default AI provider configured") {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestScreenshot_NoScreenshot400(t *testing.T) {
	svc := &fakeHandlerSvc{ss: nil}
	rec := do(t, newRouter(NewAdminHandler(nil).withSvc(svc)), "GET", "/admin/bug-reports/x/screenshot", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "no screenshot for this report") {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestScreenshot_FileMissing400(t *testing.T) {
	svc := &fakeHandlerSvc{ss: &Screenshot{Path: "/nonexistent/zzz.png", Filename: "z.png"}}
	rec := do(t, newRouter(NewAdminHandler(nil).withSvc(svc)), "GET", "/admin/bug-reports/x/screenshot", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "screenshot file missing on disk") {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestScreenshot_StreamsPNGWithSanitizedFilename(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "shot.png")
	if err := os.WriteFile(p, []byte("PNGDATA"), 0o600); err != nil {
		t.Fatal(err)
	}
	svc := &fakeHandlerSvc{ss: &Screenshot{Path: p, Filename: "my file (1).png"}}
	rec := do(t, newRouter(NewAdminHandler(nil).withSvc(svc)), "GET", "/admin/bug-reports/x/screenshot", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("content-type=%q", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); cd != `inline; filename="my_file__1_.png"` {
		t.Fatalf("content-disposition=%q", cd)
	}
	if rec.Body.String() != "PNGDATA" {
		t.Fatalf("body=%q", rec.Body.String())
	}
}

// withSvc swaps the handler's service to a fake for tests (NewAdminHandler
// takes a concrete *AdminService; the port lets tests inject a fake).
func (h *AdminHandler) withSvc(s adminHandlerService) *AdminHandler {
	return &AdminHandler{svc: s}
}
