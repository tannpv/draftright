package shared_test

import (
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// phase4aRouter wires the three Phase 4a ancillary handlers to trivial
// stubs that write 200. If any route were mounted inside the RequireAuth
// group, an unauthenticated request would 401 before reaching the stub.
func phase4aRouter() stdhttp.Handler {
	stub := stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
		w.WriteHeader(stdhttp.StatusOK)
	})
	rt := &shared.Router{
		// A real verifier so the auth group (if a route leaked into it)
		// would genuinely reject — making a false PASS impossible.
		Verifier:         auth.NewVerifier(testJWTSecret),
		ImePacksManifest: stub,
		UpdatesLatest:    stub,
		ErrorsIngest:     stub,
	}
	return rt.Build()
}

// TC: Phase4a-router — all three ancillary routes are PUBLIC: reachable
// without a JWT (status must not be 401).
func TestRouter_Phase4a_PublicRoutes_NoJWT(t *testing.T) {
	router := phase4aRouter()

	cases := []struct {
		name   string
		method string
		path   string
	}{
		{"ime-packs manifest", stdhttp.MethodGet, "/ime-packs/manifest"},
		{"updates latest", stdhttp.MethodGet, "/updates/latest"},
		{"errors ingest", stdhttp.MethodPost, "/errors"},
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
