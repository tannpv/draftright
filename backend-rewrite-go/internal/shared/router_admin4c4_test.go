package shared_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// TestAdmin4c4RoutesGuarded mounts all 19 Phase 4c-4 admin triage routes
// (every struct field set to a 200 handler) and asserts the admin guard chain
// is in front of each: no Authorization header → 401 (RequireAuth rejects); a
// valid but non-admin JWT (role="user") → 403 (RequireAdmin rejects). If a
// route were left unmounted it would 404 instead, failing the test.
//
// Static paths (run-ai-cron, inbox/counts) sit before {id} wildcards; chi
// routes static over wildcard so all are distinct and reachable. The table
// below exercises every one.
func TestAdmin4c4RoutesGuarded(t *testing.T) {
	r := &shared.Router{
		Verifier: auth.NewVerifier(testJWTSecret),

		AdminErrorsList:    adminOKHandler(200),
		AdminErrorGet:      adminOKHandler(200),
		AdminErrorPatch:    adminOKHandler(200),
		AdminErrorDelete:   adminOKHandler(200),
		AdminErrorSuggest:  adminOKHandler(200),
		AdminErrorsRunCron: adminOKHandler(200),

		AdminBugList:        adminOKHandler(200),
		AdminBugGet:         adminOKHandler(200),
		AdminBugScreenshot:  adminOKHandler(200),
		AdminBugPatch:       adminOKHandler(200),
		AdminBugDelete:      adminOKHandler(200),
		AdminBugFixProposal: adminOKHandler(200),

		AdminInboxCounts: adminOKHandler(200),
		AdminInbox:       adminOKHandler(200),

		AdminReleasesList:  adminOKHandler(200),
		AdminReleaseUpsert: adminOKHandler(200),
		AdminReleaseDelete: adminOKHandler(200),
		AdminPolicyUpsert:  adminOKHandler(200),

		AdminGrantSub: adminOKHandler(200),
	}
	srv := httptest.NewServer(r.Build())
	defer srv.Close()

	routes := []struct {
		method, path string
	}{
		{http.MethodPost, "/admin/errors/run-ai-cron"},
		{http.MethodGet, "/admin/errors"},
		{http.MethodGet, "/admin/errors/e1"},
		{http.MethodPatch, "/admin/errors/e1"},
		{http.MethodDelete, "/admin/errors/e1"},
		{http.MethodPost, "/admin/errors/e1/suggest-fix"},
		{http.MethodGet, "/admin/bug-reports"},
		{http.MethodGet, "/admin/bug-reports/b1"},
		{http.MethodGet, "/admin/bug-reports/b1/screenshot"},
		{http.MethodPatch, "/admin/bug-reports/b1"},
		{http.MethodDelete, "/admin/bug-reports/b1"},
		{http.MethodPost, "/admin/bug-reports/b1/fix-proposal"},
		{http.MethodGet, "/admin/inbox/counts"},
		{http.MethodGet, "/admin/inbox"},
		{http.MethodGet, "/admin/releases"},
		{http.MethodPost, "/admin/releases"},
		{http.MethodDelete, "/admin/releases/mac/direct"},
		{http.MethodPost, "/admin/release-policies"},
		{http.MethodPost, "/admin/subscriptions/grant"},
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
