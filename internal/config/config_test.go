package config_test

import (
	"testing"

	"github.com/luismarquitti/intellifinance-go/internal/config"
)

func TestLoadSafeLocalDefaults(t *testing.T) {
	t.Setenv("BIND_ADDR", "")
	t.Setenv("DATABASE_URL", "")
	cfg := config.Load()
	if cfg.BindAddr != "127.0.0.1" {
		t.Fatalf("admin bypass must default to loopback, got %q", cfg.BindAddr)
	}
	if cfg.DatabaseURL != "" {
		t.Fatal("database credentials must be explicitly configured, not embedded in code")
	}
}

func TestLoadExplicitLocalConfiguration(t *testing.T) {
	t.Setenv("BIND_ADDR", "127.0.0.2")
	t.Setenv("DATABASE_URL", "postgres://localhost/synthetic")
	cfg := config.Load()
	if cfg.BindAddr != "127.0.0.2" || cfg.DatabaseURL != "postgres://localhost/synthetic" {
		t.Fatal("explicit environment must be preserved")
	}
}
