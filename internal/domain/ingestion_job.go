package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// IngestionJob represents an asynchronous file ingestion task tracking status, metrics, and error logs.
type IngestionJob struct {
	ID                uuid.UUID      `json:"id"`
	TenantID          uuid.UUID      `json:"tenant_id"`
	AccountID         uuid.UUID      `json:"account_id"`
	InitiatedByUserID uuid.UUID      `json:"initiated_by_user_id"`
	Source            SourceType     `json:"source"`
	Status            JobStatus      `json:"status"`
	SourceFilename    string         `json:"source_filename"`
	FileStoragePath   string         `json:"file_storage_path"`
	FileMimeType      string         `json:"file_mime_type"`
	FileSizeBytes     int64          `json:"file_size_bytes"`
	TotalRecords      int            `json:"total_records"`
	InsertedRecords   int            `json:"inserted_records"`
	DuplicateRecords  int            `json:"duplicate_records"`
	FailedRecords     int            `json:"failed_records"`
	ErrorSummary      string         `json:"error_summary,omitempty"`
	ErrorDetails      map[string]any `json:"error_details,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
	CompletedAt       *time.Time     `json:"completed_at,omitempty"`
}

// NewIngestionJobParams contains configuration for instantiating an IngestionJob.
type NewIngestionJobParams struct {
	TenantID          uuid.UUID
	AccountID         uuid.UUID
	InitiatedByUserID uuid.UUID
	Source            SourceType
	SourceFilename    string
	FileStoragePath   string
	FileMimeType      string
	FileSizeBytes     int64
}

// NewIngestionJob constructs and validates a new pending IngestionJob entity.
func NewIngestionJob(params NewIngestionJobParams) (*IngestionJob, error) {
	now := time.Now().UTC()
	job := &IngestionJob{
		ID:                uuid.New(),
		TenantID:          params.TenantID,
		AccountID:         params.AccountID,
		InitiatedByUserID: params.InitiatedByUserID,
		Source:            params.Source,
		Status:            JobStatusPending,
		SourceFilename:    strings.TrimSpace(params.SourceFilename),
		FileStoragePath:   strings.TrimSpace(params.FileStoragePath),
		FileMimeType:      strings.TrimSpace(params.FileMimeType),
		FileSizeBytes:     params.FileSizeBytes,
		CreatedAt:         now,
	}

	if err := job.Validate(); err != nil {
		return nil, err
	}
	return job, nil
}

// Validate ensures ingestion job fields satisfy domain rules.
func (j *IngestionJob) Validate() error {
	if j.ID == uuid.Nil || j.TenantID == uuid.Nil || j.AccountID == uuid.Nil || j.InitiatedByUserID == uuid.Nil {
		return ErrInvalidID
	}
	if !j.Source.IsValid() {
		return ErrInvalidSource
	}
	if !j.Status.IsValid() {
		return ErrInvalidStatus
	}
	if strings.TrimSpace(j.SourceFilename) == "" {
		return ErrEmptyName
	}
	if strings.TrimSpace(j.FileStoragePath) == "" {
		return ErrEmptyDescription
	}
	if j.FileSizeBytes < 0 {
		return ErrInvalidAmount
	}
	if j.TotalRecords < 0 || j.InsertedRecords < 0 || j.DuplicateRecords < 0 || j.FailedRecords < 0 {
		return ErrInvalidAmount
	}
	return nil
}

// MarkProcessing transitions the job status to PROCESSING.
func (j *IngestionJob) MarkProcessing() error {
	if j.Status != JobStatusPending {
		return ErrInvalidStatus
	}
	j.Status = JobStatusProcessing
	return nil
}

// MarkCompleted sets metrics and transitions the job status to COMPLETED.
func (j *IngestionJob) MarkCompleted(total, inserted, duplicate, failed int) error {
	if j.Status != JobStatusProcessing {
		return ErrInvalidStatus
	}
	now := time.Now().UTC()
	j.Status = JobStatusCompleted
	j.TotalRecords = total
	j.InsertedRecords = inserted
	j.DuplicateRecords = duplicate
	j.FailedRecords = failed
	j.CompletedAt = &now
	return nil
}

// MarkFailed transitions the job to FAILED status and records error details.
func (j *IngestionJob) MarkFailed(summary string, details map[string]any) {
	now := time.Now().UTC()
	j.Status = JobStatusFailed
	j.ErrorSummary = strings.TrimSpace(summary)
	j.ErrorDetails = details
	j.CompletedAt = &now
}
