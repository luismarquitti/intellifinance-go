package middleware

import (
	"context"
	"net/http"

	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

// HeaderRequestID defines the standard HTTP header for tracing requests.
const HeaderRequestID = "X-Request-ID"

type requestIDKeyType string

const requestIDKey requestIDKeyType = "request_id"

// WithRequestID injects a request ID string into ctx.
func WithRequestID(ctx context.Context, reqID string) context.Context {
	ctx = context.WithValue(ctx, requestIDKey, reqID)
	return context.WithValue(ctx, chiMiddleware.RequestIDKey, reqID)
}

// RequestIDFromContext retrieves the request ID from ctx.
func RequestIDFromContext(ctx context.Context) string {
	if val, ok := ctx.Value(requestIDKey).(string); ok && val != "" {
		return val
	}
	return chiMiddleware.GetReqID(ctx)
}

// RequestID middleware extracts or generates a unique request ID (UUIDv4) and sets
// both X-Request-ID response header and context value.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get(HeaderRequestID)
		if reqID == "" {
			reqID = r.Header.Get("X-Request-Id")
		}
		if reqID == "" {
			reqID = uuid.New().String()
		}

		w.Header().Set(HeaderRequestID, reqID)
		ctx := WithRequestID(r.Context(), reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
