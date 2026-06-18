package shared_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// TestAdmin4c3RoutesGuarded mounts all 11 Phase 4c-3 admin reporting routes
// (every struct field set to a 200 handler) and asserts the admin guard chain
// is in front of each: no Authorization header → 401 (RequireAuth rejects); a
// valid but non-admin JWT (role="user") → 403 (RequireAdmin rejects). If a
// route were left unmounted it would 404 instead, failing the test.
//
// Note: GET /admin/training-data/stats and GET /admin/training-data/export are
// static paths; PATCH /admin/training-data/{id} is a wildcard — chi routes
// static paths over wildcards, so all three are distinct and reachable. The
// table below exercises all three.
func TestAdmin4c3RoutesGuarded(t *testing.T) {
	r := &shared.Router{
		Verifier: auth.NewVerifier(testJWTSecret),

		AdminStats:        adminOKHandler(200),
		AdminAnalytics:    adminOKHandler(200),
		AdminTransactions: adminOKHandler(200),

		TrainingDataStats:  adminOKHandler(200),
		TrainingDataList:   adminOKHandler(200),
		TrainingDataReview: adminOKHandler(200),
		TrainingDataExport: adminOKHandler(200),

		AdminPaymentsStats:  adminOKHandler(200),
		AdminPaymentsList:   adminOKHandler(200),
		AdminPaymentConfirm: adminOKHandler(200),
		AdminPaymentRefund:  adminOKHandler(200),
	}
	srv := httptest.NewServer(r.Build())
	defer srv.Close()

	routes := []struct {
		method, path string
	}{
		{http.MethodGet, "/admin/stats"},
		{http.MethodGet, "/admin/analytics"},
		{http.MethodGet, "/admin/transactions"},
		{http.MethodGet, "/admin/training-data/stats"},
		{http.MethodGet, "/admin/training-data"},
		{http.MethodPatch, "/admin/training-data/t1"},
		{http.MethodGet, "/admin/training-data/export"},
		{http.MethodGet, "/admin/payments/stats"},
		{http.MethodGet, "/admin/payments"},
		{http.MethodPost, "/admin/payments/p1/confirm"},
		{http.MethodPost, "/admin/payments/p1/refund"},
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
