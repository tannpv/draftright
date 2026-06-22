package imepacks

import (
	"net/http"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// Handler serves GET /ime-packs/manifest. Holds the CDN base for pack URLs.
type Handler struct{ base string }

// NewHandler constructs the handler. base is config.IMEPackBase — the CDN
// origin pack URLs are built from.
func NewHandler(base string) *Handler { return &Handler{base: base} }

// Manifest returns {"languages":[...]} with HTTP 200. Public, no auth.
func (h *Handler) Manifest(w http.ResponseWriter, r *http.Request) {
	shared.WriteJSON(w, http.StatusOK, map[string]any{"languages": Catalog(h.base)})
}
