package telemetry

import (
	"context"
	"io"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

type contextHandler struct {
	slog.Handler
}

func NewLogger(w io.Writer, level slog.Level, serviceName string) *slog.Logger {
	baseHandler := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: level,
	})

	handler := &contextHandler{
		Handler: baseHandler.WithAttrs([]slog.Attr{
			slog.String("service", serviceName),
		}),
	}

	return slog.New(handler)
}

func (h *contextHandler) Handle(ctx context.Context, r slog.Record) error {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		r.AddAttrs(
			slog.String("trace_id", span.SpanContext().TraceID().String()),
			slog.String("span_id", span.SpanContext().SpanID().String()),
		)
	}
	return h.Handler.Handle(ctx, r)
}