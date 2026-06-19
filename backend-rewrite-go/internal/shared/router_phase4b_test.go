package shared_test

import (
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// phase4bRouter wires the Phase 4b handlers to trivial stubs that write
// 200. A real verifier guards the auth group so any PUBLIC route that
// accidentally leaked into RequireAuth would genuinely 401 (no false PASS).
func phase4bRouter() stdhttp.Handler {
	stub := stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
		w.WriteHeader(stdhttp.StatusOK)
	})
	rt := &shared.Router{
		Verifier:        auth.NewVerifier(testJWTSecret),
		ExtractHandler:  stub,
		EmailWebhook:    stub,
		BugReportIngest: stub,
		FeedbackCreate:  stub,
		FeedbackList:    stub,
		FeedbackVote:    stub,
	}
	return rt.Build()
}

// TC: Phase4b-router-public — the five Phase 4b ingest routes are PUBLIC:
// reachable without a JWT (status must not be 401).
func TestRouter_Phase4b_PublicRoutes_NoJWT(t *testing.T) {
	router := phase4bRouter()

	cases := []struct {
		name   string
		method string
		path   string
	}{
		{"email webhook", stdhttp.MethodPost, "/webhooks/resend"},
		{"bug-report ingest", stdhttp.MethodPost, "/bug-reports"},
		{"feedback create", stdhttp.MethodPost, "/feedback"},
		{"feedback list", stdhttp.MethodGet, "/feedback"},
		{"feedback vote", stdhttp.MethodPost, "/feedback/abc-123/vote"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			require.NotEqual(t, stdhttp.StatusUnauthorized, rec.Code,
				"%s %s must be public (no 401); got %d", tc.method, tc.path, rec.Code)
			require.Equal(t, stdhttp.StatusOK, rec.Code,
				"%s %s should reach the stub handler", tc.method, tc.path)
		})
	}
}

// TC: Phase4b-router-extract-auth — POST /extract is JWT-gated (Node's
// @UseGuards(JwtAuthGuard)): no Authorization header → 401, the stub is
// never reached.
func TestRouter_Phase4b_Extract_RequiresJWT(t *testing.T) {
	router := phase4bRouter()

	req := httptest.NewRequest(stdhttp.MethodPost, "/extract", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, stdhttp.StatusUnauthorized, rec.Code,
		"POST /extract without a JWT must be 401 (auth-gated), got %d", rec.Code)
}
