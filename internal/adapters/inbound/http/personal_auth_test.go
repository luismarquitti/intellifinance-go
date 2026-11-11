package http_test

import (
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	inbound "github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http"
	"github.com/luismarquitti/intellifinance-go/internal/config"
	"golang.org/x/crypto/bcrypt"
)

func TestPersonalAuthenticationAndOrigin(t *testing.T) {
	t.Setenv("ENVIRONMENT", "personal")
	t.Setenv("BIND_ADDR", "127.0.0.1")
	t.Setenv("PORT", "8092")
	t.Setenv("DATABASE_URL", "postgres://pilot@127.0.0.1:55433/intellifinance_personal")
	t.Setenv("PERSONAL_USERNAME", "pilot")
	hash, err := bcrypt.GenerateFromPassword([]byte("synthetic-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PERSONAL_PASSWORD_HASH", string(hash))
	t.Setenv("PERSONAL_TENANT_ID", "10000000-0000-0000-0000-000000000001")
	t.Setenv("PERSONAL_USER_ID", "10000000-0000-0000-0000-000000000002")
	router := inbound.NewRouter(inbound.RouterConfig{AppConfig: config.Load(), Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), AccountService: &dummyAccountService{}})
	cases := []struct {
		name, method, path, password, origin, host string
		status                                     int
	}{
		{"anonymous SPA", "GET", "/", "", "", "127.0.0.1:8092", 401},
		{"anonymous alias", "GET", "/session", "", "", "127.0.0.1:8092", 401},
		{"invalid password", "GET", "/api/v1/session", "wrong", "", "127.0.0.1:8092", 401},
		{"valid session", "GET", "/api/v1/session", "synthetic-password", "", "127.0.0.1:8092", 200},
		{"valid alias", "GET", "/session", "synthetic-password", "", "127.0.0.1:8092", 200},
		{"valid SPA", "GET", "/", "synthetic-password", "", "127.0.0.1:8092", 200},
		{"cross origin read", "GET", "/session", "synthetic-password", "https://evil.example", "127.0.0.1:8092", 403},
		{"cross origin write", "POST", "/accounts", "synthetic-password", "https://evil.example", "127.0.0.1:8092", 403},
		{"missing origin write", "POST", "/accounts", "synthetic-password", "", "127.0.0.1:8092", 403},
		{"same origin reaches handler", "POST", "/accounts", "synthetic-password", "http://127.0.0.1:8092", "127.0.0.1:8092", 400},
		{"DNS rebinding", "GET", "/session", "synthetic-password", "", "evil.example:8092", 403},
		{"personal deletion forbidden", "DELETE", "/api/v1/transactions/10000000-0000-0000-0000-000000000003", "synthetic-password", "http://127.0.0.1:8092", "127.0.0.1:8092", 403},
		{"personal batch deletion forbidden", "POST", "/transactions/batch-delete", "synthetic-password", "http://127.0.0.1:8092", "127.0.0.1:8092", 403},
		{"untraced legacy import forbidden", "POST", "/api/v1/transactions/import/csv", "synthetic-password", "http://127.0.0.1:8092", "127.0.0.1:8092", 409},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(c.method, c.path, nil)
			req.Host = c.host
			if c.password != "" {
				req.SetBasicAuth("pilot", c.password)
			}
			if c.origin != "" {
				req.Header.Set("Origin", c.origin)
			}
			req.Header.Set("X-Tenant-ID", config.DefaultDevTenantID.String())
			req.Header.Set("X-User-ID", config.DefaultDevUserID.String())
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != c.status {
				t.Fatalf("expected %d got %d", c.status, w.Code)
			}
			if c.status == 200 && c.path != "/" {
				if strings.Contains(w.Body.String(), "Synthetic") {
					t.Fatal("personal session must not claim synthetic development identity")
				}
				if !strings.Contains(w.Body.String(), "10000000-0000-0000-0000-000000000001") || strings.Contains(w.Body.String(), config.DefaultDevTenantID.String()) {
					t.Fatal("identity must be fixed, never headers")
				}
			}
		})
	}
}
