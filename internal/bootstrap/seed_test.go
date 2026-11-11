package bootstrap

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/luismarquitti/intellifinance-go/internal/config"
)

func TestSeedRefusesPersonalBeforeDatabaseAccess(t *testing.T) {
	for _, env := range []string{"personal", "production"} {
		t.Run(env, func(t *testing.T) {
			defer func() {
				if recover() != nil {
					t.Fatal("seed touched database before refusing protected mode")
				}
			}()
			err := SeedDevAdmin(context.Background(), nil, &config.Config{Environment: env}, slog.New(slog.NewTextHandler(io.Discard, nil)))
			if err == nil {
				t.Fatal("seed must refuse non-development modes")
			}
		})
	}
}
