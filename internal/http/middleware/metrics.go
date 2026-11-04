package middleware

import (
	"net/http"
	"strconv"
	"time"

	chi "github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func Metrics(activeRequests metric.Int64UpDownCounter, requestDuration metric.Float64Histogram) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == "" {
				route = r.URL.Path
			}

			baseAttrs := []attribute.KeyValue{
				attribute.String("http.request.method", r.Method),
				attribute.String("http.route", route),
			}

			if activeRequests != nil {
				activeRequests.Add(r.Context(), 1, metric.WithAttributes(baseAttrs...))
				defer activeRequests.Add(r.Context(), -1, metric.WithAttributes(baseAttrs...))
			}

			start := time.Now()
			ww := wrapResponseWriter(w)

			next.ServeHTTP(ww, r)

			durationSec := time.Since(start).Seconds()

			if updatedRoute := chi.RouteContext(r.Context()).RoutePattern(); updatedRoute != "" {
				route = updatedRoute
			}

			if requestDuration != nil {
				metricAttrs := []attribute.KeyValue{
					attribute.String("http.request.method", r.Method),
					attribute.String("http.route", route),
					attribute.String("http.response.status_code", strconv.Itoa(ww.status)),
				}

				requestDuration.Record(r.Context(), durationSec, metric.WithAttributes(metricAttrs...))
			}
		})
	}
}
