package config_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/config"
	"golang.org/x/crypto/bcrypt"
)

func TestDisposableDatabaseGuard(t *testing.T) {
	cases := []struct {
		url, env string
		allowed  bool
	}{
		{"postgres://app@127.0.0.1:55432/synthetic_test", "development", true},
		{"postgres://app@127.0.0.1:55433/synthetic_test", "development", false},
		{"postgres://personal_app@127.0.0.1:55432/synthetic_test", "development", false},
		{"postgres://app@127.0.0.1:55432/intellifinance_personal_test", "development", false},
		{"postgres://app@127.0.0.1:55432/synthetic_test", "personal", false},
		{"postgres://app@remote.example/synthetic_test", "development", false},
		{"postgres://app@localhost/dev", "development", false},
		{"not-a-url", "development", false},
	}
	for _, c := range cases {
		t.Run(c.url+c.env, func(t *testing.T) {
			if (config.ValidateDisposableDatabase(c.url, c.env) == nil) != c.allowed {
				t.Fatal("incorrect disposable database decision")
			}
		})
	}
}

func TestRuntimeRefusesConflictingPersonalConfiguration(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("synthetic-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	valid := config.Config{Environment: "personal", BindAddr: "127.0.0.1", Port: "8092", DatabaseURL: "postgres://personal_app@127.0.0.1:55433/intellifinance_personal", PersonalUsername: "pilot", PersonalPasswordHash: string(hash), PersonalTenantID: uuid.New(), PersonalUserID: uuid.New()}
	if err := valid.ValidateRuntime(); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		mutate func(*config.Config)
	}{
		{"dev mode", func(c *config.Config) { c.Environment = "development" }},
		{"remote bind", func(c *config.Config) { c.BindAddr = "0.0.0.0" }},
		{"missing hash", func(c *config.Config) { c.PersonalPasswordHash = "" }},
		{"missing identity", func(c *config.Config) { c.PersonalTenantID = uuid.Nil }},
		{"dev identity", func(c *config.Config) { c.PersonalTenantID = config.DefaultDevTenantID }},
		{"dev database", func(c *config.Config) { c.DatabaseURL = "postgres://app@127.0.0.1:55432/intellifinance_dev" }},
		{"dev port", func(c *config.Config) { c.Port = "8090" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := valid
			tc.mutate(&c)
			if c.ValidateRuntime() == nil {
				t.Fatal("unsafe runtime accepted")
			}
		})
	}
}
