package config

import (
	"errors"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// ProtectedDatabase recognizes the reserved personal namespace even when mode
// is accidentally changed. A database marker and SQL grants add defense in depth.
func ProtectedDatabase(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return true
	}
	return u.Port() == "55433" || strings.Contains(strings.ToLower(u.Path), "personal") || (u.User != nil && strings.Contains(strings.ToLower(u.User.Username()), "personal"))
}

// ValidateDisposableDatabase must run before tests open any database connection.
func ValidateDisposableDatabase(raw, environment string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || environment == "personal" || ProtectedDatabase(raw) || !strings.HasSuffix(u.Path, "_test") || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost" && u.Hostname() != "::1") {
		return errors.New("tests refuse protected databases; require isolated local *_test")
	}
	return nil
}

// ValidateRuntime rejects ambiguous mode/identity/database combinations without
// ever including a URL, username, hash or password in an error.
func (c *Config) ValidateRuntime() error {
	if c.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	if c.Environment != "personal" {
		if ProtectedDatabase(c.DatabaseURL) || c.PersonalUsername != "" || c.PersonalPasswordHash != "" {
			return errors.New("protected configuration cannot run outside personal mode")
		}
		return nil
	}
	u, err := url.Parse(c.DatabaseURL)
	if err != nil || u.Hostname() != "127.0.0.1" || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		return errors.New("personal database must be explicit loopback")
	}
	personal := u.Path == "/intellifinance_personal" && u.Port() == "55433"
	qa := u.Path == "/intellifinance_personal_qa" && u.Port() == "55434"
	port, err := strconv.Atoi(c.Port)
	if (!personal && !qa) || err != nil || port < 1024 || port > 65535 || c.Port == "8090" || c.BindAddr != "127.0.0.1" {
		return errors.New("personal runtime requires reserved database and separate loopback listener")
	}
	_, err = bcrypt.Cost([]byte(c.PersonalPasswordHash))
	if err != nil || c.PersonalUsername == "" || c.PersonalTenantID == uuid.Nil || c.PersonalUserID == uuid.Nil || c.PersonalTenantID == DefaultDevTenantID || c.PersonalUserID == DefaultDevUserID {
		return errors.New("personal authentication and non-development fixed identity required")
	}
	return nil
}
