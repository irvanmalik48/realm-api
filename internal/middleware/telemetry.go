package middleware

import (
	"fmt"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/irvanmalik48/realm-api/internal/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	httpRequestsTotal   metric.Int64Counter
	httpRequestDuration metric.Float64Histogram
	metricsOnce         sync.Once
)

func getHTTPMetrics() (metric.Int64Counter, metric.Float64Histogram) {
	metricsOnce.Do(func() {
		m := telemetry.Meter()
		var err error
		httpRequestsTotal, err = m.Int64Counter(
			"http.server.request.total",
			metric.WithDescription("Total number of HTTP requests processed"),
			metric.WithUnit("{request}"),
		)
		if err != nil {
			httpRequestsTotal = nil
		}

		httpRequestDuration, err = m.Float64Histogram(
			"http.server.request.duration",
			metric.WithDescription("Duration of HTTP requests in seconds"),
			metric.WithUnit("s"),
		)
		if err != nil {
			httpRequestDuration = nil
		}
	})
	return httpRequestsTotal, httpRequestDuration
}

// fiberHeaderCarrier adapts fasthttp/fiber headers to OpenTelemetry TextMapCarrier.
type fiberHeaderCarrier struct {
	ctx *fiber.Ctx
}

func (c fiberHeaderCarrier) Get(key string) string {
	return c.ctx.Get(key)
}

func (c fiberHeaderCarrier) Set(key, value string) {
	c.ctx.Set(key, value)
}

func (c fiberHeaderCarrier) Keys() []string {
	var keys []string
	for key := range c.ctx.Request().Header.All() {
		keys = append(keys, string(key))
	}
	return keys
}

// OpenTelemetryTracing middleware instruments each incoming HTTP request with OpenTelemetry spans and metrics.
func OpenTelemetryTracing() fiber.Handler {
	propagator := otel.GetTextMapPropagator()
	tracer := telemetry.Tracer()
	requestsCounter, durationHistogram := getHTTPMetrics()

	return func(c *fiber.Ctx) error {
		start := time.Now()

		// Extract trace context from incoming HTTP headers
		ctx := propagator.Extract(c.Context(), fiberHeaderCarrier{ctx: c})

		routePath := c.Route().Path
		if routePath == "" {
			routePath = c.Path()
		}
		spanName := fmt.Sprintf("HTTP %s %s", c.Method(), routePath)

		opts := []trace.SpanStartOption{
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				semconv.HTTPRequestMethodKey.String(c.Method()),
				semconv.URLPath(c.Path()),
				semconv.URLScheme(c.Protocol()),
				semconv.UserAgentOriginal(c.Get("User-Agent")),
				semconv.ClientAddress(c.IP()),
				attribute.String("http.route", routePath),
			),
		}

		ctx, span := tracer.Start(ctx, spanName, opts...)
		defer span.End()

		// Pass trace context down into Fiber's context
		c.SetUserContext(ctx)

		// Set standard X-Trace-Id header on response
		if span.SpanContext().HasTraceID() {
			c.Set("X-Trace-Id", span.SpanContext().TraceID().String())
		}

		// Execute downstream handlers
		err := c.Next()
		durationSec := time.Since(start).Seconds()

		statusCode := c.Response().StatusCode()
		span.SetAttributes(semconv.HTTPResponseStatusCode(statusCode))

		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else if statusCode >= 400 && statusCode < 600 {
			if statusCode >= 500 {
				span.SetStatus(codes.Error, fmt.Sprintf("HTTP Error %d", statusCode))
			}
		} else {
			span.SetStatus(codes.Ok, "")
		}

		// Record HTTP request metrics
		metricAttrs := metric.WithAttributes(
			semconv.HTTPRequestMethodKey.String(c.Method()),
			semconv.HTTPResponseStatusCode(statusCode),
			attribute.String("http.route", routePath),
		)
		if requestsCounter != nil {
			requestsCounter.Add(ctx, 1, metricAttrs)
		}
		if durationHistogram != nil {
			durationHistogram.Record(ctx, durationSec, metricAttrs)
		}

		return err
	}
}
