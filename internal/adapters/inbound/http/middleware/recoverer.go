package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/problem"
)

// Recoverer handles panics from downstream handlers, logs stack traces, and returns
// an RFC 7807 500 Internal Server Error problem response.
func Recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func(ctx context.Context) {
				if rvr := recover(); rvr != nil {
					if rvr == http.ErrAbortHandler {
						panic(rvr)
					}

					stack := debug.Stack()
					logger.LogAttrs(
						ctx,
						slog.LevelError,
						"panic recovered in HTTP handler",
						slog.Any("error", rvr),
						slog.String("stack", string(stack)),
						slog.String("path", r.URL.Path),
						slog.String("method", r.Method),
					)

					prob := problem.NewInternalError(
						r.URL.Path,
						fmt.Sprintf("Internal server error: %v", rvr),
					)
					problem.Render(w, r, prob)
				}
			}(r.Context())

			next.ServeHTTP(w, r)
		})
	}
}
