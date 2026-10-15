package middleware

import (
	"log/slog"
	"net/http"
	"time"

	chiMiddleware "github.com/go-chi/chi/v5/middleware"
)

// Logger returns a structured logging middleware using log/slog.
// It records method, path, HTTP status, latency, request_id, client IP, and bytes written.
func Logger(logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := chiMiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()

			defer func() {
				duration := time.Since(start)
				reqID := chiMiddleware.GetReqID(r.Context())

				logger.LogAttrs(
					r.Context(),
					slog.LevelInfo,
					"HTTP request",
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.Int("status", ww.Status()),
					slog.Duration("latency", duration),
					slog.Int("bytes", ww.BytesWritten()),
					slog.String("request_id", reqID),
					slog.String("remote_addr", r.RemoteAddr),
				)
			}()

			next.ServeHTTP(ww, r)
		})
	}
}
