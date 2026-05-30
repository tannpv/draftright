import { Global, Module } from '@nestjs/common';
import { PrometheusModule } from '@willsoto/nestjs-prometheus';
import {
  RewriteMetricsService,
  rewriteMetricsProviders,
} from './rewrite-metrics.service';

/**
 * Application-wide Prometheus metrics.
 *
 *   /metrics                Prom text-exposition format, served by
 *                           PrometheusModule.  Gate access at Caddy
 *                           (internal CIDR or auth) — scraping data
 *                           leaks request shapes + timing distributions.
 *
 *   RewriteMetricsService   Single point that emits the rewrite
 *                           counters + histograms.  Other services
 *                           inject it.
 *
 * Global so domain modules don't need to re-import. Mirrors the Go
 * service's metrics layer for cross-backend Grafana parity.
 */
@Global()
@Module({
  imports: [
    PrometheusModule.register({
      // Default path is /metrics; we keep it explicit so the route
      // shows up in grep and is easy to change later if it ever
      // needs to move behind a /internal/ prefix.
      path: '/metrics',
      // We supply our own metrics + don't want the default Node.js
      // runtime metrics polluting the namespace until we explicitly
      // turn them on.
      defaultMetrics: { enabled: false },
    }),
  ],
  providers: [...rewriteMetricsProviders, RewriteMetricsService],
  exports: [RewriteMetricsService],
})
export class MetricsModule {}
