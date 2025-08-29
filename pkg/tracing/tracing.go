
package tracing

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

// InitTracerProvider initializes a new Jaeger tracer provider and sets it as the global tracer.
// It returns a shutdown function that should be called on application exit.
func InitTracerProvider(serviceName, jaegerEndpoint string) (func(context.Context) error, error) {
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	var tracerProvider *sdktrace.TracerProvider

	if jaegerEndpoint != "" {
		// If a Jaeger endpoint is provided, configure and use the Jaeger exporter.
		exporter, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(jaegerEndpoint)))
		if err != nil {
			return nil, fmt.Errorf("failed to create jaeger exporter: %w", err)
		}

		tracerProvider = sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.AlwaysSample()),
			sdktrace.WithBatcher(exporter),
			sdktrace.WithResource(res),
		)
		klog.InfoS("Tracing enabled with Jaeger exporter", "endpoint", jaegerEndpoint)
	} else {
		// If no Jaeger endpoint is provided, create a provider without an exporter.
		// Traces will be generated but not exported.
		tracerProvider = sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.AlwaysSample()),
			sdktrace.WithResource(res),
		)
		klog.InfoS("Tracing enabled without an exporter. TraceIDs will be available in logs but not sent to a collector.")
	}

	// Set the global tracer provider.
	otel.SetTracerProvider(tracerProvider)

	return tracerProvider.Shutdown, nil
}
