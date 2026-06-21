// admin_handler.go — HTTP edge for the admin bug-report triage routes (B1-B6).
// Mounts inside the admin group (jwtMW → RequireAdmin) wired in the router
// task; the handler itself does no auth.
//
//	GET    /admin/bug-reports                  → 200 { items, total }
//	GET    /admin/bug-reports/:id              → 200 entity; absent → 404 "bug report not found"
//	GET    /admin/bug-reports/:id/screenshot   → streams the image; 400 on missing path/file
//	PATCH  /admin/bug-reports/:id              → 200 entity; field validation → 400
//	DELETE /admin/bug-reports/:id              → 200 { success: true }
//	POST   /admin/bug-reports/:id/fix-proposal → 201 entity; no provider → 400
//
// Node parity notes (src/admin/admin.controller.ts):
//   - List: parseListQuery(q) + status/kind/target_platform as raw strings when
//     present. The service (findAllPaginated) only APPLIES a filter when the
//     value is in its allow-list; that validation is mirrored here when
//     building AdminListFilter (invalid → empty → no predicate).
//   - getBugReport(absent) → NotFoundException('bug report not found') → 404.
//   - screenshot: getScreenshotPath null → BadRequestException('no screenshot
//     for this report'); fs.access fail → BadRequestException('screenshot file
//     missing on disk'); Content-Type image/png when path ends .png else
//     image/jpeg; Content-Disposition inline filename sanitized [^\w.\-]→_.
//   - update → 200 entity; delete → 200 { success: true } (Nest @Delete default
//     200); fix-proposal is @Post → Nest default 201.
//   - ErrNoDefaultProvider → BadRequestException → 400 invalid-input.
package bugreports

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/tannpv/draftright-rewrite/internal/aiprovider"
	"github.com/tannpv/draftright-rewrite/internal/shared"
	"github.com/tannpv/draftright-rewrite/internal/shared/listquery"
)

// bugListSearchCols mirrors the Node findAllPaginated search columns.
var bugListSearchCols = []string{"br.description", "br.title", "br.user_email", "br.source"}

// bugListSortMap mirrors the Node findAllPaginated sort allow-list. The values
// are alias.field literals owned here — the SQL-injection guard for ORDER BY.
var bugListSortMap = map[string]string{
	"created_at": "br.created_at",
	"updated_at": "br.updated_at",
	"status":     "br.status",
	"source":     "br.source",
	"kind":       "br.kind",
	"vote_count": "br.vote_count",
}

// reScreenshotFilename mirrors Node's `.replace(/[^\w.\-]/g, '_')` on the
// Content-Disposition filename.
var reScreenshotFilename = regexp.MustCompile(`[^\w.\-]`)

// adminHandlerService is the handler's consumer-side port; *AdminService
// satisfies it. Kept on the consumer side so tests inject a fake without a DB.
type adminHandlerService interface {
	List(ctx context.Context, f AdminListFilter) ([]BugReportEntity, int, error)
	Get(ctx context.Context, id string) (BugReportEntity, error)
	Delete(ctx context.Context, id string) error
	Update(ctx context.Context, id string, p BugPatch) (BugReportEntity, error)
	SuggestFix(ctx context.Context, id string) (BugReportEntity, error)
	GetScreenshot(ctx context.Context, id string) (*Screenshot, error)
}

// AdminHandler serves the admin bug-report triage routes (B1-B6).
type AdminHandler struct {
	svc adminHandlerService
}

// NewAdminHandler wires the admin service.
func NewAdminHandler(svc *AdminService) *AdminHandler {
	return &AdminHandler{svc: svc}
}

// listResponse is the { rows, total } body — JSON key order matches Node's
// applyListQuery return ({ rows, total }). (Distinct from errors, which
// hand-rolls { items, total }.)
type listResponse struct {
	Rows  []BugReportEntity `json:"rows"`
	Total int               `json:"total"`
}

// successResponse is the { success: true } delete body.
type successResponse struct {
	Success bool `json:"success"`
}

// jsonStrPtr unmarshals a present field's raw JSON into a *string: a JSON
// string yields &s; an explicit null or a non-string (number/bool/object)
// yields nil. The caller already knows the key was PRESENT (it came from the
// decoded map), so a nil here means "present but not a usable string" — which
// the use case validates per Node's rules. Mirrors how `dto.x` reaches the Node
// service as null / NaN / the raw value.
func jsonStrPtr(rm json.RawMessage) *string {
	// JSON null unmarshals into a string as a no-op (leaves ""), so guard it
	// explicitly — a present-null must yield a nil pointer, not &"".
	if string(rm) == "null" {
		return nil
	}
	var s string
	if err := json.Unmarshal(rm, &s); err != nil {
		return nil
	}
	return &s
}

// jsonBoolPtr unmarshals a present field's raw JSON into a *bool: a JSON bool
// yields &b; null or a non-bool yields nil.
func jsonBoolPtr(rm json.RawMessage) *bool {
	if string(rm) == "null" {
		return nil
	}
	var b bool
	if err := json.Unmarshal(rm, &b); err != nil {
		return nil
	}
	return &b
}

// List handles GET /admin/bug-reports → 200 { items, total }. Builds the
// listquery WHERE/ORDER/page from the standard params, then layers the three
// custom filters only when their value is in the allow-list (mirroring the
// service, which silently ignores out-of-list filter values).
func (h *AdminHandler) List(w http.ResponseWriter, r *http.Request) {
	q := listquery.Parse(r.URL.Query())
	built := listquery.Build(q, bugListSearchCols, bugListSortMap, "br.created_at", "")

	f := AdminListFilter{Built: built}
	raw := r.URL.Query()
	if s := raw.Get("status"); s != "" && s != "all" && bugStatusValid(s) {
		f.Status = s
	}
	if k := raw.Get("kind"); k == "bug" || k == "feature" {
		f.Kind = k
	}
	if tp := raw.Get("target_platform"); tp != "" && bugTargetPlatformValid(tp) {
		f.TargetPlatform = tp
	}

	items, total, err := h.svc.List(r.Context(), f)
	if err != nil {
		shared.WriteError(w, r, "internal", err.Error())
		return
	}
	if items == nil {
		items = []BugReportEntity{}
	}
	shared.WriteJSON(w, http.StatusOK, listResponse{Rows: items, Total: total})
}

// Get handles GET /admin/bug-reports/:id → 200 entity; absent → 404.
func (h *AdminHandler) Get(w http.ResponseWriter, r *http.Request) {
	row, err := h.svc.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	shared.WriteJSON(w, http.StatusOK, row)
}

// Screenshot handles GET /admin/bug-reports/:id/screenshot. Streams the file
// with the image Content-Type + sanitized inline Content-Disposition. Null
// path → 400 "no screenshot for this report"; missing file → 400 "screenshot
// file missing on disk".
func (h *AdminHandler) Screenshot(w http.ResponseWriter, r *http.Request) {
	ss, err := h.svc.GetScreenshot(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	if ss == nil {
		shared.WriteError(w, r, "invalid-input", "no screenshot for this report")
		return
	}
	file, err := os.Open(ss.Path)
	if err != nil {
		shared.WriteError(w, r, "invalid-input", "screenshot file missing on disk")
		return
	}
	defer file.Close()

	contentType := "image/jpeg"
	if strings.HasSuffix(strings.ToLower(ss.Path), ".png") {
		contentType = "image/png"
	}
	safe := reScreenshotFilename.ReplaceAllString(ss.Filename, "_")
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", `inline; filename="`+safe+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, file)
}

// Patch handles PATCH /admin/bug-reports/:id → 200 entity. Per-field
// validation errors → 400 invalid-input; absent row → 404.
func (h *AdminHandler) Patch(w http.ResponseWriter, r *http.Request) {
	// Decode into a raw key map so the handler can tell an absent key from an
	// explicit null (Node's `!== undefined`). A typed-struct decode collapses
	// both to a nil pointer. See issue #39.
	raw := map[string]json.RawMessage{}
	if !shared.DecodeJSON(w, r, &raw, shared.DecodeOptional) {
		return
	}
	var p BugPatch
	if rm, ok := raw["status"]; ok {
		p.StatusSet, p.Status = true, jsonStrPtr(rm)
	}
	if rm, ok := raw["admin_notes"]; ok {
		p.AdminNotesSet, p.AdminNotes = true, jsonStrPtr(rm)
	}
	if rm, ok := raw["title"]; ok {
		p.TitleSet, p.Title = true, jsonStrPtr(rm)
	}
	if rm, ok := raw["target_platform"]; ok {
		p.TargetPlatformSet, p.TargetPlatform = true, jsonStrPtr(rm)
	}
	if rm, ok := raw["is_public"]; ok {
		p.IsPublic = jsonBoolPtr(rm)
	}
	row, err := h.svc.Update(r.Context(), chi.URLParam(r, "id"), p)
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	shared.WriteJSON(w, http.StatusOK, row)
}

// Delete handles DELETE /admin/bug-reports/:id → 200 { success: true }. Absent
// row → 404 (Node delete() does findById first).
func (h *AdminHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	shared.WriteJSON(w, http.StatusOK, successResponse{Success: true})
}

// FixProposal handles POST /admin/bug-reports/:id/fix-proposal → 201 entity.
// No default provider → 400; absent row → 404.
func (h *AdminHandler) FixProposal(w http.ResponseWriter, r *http.Request) {
	row, err := h.svc.SuggestFix(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	shared.WriteJSON(w, http.StatusCreated, row)
}

// writeServiceError maps use-case errors to the canonical envelope:
//   - ErrNotFound → 404 not-found "bug report not found" (NotFoundException).
//   - *BugValidationError → 400 invalid-input with its message.
//   - ErrNoDefaultProvider → 400 invalid-input with its message.
//   - anything else → 500 internal.
func (h *AdminHandler) writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	var ve *BugValidationError
	switch {
	case errors.Is(err, ErrNotFound):
		shared.WriteError(w, r, "not-found", ErrNotFound.Error())
	case errors.As(err, &ve):
		shared.WriteError(w, r, "invalid-input", ve.Msg)
	case errors.Is(err, aiprovider.ErrNoDefaultProvider):
		shared.WriteError(w, r, "invalid-input", aiprovider.ErrNoDefaultProvider.Error())
	default:
		shared.WriteError(w, r, "internal", err.Error())
	}
}
