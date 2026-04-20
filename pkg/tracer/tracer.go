// Package tracer provides OpenTelemetry distributed tracing initialization.
// Implements NFR-OBS-001: 100% of service requests instrumented with traces.
package tracer

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

// Config holds OpenTelemetry tracer configuration.
type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	OTLPEndpoint   string // e.g., "jaeger:4317" or "otel-collector:4317"
	SampleRate     float64
}

// Provider wraps the OpenTelemetry TracerProvider with graceful shutdown.
type Provider struct {
	tp     *sdktrace.TracerProvider
	tracer trace.Tracer
}

// InitTracer initializes OpenTelemetry tracing with OTLP gRPC exporter.
// Returns a Provider that must be shut down gracefully on service termination.
//
// Architecture: All traces propagate W3C TraceContext headers end-to-end
// through the API Gateway → Service → Database call chain, enabling
// full-stack distributed tracing as required by the BRD.
func InitTracer(ctx context.Context, cfg Config) (*Provider, error) {
	if cfg.SampleRate == 0 {
		cfg.SampleRate = 1.0 // Default: 100% sampling for dev/staging
	}
	if cfg.OTLPEndpoint == "" {
		cfg.OTLPEndpoint = "localhost:4317"
	}

	// Create OTLP gRPC exporter targeting Jaeger/OTel Collector
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
		otlptracegrpc.WithInsecure(), // Use TLS in production via config
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Define service resource attributes per OpenTelemetry semantic conventions
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			semconv.DeploymentEnvironment(cfg.Environment),
			attribute.String("service.team", "platform-engineering"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create TracerProvider with batch span processor for performance
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter,
			sdktrace.WithBatchTimeout(5*time.Second),
			sdktrace.WithMaxExportBatchSize(512),
			sdktrace.WithMaxQueueSize(2048),
		),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(cfg.SampleRate)),
	)

	// Set global TracerProvider and W3C TraceContext propagator
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &Provider{
		tp:     tp,
		tracer: tp.Tracer(cfg.ServiceName),
	}, nil
}

// Tracer returns the named tracer for creating spans.
func (p *Provider) Tracer() trace.Tracer {
	return p.tracer
}

// Shutdown gracefully flushes and shuts down the tracer provider.
// Must be called during service shutdown to ensure all traces are exported.
func (p *Provider) Shutdown(ctx context.Context) error {
	return p.tp.Shutdown(ctx)
}

// StartSpan is a convenience helper that starts a new span from context.
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return otel.Tracer("").Start(ctx, name, opts...)
}
