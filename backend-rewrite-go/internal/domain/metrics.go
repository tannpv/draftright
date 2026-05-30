package domain

import "time"

// RewriteOutcome is the terminal label for one rewrite operation.
// Cardinality is bounded + stable so it's safe to attach to a
// Prometheus counter without exploding the time-series count
// (cardinality discipline = the most common metric mistake).
type RewriteOutcome string

const (
	OutcomeOK              RewriteOutcome = "ok"
	OutcomeRateLimited     RewriteOutcome = "rate_limited"
	OutcomeQuotaExceeded   RewriteOutcome = "quota_exceeded"
	OutcomeUserNotFound    RewriteOutcome = "user_not_found"
	OutcomeInvalidInput    RewriteOutcome = "invalid_input"
	OutcomeProviderFailed  RewriteOutcome = "provider_failed"
	OutcomeProviderTimeout RewriteOutcome = "provider_timeout"
	OutcomeClientGone      RewriteOutcome = "client_gone"
	OutcomeInternal        RewriteOutcome = "internal"
)

// Metrics is the port the use case + handler call to record telemetry.
// One method per metric category — keeps the surface small + makes
// the no-op implementation trivial. Implementations:
//
//   - platform/metrics/Prometheus  — production
//   - platform/metrics/Noop        — default, used in tests + when
//                                    PROMETHEUS_ENABLED is unset
//
// Domain doesn't depend on Prometheus directly — that would drag a
// vendor lib into the innermost layer. The port stays here; the
// implementation sits in platform/ (Rule #1 — dependency inversion).
type Metrics interface {
	// ObserveRewrite records one rewrite operation's outcome,
	// elapsed time, tone label, and the provider (or chain name)
	// that produced it.
	ObserveRewrite(outcome RewriteOutcome, tone Tone, provider string, duration time.Duration)

	// AddTokensStreamed bumps a counter by n. Useful to track
	// upstream usage cost in aggregate without per-request labels.
	AddTokensStreamed(provider string, n int)
}
