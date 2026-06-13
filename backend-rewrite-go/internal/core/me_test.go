package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/shared"
)

func TestMe_ShapeMatchesNode(t *testing.T) {
	h := NewMeHandler(0)

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	ctx := shared.ContextWithClaims(req.Context(), &auth.Claims{Sub: "u-1", Email: "a@b.com", Role: "user"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req.WithContext(ctx))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Role  string `json:"role"`
		Flags struct {
			UseGoBackend bool `json:"use_go_backend"`
		} `json:"flags"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if body.ID != "u-1" || body.Email != "a@b.com" || body.Role != "user" {
		t.Fatalf("body = %+v, want id/email/role of u-1", body)
	}
	if body.Flags.UseGoBackend != false {
		t.Fatalf("use_go_backend = true, want false at ramp 0")
	}
}

// TestMe_FlagFollowsRamp proves the flag wiring uses the ramp: at ramp
// 100 every user buckets in, so use_go_backend must be true.
func TestMe_FlagFollowsRamp(t *testing.T) {
	h := NewMeHandler(100)

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	ctx := shared.ContextWithClaims(req.Context(), &auth.Claims{Sub: "u-1", Email: "a@b.com", Role: "user"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req.WithContext(ctx))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Flags struct {
			UseGoBackend bool `json:"use_go_backend"`
		} `json:"flags"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if !body.Flags.UseGoBackend {
		t.Fatalf("use_go_backend = false, want true at ramp 100")
	}
}
