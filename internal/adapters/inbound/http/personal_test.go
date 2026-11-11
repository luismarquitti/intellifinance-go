package http_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	inbound "github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http"
	"github.com/luismarquitti/intellifinance-go/internal/config"
)

func TestPersonalRoutesFailClosedWithoutCredentials(t *testing.T) {
	t.Setenv("ENVIRONMENT", "personal")
	t.Setenv("PERSONAL_USERNAME", "")
	t.Setenv("PERSONAL_PASSWORD_HASH", "")
	router := inbound.NewRouter(inbound.RouterConfig{AppConfig: config.Load(), Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	for _, path := range []string{"/", "/api/v1/session", "/session", "/api/v1/accounts", "/accounts", "/transactions", "/categories", "/assets/app.js"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("X-Tenant-ID", config.DefaultDevTenantID.String())
			req.Header.Set("X-User-ID", config.DefaultDevUserID.String())
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != http.StatusServiceUnavailable {
				t.Fatalf("expected fail-closed 503, got %d", w.Code)
			}
		})
	}
}
