package config

import (
	"os"
	"strings"

	"github.com/google/uuid"
)

// Default fixed identifiers for local development / admin bypass
var (
	DefaultDevTenantID = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	DefaultDevUserID   = uuid.MustParse("00000000-0000-0000-0000-000000000002")
)

// Config holds environment and service configuration for IntelliFinance.
type Config struct {
	Port            string
	BindAddr        string
	Environment     string
	DatabaseURL     string
	DevTenantID     uuid.UUID
	DevUserID       uuid.UUID
	DevAdminEmail   string
	DevAdminName    string
	DevTenantName   string
	AllowedOrigins  []string
}

// Load loads configuration from environment variables with sensible homelab development defaults.
func Load() *Config {
	port := getEnv("PORT", "8090")
	bindAddr := getEnv("BIND_ADDR", "0.0.0.0")
	env := strings.ToLower(getEnv("ENVIRONMENT", "development"))

	dbURL := getEnv("DATABASE_URL", "postgres://intellifinance_app:CHANGE_IN_PRODUCTION@192.168.1.10:5432/intellifinance?sslmode=disable")

	tenantIDStr := getEnv("DEV_TENANT_ID", DefaultDevTenantID.String())
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		tenantID = DefaultDevTenantID
	}

	userIDStr := getEnv("DEV_USER_ID", DefaultDevUserID.String())
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		userID = DefaultDevUserID
	}

	originsStr := getEnv("CORS_ALLOWED_ORIGINS", "*")
	var origins []string
	if originsStr == "*" {
		origins = []string{"*"}
	} else {
		for _, o := range strings.Split(originsStr, ",") {
			if trimmed := strings.TrimSpace(o); trimmed != "" {
				origins = append(origins, trimmed)
			}
		}
	}

	return &Config{
		Port:           port,
		BindAddr:       bindAddr,
		Environment:    env,
		DatabaseURL:    dbURL,
		DevTenantID:    tenantID,
		DevUserID:      userID,
		DevAdminEmail:  getEnv("DEV_ADMIN_EMAIL", "admin@intellifinance.local"),
		DevAdminName:   getEnv("DEV_ADMIN_NAME", "Luis Marquitti (Admin)"),
		DevTenantName:  getEnv("DEV_TENANT_NAME", "Familia Dev"),
		AllowedOrigins: origins,
	}
}

// IsDevelopment returns true if running in development mode.
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development" || c.Environment == "dev" || c.Environment == "local"
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val)
	}
	return defaultVal
}
