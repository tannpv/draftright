package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/tannpv/draftright-rewrite/internal/rewrite/adapter/memory"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/domain"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/usecase"
)

// drainAll collects tokens until both channels close.
func drainAll(t *testing.T, toks <-chan string, errs <-chan error) (int, error) {
	t.Helper()
	count := 0
	deadline := time.After(2 * time.Second)
	for {
		select {
		case _, ok := <-toks:
			if !ok {
				select {
				case e := <-errs:
					return count, e
				default:
					return count, nil
				}
			}
			count++
		case <-deadline:
			t.Fatal("drain timed out")
			return count, errors.New("timeout")
		}
	}
}

func TestRewrite_RecordsHappyPathMetrics(t *testing.T) {
	t.Parallel()
	u := userWithQuota(0, 100)
	m := memory.NewMetrics()
	deps := usecase.RewriteDeps{
		Users:     memory.NewUserRepo(u),
		Provider:  memory.NewProvider("memfake", []string{"a", "b", "c"}),
		RateLimit: memory.NewRateLimiter(),
		Metrics:   m,
	}
	tokens, errs, err := usecase.Rewrite(context.Background(), deps, u.ID, reqOK(t))
	require.NoError(t, err)
	count, finalErr := drainAll(t, tokens, errs)
	require.NoError(t, finalErr)
	require.Equal(t, 3, count)

	obs := m.Observed()
	require.Len(t, obs, 1)
	require.Equal(t, domain.OutcomeOK, obs[0].Outcome)
	require.Equal(t, domain.TonePolished, obs[0].Tone)
	require.Equal(t, "memfake", obs[0].Provider)
	require.Equal(t, 3, m.TokensFor("memfake"))
}

func TestRewrite_RecordsRateLimitedMetric(t *testing.T) {
	t.Parallel()
	u := userWithQuota(0, 100)
	m := memory.NewMetrics()
	deps := usecase.RewriteDeps{
		Users:     memory.NewUserRepo(u),
		Provider:  memory.NewProvider("memfake", nil),
		RateLimit: memory.NewRateLimiter().WithError(domain.ErrRateLimited),
		Metrics:   m,
	}
	_, _, err := usecase.Rewrite(context.Background(), deps, u.ID, reqOK(t))
	require.ErrorIs(t, err, domain.ErrRateLimited)
	obs := m.Observed()
	require.Len(t, obs, 1)
	require.Equal(t, domain.OutcomeRateLimited, obs[0].Outcome)
	require.Equal(t, 0, m.TokensFor("memfake"))
}

func TestRewrite_RecordsQuotaExceededMetric(t *testing.T) {
	t.Parallel()
	u := userWithQuota(5, 5) // already at limit
	m := memory.NewMetrics()
	deps := usecase.RewriteDeps{
		Users:     memory.NewUserRepo(u),
		Provider:  memory.NewProvider("memfake", nil),
		RateLimit: memory.NewRateLimiter(),
		Metrics:   m,
	}
	_, _, err := usecase.Rewrite(context.Background(), deps, u.ID, reqOK(t))
	require.ErrorIs(t, err, domain.ErrQuotaExceeded)
	obs := m.Observed()
	require.Len(t, obs, 1)
	require.Equal(t, domain.OutcomeQuotaExceeded, obs[0].Outcome)
}

func TestRewrite_RecordsProviderFailedMetric(t *testing.T) {
	t.Parallel()
	u := userWithQuota(0, 100)
	m := memory.NewMetrics()
	deps := usecase.RewriteDeps{
		Users:     memory.NewUserRepo(u),
		Provider:  memory.NewProvider("memfake", nil).WithFinalError(errors.New("boom")),
		RateLimit: memory.NewRateLimiter(),
		Metrics:   m,
	}
	tokens, errs, err := usecase.Rewrite(context.Background(), deps, u.ID, reqOK(t))
	require.NoError(t, err)
	_, finalErr := drainAll(t, tokens, errs)
	require.Error(t, finalErr)
	obs := m.Observed()
	require.Len(t, obs, 1)
	require.Equal(t, domain.OutcomeProviderFailed, obs[0].Outcome)
}
