package observability

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"k8s.io/klog/v2"
)

// InitTracerProvider initializes a Jaeger-backed tracer provider and wires it as global.
// The returned shutdown function must be invoked during graceful termination.
func InitTracerProvider(serviceName, jaegerEndpoint string) (func(context.Context) error, error) {
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	var tp *sdktrace.TracerProvider
	if jaegerEndpoint != "" {
		exporter, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(jaegerEndpoint)))
		if err != nil {
			return nil, fmt.Errorf("failed to create jaeger exporter: %w", err)
		}
		tp = sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.AlwaysSample()),
			sdktrace.WithBatcher(exporter),
			sdktrace.WithResource(res),
		)
		klog.InfoS("Tracing enabled with Jaeger exporter", "endpoint", jaegerEndpoint)
	} else {
		tp = sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.AlwaysSample()),
			sdktrace.WithResource(res),
		)
		klog.InfoS("Tracing enabled without an exporter. TraceIDs available in logs only")
	}

	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}
