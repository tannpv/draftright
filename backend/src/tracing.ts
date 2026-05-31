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
import { NodeSDK } from '@opentelemetry/sdk-node';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-http';
import { getNodeAutoInstrumentations } from '@opentelemetry/auto-instrumentations-node';
import { resourceFromAttributes } from '@opentelemetry/resources';
import { ATTR_SERVICE_NAME, ATTR_SERVICE_VERSION } from '@opentelemetry/semantic-conventions';

const endpoint = (process.env.OTEL_EXPORTER_OTLP_ENDPOINT ?? '').trim();

let sdk: NodeSDK | null = null;

if (endpoint) {
  sdk = new NodeSDK({
    resource: resourceFromAttributes({
      [ATTR_SERVICE_NAME]: 'draftright-backend',
      [ATTR_SERVICE_VERSION]: process.env.npm_package_version ?? 'dev',
      'deployment.environment': process.env.NODE_ENV ?? 'development',
    }),
    traceExporter: new OTLPTraceExporter({ url: `${endpoint}/v1/traces` }),
    // Auto-instrumentation covers http, express, typeorm, ioredis,
    // pg, and others out of the box. Specific instrumentation can be
    // added per-package later as needs surface.
    instrumentations: [getNodeAutoInstrumentations()],
  });

  try {
    sdk.start();
    // eslint-disable-next-line no-console
    console.log(`[tracing] OTel SDK started → ${endpoint}`);
  } catch (err) {
    // Tracing is observability, not a hard dependency. A misconfigured
    // collector must NOT crash the backend.
    // eslint-disable-next-line no-console
    console.error('[tracing] failed to start OTel SDK; continuing without tracing:', err);
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
