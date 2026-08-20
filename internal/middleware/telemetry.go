package middleware

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/irvanmalik48/realm-api/internal/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.30.0"
	"go.opentelemetry.io/otel/trace"
)

// fiberHeaderCarrier adapts fasthttp/fiber headers to OpenTelemetry TextMapCarrier
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

// OpenTelemetryTracing middleware instruments each incoming HTTP request with OpenTelemetry spans
func OpenTelemetryTracing() fiber.Handler {
	propagator := otel.GetTextMapPropagator()
	tracer := telemetry.Tracer()

	return func(c *fiber.Ctx) error {
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

		return err
	}
}
