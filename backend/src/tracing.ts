/**
 * OpenTelemetry bootstrap for the NestJS backend.
 *
 * MUST be imported BEFORE any other application module so the SDK can
 * patch the http/express/typeorm hooks at require time. main.ts does
 * `import './tracing'` as its very first statement.
 *
 * Behaviour:
 *   - OTEL_EXPORTER_OTLP_ENDPOINT empty/unset → no SDK started; the
 *     global TracerProvider stays the no-op default and the
 *     application runs with zero tracing overhead.
 *   - OTEL_EXPORTER_OTLP_ENDPOINT set            → OTLP-HTTP exporter
 *     ships spans to that collector. ServiceName + version + env
 *     stamped as Resource attributes so multi-service traces in the
 *     collector are pre-filtered.
 *
 * Why OTLP-HTTP (not gRPC):
 *   matches the Go /rewrite service standard (see internal/platform/
 *   tracing/otel.go). One protocol allowed through the VPS firewall;
 *   one debug story across both backends.
 *
 * Sampling:
 *   parent-based, ratio configurable via OTEL_SAMPLE_RATIO (default 1.0).
 *   1.0 = trace every request; bring it down to ~0.1 when traffic
 *   grows past ~50 req/s sustained.
 *
 * Architecture standard S13.
 */
import { Logger } from '@nestjs/common';
import { NodeSDK } from '@opentelemetry/sdk-node';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-http';
import { getNodeAutoInstrumentations } from '@opentelemetry/auto-instrumentations-node';
import { resourceFromAttributes } from '@opentelemetry/resources';
import { TraceIdRatioBasedSampler, ParentBasedSampler } from '@opentelemetry/sdk-trace-base';
import { ATTR_SERVICE_NAME, ATTR_SERVICE_VERSION } from '@opentelemetry/semantic-conventions';

const logger = new Logger('Tracing');
const endpoint = (process.env.OTEL_EXPORTER_OTLP_ENDPOINT ?? '').trim();

let sdk: NodeSDK | null = null;

if (endpoint) {
  // OTEL_SAMPLE_RATIO is parsed + clamped by env.schema.ts (Zod) at
  // boot, but tracing.ts runs BEFORE NestJS's ConfigModule. Parse
  // defensively here too — fall back to 1.0 (every span) if missing
  // or malformed so the metric isn't silently lost.
  const rawRatio = parseFloat(process.env.OTEL_SAMPLE_RATIO ?? '1');
  const sampleRatio = Number.isFinite(rawRatio) && rawRatio >= 0 && rawRatio <= 1
    ? rawRatio
    : 1.0;

  sdk = new NodeSDK({
    resource: resourceFromAttributes({
      [ATTR_SERVICE_NAME]: 'draftright-backend',
      [ATTR_SERVICE_VERSION]: process.env.npm_package_version ?? 'dev',
      'deployment.environment': process.env.NODE_ENV ?? 'development',
    }),
    traceExporter: new OTLPTraceExporter({ url: `${endpoint}/v1/traces` }),
    // ParentBased honors upstream trace context (so a span from
    // Caddy → NestJS → Go stays in one trace); root spans use the
    // ratio sampler.  Same setup as the Go service in
    // internal/platform/tracing/otel.go for cross-service parity.
    sampler: new ParentBasedSampler({
      root: new TraceIdRatioBasedSampler(sampleRatio),
    }),
    // Auto-instrumentation covers http, express, typeorm, ioredis,
    // pg, and others out of the box. Specific instrumentation can be
    // added per-package later as needs surface.
    instrumentations: [getNodeAutoInstrumentations()],
  });

  try {
    sdk.start();
    logger.log(`OTel SDK started → ${endpoint} (sample ratio ${sampleRatio})`);
  } catch (err) {
    // Tracing is observability, not a hard dependency. A misconfigured
    // collector must NOT crash the backend.
    logger.error('failed to start OTel SDK; continuing without tracing', err as Error);
  }

  // Drain on shutdown so the last few spans actually leave the
  // process. SIGTERM is sent by Docker on `docker compose down`.
  const shutdown = async () => {
    try {
      await sdk?.shutdown();
    } catch {
      /* already shutting down */
    }
  };
  process.once('SIGTERM', shutdown);
  process.once('SIGINT', shutdown);
}

export { sdk };
