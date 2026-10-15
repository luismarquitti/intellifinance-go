package middleware

import (
	"net/http"

	chiMiddleware "github.com/go-chi/chi/v5/middleware"
)

// RealIP sets a http.Request's RemoteAddr to either X-Forwarded-For or X-Real-IP.
func RealIP(next http.Handler) http.Handler {
	//nolint:staticcheck // RealIP middleware is mandated by Issue #10 specification
	return chiMiddleware.RealIP(next)
}

