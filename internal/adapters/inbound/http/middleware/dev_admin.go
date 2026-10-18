package middleware

import (
	"net/http"

	"github.com/google/uuid"
)

// DevAdminFallbackMiddleware automatically injects default dev tenant and user IDs into the context
// when X-Tenant-ID header is omitted in development mode.
func DevAdminFallbackMiddleware(isDev bool, defaultTenantID, defaultUserID uuid.UUID) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isDev && r.Header.Get(HeaderTenantID) == "" {
				ctx := WithTenantID(r.Context(), defaultTenantID)
				ctx = WithUserID(ctx, defaultUserID)
				// Also set the header so downstream handlers and logs see it
				r.Header.Set(HeaderTenantID, defaultTenantID.String())
				r.Header.Set(HeaderUserID, defaultUserID.String())
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
