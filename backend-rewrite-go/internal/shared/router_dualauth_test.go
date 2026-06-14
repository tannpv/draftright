package shared_test

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/adapter/memory"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/domain"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/transport"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/usecase"
	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// stubExt resolves any raw token to a fixed userID (or an error). Used
// to drive the dual-auth path end-to-end through the real router +
// rewrite handler.
type stubExt struct {
	uid string
	err error
}

func (s stubExt) Verify(_ context.Context, _ string) (string, error) {
	return s.uid, s.err
}

func dualAuthRouter(t *testing.T, ext shared.ExtVerifier, user *domain.User) stdhttp.Handler {
	t.Helper()
	deps := usecase.RewriteDeps{
		Users:     memory.NewUserRepo(user),
		Provider:  memory.NewProvider("memory-test", []string{"Hi"}),
		RateLimit: memory.NewRateLimiter(),
		Now:       func() time.Time { return time.Unix(1700000000, 0) },
	}
	rt := &shared.Router{
		Verifier: auth.NewVerifier(testJWTSecret),
		Rewrite: &transport.RewriteHandler{
			Deps: deps,
			Now:  deps.Now,
		},
		ExtVerifier: ext,
	}
	return rt.Build()
}

// A valid dr_ext_ token, resolved by the ext service to the user's UUID,
// drives /v1/rewrite to 200 — proving the injected Claims{Sub} reach the
// rewrite handler exactly like a JWT would.
func TestRouter_DualAuth_ExtToken_DrivesRewrite(t *testing.T) {
	uid, err := uuid.Parse("44444444-4444-4444-4444-444444444444")
	require.NoError(t, err)
	user := &domain.User{
		ID:    domain.UserID(uid),
		Email: "ext@draftright.test",
		Role:  "user",
		Plan:  domain.Plan{Name: "pro", DailyLimit: 1000},
	}
	r := dualAuthRouter(t, stubExt{uid: uid.String()}, user)

	req := httptest.NewRequest(stdhttp.MethodPost, "/v1/rewrite",
		strings.NewReader(`{"text":"hi","tone":"polished"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer dr_ext_livetoken")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, stdhttp.StatusOK, rec.Code, "ext-token rewrite should succeed; body=%s", rec.Body.String())
}

// A JWT token still drives /v1/rewrite to 200 through the dual-auth
// mount — byte-identical to the JWT-only path.
func TestRouter_DualAuth_JWT_StillWorks(t *testing.T) {
	uid, err := uuid.Parse("55555555-5555-5555-5555-555555555555")
	require.NoError(t, err)
	user := &domain.User{
		ID:    domain.UserID(uid),
		Email: "jwt@draftright.test",
		Role:  "user",
		Plan:  domain.Plan{Name: "pro", DailyLimit: 1000},
	}
	// ext stub would error if ever consulted — proves the JWT branch
	// never touches it.
	r := dualAuthRouter(t, stubExt{err: context.Canceled}, user)

	req := httptest.NewRequest(stdhttp.MethodPost, "/v1/rewrite",
		strings.NewReader(`{"text":"hi","tone":"polished"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+signTestToken(t, uid.String()))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, stdhttp.StatusOK, rec.Code, "jwt rewrite should succeed; body=%s", rec.Body.String())
}

// A bad JWT on the dual-auth mount returns the unchanged RequireAuth
// 401 envelope: {error:"Unauthorized", code:"invalid-token"}.
func TestRouter_DualAuth_BadJWT_Unauthorized(t *testing.T) {
	r := dualAuthRouter(t, stubExt{err: context.Canceled}, nil)

	req := httptest.NewRequest(stdhttp.MethodPost, "/v1/rewrite",
		strings.NewReader(`{"text":"hi","tone":"polished"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer not.a.valid.jwt")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, stdhttp.StatusUnauthorized, rec.Code)
	var env map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	require.Equal(t, "Unauthorized", env["error"])
	require.Equal(t, "invalid-token", env["code"])
}
