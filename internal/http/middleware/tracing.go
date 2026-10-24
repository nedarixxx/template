package middleware

import (
	"fmt"
	"net/http"

	chi "github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

func Tracing(serviceName string) func(http.Handler) http.Handler {
	tracer := otel.GetTracerProvider().Tracer(serviceName)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			routePattern := chi.RouteContext(r.Context()).RoutePattern()
			if routePattern == "" {
				routePattern = r.URL.Path
			}

			spanName := fmt.Sprintf("%s %s", r.Method, routePattern)

			ctx, span := tracer.Start(r.Context(), spanName,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					semconv.HTTPRequestMethodKey.String(r.Method),
					attribute.String("http.route", routePattern),
				),
			)
			defer span.End()

			ww := wrapResponseWriter(w)
			next.ServeHTTP(ww, r.WithContext(ctx))

			if updatedPattern := chi.RouteContext(r.Context()).RoutePattern(); updatedPattern != "" && updatedPattern != routePattern {
				span.SetName(fmt.Sprintf("%s %s", r.Method, updatedPattern))
				span.SetAttributes(attribute.String("http.route", updatedPattern))
			}

			if ww.status >= 500 {
				span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", ww.status))
				span.SetAttributes(attribute.String("error.type", fmt.Sprintf("%d", ww.status)))
			}
		})
	}
}