package chain_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/tannpv/draftright-rewrite/internal/rewrite/adapter/chain"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/adapter/memory"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/domain"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/provenance"
)

func mustReq(t *testing.T) domain.RewriteRequest {
	t.Helper()
	r, err := domain.NewRewriteRequest("hi", "polished", "", "")
	require.NoError(t, err)
	return r
}

// drain reads tokens until both channels close. Returns the collected
// tokens + the terminal error (nil on clean close).
func drain(t *testing.T, tokens <-chan string, errs <-chan error) ([]string, error) {
	t.Helper()
	var got []string
	deadline := time.After(3 * time.Second)
	for {
		select {
		case tok, ok := <-tokens:
			if !ok {
				select {
				case e := <-errs:
					return got, e
				default:
					return got, nil
				}
			}
			got = append(got, tok)
		case <-deadline:
			t.Fatal("drain timed out")
			return got, errors.New("timeout")
		}
	}
}

func TestChain_PrimaryHappyPath(t *testing.T) {
	t.Parallel()
	primary := memory.NewProvider("primary", []string{"Hello", " ", "world"})
	fallback := memory.NewProvider("fallback", []string{"NEVER", "USED"})
	c := chain.New("test", []domain.AiProvider{primary, fallback})

	tokens, errs := c.Stream(context.Background(), mustReq(t))
	got, err := drain(t, tokens, errs)
	require.NoError(t, err)
	require.Equal(t, []string{"Hello", " ", "world"}, got)
}

func TestChain_FailsOverWhenPrimaryErrorsBeforeFirstToken(t *testing.T) {
	t.Parallel()
	primary := memory.NewProvider("primary", nil).
		WithFinalError(errors.New("primary down"))
	fallback := memory.NewProvider("fallback", []string{"rescue"})
	c := chain.New("test", []domain.AiProvider{primary, fallback})

	tokens, errs := c.Stream(context.Background(), mustReq(t))
	got, err := drain(t, tokens, errs)
	require.NoError(t, err)
	require.Equal(t, []string{"rescue"}, got)
}

func TestChain_DoesNotFailOverAfterFirstToken(t *testing.T) {
	t.Parallel()
	// Primary streams one token then errors. Chain MUST surface the
	// error, NOT switch to the fallback (mid-stream switch would
	// produce garbled output).
	primary := memory.NewProvider("primary", []string{"partial"}).
		WithFinalError(errors.New("crashed mid-stream"))
	fallback := memory.NewProvider("fallback", []string{"would-corrupt"})
	c := chain.New("test", []domain.AiProvider{primary, fallback})

	tokens, errs := c.Stream(context.Background(), mustReq(t))
	got, err := drain(t, tokens, errs)
	require.Error(t, err)
	require.Equal(t, []string{"partial"}, got)
	require.Contains(t, err.Error(), "crashed mid-stream")
}

func TestChain_AllProvidersFail(t *testing.T) {
	t.Parallel()
	p1 := memory.NewProvider("p1", nil).WithFinalError(errors.New("p1 down"))
	p2 := memory.NewProvider("p2", nil).WithFinalError(errors.New("p2 down"))
	c := chain.New("test", []domain.AiProvider{p1, p2})

	tokens, errs := c.Stream(context.Background(), mustReq(t))
	got, err := drain(t, tokens, errs)
	require.Empty(t, got)
	require.Error(t, err)
	require.True(t, chain.IsExhausted(err), "want ErrProviderUnavailable, got %v", err)
}

func TestChain_EmptyProvidersListErrors(t *testing.T) {
	t.Parallel()
	c := chain.New("empty", nil)
	tokens, errs := c.Stream(context.Background(), mustReq(t))
	_, err := drain(t, tokens, errs)
	require.True(t, errors.Is(err, domain.ErrProviderUnavailable))
}

func TestChain_ContextCancelDuringPrimary(t *testing.T) {
	t.Parallel()
	primary := memory.NewProvider("primary", []string{"one", "two", "three"})
	fallback := memory.NewProvider("fallback", []string{"NEVER"})
	c := chain.New("test", []domain.AiProvider{primary, fallback})

	ctx, cancel := context.WithCancel(context.Background())
	tokens, errs := c.Stream(ctx, mustReq(t))
	cancel()
	for range tokens {
	}
	select {
	case <-errs:
	case <-time.After(2 * time.Second):
		t.Fatal("errs did not close after cancel")
	}
}

func TestChain_StampsWinningProviderProvenance(t *testing.T) {
	t.Parallel()
	primary := memory.NewProvider("openai", nil).
		WithModel("gpt-4o-mini").
		WithFinalError(errors.New("primary down"))
	fallback := memory.NewProvider("anthropic", []string{"rescue"}).
		WithModel("claude-3-5-sonnet-20241022")
	c := chain.New("test", []domain.AiProvider{primary, fallback})

	ctx, p := provenance.NewContext(context.Background())
	tokens, errs := c.Stream(ctx, mustReq(t))
	got, err := drain(t, tokens, errs)
	require.NoError(t, err)
	require.Equal(t, []string{"rescue"}, got)
	m, ty := p.Read()
	require.Equal(t, "claude-3-5-sonnet-20241022", m)
	require.Equal(t, "anthropic", ty)
}

func TestChain_IDAndName(t *testing.T) {
	id := uuid.New()
	c := chain.New("openai>anthropic", nil, chain.WithID(id))
	require.Equal(t, id, c.ID())
	require.Equal(t, "openai>anthropic", c.Name())
}
