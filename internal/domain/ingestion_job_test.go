package domain_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
)

func TestNewIngestionJob_Success(t *testing.T) {
	tenantID := uuid.New()
	accountID := uuid.New()
	userID := uuid.New()

	job, err := domain.NewIngestionJob(domain.NewIngestionJobParams{
		TenantID:          tenantID,
		AccountID:         accountID,
		InitiatedByUserID: userID,
		Source:            domain.SourceTypeOFX,
		SourceFilename:    "extrato_itau.ofx",
		FileStoragePath:   "storage/statements/extrato_itau.ofx",
		FileMimeType:      "application/x-ofx",
		FileSizeBytes:     10240,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if job.ID == uuid.Nil {
		t.Error("expected valid job UUID")
	}
	if job.Status != domain.JobStatusPending {
		t.Errorf("expected initial status PENDING, got %s", job.Status)
	}
	if job.TotalRecords != 0 || job.InsertedRecords != 0 || job.DuplicateRecords != 0 || job.FailedRecords != 0 {
		t.Error("expected zero initial counters")
	}
	if job.CompletedAt != nil {
		t.Error("expected completedAt to be nil on pending job")
	}
}

func TestNewIngestionJob_ValidationErrors(t *testing.T) {
	tenantID := uuid.New()
	accountID := uuid.New()
	userID := uuid.New()

	tests := []struct {
		name        string
		params      domain.NewIngestionJobParams
		expectedErr error
	}{
		{
			name: "Nil tenant ID",
			params: domain.NewIngestionJobParams{
				TenantID:          uuid.Nil,
				AccountID:         accountID,
				InitiatedByUserID: userID,
				Source:            domain.SourceTypeOFX,
				SourceFilename:    "extrato.ofx",
				FileStoragePath:   "path/file.ofx",
			},
			expectedErr: domain.ErrInvalidID,
		},
		{
			name: "Nil account ID",
			params: domain.NewIngestionJobParams{
				TenantID:          tenantID,
				AccountID:         uuid.Nil,
				InitiatedByUserID: userID,
				Source:            domain.SourceTypeOFX,
				SourceFilename:    "extrato.ofx",
				FileStoragePath:   "path/file.ofx",
			},
			expectedErr: domain.ErrInvalidID,
		},
		{
			name: "Nil user ID",
			params: domain.NewIngestionJobParams{
				TenantID:          tenantID,
				AccountID:         accountID,
				InitiatedByUserID: uuid.Nil,
				Source:            domain.SourceTypeOFX,
				SourceFilename:    "extrato.ofx",
				FileStoragePath:   "path/file.ofx",
			},
			expectedErr: domain.ErrInvalidID,
		},
		{
			name: "Invalid source type",
			params: domain.NewIngestionJobParams{
				TenantID:          tenantID,
				AccountID:         accountID,
				InitiatedByUserID: userID,
				Source:            domain.SourceType("INVALID"),
				SourceFilename:    "extrato.ofx",
				FileStoragePath:   "path/file.ofx",
			},
			expectedErr: domain.ErrInvalidSource,
		},
		{
			name: "Empty filename",
			params: domain.NewIngestionJobParams{
				TenantID:          tenantID,
				AccountID:         accountID,
				InitiatedByUserID: userID,
				Source:            domain.SourceTypeOFX,
				SourceFilename:    "   ",
				FileStoragePath:   "path/file.ofx",
			},
			expectedErr: domain.ErrEmptyName,
		},
		{
			name: "Empty storage path",
			params: domain.NewIngestionJobParams{
				TenantID:          tenantID,
				AccountID:         accountID,
				InitiatedByUserID: userID,
				Source:            domain.SourceTypeOFX,
				SourceFilename:    "extrato.ofx",
				FileStoragePath:   "   ",
			},
			expectedErr: domain.ErrEmptyDescription,
		},
		{
			name: "Negative file size",
			params: domain.NewIngestionJobParams{
				TenantID:          tenantID,
				AccountID:         accountID,
				InitiatedByUserID: userID,
				Source:            domain.SourceTypeOFX,
				SourceFilename:    "extrato.ofx",
				FileStoragePath:   "path/file.ofx",
				FileSizeBytes:     -50,
			},
			expectedErr: domain.ErrInvalidAmount,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := domain.NewIngestionJob(tc.params)
			if !errors.Is(err, tc.expectedErr) {
				t.Errorf("expected error %v, got %v", tc.expectedErr, err)
			}
		})
	}
}

func TestIngestionJob_LifecycleTransitions(t *testing.T) {
	job, err := domain.NewIngestionJob(domain.NewIngestionJobParams{
		TenantID:          uuid.New(),
		AccountID:         uuid.New(),
		InitiatedByUserID: uuid.New(),
		Source:            domain.SourceTypeOFX,
		SourceFilename:    "statement.ofx",
		FileStoragePath:   "storage/statement.ofx",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 1. Cannot complete while pending
	if err := job.MarkCompleted(10, 8, 2, 0); !errors.Is(err, domain.ErrInvalidStatus) {
		t.Errorf("expected ErrInvalidStatus when completing pending job, got %v", err)
	}

	// 2. Transition to PROCESSING
	if err := job.MarkProcessing(); err != nil {
		t.Fatalf("unexpected error on MarkProcessing: %v", err)
	}
	if job.Status != domain.JobStatusProcessing {
		t.Errorf("expected status PROCESSING, got %s", job.Status)
	}

	// 3. Cannot mark processing again
	if err := job.MarkProcessing(); !errors.Is(err, domain.ErrInvalidStatus) {
		t.Errorf("expected ErrInvalidStatus when re-processing, got %v", err)
	}

	// 4. Mark COMPLETED
	if err := job.MarkCompleted(100, 95, 5, 0); err != nil {
		t.Fatalf("unexpected error on MarkCompleted: %v", err)
	}
	if job.Status != domain.JobStatusCompleted {
		t.Errorf("expected status COMPLETED, got %s", job.Status)
	}
	if job.TotalRecords != 100 || job.InsertedRecords != 95 || job.DuplicateRecords != 5 || job.FailedRecords != 0 {
		t.Error("metrics mismatch on completed job")
	}
	if job.CompletedAt == nil {
		t.Error("expected non-nil completedAt on completed job")
	}

	// 5. Failure transition
	job2, _ := domain.NewIngestionJob(domain.NewIngestionJobParams{
		TenantID:          uuid.New(),
		AccountID:         uuid.New(),
		InitiatedByUserID: uuid.New(),
		Source:            domain.SourceTypeCSV,
		SourceFilename:    "bad.csv",
		FileStoragePath:   "storage/bad.csv",
	})
	_ = job2.MarkProcessing()
	errDetails := map[string]any{"reason": "corrupted delimiter"}
	job2.MarkFailed("Corrupted CSV format", errDetails)

	if job2.Status != domain.JobStatusFailed {
		t.Errorf("expected status FAILED, got %s", job2.Status)
	}
	if job2.ErrorSummary != "Corrupted CSV format" {
		t.Errorf("expected error summary, got %q", job2.ErrorSummary)
	}
	if job2.ErrorDetails["reason"] != "corrupted delimiter" {
		t.Errorf("expected error details preserved")
	}
	if job2.CompletedAt == nil {
		t.Error("expected non-nil completedAt on failed job")
	}
}

func TestIngestionJob_Validate_NegativeCounters(t *testing.T) {
	baseJob := func() *domain.IngestionJob {
		return &domain.IngestionJob{
			ID:                uuid.New(),
			TenantID:          uuid.New(),
			AccountID:         uuid.New(),
			InitiatedByUserID: uuid.New(),
			Source:            domain.SourceTypeOFX,
			Status:            domain.JobStatusCompleted,
			SourceFilename:    "test.ofx",
			FileStoragePath:   "storage/test.ofx",
		}
	}

	j1 := baseJob()
	j1.TotalRecords = -1
	if err := j1.Validate(); !errors.Is(err, domain.ErrInvalidAmount) {
		t.Errorf("expected ErrInvalidAmount on negative TotalRecords, got %v", err)
	}

	j2 := baseJob()
	j2.InsertedRecords = -1
	if err := j2.Validate(); !errors.Is(err, domain.ErrInvalidAmount) {
		t.Errorf("expected ErrInvalidAmount on negative InsertedRecords, got %v", err)
	}

	j3 := baseJob()
	j3.DuplicateRecords = -1
	if err := j3.Validate(); !errors.Is(err, domain.ErrInvalidAmount) {
		t.Errorf("expected ErrInvalidAmount on negative DuplicateRecords, got %v", err)
	}

	j4 := baseJob()
	j4.FailedRecords = -1
	if err := j4.Validate(); !errors.Is(err, domain.ErrInvalidAmount) {
		t.Errorf("expected ErrInvalidAmount on negative FailedRecords, got %v", err)
	}
}
