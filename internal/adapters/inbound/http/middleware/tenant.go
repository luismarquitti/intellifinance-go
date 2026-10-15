package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/problem"
)

type contextKey string

const (
	tenantIDKey contextKey = "tenant_id"
	userIDKey   contextKey = "user_id"
)

// Header names for tenant and user context.
const (
	HeaderTenantID = "X-Tenant-ID"
	HeaderUserID   = "X-User-ID"
)

// WithTenantID injects a tenant UUID into the context.
func WithTenantID(ctx context.Context, tenantID uuid.UUID) context.Context {
	return context.WithValue(ctx, tenantIDKey, tenantID)
}

// TenantIDFromContext extracts the tenant UUID from the context if present.
func TenantIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	val, ok := ctx.Value(tenantIDKey).(uuid.UUID)
	return val, ok && val != uuid.Nil
}

// MustTenantIDFromContext returns the tenant UUID from context or uuid.Nil.
func MustTenantIDFromContext(ctx context.Context) uuid.UUID {
	val, _ := TenantIDFromContext(ctx)
	return val
}

// WithUserID injects a user UUID into the context.
func WithUserID(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// UserIDFromContext extracts the user UUID from the context if present.
func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	val, ok := ctx.Value(userIDKey).(uuid.UUID)
	return val, ok && val != uuid.Nil
}

// TenantContextMiddleware extracts and enforces tenant identity from the X-Tenant-ID HTTP header.
// If missing, it returns 401 Unauthorized; if invalid UUID, it returns 400 Bad Request.
// It also extracts X-User-ID if present.
func TenantContextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantHeader := r.Header.Get(HeaderTenantID)
		if tenantHeader == "" {
			prob := problem.NewUnauthorized(
				r.URL.Path,
				"Missing required X-Tenant-ID header.",
			)
			problem.Render(w, r, prob)
			return
		}

		tenantID, err := uuid.Parse(tenantHeader)
		if err != nil || tenantID == uuid.Nil {
			prob := problem.NewBadRequest(
				r.URL.Path,
				"Invalid X-Tenant-ID header: must be a valid non-nil UUID.",
			)
			problem.Render(w, r, prob)
			return
		}

		ctx := WithTenantID(r.Context(), tenantID)

		userHeader := r.Header.Get(HeaderUserID)
		if userHeader != "" {
			if userID, err := uuid.Parse(userHeader); err == nil && userID != uuid.Nil {
				ctx = WithUserID(ctx, userID)
			}
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
