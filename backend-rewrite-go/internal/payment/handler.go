package payment

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// Handler is the HTTP edge for /payment read routes. Calls the Service only.
//
//	GET /payment/methods      → Methods (200, public)
//	GET /payment/status/:ref  → Status  (200, public)
//	GET /payment/history      → History (200, JWT)
type Handler struct{ svc *Service }

// NewHandler wires the service.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// MarshalJSON pins the status response shape. Not-found collapses to the single
// {status:"not_found"} key; otherwise the full object in Node controller order.
func (v StatusView) MarshalJSON() ([]byte, error) {
	if v.notFound {
		return []byte(`{"status":"not_found"}`), nil
	}
	return json.Marshal(struct {
		Status        string  `json:"status"`
		Method        string  `json:"method"`
		Amount        int     `json:"amount"`
		Currency      string  `json:"currency"`
		ReferenceCode string  `json:"reference_code"`
		PlanName      *string `json:"plan_name"`
		CompletedAt   *string `json:"completed_at"`
		ExpiresAt     *string `json:"expires_at"`
	}{
		Status: v.Status, Method: v.Method, Amount: v.Amount, Currency: v.Currency,
		ReferenceCode: v.ReferenceCode, PlanName: v.PlanName,
		CompletedAt: v.CompletedAt, ExpiresAt: v.ExpiresAt,
	})
}

// Methods: GET /payment/methods (public) → {methods:[...]}.
func (h *Handler) Methods(w http.ResponseWriter, r *http.Request) {
	methods, err := h.svc.EnabledMethods(r.Context())
	if err != nil {
		shared.WriteError(w, r, "internal", "methods failed")
		return
	}
	if methods == nil {
		methods = []string{}
	}
	shared.WriteJSON(w, http.StatusOK, map[string][]string{"methods": methods})
}

// Status: GET /payment/status/{ref} (public) → status view (or not_found).
func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	ref := chi.URLParam(r, "ref")
	v, err := h.svc.Status(r.Context(), ref)
	if err != nil {
		shared.WriteError(w, r, "internal", "status failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, v)
}

// History: GET /payment/history (JWT) → bare JSON array of payments. Empty → [].
func (h *Handler) History(w http.ResponseWriter, r *http.Request) {
	claims, ok := shared.ClaimsFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, "internal", "auth context missing")
		return
	}
	rows, err := h.svc.History(r.Context(), claims.Sub)
	if err != nil {
		shared.WriteError(w, r, "internal", "history failed")
		return
	}
	if rows == nil {
		rows = []PaymentRow{}
	}
	shared.WriteJSON(w, http.StatusOK, rows)
}
