package subscription

import (
	"net/http"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// Handler is the HTTP edge for /subscription.
type Handler struct{ svc *Service }

// NewHandler wires the service.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Get: GET /subscription (JWT) → 200.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	claims, ok := shared.ClaimsFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, "internal", "auth context missing")
		return
	}
	view, err := h.svc.GetSubscription(r.Context(), claims.Sub, time.Now())
	if err != nil {
		shared.WriteError(w, r, "internal", "subscription failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, view)
}

// VerifyReceipt: POST /subscription/verify-receipt (JWT) → 201. Body ignored.
func (h *Handler) VerifyReceipt(w http.ResponseWriter, r *http.Request) {
	claims, ok := shared.ClaimsFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, "internal", "auth context missing")
		return
	}
	view, err := h.svc.VerifyReceipt(r.Context(), claims.Sub)
	if err != nil {
		shared.WriteError(w, r, "internal", "verify-receipt failed")
		return
	}
	shared.WriteJSON(w, http.StatusCreated, view)
}
