// HTTP edge for GET /admin/email-templates. Returns a BARE JSON array merging
// the 6 builtin templates with DB customizations. Mounts inside the admin group
// (jwtMW → RequireAdmin) wired in the router task; the handler does no auth.
// Wiring into main.go is a LATER task.
//
//	GET /admin/email-templates → [ {key,label,variables,subject,html,customized,default_subject,default_html}, ... ] → 200
package email

import (
	"context"
	"net/http"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// adminTemplatesList is the handler's consumer-side port; *AdminTemplatesService
// satisfies it. Kept on the consumer side so tests inject a fake without a DB.
type adminTemplatesList interface {
	List(ctx context.Context) ([]TemplateRow, error)
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
