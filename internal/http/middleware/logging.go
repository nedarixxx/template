package middleware

import (
	"log/slog"
	"net/http"
	"time"

	chi "github.com/go-chi/chi/v5"
)

func Logging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := wrapResponseWriter(w)

			next.ServeHTTP(ww, r)

			duration := time.Since(start)
			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == "" {
				route = r.URL.Path
			}

			level := slog.LevelInfo
			if ww.status >= 500 {
				level = slog.LevelError
			}

			logger.Log(r.Context(), level, "request handled",
				slog.String("method", r.Method),
				slog.String("route", route),
				slog.Int("status", ww.status),
				slog.Int64("duration_ms", duration.Milliseconds()),
			)
		})
	}
}