package middleware

import (
	"crypto/subtle"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/config"
	"golang.org/x/crypto/bcrypt"
)

// PersonalAuthentication protects the complete router, including SPA and aliases.
// Only suitable for loopback behind an SSH tunnel; Basic credentials must not
// travel over an unencrypted network. Headers never establish identity.
func PersonalAuthentication(cfg *config.Config) func(http.Handler) http.Handler {
	valid := cfg.ValidateRuntime() == nil && cfg.PersonalTenantID != uuid.Nil
	u, _ := url.Parse(cfg.DatabaseURL)
	isQA := u != nil && u.Path == "/intellifinance_personal_qa" && u.Port() == "55434"
	host := "127.0.0.1:" + cfg.Port
	origin := "http://" + host
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Referrer-Policy", "no-referrer")
			if !valid {
				http.Error(w, "Personal configuration required", http.StatusServiceUnavailable)
				return
			}
			if r.Host != host || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
				http.Error(w, "Origin not allowed", http.StatusForbidden)
				return
			}
			givenOrigin := r.Header.Get("Origin")
			safe := r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions
			if (givenOrigin != "" && givenOrigin != origin) || (!safe && givenOrigin != origin) {
				http.Error(w, "Origin not allowed", http.StatusForbidden)
				return
			}
			username, password, ok := r.BasicAuth()
			if !ok || subtle.ConstantTimeCompare([]byte(username), []byte(cfg.PersonalUsername)) != 1 || bcrypt.CompareHashAndPassword([]byte(cfg.PersonalPasswordHash), []byte(password)) != nil {
				w.Header().Set("WWW-Authenticate", `Basic realm="IntelliFinance pessoal", charset="UTF-8"`)
				http.Error(w, "Authentication required", http.StatusUnauthorized)
				return
			}
			r.Header.Del(HeaderTenantID)
			r.Header.Del(HeaderUserID)
			path := strings.TrimSuffix(r.URL.Path, "/")
			if !isQA && (r.Method == http.MethodDelete || strings.HasSuffix(path, "/batch-delete")) {
				http.Error(w, "Automatic deletion is disabled in personal mode", http.StatusForbidden)
				return
			}
			if strings.HasSuffix(path, "/transactions/import/csv") {
				http.Error(w, "Personal imports require the provenance-preserving importer", http.StatusConflict)
				return
			}
			ctx := WithUserID(WithTenantID(r.Context(), cfg.PersonalTenantID), cfg.PersonalUserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
