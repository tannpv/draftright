// Package tracing bootstraps OpenTelemetry. One Setup() call at boot
// installs a global TracerProvider that exports spans over OTLP HTTP.
// Shutdown() drains the buffer on graceful termination.
//
// When OTEL_EXPORTER_OTLP_ENDPOINT is empty, Setup is a no-op + the
// returned Shutdown is a no-op closer. Means we always import
// otelhttp in the router; spans just disappear when tracing is off.
//
// Why OTLP HTTP (not gRPC): one fewer protocol to allow through the
// VPS firewall + works with any OTel collector unchanged. gRPC is
// faster but only matters at >10k req/s.
package tracing

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Config is the input to Setup.
type Config struct {
	// Endpoint is OTEL_EXPORTER_OTLP_ENDPOINT (e.g.
	// "otel-collector.observability.svc:4318"). Scheme/path are
	// derived; pass host:port.
	Endpoint string

	// ServiceName populates service.name resource attribute.
	ServiceName string

	// SampleRatio: 1.0 = all spans, 0.1 = 10% sampling.
	SampleRatio float64
}

// Setup installs a global TracerProvider + propagator and returns a
// Shutdown closer. When cfg.Endpoint is empty, Setup is a no-op (the
// global tracer stays noop) and Shutdown does nothing.
func Setup(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return func(context.Context) error { return nil }, nil
	}

	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(cfg.Endpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("tracing: build exporter: %w", err)
	}

	res, err := sdkresource.Merge(
		sdkresource.Default(),
		sdkresource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("tracing: build resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter,
			sdktrace.WithBatchTimeout(5*time.Second),
		),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRatio))),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))

	return tp.Shutdown, nil
}
