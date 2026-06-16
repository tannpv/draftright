package shared

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	auth "github.com/tannpv/draftright-rewrite/internal/platform/auth"
)

func TestRequireAdmin(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) })

	cases := []struct {
		name   string
		claims *auth.Claims
		status int
	}{
		{"isAdmin flag passes", &auth.Claims{Sub: "a", IsAdminFlag: true}, 204},
		{"role admin passes", &auth.Claims{Sub: "a", Role: "admin"}, 204},
		{"customer rejected", &auth.Claims{Sub: "u", Role: "user"}, http.StatusForbidden},
		{"no role rejected", &auth.Claims{Sub: "u"}, http.StatusForbidden},
		{"missing claims rejected", nil, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/admin/auth/me", nil)
			if tc.claims != nil {
				req = req.WithContext(ContextWithClaims(req.Context(), tc.claims))
			}
			RequireAdmin(ok).ServeHTTP(rec, req)
			if rec.Code != tc.status {
				t.Errorf("status = %d, want %d", rec.Code, tc.status)
			}
		})
	}
}

func TestRequireAdmin_ForbiddenEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/auth/me", nil)
	req = req.WithContext(ContextWithClaims(req.Context(), &auth.Claims{Sub: "u", Role: "user"}))
	RequireAdmin(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != 403 {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if !strings.Contains(body, `"error":"Admin access required"`) || !strings.Contains(body, `"code":"forbidden"`) {
		t.Errorf("body = %s, want Admin access required / forbidden", body)
	}
}
