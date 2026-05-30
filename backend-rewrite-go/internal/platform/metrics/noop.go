// Package metrics implements the domain.Metrics port. Two
// implementations:
//
//   - Prometheus — production. Exposes /metrics for scraping.
//   - Noop       — default; used in tests + when prom is disabled.
//
// Domain stays vendor-free: callers depend on the port, not on
// prometheus.NewCounterVec. Swap is one line in composeDeps.
package metrics

import (
	"time"

	"github.com/tannpv/draftright-rewrite/internal/domain"
)

// Noop satisfies domain.Metrics with no side effects. Use when
// metrics are disabled or in unit tests that don't care.
type Noop struct{}

// NewNoop returns a zero-cost no-op metrics sink.
func NewNoop() *Noop { return &Noop{} }

// ObserveRewrite discards the call.
func (Noop) ObserveRewrite(_ domain.RewriteOutcome, _ domain.Tone, _ string, _ time.Duration) {
}

// AddTokensStreamed discards the call.
func (Noop) AddTokensStreamed(_ string, _ int) {}
