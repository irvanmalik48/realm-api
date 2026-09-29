package telemetry

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel/trace"
)

// TraceHandler wraps a slog.Handler and automatically enriches log records
// with OpenTelemetry trace_id and span_id when a valid span is in context.
type TraceHandler struct {
	slog.Handler
}

// NewTraceHandler wraps an existing slog.Handler with OpenTelemetry trace injection.
func NewTraceHandler(h slog.Handler) *TraceHandler {
	return &TraceHandler{Handler: h}
}

func (h *TraceHandler) Handle(ctx context.Context, r slog.Record) error {
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		r.AddAttrs(
			slog.String("trace_id", span.SpanContext().TraceID().String()),
			slog.String("span_id", span.SpanContext().SpanID().String()),
		)
	}
	return h.Handler.Handle(ctx, r)
}

func (h *TraceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &TraceHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *TraceHandler) WithGroup(name string) slog.Handler {
	return &TraceHandler{Handler: h.Handler.WithGroup(name)}
}

// InitLogger configures and returns the global slog logger with trace correlation.
func InitLogger(env string) *slog.Logger {
	var baseHandler slog.Handler
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}

	if env == "development" {
		opts.Level = slog.LevelDebug
		baseHandler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		baseHandler = slog.NewJSONHandler(os.Stdout, opts)
	}

	logger := slog.New(NewTraceHandler(baseHandler))
	slog.SetDefault(logger)
	return logger
}
