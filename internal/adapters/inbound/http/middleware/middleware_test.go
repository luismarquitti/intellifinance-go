package middleware_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/middleware"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/problem"
)

func TestRequestIDMiddleware(t *testing.T) {
	t.Parallel()

	handler := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	handler.ServeHTTP(rec, req)

	reqID := rec.Header().Get("X-Request-ID")
	if reqID == "" {
		t.Error("expected X-Request-ID header to be set")
	}

	// Test preserving existing X-Request-ID
	customID := "custom-trace-1234"
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.Header.Set("X-Request-ID", customID)
	handler.ServeHTTP(rec2, req2)

	if rec2.Header().Get("X-Request-ID") != customID {
		t.Errorf("expected preserved X-Request-ID %s, got %s", customID, rec2.Header().Get("X-Request-ID"))
	}
}

func TestRecovererMiddleware(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	recoverer := middleware.Recoverer(logger)

	handler := recoverer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom unexpected database crash")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != problem.ContentTypeProblemJSON {
		t.Errorf("expected Content-Type %s, got %s", problem.ContentTypeProblemJSON, contentType)
	}

	var prob problem.ProblemDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &prob); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if prob.Status != http.StatusInternalServerError {
		t.Errorf("expected problem status 500, got %d", prob.Status)
	}
	if prob.Code != problem.CodeInternalError {
		t.Errorf("expected code %s, got %s", problem.CodeInternalError, prob.Code)
	}
}

func TestTenantContextMiddleware(t *testing.T) {
	t.Parallel()

	validTenantID := uuid.New()
	validUserID := uuid.New()

	var capturedTenantID uuid.UUID
	var capturedUserID uuid.UUID
	var called bool

	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		capturedTenantID, _ = middleware.TenantIDFromContext(r.Context())
		capturedUserID, _ = middleware.UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	mw := middleware.TenantContextMiddleware(innerHandler)

	t.Run("Missing X-Tenant-ID returns 401 Unauthorized", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts", nil)
		called = false

		mw.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", rec.Code)
		}
		if called {
			t.Error("inner handler should not have been called")
		}

		var prob problem.ProblemDetail
		if err := json.Unmarshal(rec.Body.Bytes(), &prob); err != nil {
			t.Fatalf("failed to unmarshal problem: %v", err)
		}
		if prob.Code != problem.CodeUnauthorized {
			t.Errorf("expected code %s, got %s", problem.CodeUnauthorized, prob.Code)
		}
	})

	t.Run("Invalid X-Tenant-ID returns 400 Bad Request", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts", nil)
		req.Header.Set(middleware.HeaderTenantID, "not-a-uuid")
		called = false

		mw.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rec.Code)
		}
		if called {
			t.Error("inner handler should not have been called")
		}

		var prob problem.ProblemDetail
		if err := json.Unmarshal(rec.Body.Bytes(), &prob); err != nil {
			t.Fatalf("failed to unmarshal problem: %v", err)
		}
		if prob.Code != problem.CodeBadRequest {
			t.Errorf("expected code %s, got %s", problem.CodeBadRequest, prob.Code)
		}
	})

	t.Run("Valid X-Tenant-ID and X-User-ID inject into context", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts", nil)
		req.Header.Set(middleware.HeaderTenantID, validTenantID.String())
		req.Header.Set(middleware.HeaderUserID, validUserID.String())
		called = false

		mw.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		if !called {
			t.Fatal("inner handler was not called")
		}
		if capturedTenantID != validTenantID {
			t.Errorf("expected tenant ID %s, got %s", validTenantID, capturedTenantID)
		}
		if capturedUserID != validUserID {
			t.Errorf("expected user ID %s, got %s", validUserID, capturedUserID)
		}
	})
}
