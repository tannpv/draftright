package usecase_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/tannpv/draftright-rewrite/internal/domain"
	"github.com/tannpv/draftright-rewrite/internal/usecase"
)

// ----------------------------------------------------------------------
// Test doubles — inline so the use case can be exercised without yet
// implementing real adapters. Task 6 will extract these into
// internal/adapter/memory/ for reuse across HTTP handler tests.
// ----------------------------------------------------------------------

type fakeRateLimiter struct {
	err error
}

func (f *fakeRateLimiter) Check(ctx context.Context, id domain.UserID) error { return f.err }

type fakeUserRepo struct {
	user        *domain.User
	findErr     error
	loggedUsage []domain.UsageLog
	logErr      error
	mu          sync.Mutex
}

func (f *fakeUserRepo) Find(ctx context.Context, id domain.UserID) (*domain.User, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	return f.user, nil
}

func (f *fakeUserRepo) LogUsage(ctx context.Context, log domain.UsageLog) error {
	if f.logErr != nil {
		return f.logErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.loggedUsage = append(f.loggedUsage, log)
	return nil
}

func (f *fakeUserRepo) logsLen() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.loggedUsage)
}

// fakeProvider streams a scripted sequence of tokens + an optional
// final error. Stream goroutine respects ctx so cancellation tests work.
type fakeProvider struct {
	id     uuid.UUID
	name   string
	tokens []string
	final  error // nil = clean close; non-nil = sent on errs then return
}

func (p *fakeProvider) ID() uuid.UUID { return p.id }
func (p *fakeProvider) Name() string  { return p.name }

func (p *fakeProvider) Stream(ctx context.Context, req domain.RewriteRequest) (<-chan string, <-chan error) {
	tokens := make(chan string)
	errs := make(chan error, 1)
	go func() {
		defer close(tokens)
		defer close(errs)
		for _, t := range p.tokens {
			select {
			case <-ctx.Done():
				return
			case tokens <- t:
			}
		}
		if p.final != nil {
			errs <- p.final
		}
	}()
	return tokens, errs
}

// ----------------------------------------------------------------------
// Fixtures
// ----------------------------------------------------------------------

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

func drain(ctx context.Context, t *testing.T, tokens <-chan string, errs <-chan error) (collected []string, finalErr error) {
	t.Helper()
	for {
		select {
		case tok, ok := <-tokens:
			if !ok {
				// Final error chan may still emit ctx.Err() / nil; check non-blocking.
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
				// Drain remaining tokens (if any) so the producer goroutine exits.
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

// ----------------------------------------------------------------------
// Tests
// ----------------------------------------------------------------------

func TestRewrite_HappyPath_LogsUsageOnCleanClose(t *testing.T) {
	t.Parallel()
	users := &fakeUserRepo{user: userWithQuota(0, 100)}
	prov := &fakeProvider{
		id:     uuid.New(),
		name:   "fake-openai",
		tokens: []string{"Hello", " ", "world!"},
	}
	deps := usecase.RewriteDeps{
		Users: users, Provider: prov, RateLimit: &fakeRateLimiter{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	tokens, errs, err := usecase.Rewrite(ctx, deps, users.user.ID, reqOK(t))
	require.NoError(t, err)

	collected, finalErr := drain(ctx, t, tokens, errs)
	require.NoError(t, finalErr)
	require.Equal(t, []string{"Hello", " ", "world!"}, collected)

	// Usage log: one row, with output length = sum of token lengths.
	require.Equal(t, 1, users.logsLen())
	require.Equal(t, int32(len("Hello world!")), users.loggedUsage[0].OutputLength)
	require.Equal(t, domain.TonePolished, users.loggedUsage[0].Tone)
	require.Equal(t, prov.id, users.loggedUsage[0].AIProviderID)
}

func TestRewrite_RateLimited_ShortCircuitsBeforeDB(t *testing.T) {
	t.Parallel()
	users := &fakeUserRepo{findErr: errors.New("repo should not be called")}
	prov := &fakeProvider{}
	deps := usecase.RewriteDeps{
		Users: users, Provider: prov,
		RateLimit: &fakeRateLimiter{err: domain.ErrRateLimited},
	}

	_, _, err := usecase.Rewrite(context.Background(), deps, domain.UserID(uuid.New()), reqOK(t))
	require.ErrorIs(t, err, domain.ErrRateLimited)
	require.Equal(t, 0, users.logsLen(), "should not log usage on rate-limit reject")
}

func TestRewrite_UserNotFound_ReturnsDomainError(t *testing.T) {
	t.Parallel()
	users := &fakeUserRepo{findErr: domain.ErrUserNotFound}
	prov := &fakeProvider{}
	deps := usecase.RewriteDeps{
		Users: users, Provider: prov, RateLimit: &fakeRateLimiter{},
	}
	_, _, err := usecase.Rewrite(context.Background(), deps, domain.UserID(uuid.New()), reqOK(t))
	require.ErrorIs(t, err, domain.ErrUserNotFound)
}

func TestRewrite_QuotaExceeded_BeforeProviderCall(t *testing.T) {
	t.Parallel()
	users := &fakeUserRepo{user: userWithQuota(100, 100)} // at limit
	prov := &fakeProvider{}
	deps := usecase.RewriteDeps{
		Users: users, Provider: prov, RateLimit: &fakeRateLimiter{},
	}
	_, _, err := usecase.Rewrite(context.Background(), deps, users.user.ID, reqOK(t))
	require.ErrorIs(t, err, domain.ErrQuotaExceeded)
	require.Equal(t, 0, users.logsLen(), "quota reject must not write usage")
}

func TestRewrite_ProviderError_PropagatesAndSkipsUsageLog(t *testing.T) {
	t.Parallel()
	users := &fakeUserRepo{user: userWithQuota(0, 100)}
	upstreamErr := errors.New("openai 429")
	prov := &fakeProvider{
		id: uuid.New(), name: "fake",
		tokens: []string{"partial"},
		final:  upstreamErr,
	}
	deps := usecase.RewriteDeps{
		Users: users, Provider: prov, RateLimit: &fakeRateLimiter{},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	tokens, errs, err := usecase.Rewrite(ctx, deps, users.user.ID, reqOK(t))
	require.NoError(t, err)

	_, finalErr := drain(ctx, t, tokens, errs)
	require.ErrorIs(t, finalErr, upstreamErr)
	require.Equal(t, 0, users.logsLen(), "failed stream must not bill the user")
}

func TestRewrite_ContextCancel_StopsStreamCleanly(t *testing.T) {
	t.Parallel()
	users := &fakeUserRepo{user: userWithQuota(0, 100)}
	prov := &fakeProvider{
		id: uuid.New(), name: "fake",
		tokens: []string{"will-never-fully-stream"},
	}
	deps := usecase.RewriteDeps{
		Users: users, Provider: prov, RateLimit: &fakeRateLimiter{},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel BEFORE we call so the use case sees it immediately
	tokens, errs, err := usecase.Rewrite(ctx, deps, users.user.ID, reqOK(t))
	require.NoError(t, err)
	// Whatever shape it lands in, the channels must close + final
	// error must be ctx.Err() or nil (race-safe).
	for range tokens {
	}
	select {
	case e := <-errs:
		if e != nil {
			require.True(t, errors.Is(e, context.Canceled),
				"expected ctx.Canceled, got %v", e)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("errs channel did not close after cancel")
	}
	require.Equal(t, 0, users.logsLen(), "canceled stream must not log usage")
}

func TestRewrite_InjectedClock_RecordsResponseTime(t *testing.T) {
	t.Parallel()
	users := &fakeUserRepo{user: userWithQuota(0, 100)}
	prov := &fakeProvider{
		id: uuid.New(), name: "fake",
		tokens: []string{"hi"},
	}
	// Fake clock: first call = t0, second = t0 + 250ms.
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	calls := 0
	deps := usecase.RewriteDeps{
		Users: users, Provider: prov, RateLimit: &fakeRateLimiter{},
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
	tokens, errs, err := usecase.Rewrite(ctx, deps, users.user.ID, reqOK(t))
	require.NoError(t, err)
	_, finalErr := drain(ctx, t, tokens, errs)
	require.NoError(t, finalErr)
	require.Equal(t, 1, users.logsLen())
	require.Equal(t, int32(250), users.loggedUsage[0].ResponseTimeMs,
		"injected clock must drive response_time_ms exactly")
}
