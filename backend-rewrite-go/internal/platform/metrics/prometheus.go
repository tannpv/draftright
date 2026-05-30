package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/tannpv/draftright-rewrite/internal/domain"
)

// Prometheus is the production domain.Metrics implementation.
//
// Series cardinality: outcome (~9) × tone (7) × provider (~5) ≈ 315
// time-series per histogram — well within Prometheus best practice
// (<10k per metric). Tags are deliberately enumerated; never accept
// free-form user input as a label value (cardinality explosion is
// the single biggest Prometheus footgun).
type Prometheus struct {
	rewriteCounter  *prometheus.CounterVec
	rewriteHist     *prometheus.HistogramVec
	tokensStreamed  *prometheus.CounterVec
	registry        *prometheus.Registry
}

// NewPrometheus builds + registers every metric on a fresh Registry.
// Returns the implementation + a Handler ready to mount at /metrics.
// Owning the registry (not using the default global) keeps tests
// isolated + leaves room to expose multiple registries later if we
// want internal-only metrics (Rule #1 — explicit ownership).
func NewPrometheus() *Prometheus {
	reg := prometheus.NewRegistry()
	p := &Prometheus{
		registry: reg,
		rewriteCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "draftright",
				Subsystem: "rewrite",
				Name:      "requests_total",
				Help:      "Total /v1/rewrite calls, labeled by terminal outcome, tone, and provider.",
			},
			[]string{"outcome", "tone", "provider"},
		),
		rewriteHist: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "draftright",
				Subsystem: "rewrite",
				Name:      "duration_seconds",
				Help:      "Latency of /v1/rewrite from first byte in to last byte out.",
				// Long tail because LLM streams run multi-second.
				Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 30, 60, 120},
			},
			[]string{"outcome", "tone", "provider"},
		),
		tokensStreamed: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "draftright",
				Subsystem: "rewrite",
				Name:      "tokens_streamed_total",
				Help:      "Tokens forwarded from upstream provider to client.",
			},
			[]string{"provider"},
		),
	}
	reg.MustRegister(p.rewriteCounter, p.rewriteHist, p.tokensStreamed)
	return p
}

// ObserveRewrite records one operation's terminal outcome + duration.
func (p *Prometheus) ObserveRewrite(outcome domain.RewriteOutcome, tone domain.Tone, provider string, dur time.Duration) {
	labels := prometheus.Labels{
		"outcome":  string(outcome),
		"tone":     string(tone),
		"provider": provider,
	}
	p.rewriteCounter.With(labels).Inc()
	p.rewriteHist.With(labels).Observe(dur.Seconds())
}

// AddTokensStreamed bumps a per-provider counter.
func (p *Prometheus) AddTokensStreamed(provider string, n int) {
	p.tokensStreamed.With(prometheus.Labels{"provider": provider}).Add(float64(n))
}

// Handler returns the /metrics http.Handler. Mount behind an internal
// listener or auth in production — Prometheus scrape data leaks
// timing + cardinality info.
func (p *Prometheus) Handler() http.Handler {
	return promhttp.HandlerFor(p.registry, promhttp.HandlerOpts{})
}
