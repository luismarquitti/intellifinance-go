package problem_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/problem"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
)

func TestProblemRender(t *testing.T) {
	t.Parallel()

	prob := problem.NewNotFound("/accounts/123", "Account not found")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/accounts/123", nil)

	problem.Render(rec, req, prob)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != problem.ContentTypeProblemJSON {
		t.Fatalf("expected Content-Type %s, got %s", problem.ContentTypeProblemJSON, contentType)
	}

	var parsed problem.ProblemDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to unmarshal problem JSON: %v", err)
	}

	if parsed.Status != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", parsed.Status)
	}
	if parsed.Title != "Resource Not Found" {
		t.Errorf("expected title 'Resource Not Found', got '%s'", parsed.Title)
	}
	if parsed.Detail != "Account not found" {
		t.Errorf("expected detail 'Account not found', got '%s'", parsed.Detail)
	}
	if parsed.Instance != "/accounts/123" {
		t.Errorf("expected instance '/accounts/123', got '%s'", parsed.Instance)
	}
	if parsed.Code != problem.CodeNotFound {
		t.Errorf("expected code '%s', got '%s'", problem.CodeNotFound, parsed.Code)
	}
	if parsed.Timestamp == "" {
		t.Error("expected non-empty timestamp")
	}
}

func TestFromDomainError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "NotFound",
			err:        domain.ErrNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   problem.CodeNotFound,
		},
		{
			name:       "Unauthorized",
			err:        domain.ErrUnauthorized,
			wantStatus: http.StatusUnauthorized,
			wantCode:   problem.CodeUnauthorized,
		},
		{
			name:       "DuplicateTransaction",
			err:        domain.ErrDuplicateTransaction,
			wantStatus: http.StatusConflict,
			wantCode:   problem.CodeDuplicateTransaction,
		},
		{
			name:       "InvalidAmount",
			err:        domain.ErrInvalidAmount,
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   problem.CodeValidationFailed,
		},
		{
			name:       "EmptyDescription",
			err:        domain.ErrEmptyDescription,
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   problem.CodeValidationFailed,
		},
		{
			name:       "UnknownError",
			err:        errors.New("something crashed"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   problem.CodeInternalError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := problem.FromDomainError("/test", tc.err)
			if p.Status != tc.wantStatus {
				t.Errorf("expected status %d, got %d", tc.wantStatus, p.Status)
			}
			if p.Code != tc.wantCode {
				t.Errorf("expected code %s, got %s", tc.wantCode, p.Code)
			}
		})
	}
}
