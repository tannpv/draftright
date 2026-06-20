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

	// Ordered struct, NOT map[string]any: a map marshals keys sorted
	// (email,flags,id,role), diverging byte-for-byte from Node's
	// declaration order id,email,role,flags. Field order here = wire
	// order = Node order. Locked by the shadow-gate raw-body fixture
	// (#56, divergence found by the #47 raw-body mode).
	shared.WriteJSON(w, http.StatusOK, meResponse{
		ID:    claims.Sub,
		Email: claims.Email,
		Role:  claims.Role,
		Flags: meFlags{UseGoBackend: UseGoBackend(claims.Sub, h.rampPercent)},
	})
}

// meResponse is the GET /auth/me body in Node declaration order:
// { id, email, role, flags: { use_go_backend } }.
type meResponse struct {
	ID    string  `json:"id"`
	Email string  `json:"email"`
	Role  string  `json:"role"`
	Flags meFlags `json:"flags"`
}

// meFlags is the server-controlled rollout state nested under `flags`.
type meFlags struct {
	UseGoBackend bool `json:"use_go_backend"`
}
