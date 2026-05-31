import { Module } from '@nestjs/common';
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
/**
 * Not @Global() — explicit imports keep DI graph greppable.  Each
 * consuming module (today: RewriteModule) lists MetricsModule in its
 * imports + Prometheus's PrometheusController stays mounted by virtue
 * of being inside this module's PrometheusModule.register() (the
 * root AppModule imports MetricsModule, which transitively mounts the
 * /metrics route).
 */
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
