package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/tannpv/draftright-rewrite/internal/rewrite/adapter/memory"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/domain"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/parity"
	"github.com/tannpv/draftright-rewrite/internal/rewrite/usecase"
)

// capturingProvider records the system prompt the use case resolved onto the
// request, so a test can assert it equals what the parity /rewrite path sends.
type capturingProvider struct{ gotPrompt string }

func (p *capturingProvider) Name() string  { return "capture" }
func (p *capturingProvider) ID() uuid.UUID { return uuid.Nil }

func (p *capturingProvider) Stream(_ context.Context, req domain.RewriteRequest) (<-chan string, <-chan error) {
	p.gotPrompt = req.SystemPrompt()
	tokens := make(chan string)
	errs := make(chan error)
	close(tokens)
	close(errs)
	return tokens, errs
}

// #192: the streaming /v1/rewrite path must send the SAME system prompt as the
// parity /rewrite path. The use case resolves it via parity.ResolvePrompt and
// the provider receives it verbatim — this locks the two paths together so the
// old short per-adapter prompts (missing the anti-injection guard, dropping the
// translate target language) can never come back.
func TestRewrite_SystemPromptMatchesParity(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, tone, lang, inputKind string }{
		{"polished typed", "polished", "", ""},
		{"simple speech prepends preamble", "simple", "", "speech"},
		{"translate carries the target language", "translate", "Vietnamese", ""},
		{"claude", "claude", "", ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			u := userWithQuota(0, 100)
			prov := &capturingProvider{}
			deps := usecase.RewriteDeps{
				Users:         memory.NewUserRepo(u),
				Provider:      prov,
				RateLimit:     memory.NewRateLimiter(),
				ResolvePrompt: parity.ResolvePrompt,
			}

			req, err := domain.NewRewriteRequest("hello world", tc.tone, tc.lang, tc.inputKind)
			require.NoError(t, err)

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			tokens, errs, err := usecase.Rewrite(ctx, deps, u.ID, req)
			require.NoError(t, err)
			_, finalErr := drain(ctx, t, tokens, errs)
			require.NoError(t, finalErr)

			want := parity.ResolvePrompt(tc.tone, tc.lang, "", tc.inputKind)
			require.Equal(t, want, prov.gotPrompt)
			require.NotEmpty(t, prov.gotPrompt, "resolved prompt must not be empty")
		})
	}
}
