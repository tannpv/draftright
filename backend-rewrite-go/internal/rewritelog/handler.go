// handler.go — HTTP edge for admin training-data routes.
// Mounts inside the admin group (jwtMW → RequireAdmin) wired in the router
// task; the handler itself does no auth.
//
//	GET   /admin/training-data/stats  → 200 { total, pending, approved, rejected }
//	GET   /admin/training-data        → 200 { logs, total }; empty logs → []
//	PATCH /admin/training-data/:id    → 200 { success: true }
//	GET   /admin/training-data/export → 200 raw JSONL body
//
// Node parity notes:
//   - List defaults: page=1, limit=20 (parseInt || default in Node controller)
//   - Export headers: Content-Type = "application/jsonl; charset=utf-8"
//     (Express res.send appends the charset), Content-Disposition =
//     "attachment; filename=draftright-training-data.jsonl"
//   - PATCH JSON decode error → 400 invalid-input "Invalid request body"
//   - PATCH service error (e.g. bad UUID → Postgres throws) → 500 internal
//     (Node: TypeORM update throws → AllExceptionsFilter 500)
//   - PATCH missing quality key → valid object, empty string forwarded (Node
//     body.quality = undefined, no DTO validation)
package rewritelog

import (
	"context"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// rewritelogService is the handler's consumer-side port; *Service satisfies
// it. Kept on the consumer side (CLAUDE.md guardrail) so tests inject a fake
// without a DB.
type rewritelogService interface {
	Stats(ctx context.Context) (StatsResult, error)
	ListPending(ctx context.Context, page, limit int) ([]RewriteLog, int, error)
	Review(ctx context.Context, id, quality string) error
	ExportJSONL(ctx context.Context) (string, error)
}

// Handler serves the admin training-data routes.
type Handler struct {
	svc rewritelogService
}

// NewHandler wires the service.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// statsResponse is the { total, pending, approved, rejected } body.
// Field order = JSON key order, matching Node's trainingDataStats return.
type statsResponse struct {
	Total    int `json:"total"`
	Pending  int `json:"pending"`
	Approved int `json:"approved"`
	Rejected int `json:"rejected"`
}

// listResponse is the { logs, total } body.
// Field order: logs first, total second — mirrors Node's findPending return.
type listResponse struct {
	Logs  []RewriteLog `json:"logs"`
	Total int          `json:"total"`
}

// reviewBody is the PATCH body: { quality }. No validation — Node accepts any
// string (no DTO/class-validator on this endpoint).
type reviewBody struct {
	Quality string `json:"quality"`
}

// reviewSuccessResponse is { success: true }.
type reviewSuccessResponse struct {
	Success bool `json:"success"`
}

// Stats handles GET /admin/training-data/stats → 200 { total, pending, approved, rejected }.
func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	result, err := h.svc.Stats(r.Context())
	if err != nil {
		shared.WriteError(w, r, "internal", err.Error())
		return
	}
	shared.WriteJSON(w, http.StatusOK, statsResponse{
		Total:    result.Total,
		Pending:  result.Pending,
		Approved: result.Approved,
		Rejected: result.Rejected,
	})
}

// List handles GET /admin/training-data → 200 { logs, total }.
//
// Query params:
//   - page: parseInt || 1  (Node: page ? parseInt(page) : 1)
//   - limit: parseInt || 20 (Node: limit ? parseInt(limit) : 20)
//
// A nil slice from the service is normalized to [] so JSON never emits null.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	page := 1
	if v, err := strconv.Atoi(q.Get("page")); err == nil {
		page = v
	}
	limit := 20
	if v, err := strconv.Atoi(q.Get("limit")); err == nil {
		limit = v
	}

	logs, total, err := h.svc.ListPending(r.Context(), page, limit)
	if err != nil {
		shared.WriteError(w, r, "internal", err.Error())
		return
	}
	if logs == nil {
		logs = []RewriteLog{}
	}
	shared.WriteJSON(w, http.StatusOK, listResponse{Logs: logs, Total: total})
}

// Review handles PATCH /admin/training-data/:id → 200 { success: true }.
//
// Body: { quality: string } — no validation, any string is forwarded.
//
// Error mapping:
//   - JSON decode error (malformed/non-object body) → 400 invalid-input
//     "Invalid request body"
//   - service.Review error (e.g. Postgres rejects bad UUID) → 500 internal
//
// A valid JSON object with quality absent is NOT an error: quality becomes ""
// and is forwarded to the service (Node parity — body.quality = undefined).
func (h *Handler) Review(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var body reviewBody
	if !shared.DecodeJSON(w, r, &body, shared.DecodeStrict) {
		return
	}

	if err := h.svc.Review(r.Context(), id, body.Quality); err != nil {
		shared.WriteError(w, r, "internal", err.Error())
		return
	}
	shared.WriteJSON(w, http.StatusOK, reviewSuccessResponse{Success: true})
}

// Export handles GET /admin/training-data/export → 200 raw JSONL body.
//
// Headers (byte-identical to Node's res.setHeader calls):
//
//	Content-Type: application/jsonl; charset=utf-8
//	Content-Disposition: attachment; filename=draftright-training-data.jsonl
//
// Body is the raw JSONL string from ExportJSONL. Empty approved set → 200
// with an empty body (not WriteJSON — raw write avoids the application/json
// envelope and preserves the Node Content-Type override).
//
// Headers are set ONLY after ExportJSONL succeeds. Node calls res.setHeader
// after `await service.export()`, so on an export error Node's response carries
// no jsonl headers (clean AllExceptionsFilter JSON). Setting them earlier here
// would leak Content-Disposition onto the 500 — a parity break.
func (h *Handler) Export(w http.ResponseWriter, r *http.Request) {
	jsonl, err := h.svc.ExportJSONL(r.Context())
	if err != nil {
		shared.WriteError(w, r, "internal", err.Error())
		return
	}

	// Node: res.setHeader('Content-Type','application/jsonl') + res.send(str).
	// Express's res.send appends the charset to a string body (setCharset), so
	// the wire value is "application/jsonl; charset=utf-8" — match it exactly
	// (caught by the shadow-gate header diff, #47).
	w.Header().Set("Content-Type", "application/jsonl; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=draftright-training-data.jsonl")
	if jsonl == "" {
		// Empty dataset → 200 empty body. WriteHeader locks the status to 200
		// explicitly (net/http default, but explicit is clearer).
		w.WriteHeader(http.StatusOK)
		return
	}
	_, _ = w.Write([]byte(jsonl)) //nolint:errcheck — same pattern as std-lib examples
}
