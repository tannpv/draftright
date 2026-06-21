package exttoken

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// Handler is the HTTP edge for /auth/extension-tokens. It calls the Service
// only — no persistence or token logic leaks in here.
//
// Routes mirror the NestJS ExtensionTokenController (extension-token.controller.ts),
// all behind the session-JWT guard (mounted by the composition root):
//
//	POST   /auth/extension-tokens      → Mint   (200, @HttpCode(200))
//	GET    /auth/extension-tokens      → List   (200)
//	DELETE /auth/extension-tokens/{id} → Revoke (204, @HttpCode(204))
type Handler struct{ svc *Service }

// NewHandler wires the service.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// mintRequest is the decoded MintExtensionTokenDto. JSON field names match the
// Node DTO (snake_case) exactly.
type mintRequest struct {
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
}

// mintResponse is Node's mint() return shape — { token, id } — byte-for-byte.
// The plaintext token is exposed here only; the controller returns the service
// result verbatim.
type mintResponse struct {
	Token string `json:"token"`
	ID    string `json:"id"`
}

// Mint: POST /auth/extension-tokens (JWT) → 200. Body {device_id, device_name}.
// Validation failures → 400 invalid-input (parity envelope). A malformed or
// empty body is treated as a validation failure — the empty device_id/name
// fail ValidateMint, matching Node's ValidationPipe rejecting a missing body.
func (h *Handler) Mint(w http.ResponseWriter, r *http.Request) {
	claims, ok := shared.ClaimsFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, "internal", "auth context missing")
		return
	}

	var req mintRequest
	// A decode error (malformed/empty body) leaves req zero-valued; the empty
	// fields then fail ValidateMint, which is exactly what Node emits when the
	// body is absent (ValidationPipe runs against undefined fields). So we do
	// not short-circuit on decode error — we fall through to validation. A
	// literal-null body still short-circuits with a 400 (Node parity).
	if !shared.DecodeJSON(w, r, &req, shared.DecodeLenient) {
		return
	}

	if msg := ValidateMint(req.DeviceID, req.DeviceName); msg != "" {
		shared.WriteError(w, r, "invalid-input", msg)
		return
	}

	res, err := h.svc.Mint(r.Context(), claims.Sub, req.DeviceID, req.DeviceName)
	if err != nil {
		shared.WriteError(w, r, "internal", "mint failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, mintResponse{Token: res.Token, ID: res.ID})
}

// List: GET /auth/extension-tokens (JWT) → 200, bare JSON array of TokenRow.
// TokenRow already omits token_hash/user_id (Node strips them). An empty list
// serialises as [] (the repo returns a non-nil empty slice), never null.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	claims, ok := shared.ClaimsFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, "internal", "auth context missing")
		return
	}
	rows, err := h.svc.List(r.Context(), claims.Sub)
	if err != nil {
		shared.WriteError(w, r, "internal", "list failed")
		return
	}
	if rows == nil {
		// Defensive: guarantee [] over null even if a future Store returns nil.
		rows = []TokenRow{}
	}
	shared.WriteJSON(w, http.StatusOK, rows)
}

// Revoke: DELETE /auth/extension-tokens/{id} (JWT) → 204 No Content, always.
// Idempotent — unknown/already-revoked ids still return 204 (Node @HttpCode(204)
// with a service that filters on user_id and never errors on a no-op update).
func (h *Handler) Revoke(w http.ResponseWriter, r *http.Request) {
	claims, ok := shared.ClaimsFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, "internal", "auth context missing")
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.svc.Revoke(r.Context(), id, claims.Sub); err != nil {
		shared.WriteError(w, r, "internal", "revoke failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
