package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	inboundHTTP "github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http"
)

type readinessFunc func(context.Context) error

func (f readinessFunc) Ping(ctx context.Context) error { return f(ctx) }

func TestRouter_ReadinessChecksDependency(t *testing.T) {
	for _, healthy := range []bool{true, false} {
		t.Run(map[bool]string{true: "healthy", false: "degraded"}[healthy], func(t *testing.T) {
			called := false
			router := inboundHTTP.NewRouter(inboundHTTP.RouterConfig{
				Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
				ReadinessChecker: readinessFunc(func(ctx context.Context) error {
					called = true
					deadline, ok := ctx.Deadline()
					if !ok || time.Until(deadline) > 2*time.Second {
						t.Error("readiness must have a two-second deadline")
					}
					if !healthy {
						return errors.New("private database connection detail")
					}
					return nil
				}),
			})
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))
			if !called {
				t.Fatal("dependency not checked")
			}
			if healthy {
				if response.Code != http.StatusOK {
					t.Fatalf("got %d", response.Code)
				}
				var result struct {
					Status string            `json:"status"`
					Checks map[string]string `json:"checks"`
				}
				if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				if result.Status != "UP" || result.Checks["postgres"] != "UP" {
					t.Fatalf("unexpected report: %s", response.Body.String())
				}
			} else {
				if response.Code != http.StatusServiceUnavailable {
					t.Fatalf("got %d", response.Code)
				}
				if response.Header().Get("Content-Type") != "application/problem+json" {
					t.Fatal("expected RFC 7807")
				}
				if strings.Contains(response.Body.String(), "private database") {
					t.Fatal("dependency detail leaked")
				}
				live := httptest.NewRecorder()
				router.ServeHTTP(live, httptest.NewRequest(http.MethodGet, "/healthz", nil))
				if live.Code != http.StatusOK {
					t.Fatal("liveness must remain independent of the database")
				}
			}
		})
	}
}

func TestRouter_ReadinessWithoutDependency(t *testing.T) {
	router := inboundHTTP.NewRouter(inboundHTTP.RouterConfig{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured dependency must fail closed: got %d, body %s", response.Code, response.Body.String())
	}
}
