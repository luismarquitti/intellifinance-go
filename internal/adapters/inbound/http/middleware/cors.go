package middleware

import (
	"net/http"

	"github.com/go-chi/cors"
)

// DefaultCORSOptions returns sensible, production-ready CORS configuration options.
func DefaultCORSOptions() cors.Options {
	return cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID", "X-Tenant-ID", "X-User-ID", "Idempotency-Key"},
		ExposedHeaders:   []string{"Link", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}
}

// CORS returns a middleware configured with the specified CORS options or default options.
func CORS(opts ...cors.Options) func(http.Handler) http.Handler {
	options := DefaultCORSOptions()
	if len(opts) > 0 {
		options = opts[0]
	}
	return cors.Handler(options)
}
