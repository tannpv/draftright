package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/shared"
)

type stubUser struct {
	id, email, role string
	err             error
}

func (s stubUser) ByID(context.Context, string) (UserRow, error) {
	return UserRow{ID: s.id, Email: s.email, Role: s.role}, s.err
}

func TestMe_ShapeMatchesNode(t *testing.T) {
	h := NewMeHandler(stubUser{id: "u-1", email: "a@b.com", role: "user"}, 0)

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

func TestMe_UserNotFoundReturns401(t *testing.T) {
	h := NewMeHandler(stubUser{err: errUserNotFound}, 0)

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	ctx := shared.ContextWithClaims(req.Context(), &auth.Claims{Sub: "u-gone", Email: "x@y.com", Role: "user"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req.WithContext(ctx))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (user-not-found)", rec.Code)
	}
}
