package exttoken_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/tannpv/draftright-rewrite/internal/exttoken"
	"github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// withClaims stamps a user id into the request context the way RequireAuth
// would, so the handler can read it via shared.ClaimsFromContext.
func withClaims(r *http.Request, sub string) *http.Request {
	ctx := shared.ContextWithClaims(r.Context(), &auth.Claims{Sub: sub})
	return r.WithContext(ctx)
}

func newHandler(repo *fakeRepo) *exttoken.Handler {
	svc := exttoken.NewServiceWithGen(repo, func() (string, string, error) {
		return "dr_ext_PLAIN", "HASH", nil
	})
	return exttoken.NewHandler(svc)
}

const validUUID = "550e8400-e29b-41d4-a716-446655440000" // v4

func TestHandler_Mint_200(t *testing.T) {
	repo := &fakeRepo{insertRow: exttoken.TokenRow{ID: "row-id-1"}}
	h := newHandler(repo)

	body := strings.NewReader(`{"device_id":"` + validUUID + `","device_name":"iPhone"}`)
	req := withClaims(httptest.NewRequest(http.MethodPost, "/auth/extension-tokens", body), "user-1")
	rec := httptest.NewRecorder()
	h.Mint(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body not JSON: %v (%s)", err, rec.Body.String())
	}
	if got["token"] != "dr_ext_PLAIN" {
		t.Fatalf("token = %v, want dr_ext_PLAIN", got["token"])
	}
	if got["id"] != "row-id-1" {
		t.Fatalf("id = %v, want row-id-1", got["id"])
	}
	// Mint response is exactly {token, id} — no extra fields leak.
	if len(got) != 2 {
		t.Fatalf("mint body has %d keys, want 2: %s", len(got), rec.Body.String())
	}
	eq(t, repo.calls, []string{"RevokeActiveForDevice", "Insert"})
}

func TestHandler_Mint_ReadsUserIDFromClaims(t *testing.T) {
	repo := &fakeRepo{insertRow: exttoken.TokenRow{ID: "row-id-1"}}
	h := newHandler(repo)

	body := strings.NewReader(`{"device_id":"` + validUUID + `","device_name":"iPhone"}`)
	req := withClaims(httptest.NewRequest(http.MethodPost, "/auth/extension-tokens", body), "claims-user")
	rec := httptest.NewRecorder()
	h.Mint(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if repo.mintUserID != "claims-user" {
		t.Fatalf("Mint userID = %q, want claims-user (from claims context)", repo.mintUserID)
	}
}

func TestHandler_Mint_MissingClaims_Internal(t *testing.T) {
	repo := &fakeRepo{insertRow: exttoken.TokenRow{ID: "row-id-1"}}
	h := newHandler(repo)

	body := strings.NewReader(`{"device_id":"` + validUUID + `","device_name":"iPhone"}`)
	// No withClaims → no auth context.
	req := httptest.NewRequest(http.MethodPost, "/auth/extension-tokens", body)
	rec := httptest.NewRecorder()
	h.Mint(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandler_Mint_InvalidDeviceID_400(t *testing.T) {
	repo := &fakeRepo{}
	h := newHandler(repo)

	body := strings.NewReader(`{"device_id":"not-a-uuid","device_name":"iPhone"}`)
	req := withClaims(httptest.NewRequest(http.MethodPost, "/auth/extension-tokens", body), "user-1")
	rec := httptest.NewRecorder()
	h.Mint(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	var env map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if env["code"] != "invalid-input" {
		t.Fatalf("code = %v, want invalid-input", env["code"])
	}
	if env["error"] != "device_id must be a UUID" {
		t.Fatalf("error = %q, want %q", env["error"], "device_id must be a UUID")
	}
	if _, ok := env["request_id"]; !ok {
		t.Fatalf("envelope missing request_id: %s", rec.Body.String())
	}
	// Validation fails before the service is touched.
	if len(repo.calls) != 0 {
		t.Fatalf("expected no repo calls on validation failure, got %v", repo.calls)
	}
}

func TestHandler_Mint_EmptyBody_400(t *testing.T) {
	repo := &fakeRepo{}
	h := newHandler(repo)

	// Empty body → zero-valued fields → ValidateMint fails (matches Node's
	// ValidationPipe rejecting a missing body).
	req := withClaims(httptest.NewRequest(http.MethodPost, "/auth/extension-tokens", nil), "user-1")
	rec := httptest.NewRecorder()
	h.Mint(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	if len(repo.calls) != 0 {
		t.Fatalf("expected no repo calls, got %v", repo.calls)
	}
}

func TestHandler_List_200_Rows(t *testing.T) {
	repo := &fakeRepo{listRows: []exttoken.TokenRow{
		{ID: "tok-1", DeviceName: "iPhone", Scopes: []string{"rewrite"}},
	}}
	h := newHandler(repo)

	req := withClaims(httptest.NewRequest(http.MethodGet, "/auth/extension-tokens", nil), "user-1")
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var rows []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatalf("body not a JSON array: %v (%s)", err, rec.Body.String())
	}
	if len(rows) != 1 || rows[0]["id"] != "tok-1" {
		t.Fatalf("rows = %v", rows)
	}
	// Server-only fields never leak (TokenRow has no JSON tags for them).
	if _, ok := rows[0]["token_hash"]; ok {
		t.Fatalf("token_hash leaked: %s", rec.Body.String())
	}
	if _, ok := rows[0]["user_id"]; ok {
		t.Fatalf("user_id leaked: %s", rec.Body.String())
	}
	eq(t, repo.calls, []string{"ListActive"})
}

func TestHandler_List_200_EmptyArrayNotNull(t *testing.T) {
	// Repo returns a non-nil empty slice (T12 guarantee).
	repo := &fakeRepo{listRows: []exttoken.TokenRow{}}
	h := newHandler(repo)

	req := withClaims(httptest.NewRequest(http.MethodGet, "/auth/extension-tokens", nil), "user-1")
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	got := strings.TrimSpace(rec.Body.String())
	if got != "[]" {
		t.Fatalf("body = %q, want []", got)
	}
}

func TestHandler_List_200_NilSliceStillEmptyArray(t *testing.T) {
	// Defensive: even a nil slice must serialise as [] not null.
	repo := &fakeRepo{listRows: nil}
	h := newHandler(repo)

	req := withClaims(httptest.NewRequest(http.MethodGet, "/auth/extension-tokens", nil), "user-1")
	rec := httptest.NewRecorder()
	h.List(rec, req)

	got := strings.TrimSpace(rec.Body.String())
	if got != "[]" {
		t.Fatalf("body = %q, want [] (never null)", got)
	}
}

func TestHandler_Revoke_204(t *testing.T) {
	repo := &fakeRepo{}
	h := newHandler(repo)

	req := httptest.NewRequest(http.MethodDelete, "/auth/extension-tokens/tok-1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "tok-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = withClaims(req, "user-1")
	rec := httptest.NewRecorder()
	h.Revoke(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", rec.Code, rec.Body.String())
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("204 body not empty: %q", rec.Body.String())
	}
	eq(t, repo.calls, []string{"RevokeByID"})
}

func TestHandler_Revoke_UnknownID_Still204(t *testing.T) {
	// Repo returns nil even for an unknown id → idempotent 204.
	repo := &fakeRepo{}
	h := newHandler(repo)

	req := httptest.NewRequest(http.MethodDelete, "/auth/extension-tokens/does-not-exist", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "does-not-exist")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = withClaims(req, "user-1")
	rec := httptest.NewRecorder()
	h.Revoke(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", rec.Code, rec.Body.String())
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("204 body not empty: %q", rec.Body.String())
	}
}
