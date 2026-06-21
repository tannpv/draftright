// admin_handler.go — HTTP edge for the admin error-triage routes (E1–E6).
// Mounts inside the admin group (jwtMW → RequireAdmin) wired in the router
// task; the handler itself does no auth.
//
//	GET    /admin/errors                  → 200 { items, total }
//	GET    /admin/errors/:id              → 200 entity; absent → 400 "not found"
//	PATCH  /admin/errors/:id              → 200 entity; missing status → 400 "status required"
//	DELETE /admin/errors/:id              → 200 { id, deleted } (idempotent, NOT 404)
//	POST   /admin/errors/:id/suggest-fix  → 201 entity
//	POST   /admin/errors/run-ai-cron      → 201 { ok: true }
//
// Node parity notes (src/admin/admin.controller.ts):
//   - List query parse: `limit ? Number(limit) : undefined`, then
//     `Math.min(limit || 50, 200)` and `offset || 0`. Number("") / Number(NaN)
//     are falsy → fall to the default. status: present → Number(status); empty
//     query value parses to 0 (Node Number("") = 0).
//   - getOne(null) → BadRequestException('not found') → 400 invalid-input.
//   - PATCH: `body.status === undefined` → BadRequestException('status required').
//     status:0 is present, so it passes the required check.
//   - suggest-fix is an @Post → Nest default 201; run-ai-cron likewise 201.
//   - ErrNoDefaultProvider → BadRequestException → 400 invalid-input with its message.
package errreport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/tannpv/draftright-rewrite/internal/aiprovider"
	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// adminHandlerService is the handler's consumer-side port; *AdminService
// satisfies it. Kept on the consumer side (CLAUDE.md guardrail) so tests
// inject a fake without a DB.
type adminHandlerService interface {
	List(ctx context.Context, f AdminListFilter) ([]ErrorReportEntity, int, error)
	Get(ctx context.Context, id string) (ErrorReportEntity, error)
	Delete(ctx context.Context, id string) (bool, error)
	SetStatusRaw(ctx context.Context, id string, statusRaw json.RawMessage, resolvedBy string) (ErrorReportEntity, error)
	SuggestFix(ctx context.Context, id string) (ErrorReportEntity, error)
}

// adminHandlerCron is the consumer-side port for the E6 manual trigger;
// *Cron satisfies it.
type adminHandlerCron interface {
	RunOnce(ctx context.Context) CronResult
}

// AdminHandler serves the admin error-triage routes (E1–E6).
type AdminHandler struct {
	svc  adminHandlerService
	cron adminHandlerCron
}

// NewAdminHandler wires the admin service + the fix-proposal cron (E6).
func NewAdminHandler(svc *AdminService, cron *Cron) *AdminHandler {
	return &AdminHandler{svc: svc, cron: cron}
}

// listResponse is the { items, total } body. Field order = JSON key order,
// matching Node's ErrorsService.list return.
type listResponse struct {
	Items []ErrorReportEntity `json:"items"`
	Total int                 `json:"total"`
}

// deleteResponse is the { id, deleted } body (Node ErrorsService.deleteOne).
type deleteResponse struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// patchBody is the PATCH inline body: { status, resolved_by }. status is a
// json.RawMessage (not a typed int) so the handler can distinguish "absent"
// (nil → 400 "status required") from any present JSON value — including a
// non-numeric string or null, which Node forwards raw to Postgres for coercion
// (→ 500), not a Go-side 400. An explicit `null` arrives as []byte("null")
// (non-nil), so it passes the required check, exactly like Node.
type patchBody struct {
	Status     json.RawMessage `json:"status"`
	ResolvedBy string          `json:"resolved_by"`
}

// okResponse is the { ok: true } run-ai-cron body.
type okResponse struct {
	OK bool `json:"ok"`
}

// List handles GET /admin/errors → 200 { items, total }.
//
// Query parse mirrors the Node controller + service: limit defaults to 50,
// caps at 200; offset defaults to 0; both fall to their default on a missing,
// empty, zero, or unparseable value (Node `Number(x)` falsy → `|| default`).
// platform / severity are string filters (empty = absent). status is parsed
// when present.
func (h *AdminHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	f := AdminListFilter{
		Limit:  numberOrDefaultCapped(q.Get("limit"), 50, 200),
		Offset: numberOrDefault(q.Get("offset"), 0),
	}
	if v := q.Get("platform"); v != "" {
		f.Platform = &v
	}
	if v := q.Get("severity"); v != "" {
		f.Severity = &v
	}
	if q.Has("status") {
		// Node: `status !== undefined ? Number(status) : undefined`. An empty
		// query value (`?status=`) → Number("") = 0.
		s := parseNumber(q.Get("status"))
		f.Status = &s
	}

	items, total, err := h.svc.List(r.Context(), f)
	if err != nil {
		shared.WriteError(w, r, "internal", err.Error())
		return
	}
	if items == nil {
		items = []ErrorReportEntity{}
	}
	shared.WriteJSON(w, http.StatusOK, listResponse{Items: items, Total: total})
}

// Get handles GET /admin/errors/:id → 200 entity. Absent → 400 "not found".
func (h *AdminHandler) Get(w http.ResponseWriter, r *http.Request) {
	row, err := h.svc.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	shared.WriteJSON(w, http.StatusOK, row)
}

// Patch handles PATCH /admin/errors/:id → 200 entity. Missing status →
// 400 "status required". Absent row → 400 "not found".
func (h *AdminHandler) Patch(w http.ResponseWriter, r *http.Request) {
	var body patchBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
		shared.WriteError(w, r, "invalid-input", "Invalid request body")
		return
	}
	if body.Status == nil {
		shared.WriteError(w, r, "invalid-input", "status required")
		return
	}
	row, err := h.svc.SetStatusRaw(r.Context(), chi.URLParam(r, "id"), body.Status, body.ResolvedBy)
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	shared.WriteJSON(w, http.StatusOK, row)
}

// Delete handles DELETE /admin/errors/:id → 200 { id, deleted }. Idempotent:
// an already-gone id resolves with deleted:false (NOT 404).
func (h *AdminHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	deleted, err := h.svc.Delete(r.Context(), id)
	if err != nil {
		shared.WriteError(w, r, "internal", err.Error())
		return
	}
	shared.WriteJSON(w, http.StatusOK, deleteResponse{ID: id, Deleted: deleted})
}

// SuggestFix handles POST /admin/errors/:id/suggest-fix → 201 entity.
// No default provider → 400 "No default AI provider configured"; absent row →
// 400 "not found".
func (h *AdminHandler) SuggestFix(w http.ResponseWriter, r *http.Request) {
	row, err := h.svc.SuggestFix(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	shared.WriteJSON(w, http.StatusCreated, row)
}

// RunCron handles POST /admin/errors/run-ai-cron → 201 { ok: true }. The cron's
// RunOnce never errors (it catches per-id failures internally), mirroring Node's
// `await this.fixProposalCron.run(); return { ok: true };`.
func (h *AdminHandler) RunCron(w http.ResponseWriter, r *http.Request) {
	h.cron.RunOnce(r.Context())
	shared.WriteJSON(w, http.StatusCreated, okResponse{OK: true})
}

// writeServiceError maps use-case errors to the canonical envelope. Both
// ErrNotFound and ErrNoDefaultProvider are BadRequestException(400) in Node →
// 400 invalid-input with the exact message; anything else → 500 internal.
func (h *AdminHandler) writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		shared.WriteError(w, r, "invalid-input", ErrNotFoundMsg)
	case errors.Is(err, aiprovider.ErrNoDefaultProvider):
		shared.WriteError(w, r, "invalid-input", aiprovider.ErrNoDefaultProvider.Error())
	default:
		shared.WriteError(w, r, "internal", err.Error())
	}
}

// parseNumber mirrors JS Number(x): an empty or unparseable string yields 0
// here (Number("") = 0). Only used where the surrounding logic already decided
// the value is present.
func parseNumber(s string) int {
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return 0
}

// numberOrDefault mirrors Node `x || def` over `Number(x)`: a missing, empty,
// zero, or unparseable value falls to def (all falsy in JS).
func numberOrDefault(s string, def int) int {
	if v, err := strconv.Atoi(s); err == nil && v != 0 {
		return v
	}
	return def
}

// numberOrDefaultCapped applies numberOrDefault then Math.min(_, cap), matching
// the service's `Math.min(limit || 50, 200)`.
func numberOrDefaultCapped(s string, def, ceiling int) int {
	v := numberOrDefault(s, def)
	if v > ceiling {
		return ceiling
	}
	return v
}
