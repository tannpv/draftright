package usecase_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/tannpv/draftright-rewrite/internal/rewrite/adapter/memory"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/domain"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/usecase"
)

// captureLogger is a synchronous RewriteLogger double: it records every
// entry the use case fires so a test can assert on it immediately after
// draining (the use case calls LogRewrite directly — no goroutine).
type captureLogger struct {
	mu      sync.Mutex
	entries []usecase.RewriteLogEntry
}

func (c *captureLogger) LogRewrite(_ context.Context, e usecase.RewriteLogEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = append(c.entries, e)
}

func (c *captureLogger) all() []usecase.RewriteLogEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]usecase.RewriteLogEntry(nil), c.entries...)
}

// Fakes live in internal/rewrite/adapter/memory so this file + future HTTP
// handler tests share one set of doubles (Rule #1 — reusable).

func userWithQuota(used int64, limit int32) *domain.User {
	return &domain.User{
		ID:        domain.UserID(uuid.New()),
		Email:     "u@example.com",
		Role:      "user",
		Plan:      domain.Plan{ID: uuid.New(), Name: "test", DailyLimit: limit},
		UsedToday: used,
	}
}

func reqOK(t *testing.T) domain.RewriteRequest {
	t.Helper()
	r, err := domain.NewRewriteRequest("hello world", "polished", "")
	require.NoError(t, err)
	return r
}

// drain consumes both channels until tokens closes, collecting tokens
// + the terminal error. Bounded by a timeout so a producer bug can't
// hang the suite.
func drain(ctx context.Context, t *testing.T, tokens <-chan string, errs <-chan error) (collected []string, finalErr error) {
	t.Helper()
	for {
		select {
		case tok, ok := <-tokens:
			if !ok {
				select {
				case e := <-errs:
					finalErr = e
				default:
				}
				return
			}
			collected = append(collected, tok)
		case e, ok := <-errs:
			if ok && e != nil {
				finalErr = e
				for range tokens {
				}
				return
			}
		case <-ctx.Done():
			finalErr = ctx.Err()
			return
		case <-time.After(2 * time.Second):
			t.Fatal("drain timed out")
			return
		}
	}
}

func TestRewrite_HappyPath_LogsUsageOnCleanClose(t *testing.T) {
	t.Parallel()
	u := userWithQuota(0, 100)
	users := memory.NewUserRepo(u)
	prov := memory.NewProvider("fake-openai", []string{"Hello", " ", "world!"})
	deps := usecase.RewriteDeps{Users: users, Provider: prov, RateLimit: memory.NewRateLimiter()}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	tokens, errs, err := usecase.Rewrite(ctx, deps, u.ID, reqOK(t))
	require.NoError(t, err)

	collected, finalErr := drain(ctx, t, tokens, errs)
	require.NoError(t, finalErr)
	require.Equal(t, []string{"Hello", " ", "world!"}, collected)

	require.Equal(t, 1, users.LogsLen())
	logs := users.Logs()
	require.Equal(t, int32(len("Hello world!")), logs[0].OutputLength)
	require.Equal(t, domain.TonePolished, logs[0].Tone)
	require.Equal(t, prov.ID(), logs[0].AIProviderID)
}

func TestRewrite_RateLimited_ShortCircuitsBeforeDB(t *testing.T) {
	t.Parallel()
	users := memory.NewUserRepo(nil).WithFindErr(errors.New("repo should not be called"))
	prov := memory.NewProvider("fake", nil)
	deps := usecase.RewriteDeps{
		Users: users, Provider: prov,
		RateLimit: memory.NewRateLimiter().WithError(domain.ErrRateLimited),
	}
	_, _, err := usecase.Rewrite(context.Background(), deps, domain.UserID(uuid.New()), reqOK(t))
	require.ErrorIs(t, err, domain.ErrRateLimited)
	require.Equal(t, 0, users.LogsLen())
}

func TestRewrite_UserNotFound_ReturnsDomainError(t *testing.T) {
	t.Parallel()
	users := memory.NewUserRepo(nil) // nil user → ErrUserNotFound
	prov := memory.NewProvider("fake", nil)
	deps := usecase.RewriteDeps{Users: users, Provider: prov, RateLimit: memory.NewRateLimiter()}

	_, _, err := usecase.Rewrite(context.Background(), deps, domain.UserID(uuid.New()), reqOK(t))
	require.ErrorIs(t, err, domain.ErrUserNotFound)
}

func TestRewrite_QuotaExceeded_BeforeProviderCall(t *testing.T) {
	t.Parallel()
	u := userWithQuota(100, 100)
	users := memory.NewUserRepo(u)
	prov := memory.NewProvider("fake", []string{"unreachable"})
	deps := usecase.RewriteDeps{Users: users, Provider: prov, RateLimit: memory.NewRateLimiter()}

	_, _, err := usecase.Rewrite(context.Background(), deps, u.ID, reqOK(t))
	require.ErrorIs(t, err, domain.ErrQuotaExceeded)
	require.Equal(t, 0, users.LogsLen())
}

func TestRewrite_ProviderError_PropagatesAndSkipsUsageLog(t *testing.T) {
	t.Parallel()
	u := userWithQuota(0, 100)
	users := memory.NewUserRepo(u)
	upstreamErr := errors.New("openai 429")
	prov := memory.NewProvider("fake", []string{"partial"}).WithFinalError(upstreamErr)
	deps := usecase.RewriteDeps{Users: users, Provider: prov, RateLimit: memory.NewRateLimiter()}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	tokens, errs, err := usecase.Rewrite(ctx, deps, u.ID, reqOK(t))
	require.NoError(t, err)

	_, finalErr := drain(ctx, t, tokens, errs)
	require.ErrorIs(t, finalErr, upstreamErr)
	require.Equal(t, 0, users.LogsLen(), "failed stream must not bill the user")
}

func TestRewrite_ContextCancel_StopsStreamCleanly(t *testing.T) {
	t.Parallel()
	u := userWithQuota(0, 100)
	users := memory.NewUserRepo(u)
	prov := memory.NewProvider("fake", []string{"would-stream-forever"})
	deps := usecase.RewriteDeps{Users: users, Provider: prov, RateLimit: memory.NewRateLimiter()}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	tokens, errs, err := usecase.Rewrite(ctx, deps, u.ID, reqOK(t))
	require.NoError(t, err)

	for range tokens {
	}
	select {
	case e := <-errs:
		if e != nil {
			require.True(t, errors.Is(e, context.Canceled),
				"expected context.Canceled, got %v", e)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("errs channel did not close after cancel")
	}
	require.Equal(t, 0, users.LogsLen())
}

func TestRewrite_InjectedClock_RecordsResponseTime(t *testing.T) {
	t.Parallel()
	u := userWithQuota(0, 100)
	users := memory.NewUserRepo(u)
	prov := memory.NewProvider("fake", []string{"hi"})

	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	calls := 0
	deps := usecase.RewriteDeps{
		Users: users, Provider: prov, RateLimit: memory.NewRateLimiter(),
		Now: func() time.Time {
			calls++
			if calls == 1 {
				return t0
			}
			return t0.Add(250 * time.Millisecond)
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	tokens, errs, err := usecase.Rewrite(ctx, deps, u.ID, reqOK(t))
	require.NoError(t, err)
	_, finalErr := drain(ctx, t, tokens, errs)
	require.NoError(t, finalErr)
	require.Equal(t, 1, users.LogsLen())
	require.Equal(t, int32(250), users.Logs()[0].ResponseTimeMs)
}

func TestRewriteLog_CleanFinish_LogsExactlyOneEntry(t *testing.T) {
	t.Parallel()
	u := userWithQuota(0, 100)
	users := memory.NewUserRepo(u)
	// Provider name == provider_type ("t"); WithModel sets model ("m").
	// The fake stamps provenance at the top of Stream onto the
	// carrier-bearing ctx the use case derives.
	prov := memory.NewProvider("t", []string{"Hello", " ", "world!"}).WithModel("m")
	rl := &captureLogger{}
	deps := usecase.RewriteDeps{
		Users: users, Provider: prov, RateLimit: memory.NewRateLimiter(),
		RewriteLog: rl,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	tokens, errs, err := usecase.Rewrite(ctx, deps, u.ID, reqOK(t))
	require.NoError(t, err)

	collected, finalErr := drain(ctx, t, tokens, errs)
	require.NoError(t, finalErr)
	require.Equal(t, []string{"Hello", " ", "world!"}, collected)

	entries := rl.all()
	require.Len(t, entries, 1)
	got := entries[0]
	require.Equal(t, "polished", got.Tone)
	require.Equal(t, "hello world", got.InputText)
	require.Equal(t, "Hello world!", got.OutputText)
	require.Equal(t, "m", got.Model)
	require.Equal(t, "t", got.ProviderType)
	require.GreaterOrEqual(t, got.ResponseTimeMs, int64(0))
}

func TestRewriteLog_ProviderError_NoLog(t *testing.T) {
	t.Parallel()
	u := userWithQuota(0, 100)
	users := memory.NewUserRepo(u)
	prov := memory.NewProvider("t", []string{"partial"}).
		WithModel("m").
		WithFinalError(errors.New("openai 429"))
	rl := &captureLogger{}
	deps := usecase.RewriteDeps{
		Users: users, Provider: prov, RateLimit: memory.NewRateLimiter(),
		RewriteLog: rl,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	tokens, errs, err := usecase.Rewrite(ctx, deps, u.ID, reqOK(t))
	require.NoError(t, err)

	_, finalErr := drain(ctx, t, tokens, errs)
	require.Error(t, finalErr)
	require.Len(t, rl.all(), 0, "failed stream must not capture training data")
}

func TestRewriteLog_ContextCancel_NoLog(t *testing.T) {
	t.Parallel()
	u := userWithQuota(0, 100)
	users := memory.NewUserRepo(u)
	prov := memory.NewProvider("t", []string{"would-stream-forever"}).WithModel("m")
	rl := &captureLogger{}
	deps := usecase.RewriteDeps{
		Users: users, Provider: prov, RateLimit: memory.NewRateLimiter(),
		RewriteLog: rl,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	tokens, errs, err := usecase.Rewrite(ctx, deps, u.ID, reqOK(t))
	require.NoError(t, err)

	for range tokens {
	}
	select {
	case <-errs:
	case <-time.After(2 * time.Second):
		t.Fatal("errs channel did not close after cancel")
	}
	require.Len(t, rl.all(), 0, "cancelled stream must not capture training data")
}

func TestRewriteLog_NilSink_NoPanic(t *testing.T) {
	t.Parallel()
	u := userWithQuota(0, 100)
	users := memory.NewUserRepo(u)
	prov := memory.NewProvider("t", []string{"Hello", " ", "world!"}).WithModel("m")
	// RewriteLog deliberately left nil — clean finish must still
	// stream to completion without panicking.
	deps := usecase.RewriteDeps{Users: users, Provider: prov, RateLimit: memory.NewRateLimiter()}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	tokens, errs, err := usecase.Rewrite(ctx, deps, u.ID, reqOK(t))
	require.NoError(t, err)

	collected, finalErr := drain(ctx, t, tokens, errs)
	require.NoError(t, finalErr)
	require.Equal(t, []string{"Hello", " ", "world!"}, collected)
}
