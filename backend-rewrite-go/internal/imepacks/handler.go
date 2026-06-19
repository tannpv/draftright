package imepacks

import (
	"net/http"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// Handler serves GET /ime-packs/manifest. Stateless.
type Handler struct{}

// NewHandler constructs the handler.
func NewHandler() *Handler { return &Handler{} }

// Manifest returns {"languages":[...]} with HTTP 200. Public, no auth.
func (h *Handler) Manifest(w http.ResponseWriter, r *http.Request) {
	shared.WriteJSON(w, http.StatusOK, map[string]any{"languages": Catalog()})
}
