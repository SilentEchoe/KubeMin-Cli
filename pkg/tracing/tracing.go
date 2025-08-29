
package tracing

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

// InitTracerProvider initializes a new Jaeger tracer provider and sets it as the global tracer.
// It returns a shutdown function that should be called on application exit.
func InitTracerProvider(serviceName, jaegerEndpoint string) (func(context.Context) error, error) {
	// Create the Jaeger exporter
	exporter, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(jaegerEndpoint)))
	if err != nil {
		return nil, err
	}

	// Create a new tracer provider with a batch span processor and the Jaeger exporter.
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
		)),
	)

	// Set the global tracer provider.
	otel.SetTracerProvider(tp)

	return tp.Shutdown, nil
}
