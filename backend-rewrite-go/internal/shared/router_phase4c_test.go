package shared_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/shared"
)

func adminOKHandler(code int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(code) })
}

func TestRouter_AdminLoginPublic(t *testing.T) {
	r := &shared.Router{
		Verifier:   auth.NewVerifier(testJWTSecret),
		AdminLogin: adminOKHandler(201),
	}
	srv := httptest.NewServer(r.Build())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/admin/auth/login", "application/json", nil)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Errorf("login status = %d, want 201 (public, no JWT)", resp.StatusCode)
	}
}

func TestRouter_AdminRoutesGuarded(t *testing.T) {
	r := &shared.Router{
		Verifier:            auth.NewVerifier(testJWTSecret),
		AdminChangePassword: adminOKHandler(201),
		AdminMe:             adminOKHandler(200),
	}
	srv := httptest.NewServer(r.Build())
	defer srv.Close()

	for _, p := range []struct {
		method, path string
	}{
		{http.MethodGet, "/admin/auth/me"},
		{http.MethodPost, "/admin/auth/change-password"},
	} {
		req, _ := http.NewRequest(p.method, srv.URL+p.path, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", p.method, p.path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != 401 {
			t.Errorf("%s %s status = %d, want 401 without JWT", p.method, p.path, resp.StatusCode)
		}
	}
}

// TestRouter_AdminRoutesRejectNonAdmin proves that RequireAdmin is actually
// chained in the router: a VALID JWT carrying a non-admin claim (role="user",
// isAdmin=false) must be rejected with 403, not passed through to the handler.
// Without this test, dropping RequireAdmin from the router would leave the
// 401 tests above still passing.
func TestRouter_AdminRoutesRejectNonAdmin(t *testing.T) {
	r := &shared.Router{
		Verifier: auth.NewVerifier(testJWTSecret),
		AdminMe:  adminOKHandler(200),
	}
	srv := httptest.NewServer(r.Build())
	defer srv.Close()

	// signTestToken (defined in router_observability_test.go) mints an HS256
	// token signed with testJWTSecret and role="user" — a valid but non-admin
	// claim. The Verifier above uses the same secret, so the token passes
	// RequireAuth; RequireAdmin must then reject it with 403.
	token := signTestToken(t, "non-admin-user-id")

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/admin/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403 (valid JWT, non-admin → RequireAdmin rejects)", resp.StatusCode)
	}
}
