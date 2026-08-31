package telemetry

import (
	"context"
	"fmt"
	"os"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("realm-api")

// Tracer returns the global tracer for realm-api
func Tracer() trace.Tracer {
	return tracer
}

// InitTracer initializes OpenTelemetry TracerProvider and global propagators
func InitTracer(ctx context.Context, serviceName, environment string) (func(context.Context) error, error) {
	if serviceName == "" {
		serviceName = "realm-api"
	}
	if environment == "" {
		environment = "production"
	}

	res, err := sdkresource.Merge(
		sdkresource.Default(),
		sdkresource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion("1.0.0"),
			semconv.DeploymentEnvironmentNameKey.String(environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create otel resource: %w", err)
	}

	var exporter sdktrace.SpanExporter

	otlpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if otlpEndpoint != "" {
		opts := []otlptracehttp.Option{
			otlptracehttp.WithEndpoint(strings.TrimPrefix(strings.TrimPrefix(otlpEndpoint, "https://"), "http://")),
		}
		if !strings.HasPrefix(otlpEndpoint, "https://") {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		exporter, err = otlptracehttp.New(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create otlp trace exporter: %w", err)
		}
	} else if os.Getenv("OTEL_STDOUT_TRACING") == "true" {
		exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout trace exporter: %w", err)
		}
	}

	var bsp sdktrace.TracerProviderOption
	if exporter != nil {
		bsp = sdktrace.WithBatcher(exporter)
	} else {
		// No-op / memory sampler when no exporter is configured
		bsp = sdktrace.WithSampler(sdktrace.AlwaysSample())
	}

	tp := sdktrace.NewTracerProvider(
		bsp,
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	tracer = otel.Tracer("realm-api")

	return tp.Shutdown, nil
}
