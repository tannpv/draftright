package core

import (
	"net/http"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// MeHandler serves GET /auth/me with the exact Node body shape:
// { id, email, role, flags: { use_go_backend } }. Identity comes
// straight from the verified JWT claims (matching Node's jwt.strategy +
// controller, which never hit the DB), so the handler does NO database
// access and never 401s a valid-but-deleted-user token. use_go_backend
// is computed from the configured ramp.
type MeHandler struct {
	rampPercent int
}

// NewMeHandler wires the GO_BACKEND_RAMP_PERCENT value the flag is
// bucketed against. Needs no DB — /auth/me echoes the JWT claims.
func NewMeHandler(rampPercent int) *MeHandler {
	return &MeHandler{rampPercent: rampPercent}
}

func (h *MeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	claims, ok := shared.ClaimsFromContext(r.Context())
	if !ok {
		// Route reached without RequireAuth wrapping it — a router
		// misconfiguration, not a client error.
		shared.WriteError(w, r, "internal", "auth context missing")
		return
	}

	shared.WriteJSON(w, http.StatusOK, map[string]any{
		"id":    claims.Sub,
		"email": claims.Email,
		"role":  claims.Role,
		"flags": map[string]any{
			"use_go_backend": UseGoBackend(claims.Sub, h.rampPercent),
		},
	})
}
