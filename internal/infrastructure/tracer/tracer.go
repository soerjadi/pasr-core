package tracer

import (
	"context"
	"fmt"

	log "github.com/oemahdev/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/oemah/nusantara/core/internal/infrastructure/config"
)

const defaultName = "nusantara-core"

var tracer trace.Tracer = otel.Tracer(defaultName)

// Init registers a TracerProvider as the global OTel provider. When tracing is enabled, spans are
// exported via OTLP gRPC; otherwise spans are created in-process (so trace_id still lands in logs)
// but never exported. The returned provider must be shut down on exit.
func Init(ctx context.Context, cfg config.Tracing) (*sdktrace.TracerProvider, error) {
	if cfg.ServiceName == "" {
		cfg.ServiceName = defaultName
	}

	opts := []sdktrace.TracerProviderOption{}
	if cfg.Enabled {
		exp, err := otlptracegrpc.New(ctx,
			otlptracegrpc.WithEndpoint(cfg.Endpoint),
			otlptracegrpc.WithInsecure(),
		)
		if err != nil {
			return nil, fmt.Errorf("create OTLP exporter: %w", err)
		}
		opts = append(opts,
			sdktrace.WithBatcher(exp),
			sdktrace.WithResource(resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceName(cfg.ServiceName))),
		)
		log.Info("tracing: OTLP exporter configured", log.Field("endpoint", cfg.Endpoint))
	}

	tp := sdktrace.NewTracerProvider(opts...)
	otel.SetTracerProvider(tp)
	tracer = tp.Tracer(cfg.ServiceName)
	return tp, nil
}

func Start(ctx context.Context, spanName string) (context.Context, trace.Span) {
	return tracer.Start(ctx, spanName)
}

// RecordError records err on the span and marks its status as Error.
func RecordError(span trace.Span, err error) {
	if span != nil && err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}
