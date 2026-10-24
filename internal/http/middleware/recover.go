package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/nedarixxx/template/api"
)

func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					stack := string(debug.Stack())
					logger.ErrorContext(r.Context(), "panic recovered",
						slog.Any("error", rec),
						slog.String("stack", stack),
					)

					w.Header().Set("Content-Type", "application/problem+json")
					w.WriteHeader(http.StatusInternalServerError)
					detail := "Internal server error"
					path := r.URL.Path
					_ = json.NewEncoder(w).Encode(api.Problem{
						Type:     "https://tripgo.example/problems/internal_error",
						Title:    "Internal Server Error",
						Status:   http.StatusInternalServerError,
						Detail:   &detail,
						Code:     "internal_error",
						Instance: &path,
					})
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}