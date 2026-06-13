package memory

import (
	"sync"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/rewrite/domain"
)

// Metrics is the in-memory domain.Metrics for tests. Records every
// ObserveRewrite call so a test can assert on outcome + tone +
// provider + a duration cap.
//
// Lives next to the other memory fakes so every test in the repo has
// one place to pull its doubles from (Rule #1 — reusable).
type Metrics struct {
	mu       sync.Mutex
	observed []ObservedRewrite
	tokens   map[string]int
}

// ObservedRewrite is one captured ObserveRewrite call.
type ObservedRewrite struct {
	Outcome  domain.RewriteOutcome
	Tone     domain.Tone
	Provider string
	Duration time.Duration
}

// NewMetrics returns a fresh recorder.
func NewMetrics() *Metrics {
	return &Metrics{tokens: map[string]int{}}
}

// ObserveRewrite appends to the in-memory log.
func (m *Metrics) ObserveRewrite(outcome domain.RewriteOutcome, tone domain.Tone, provider string, dur time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.observed = append(m.observed, ObservedRewrite{
		Outcome: outcome, Tone: tone, Provider: provider, Duration: dur,
	})
}

// AddTokensStreamed sums per-provider token counts.
func (m *Metrics) AddTokensStreamed(provider string, n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokens[provider] += n
}

// Observed returns a defensive copy of every captured call.
func (m *Metrics) Observed() []ObservedRewrite {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]ObservedRewrite, len(m.observed))
	copy(out, m.observed)
	return out
}

// TokensFor returns the running total for one provider.
func (m *Metrics) TokensFor(provider string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.tokens[provider]
}
