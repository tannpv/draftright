// HTTP edge for GET /admin/email-templates. Returns a BARE JSON array merging
// the 6 builtin templates with DB customizations. Mounts inside the admin group
// (jwtMW → RequireAdmin) wired in the router task; the handler does no auth.
// Wiring into main.go is a LATER task.
//
//	GET /admin/email-templates → [ {key,label,variables,subject,html,customized,default_subject,default_html}, ... ] → 200
package email

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// adminTemplatesList is the handler's consumer-side port; *AdminTemplatesService
// satisfies it. Kept on the consumer side so tests inject a fake without a DB.
type adminTemplatesList interface {
	List(ctx context.Context) ([]TemplateRow, error)
	Update(ctx context.Context, key, subject, html string) error
	Reset(ctx context.Context, key string) error
	Preview(ctx context.Context, key string) (subject, html string, err error)
}

// okResponse is the body for PATCH/DELETE success: {"ok":true}.
type okResponse struct {
	OK bool `json:"ok"`
}

// previewResponse is the GET preview body; field order pins {subject, html}.
type previewResponse struct {
	Subject string `json:"subject"`
	HTML    string `json:"html"`
}

// updateTemplateBody is the PATCH request body; missing fields → "".
type updateTemplateBody struct {
	Subject string `json:"subject"`
	HTML    string `json:"html"`
}

// AdminTemplatesHandler serves the admin email-templates route.
type AdminTemplatesHandler struct {
	svc adminTemplatesList
}

// NewAdminTemplatesHandler wires the service.
func NewAdminTemplatesHandler(svc *AdminTemplatesService) *AdminTemplatesHandler {
	return &AdminTemplatesHandler{svc: svc}
}

// List handles GET /admin/email-templates → bare array of merged rows.
func (h *AdminTemplatesHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.List(r.Context())
	if err != nil {
		shared.WriteError(w, r, "internal", err.Error())
		return
	}
	if rows == nil {
		rows = []TemplateRow{}
	}
	shared.WriteJSON(w, http.StatusOK, rows)
}

// Update handles PATCH /admin/email-templates/:key — save a customization.
// Unknown builtin key → 404 not-found "Unknown template". Success → 200
// {"ok":true}.
func (h *AdminTemplatesHandler) Update(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	var body updateTemplateBody
	if r.Body != nil {
		// Ignore decode errors: missing/invalid body → zero-value fields, matching
		// Nest's lenient body binding (empty subject/html allowed).
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	if err := h.svc.Update(r.Context(), key, body.Subject, body.HTML); err != nil {
		if errors.Is(err, ErrUnknownTemplate) {
			shared.WriteError(w, r, "not-found", "Unknown template")
			return
		}
		shared.WriteError(w, r, "internal", err.Error())
		return
	}
	shared.WriteJSON(w, http.StatusOK, okResponse{OK: true})
}

// Reset handles DELETE /admin/email-templates/:key — drop the customization.
// Mirrors Node: NO key validation, idempotent (any key → 200 {"ok":true}).
func (h *AdminTemplatesHandler) Reset(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if err := h.svc.Reset(r.Context(), key); err != nil {
		shared.WriteError(w, r, "internal", err.Error())
		return
	}
	shared.WriteJSON(w, http.StatusOK, okResponse{OK: true})
}

// Preview handles GET /admin/email-templates/:key/preview — render with sample
// vars (DB-override-aware). Unknown key → 404. Success → 200 {subject, html}.
func (h *AdminTemplatesHandler) Preview(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	subject, html, err := h.svc.Preview(r.Context(), key)
	if err != nil {
		if errors.Is(err, ErrUnknownTemplate) {
			shared.WriteError(w, r, "not-found", "Unknown template")
			return
		}
		shared.WriteError(w, r, "internal", err.Error())
		return
	}
	shared.WriteJSON(w, http.StatusOK, previewResponse{Subject: subject, HTML: html})
}
