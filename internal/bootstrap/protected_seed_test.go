package bootstrap

import (
	"context"
	"github.com/luismarquitti/intellifinance-go/internal/config"
	"io"
	"log/slog"
	"testing"
)

func TestSeedRefusesProtectedDatabaseInDevelopment(t *testing.T) {
	defer func() {
		if recover() != nil {
			t.Fatal("seed touched protected database")
		}
	}()
	err := SeedDevAdmin(context.Background(), nil, &config.Config{Environment: "development", DatabaseURL: "postgres://app@127.0.0.1:55433/synthetic_test"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err == nil {
		t.Fatal("seed must refuse protected database even in development mode")
	}
}
