package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/tannpv/draftright-rewrite/internal/adapter/memory"
	"github.com/tannpv/draftright-rewrite/internal/domain"
	internalhttp "github.com/tannpv/draftright-rewrite/internal/http"
	"github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/usecase"
)

// Test-only shared secret; production wires JWT_SECRET from env.
const testJWTSecret = "test-secret-must-match-nestjs"

// signTestToken signs an HS256 JWT with the test secret. Mirrors the
// shape the NestJS backend issues (sub = user uuid, role = "user").
// Centralised so every test request is signed the same way (Rule #1).
func signTestToken(t *testing.T, sub string) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  sub,
		"role": "user",
		"exp":  time.Now().Add(time.Hour).Unix(),
		"iat":  time.Now().Unix(),
	})
	signed, err := tok.SignedString([]byte(testJWTSecret))
	require.NoError(t, err)
	return signed
}

// fixture bundles the wiring every test needs. Constructor accepts an
// override hook so individual tests can swap one piece (e.g. force
// the provider into an error path) without re-stating the rest.
type fixture struct {
	users    *memory.UserRepo
	provider *memory.Provider
	limiter  *memory.RateLimiter
	user     *domain.User
	router   stdhttp.Handler
}

// newFixture wires the standard happy-path deps: a user with a
// generous plan, a token-stream provider that emits "Hello world",
// and a no-error rate limiter. Mutator funcs let a test tweak any of
// these in place before calling .build().
func newFixture(t *testing.T, mutators ...func(*fixture)) *fixture {
	t.Helper()
	uid, err := uuid.Parse("11111111-1111-1111-1111-111111111111")
	require.NoError(t, err)

	user := &domain.User{
		ID:    domain.UserID(uid),
		Email: "tester@draftright.test",
		Role:  "user",
		Plan: domain.Plan{
			ID:         uuid.New(),
			Name:       "pro",
			DailyLimit: 1000,
		},
		UsedToday: 0,
	}
	fx := &fixture{
		users:    memory.NewUserRepo(user),
		provider: memory.NewProvider("memory-test", []string{"Hello", " ", "world"}),
		limiter:  memory.NewRateLimiter(),
		user:     user,
	}
	for _, m := range mutators {
		m(fx)
	}
	verifier := auth.NewVerifier(testJWTSecret)
	deps := usecase.RewriteDeps{
		Users:     fx.users,
		Provider:  fx.provider,
		RateLimit: fx.limiter,
		Now:       func() time.Time { return time.Unix(1700000000, 0) },
	}
	fx.router = (&internalhttp.Router{
		Verifier: verifier,
		Rewrite: &internalhttp.RewriteHandler{
			Deps: deps,
			Now:  deps.Now,
		},
	}).Build()
	return fx
}

// doRequest fires a request against the wired router and returns the
// recorder. accept="" → JSON path; accept="text/event-stream" → SSE.
func (fx *fixture) doRequest(t *testing.T, body, accept string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(stdhttp.MethodPost, "/v1/rewrite", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+signTestToken(t, fx.user.ID.String()))
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	rec := httptest.NewRecorder()
	fx.router.ServeHTTP(rec, req)
	return rec
}

// ---------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------

func TestRewrite_JSON_HappyPath(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	rec := fx.doRequest(t, `{"text":"hi there","tone":"polished"}`, "")
	require.Equal(t, stdhttp.StatusOK, rec.Code)

	var got map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, "Hello world", got["text"])
	require.Equal(t, "rewrite-go", got["service"])

	// Usage log written once via memory UserRepo.
	require.Equal(t, 1, fx.users.LogsLen())
	logs := fx.users.Logs()
	require.Equal(t, domain.TonePolished, logs[0].Tone)
}

func TestRewrite_SSE_HappyPath(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	rec := fx.doRequest(t, `{"text":"hi","tone":"polished"}`, "text/event-stream")
	require.Equal(t, stdhttp.StatusOK, rec.Code)
	require.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))

	body := rec.Body.String()
	// Three delta events.
	require.Equal(t, 3, strings.Count(body, `"delta":`),
		"expected 3 delta events, got body: %s", body)
	// Terminal usage + DONE.
	require.Contains(t, body, `"usage"`)
	require.Contains(t, body, "data: [DONE]")
}

func TestRewrite_400_OnInvalidJSON(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	rec := fx.doRequest(t, `{not-json`, "")
	require.Equal(t, stdhttp.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), `"invalid-input"`)
}

func TestRewrite_400_OnUnknownTone(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	rec := fx.doRequest(t, `{"text":"hi","tone":"sarcastic"}`, "")
	require.Equal(t, stdhttp.StatusBadRequest, rec.Code)
}

func TestRewrite_401_OnMissingAuth(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	req := httptest.NewRequest(stdhttp.MethodPost, "/v1/rewrite",
		strings.NewReader(`{"text":"hi","tone":"polished"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	fx.router.ServeHTTP(rec, req)
	require.Equal(t, stdhttp.StatusUnauthorized, rec.Code)
}

func TestRewrite_401_OnUnknownUser(t *testing.T) {
	t.Parallel()
	fx := newFixture(t, func(f *fixture) {
		// Repo returns ErrUserNotFound — represents a JWT for a
		// deleted account.
		f.users = memory.NewUserRepo(nil)
	})
	rec := fx.doRequest(t, `{"text":"hi","tone":"polished"}`, "")
	require.Equal(t, stdhttp.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), `"user-not-found"`)
}

func TestRewrite_402_OnQuotaExceeded(t *testing.T) {
	t.Parallel()
	uid, _ := uuid.Parse("22222222-2222-2222-2222-222222222222")
	user := &domain.User{
		ID:        domain.UserID(uid),
		Email:     "exhausted@draftright.test",
		Role:      "user",
		Plan:      domain.Plan{ID: uuid.New(), Name: "free", DailyLimit: 5},
		UsedToday: 5,
	}
	fx := newFixture(t, func(f *fixture) {
		f.users = memory.NewUserRepo(user)
		f.user = user
	})
	rec := fx.doRequest(t, `{"text":"hi","tone":"polished"}`, "")
	require.Equal(t, stdhttp.StatusPaymentRequired, rec.Code)
	require.Contains(t, rec.Body.String(), `"quota-exceeded"`)
}

func TestRewrite_429_OnRateLimited(t *testing.T) {
	t.Parallel()
	fx := newFixture(t, func(f *fixture) {
		f.limiter = memory.NewRateLimiter().WithError(domain.ErrRateLimited)
	})
	rec := fx.doRequest(t, `{"text":"hi","tone":"polished"}`, "")
	require.Equal(t, stdhttp.StatusTooManyRequests, rec.Code)
	require.Contains(t, rec.Body.String(), `"rate-limited"`)
}

func TestRewrite_SSE_ProviderErrorMidStream(t *testing.T) {
	t.Parallel()
	fx := newFixture(t, func(f *fixture) {
		f.provider = memory.NewProvider("memory-test",
			[]string{"Hello"}).WithFinalError(errors.New("upstream blew up"))
	})
	rec := fx.doRequest(t, `{"text":"hi","tone":"polished"}`, "text/event-stream")
	require.Equal(t, stdhttp.StatusOK, rec.Code) // headers already flushed
	body := rec.Body.String()
	require.Contains(t, body, "event: error")
	require.Contains(t, body, "data: [DONE]")
}

func TestRewrite_502_OnProviderErrorJSON(t *testing.T) {
	t.Parallel()
	// Provider streams nothing then errors — JSON path surfaces 502.
	fx := newFixture(t, func(f *fixture) {
		f.provider = memory.NewProvider("memory-test", nil).
			WithFinalError(fmt.Errorf("wrap: %w", domain.ErrProviderFailed))
	})
	rec := fx.doRequest(t, `{"text":"hi","tone":"polished"}`, "")
	require.Equal(t, stdhttp.StatusBadGateway, rec.Code)
	require.Contains(t, rec.Body.String(), `"provider-failed"`)
}

func TestRewrite_RejectsNonJSON(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	req := httptest.NewRequest(stdhttp.MethodPost, "/v1/rewrite",
		strings.NewReader("text=hi"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+signTestToken(t, fx.user.ID.String()))
	rec := httptest.NewRecorder()
	fx.router.ServeHTTP(rec, req)
	require.Equal(t, stdhttp.StatusBadRequest, rec.Code)
}

func TestRewrite_BodySizeLimit(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	// Build a body larger than MaxBodyBytes.
	big := bytes.Repeat([]byte("a"), internalhttp.MaxBodyBytes+1024)
	body := `{"text":"` + string(big) + `","tone":"polished"}`
	rec := fx.doRequest(t, body, "")
	require.Equal(t, stdhttp.StatusBadRequest, rec.Code)
}

func TestHealth_PublicNoAuth(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	req := httptest.NewRequest(stdhttp.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	fx.router.ServeHTTP(rec, req)
	require.Equal(t, stdhttp.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"status":"ok"`)
}

// Sanity check: drain helper for streaming responses in case future
// tests want to scan event-by-event rather than search the bytes.
func readSSE(t *testing.T, r io.Reader) []string {
	t.Helper()
	raw, err := io.ReadAll(r)
	require.NoError(t, err)
	out := []string{}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "data: ") {
			out = append(out, strings.TrimPrefix(line, "data: "))
		}
	}
	return out
}

// Compile-time check that readSSE stays used. Remove when consumed by
// a real test.
var _ = readSSE
var _ = context.Background
