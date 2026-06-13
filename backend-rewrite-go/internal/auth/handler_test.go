package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/auth"
	platauth "github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/shared"
	"github.com/tannpv/draftright-rewrite/internal/subscription"
	"github.com/tannpv/draftright-rewrite/internal/user"
)

type users struct {
	u    map[string]user.User
	byID map[string]user.User
}

func (s users) ByEmail(_ context.Context, e string) (user.User, error) {
	if v, ok := s.u[e]; ok {
		return v, nil
	}
	return user.User{}, user.ErrNotFound
}
func (s users) ByID(_ context.Context, id string) (user.User, error) {
	if v, ok := s.byID[id]; ok {
		return v, nil
	}
	return user.User{}, user.ErrNotFound
}
func (s users) UpdatePasswordHash(context.Context, string, string) error { return nil }
func (s users) DeleteAccount(context.Context, string) error              { return nil }

type subs struct{ s *subscription.AccountSub }

func (s subs) ActiveByUser(context.Context, string) (*subscription.AccountSub, error) {
	return s.s, nil
}

type usg struct{ n int }

func (u usg) CountToday(context.Context, string) (int, error) { return u.n, nil }

type realTTL struct{}

func (realTTL) TokenTTLs(context.Context) (time.Duration, time.Duration, error) {
	return 15 * time.Minute, 90 * 24 * time.Hour, nil
}

func TestLoginHandler_201AndBody(t *testing.T) {
	hash, _ := shared.HashPassword("pw")
	svc := auth.NewService(
		users{u: map[string]user.User{"a@b.com": {ID: "u1", Email: "a@b.com", Name: "Al", IsActive: true, PasswordHash: hash}}},
		subs{}, usg{}, realTTL{}, "access", "refresh",
	)
	h := auth.NewHandler(svc)
	body, _ := json.Marshal(map[string]string{"email": "a@b.com", "password": "pw"})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Login(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	var out struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		User         struct {
			ID    string `json:"id"`
			Email string `json:"email"`
			Name  string `json:"name"`
		} `json:"user"`
	}
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out.AccessToken == "" || out.User.ID != "u1" {
		t.Fatalf("bad body: %s", rec.Body.String())
	}
}

func TestLoginHandler_401Envelope(t *testing.T) {
	svc := auth.NewService(users{u: map[string]user.User{}}, subs{}, usg{}, realTTL{}, "access", "refresh")
	h := auth.NewHandler(svc)
	body, _ := json.Marshal(map[string]string{"email": "no@b.com", "password": "x"})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Login(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	var out struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Error != "Invalid credentials" || out.Code != "invalid-token" {
		t.Fatalf("envelope = %s", rec.Body.String())
	}
}

func TestAccountHandler_Shape(t *testing.T) {
	svc := auth.NewService(
		users{byID: map[string]user.User{"u1": {ID: "u1", Email: "a@b.com", Name: "Al", EmailVerified: true}}},
		subs{}, usg{n: 3}, realTTL{}, "access", "refresh",
	)
	h := auth.NewHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/auth/account", nil)
	ctx := shared.ContextWithClaims(req.Context(), &platauth.Claims{Sub: "u1", Email: "a@b.com", Role: "user"})
	rec := httptest.NewRecorder()
	h.Account(rec, req.WithContext(ctx))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var out struct {
		ID            string    `json:"id"`
		EmailVerified bool      `json:"email_verified"`
		Subscription  *struct{} `json:"subscription"`
	}
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out.ID != "u1" || !out.EmailVerified || out.Subscription != nil {
		t.Fatalf("bad account body: %s", rec.Body.String())
	}
}

func TestDeleteAccountHandler_200(t *testing.T) {
	svc := auth.NewService(
		users{byID: map[string]user.User{"u1": {ID: "u1"}}},
		subs{}, usg{}, realTTL{}, "access", "refresh",
	)
	h := auth.NewHandler(svc)
	req := httptest.NewRequest(http.MethodDelete, "/auth/account", nil)
	ctx := shared.ContextWithClaims(req.Context(), &platauth.Claims{Sub: "u1"})
	rec := httptest.NewRecorder()
	h.DeleteAccount(rec, req.WithContext(ctx))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rec.Code)
	}
	var out struct {
		Deleted bool `json:"deleted"`
	}
	json.Unmarshal(rec.Body.Bytes(), &out)
	if !out.Deleted {
		t.Fatalf("bad delete body: %s", rec.Body.String())
	}
}
