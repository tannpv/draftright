import { Injectable } from '@nestjs/common';
import {
  InjectMetric,
  makeCounterProvider,
  makeHistogramProvider,
} from '@willsoto/nestjs-prometheus';
import { Counter, Histogram } from 'prom-client';

/**
 * Prometheus metrics for the NestJS /rewrite path.
 *
 * Series mirror the Go /rewrite service exactly so a single Grafana
 * dashboard joins both backends under one panel set:
 *
 *   draftright_rewrite_requests_total{outcome, tone, provider}
 *   draftright_rewrite_duration_seconds{outcome, tone, provider}
 *
 * Cardinality budget:
 *   outcomes (~9) × tones (7) × providers (~5) ≈ 315 series per
 *   histogram. Well under any sane budget.
 *
 * `outcome` label values are enumerated below — same set as Go's
 * domain.RewriteOutcome so dashboards aggregate without translation.
 */
export const REWRITE_OUTCOMES = {
  ok: 'ok',
  rateLimited: 'rate_limited',
  quotaExceeded: 'quota_exceeded',
  userNotFound: 'user_not_found',
  invalidInput: 'invalid_input',
  providerFailed: 'provider_failed',
  providerTimeout: 'provider_timeout',
  clientGone: 'client_gone',
  internal: 'internal',
} as const;

export type RewriteOutcome = (typeof REWRITE_OUTCOMES)[keyof typeof REWRITE_OUTCOMES];

const REQUESTS_METRIC = 'draftright_rewrite_requests_total';
const DURATION_METRIC = 'draftright_rewrite_duration_seconds';

@Injectable()
export class RewriteMetricsService {
  constructor(
    @InjectMetric(REQUESTS_METRIC)
    private readonly requests: Counter<string>,
    @InjectMetric(DURATION_METRIC)
    private readonly duration: Histogram<string>,
  ) {}

  /**
   * Record one rewrite operation.  Bumps the counter + adds an
   * observation to the histogram in a single call so handlers don't
   * forget one of the two.
   */
  observe(args: {
    outcome: RewriteOutcome;
    tone: string;
    provider: string;
    durationMs: number;
  }): void {
    const labels = {
      outcome: args.outcome,
      tone: args.tone,
      provider: args.provider,
    };
    this.requests.inc(labels);
    this.duration.observe(labels, args.durationMs / 1000);
  }
}

/**
 * Provider factories the module passes to PrometheusModule.register().
 * Bucket choice tuned for LLM-shaped latency (50 ms steady at the low
 * end → 120 s tail under Ollama Cloud burst): 50 ms, 100 ms, 250 ms,
 * 500 ms, 1 s, 2 s, 5 s, 10 s, 30 s, 60 s, 120 s. Same buckets the
 * Go service uses.
 */
export const rewriteMetricsProviders = [
  makeCounterProvider({
    name: REQUESTS_METRIC,
    help: 'Total /rewrite calls served by the NestJS backend, labeled by terminal outcome, tone, and provider.',
    labelNames: ['outcome', 'tone', 'provider'],
  }),
  makeHistogramProvider({
    name: DURATION_METRIC,
    help: 'Latency of NestJS /rewrite handler from accept to response.',
    labelNames: ['outcome', 'tone', 'provider'],
    buckets: [0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 30, 60, 120],
  }),
];
