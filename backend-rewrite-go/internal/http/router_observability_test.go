package http_test

import (
	"context"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/tannpv/draftright-rewrite/internal/adapter/memory"
	"github.com/tannpv/draftright-rewrite/internal/domain"
	internalhttp "github.com/tannpv/draftright-rewrite/internal/http"
	"github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/platform/metrics"
	"github.com/tannpv/draftright-rewrite/internal/usecase"
)

func TestMetrics_EndpointExposedWhenWired(t *testing.T) {
	t.Parallel()
	prom := metrics.NewPrometheus()
	r := (&internalhttp.Router{
		Verifier:       auth.NewVerifier(testJWTSecret),
		MetricsHandler: prom.Handler(),
		Rewrite: &internalhttp.RewriteHandler{
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
	r := (&internalhttp.Router{
		Verifier: auth.NewVerifier(testJWTSecret),
		Rewrite: &internalhttp.RewriteHandler{
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
	r := (&internalhttp.Router{
		Verifier:       auth.NewVerifier(testJWTSecret),
		MetricsHandler: prom.Handler(),
		Rewrite: &internalhttp.RewriteHandler{
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
	l := internalhttp.LogFromContext(context.Background())
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
