package adminauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	auth "github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/shared"
)

type fakeSvc struct {
	login   LoginResult
	loginEr error
	cpEr    error
	prof    AdminUser
	profEr  error
}

func (f *fakeSvc) Login(context.Context, string, string) (LoginResult, error) {
	return f.login, f.loginEr
}
func (f *fakeSvc) ChangePassword(context.Context, string, string, string) error { return f.cpEr }
func (f *fakeSvc) GetProfile(context.Context, string) (AdminUser, error)        { return f.prof, f.profEr }

func TestLoginHandler_Success(t *testing.T) {
	h := &Handler{svc: &fakeSvc{login: LoginResult{
		AccessToken: "acc", RefreshToken: "ref",
		User: AdminUser{ID: "a1", Email: "a@b.c", Name: "Root", Role: "admin"},
	}}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/auth/login", strings.NewReader(`{"email":"a@b.c","password":"pw"}`))
	h.Login(rec, req)

	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	want := `{"access_token":"acc","refresh_token":"ref","user":{"id":"a1","email":"a@b.c","name":"Root","role":"admin"}}`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Errorf("body = %s\nwant   %s", got, want)
	}
}

func TestLoginHandler_BadCreds(t *testing.T) {
	h := &Handler{svc: &fakeSvc{loginEr: ErrInvalidCredentials}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/auth/login", strings.NewReader(`{"email":"x","password":"y"}`))
	h.Login(rec, req)
	if rec.Code != 401 {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if b := rec.Body.String(); !strings.Contains(b, `"error":"Invalid credentials"`) || !strings.Contains(b, `"code":"invalid-token"`) {
		t.Errorf("body = %s", b)
	}
}

func TestChangePasswordHandler_Success(t *testing.T) {
	h := &Handler{svc: &fakeSvc{}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/auth/change-password", strings.NewReader(`{"current_password":"a","new_password":"b"}`))
	req = req.WithContext(shared.ContextWithClaims(req.Context(), &auth.Claims{Sub: "a1", IsAdminFlag: true}))
	h.ChangePassword(rec, req)
	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `{"success":true}` {
		t.Errorf("body = %s, want {\"success\":true}", got)
	}
}

func TestMeHandler_Success(t *testing.T) {
	created := time.Date(2026, 1, 2, 3, 4, 5, 678000000, time.UTC)
	updated := time.Date(2026, 1, 2, 3, 4, 6, 0, time.UTC)
	h := &Handler{svc: &fakeSvc{prof: AdminUser{
		ID: "a1", Email: "a@b.c", Name: "Root", IsActive: true, Role: "admin",
		PasswordHash: "$2a$10$secret", CreatedAt: created, UpdatedAt: updated,
	}}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/auth/me", nil)
	req = req.WithContext(shared.ContextWithClaims(req.Context(), &auth.Claims{Sub: "a1", IsAdminFlag: true}))
	h.Me(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "password_hash") || strings.Contains(body, "secret") {
		t.Fatalf("password_hash leaked: %s", body)
	}
	want := `{"id":"a1","email":"a@b.c","name":"Root","is_active":true,"role":"admin","created_at":"2026-01-02T03:04:05.678Z","updated_at":"2026-01-02T03:04:06.000Z"}`
	if got := strings.TrimSpace(body); got != want {
		t.Errorf("body = %s\nwant   %s", got, want)
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &probe); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
}

func TestMeHandler_AdminGone(t *testing.T) {
	h := &Handler{svc: &fakeSvc{profEr: ErrUnauthorized}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/auth/me", nil)
	req = req.WithContext(shared.ContextWithClaims(req.Context(), &auth.Claims{Sub: "a1", IsAdminFlag: true}))
	h.Me(rec, req)
	if rec.Code != 401 {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if b := rec.Body.String(); !strings.Contains(b, `"error":"Unauthorized"`) || !strings.Contains(b, `"code":"invalid-token"`) {
		t.Errorf("body = %s", b)
	}
}
