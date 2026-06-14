package plans

import (
	"net/http"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// Handler is the HTTP edge for GET /plans.
type Handler struct{ svc *Service }

// NewHandler wires the service.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// List: GET /plans → 200, JSON array of raw plan entities. Public.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.ListActive(r.Context())
	if err != nil {
		shared.WriteError(w, r, "internal", "plans failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, list)
}
