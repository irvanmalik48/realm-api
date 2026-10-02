package telemetry

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/irvanmalik48/realm-api/internal/database"
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

// DBLogSink wraps a handler and mirrors log records asynchronously to system_logs table
type DBLogSink struct {
	slog.Handler
	logChan chan dbLogItem
}

type dbLogItem struct {
	timestamp  time.Time
	level      string
	message    string
	traceID    string
	spanID     string
	attributes map[string]interface{}
}

func NewDBLogSink(base slog.Handler, db *database.DB) *DBLogSink {
	sink := &DBLogSink{
		Handler: base,
		logChan: make(chan dbLogItem, 1000),
	}

	go sink.worker(db)
	return sink
}

func (s *DBLogSink) Handle(ctx context.Context, r slog.Record) error {
	var traceID, spanID string
	attrs := make(map[string]interface{})

	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "trace_id" {
			traceID = a.Value.String()
		} else if a.Key == "span_id" {
			spanID = a.Value.String()
		} else {
			attrs[a.Key] = a.Value.Any()
		}
		return true
	})

	select {
	case s.logChan <- dbLogItem{
		timestamp:  r.Time,
		level:      r.Level.String(),
		message:    r.Message,
		traceID:    traceID,
		spanID:     spanID,
		attributes: attrs,
	}:
	default:
		// Drop log if buffer full to avoid blocking server
	}

	return s.Handler.Handle(ctx, r)
}

func (s *DBLogSink) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &DBLogSink{Handler: s.Handler.WithAttrs(attrs), logChan: s.logChan}
}

func (s *DBLogSink) WithGroup(name string) slog.Handler {
	return &DBLogSink{Handler: s.Handler.WithGroup(name), logChan: s.logChan}
}

func (s *DBLogSink) worker(db *database.DB) {
	query := `
		INSERT INTO system_logs (id, timestamp, level, message, trace_id, span_id, attributes)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)
	`
	for item := range s.logChan {
		attrJSON, err := json.Marshal(item.attributes)
		if err != nil {
			attrJSON = []byte("{}")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_, _ = db.Pool.Exec(ctx, query,
			uuid.New(), item.timestamp, item.level, item.message, item.traceID, item.spanID, string(attrJSON),
		)
		cancel()
	}
}

// InitLogger configures and returns the global slog logger with trace correlation.
func InitLogger(env string, db ...*database.DB) *slog.Logger {
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

	wrapped := NewTraceHandler(baseHandler)
	if len(db) > 0 && db[0] != nil {
		logger := slog.New(NewDBLogSink(wrapped, db[0]))
		slog.SetDefault(logger)
		return logger
	}

	logger := slog.New(wrapped)
	slog.SetDefault(logger)
	return logger
}
