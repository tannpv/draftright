package shared_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// TestAdmin4c2RoutesGuarded mounts all 25 Phase 4c-2 admin content/ops CRUD
// routes (every struct field set to a 200 handler) and asserts the admin
// guard chain is in front of each: no Authorization header → 401 (RequireAuth
// rejects); a valid but non-admin JWT (role="user") → 403 (RequireAdmin
// rejects). If a route were left unmounted it would 404 instead, failing here.
func TestAdmin4c2RoutesGuarded(t *testing.T) {
	r := &shared.Router{
		Verifier: auth.NewVerifier(testJWTSecret),

		AiProvidersList:      adminOKHandler(200),
		AiProvidersPaginated: adminOKHandler(200),
		AiProviderCreate:     adminOKHandler(200),
		AiProviderUpdate:     adminOKHandler(200),
		AiProviderDelete:     adminOKHandler(200),
		AiProviderTest:       adminOKHandler(200),

		AppSettingsGet:       adminOKHandler(200),
		AppSettingsPatch:     adminOKHandler(200),
		AppSettingsTestEmail: adminOKHandler(200),

		AdminPlansList:  adminOKHandler(200),
		AdminPlanCreate: adminOKHandler(200),
		AdminPlanUpdate: adminOKHandler(200),
		AdminPlanDelete: adminOKHandler(200),

		AdminUsersList:  adminOKHandler(200),
		AdminUserGet:    adminOKHandler(200),
		AdminUserUpdate: adminOKHandler(200),

		AdminAccountsList:  adminOKHandler(200),
		AdminAccountCreate: adminOKHandler(200),
		AdminAccountUpdate: adminOKHandler(200),
		AdminAccountDelete: adminOKHandler(200),

		AdminEmailLogs: adminOKHandler(200),

		AdminEmailTemplatesList:   adminOKHandler(200),
		AdminEmailTemplateUpdate:  adminOKHandler(200),
		AdminEmailTemplateReset:   adminOKHandler(200),
		AdminEmailTemplatePreview: adminOKHandler(200),
	}
	srv := httptest.NewServer(r.Build())
	defer srv.Close()

	routes := []struct {
		method, path string
	}{
		{http.MethodGet, "/admin/ai-providers"},
		{http.MethodGet, "/admin/ai-providers/paginated"},
		{http.MethodPost, "/admin/ai-providers"},
		{http.MethodPatch, "/admin/ai-providers/a1"},
		{http.MethodDelete, "/admin/ai-providers/a1"},
		{http.MethodPost, "/admin/ai-providers/a1/test"},
		{http.MethodGet, "/admin/settings"},
		{http.MethodPatch, "/admin/settings"},
		{http.MethodPost, "/admin/settings/test-email"},
		{http.MethodGet, "/admin/plans"},
		{http.MethodPost, "/admin/plans"},
		{http.MethodPatch, "/admin/plans/p1"},
		{http.MethodDelete, "/admin/plans/p1"},
		{http.MethodGet, "/admin/users"},
		{http.MethodGet, "/admin/users/u1"},
		{http.MethodPatch, "/admin/users/u1"},
		{http.MethodGet, "/admin/admin-users"},
		{http.MethodPost, "/admin/admin-users"},
		{http.MethodPatch, "/admin/admin-users/a1"},
		{http.MethodDelete, "/admin/admin-users/a1"},
		{http.MethodGet, "/admin/email-logs"},
		{http.MethodGet, "/admin/email-templates"},
		{http.MethodPatch, "/admin/email-templates/welcome"},
		{http.MethodDelete, "/admin/email-templates/welcome"},
		{http.MethodGet, "/admin/email-templates/welcome/preview"},
	}

	// No Authorization header → 401 (RequireAuth).
	for _, rt := range routes {
		req, _ := http.NewRequest(rt.method, srv.URL+rt.path, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", rt.method, rt.path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s %s no-token status = %d, want 401", rt.method, rt.path, resp.StatusCode)
		}
	}

	// Valid non-admin JWT → 403 (RequireAdmin).
	token := signTestToken(t, "non-admin-user-id")
	for _, rt := range routes {
		req, _ := http.NewRequest(rt.method, srv.URL+rt.path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", rt.method, rt.path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("%s %s non-admin status = %d, want 403", rt.method, rt.path, resp.StatusCode)
		}
	}
}
