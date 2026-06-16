package shared_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/shared"
)

func okHandler(code int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(code) })
}

func TestRouter_AdminLoginPublic(t *testing.T) {
	r := &shared.Router{
		Verifier:   auth.NewVerifier(testJWTSecret),
		AdminLogin: okHandler(201),
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
		AdminChangePassword: okHandler(201),
		AdminMe:             okHandler(200),
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
