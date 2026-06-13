package shared_test

import (
	"context"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/tannpv/draftright-rewrite/internal/rewrite/adapter/memory"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/domain"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/transport"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/usecase"
	"github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/platform/metrics"
	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// testJWTSecret is the test-only shared secret; production wires JWT_SECRET from env.
const testJWTSecret = "test-secret-must-match-nestjs"

// signTestToken signs an HS256 JWT with the test secret.
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

// fixture is a local test helper for the observability tests.
type fixture struct {
	router stdhttp.Handler
	user   *domain.User
}

// newFixture wires a standard router for observability tests.
func newFixture(t *testing.T, mutators ...func(*fixture)) *fixture {
	t.Helper()
	uid, err := uuid.Parse("11111111-1111-1111-1111-111111111111")
	require.NoError(t, err)
	user := &domain.User{
		ID:    domain.UserID(uid),
		Email: "tester@draftright.test",
		Role:  "user",
		Plan:  domain.Plan{Name: "pro", DailyLimit: 1000},
	}
	fx := &fixture{user: user}
	for _, m := range mutators {
		m(fx)
	}
	deps := usecase.RewriteDeps{
		Users:     memory.NewUserRepo(user),
		Provider:  memory.NewProvider("memory-test", []string{"Hello", " ", "world"}),
		RateLimit: memory.NewRateLimiter(),
		Now:       func() time.Time { return time.Unix(1700000000, 0) },
	}
	fx.router = (&shared.Router{
		Verifier: auth.NewVerifier(testJWTSecret),
		Rewrite: &transport.RewriteHandler{
			Deps: deps,
			Now:  deps.Now,
		},
	}).Build()
	return fx
}

func TestMetrics_EndpointExposedWhenWired(t *testing.T) {
	t.Parallel()
	prom := metrics.NewPrometheus()
	r := (&shared.Router{
		Verifier:       auth.NewVerifier(testJWTSecret),
		MetricsHandler: prom.Handler(),
		Rewrite: &transport.RewriteHandler{
			Deps: usecase.RewriteDeps{
				Users:     memory.NewUserRepo(nil),
				Provider:  memory.NewProvider("memfake", nil),
				RateLimit: memory.NewRateLimiter(),
				Metrics:   prom,
			},
		},
	}).Build()

	req := httptest.NewRequest(stdhttp.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, stdhttp.StatusOK, rec.Code)
	require.Contains(t, rec.Header().Get("Content-Type"), "text/plain")
}

func TestMetrics_EndpointHidden_WhenNotWired(t *testing.T) {
	t.Parallel()
	r := (&shared.Router{
		Verifier: auth.NewVerifier(testJWTSecret),
		Rewrite: &transport.RewriteHandler{
			Deps: usecase.RewriteDeps{
				Users:     memory.NewUserRepo(nil),
				Provider:  memory.NewProvider("memfake", nil),
				RateLimit: memory.NewRateLimiter(),
			},
		},
	}).Build()
	req := httptest.NewRequest(stdhttp.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, stdhttp.StatusNotFound, rec.Code)
}

func TestMetrics_RewriteEmitsCounter(t *testing.T) {
	t.Parallel()
	fx := newFixture(t, func(f *fixture) {
		// Already has memory.NewMetrics via the default; need to
		// rebuild the router with metrics wired. Easier path: bypass
		// the fixture and assemble directly.
	})
	_ = fx
	// Standalone wiring to keep the assertion crisp.
	prom := metrics.NewPrometheus()
	user := newTestUser(t)
	r := (&shared.Router{
		Verifier:       auth.NewVerifier(testJWTSecret),
		MetricsHandler: prom.Handler(),
		Rewrite: &transport.RewriteHandler{
			Deps: usecase.RewriteDeps{
				Users:     memory.NewUserRepo(user),
				Provider:  memory.NewProvider("memfake", []string{"hi"}),
				RateLimit: memory.NewRateLimiter(),
				Metrics:   prom,
			},
		},
	}).Build()

	// Drive one rewrite, then scrape.
	req := httptest.NewRequest(stdhttp.MethodPost, "/v1/rewrite",
		strings.NewReader(`{"text":"hi","tone":"polished"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+signTestToken(t, user.ID.String()))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, stdhttp.StatusOK, rec.Code)

	scrape := httptest.NewRecorder()
	r.ServeHTTP(scrape, httptest.NewRequest(stdhttp.MethodGet, "/metrics", nil))
	body := scrape.Body.String()
	require.Contains(t, body, "draftright_rewrite_requests_total")
	require.Contains(t, body, `outcome="ok"`)
	require.Contains(t, body, `tone="polished"`)
	require.Contains(t, body, `provider="memfake"`)
	require.Contains(t, body, "draftright_rewrite_duration_seconds")
	require.Contains(t, body, "draftright_rewrite_tokens_streamed_total")
}

func TestLogContext_ProvidesRequestScopedLogger(t *testing.T) {
	t.Parallel()
	// Sanity-check the LogFromContext helper returns a non-nil
	// logger even when the middleware path doesn't wire one (test
	// path), so handlers never panic.
	l := shared.LogFromContext(context.Background())
	require.NotNil(t, l)
}

// newTestUser builds a stock domain.User for inline router wiring.
func newTestUser(t *testing.T) *domain.User {
	t.Helper()
	uid, err := uuid.Parse("33333333-3333-3333-3333-333333333333")
	require.NoError(t, err)
	return &domain.User{
		ID:    domain.UserID(uid),
		Email: "obs@draftright.test",
		Role:  "user",
		Plan:  domain.Plan{Name: "test", DailyLimit: 100},
	}
}
