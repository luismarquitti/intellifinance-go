package ports

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrInvalidTaskPayload identifies permanent task input errors.
	ErrInvalidTaskPayload = errors.New("invalid task payload")
	// ErrMissingTenantID identifies a missing tenant boundary.
	ErrMissingTenantID = errors.New("missing tenant ID")
)

// IngestStatementPayload addresses a statement in shared storage.
type IngestStatementPayload struct {
	JobID          uuid.UUID    `json:"job_id"`
	TenantID       uuid.UUID    `json:"tenant_id"`
	AccountID      uuid.UUID    `json:"account_id"`
	StorageURI     string       `json:"storage_uri"`
	FileType       SourceFormat `json:"file_type"`
	SourceFilename string       `json:"source_filename"`
	UserID         uuid.UUID    `json:"user_id"`
	UploadedAt     time.Time    `json:"uploaded_at"`
}

// Validate checks the statement task before any application I/O.
func (p IngestStatementPayload) Validate() error {
	if err := validateTaskTenant(p.TenantID); err != nil {
		return err
	}
	if p.JobID == uuid.Nil || p.AccountID == uuid.Nil || p.UserID == uuid.Nil {
		return fmt.Errorf("%w: job_id, account_id and user_id must be nonzero UUIDs", ErrInvalidTaskPayload)
	}
	u, err := url.Parse(p.StorageURI)
	if err != nil {
		return fmt.Errorf("%w: malformed storage_uri", ErrInvalidTaskPayload)
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("%w: storage_uri must not contain credentials, query or fragment", ErrInvalidTaskPayload)
	}
	switch u.Scheme {
	case "gs", "s3":
		if u.Host == "" || strings.Trim(u.Path, "/ ") == "" {
			return fmt.Errorf("%w: storage_uri requires bucket and object", ErrInvalidTaskPayload)
		}
	case "file":
		if !strings.HasPrefix(u.Path, "/") || strings.Trim(u.Path, "/ ") == "" {
			return fmt.Errorf("%w: storage_uri requires an absolute file path", ErrInvalidTaskPayload)
		}
	default:
		return fmt.Errorf("%w: unsupported storage_uri scheme", ErrInvalidTaskPayload)
	}
	switch p.FileType {
	case SourceFormatOFX, SourceFormatCSV, SourceFormatExcel, SourceFormatPDF, "XLSX":
	default:
		return fmt.Errorf("%w: unsupported file_type", ErrInvalidTaskPayload)
	}
	if strings.TrimSpace(p.SourceFilename) == "" || p.UploadedAt.IsZero() {
		return fmt.Errorf("%w: source_filename and uploaded_at are required", ErrInvalidTaskPayload)
	}
	return nil
}

// SyncOpenFinancePayload requests a tenant-scoped banking synchronization.
type SyncOpenFinancePayload struct {
	ItemID      uuid.UUID `json:"item_id"`
	TenantID    uuid.UUID `json:"tenant_id"`
	SyncType    string    `json:"sync_type"`
	FromDate    string    `json:"from_date"`
	ToDate      string    `json:"to_date"`
	TriggeredBy string    `json:"triggered_by"`
}

// Validate checks identifiers, sync mode and the inclusive date range.
func (p SyncOpenFinancePayload) Validate() error {
	if err := validateTaskTenant(p.TenantID); err != nil {
		return err
	}
	if p.ItemID == uuid.Nil {
		return fmt.Errorf("%w: item_id must be a nonzero UUID", ErrInvalidTaskPayload)
	}
	if p.SyncType != "INCREMENTAL" && p.SyncType != "FULL" {
		return fmt.Errorf("%w: sync_type must be INCREMENTAL or FULL", ErrInvalidTaskPayload)
	}
	from, err := time.Parse(time.DateOnly, p.FromDate)
	if err != nil {
		return fmt.Errorf("%w: from_date must use YYYY-MM-DD", ErrInvalidTaskPayload)
	}
	to, err := time.Parse(time.DateOnly, p.ToDate)
	if err != nil {
		return fmt.Errorf("%w: to_date must use YYYY-MM-DD", ErrInvalidTaskPayload)
	}
	if from.After(to) || strings.TrimSpace(p.TriggeredBy) == "" {
		return fmt.Errorf("%w: date range or triggered_by is invalid", ErrInvalidTaskPayload)
	}
	return nil
}

// CategorizeTransactionPayload requests inference for a batch of transactions.
type CategorizeTransactionPayload struct {
	TenantID        uuid.UUID   `json:"tenant_id"`
	TransactionIDs  []uuid.UUID `json:"transaction_ids"`
	ModelPreference string      `json:"model_preference"`
	FallbackAllowed bool        `json:"fallback_allowed"`
}

// Validate rejects empty batches and nil transaction identifiers.
func (p CategorizeTransactionPayload) Validate() error {
	if err := validateTaskTenant(p.TenantID); err != nil {
		return err
	}
	if len(p.TransactionIDs) == 0 {
		return fmt.Errorf("%w: transaction_ids is required", ErrInvalidTaskPayload)
	}
	for _, id := range p.TransactionIDs {
		if id == uuid.Nil {
			return fmt.Errorf("%w: transaction_ids must contain nonzero UUIDs", ErrInvalidTaskPayload)
		}
	}
	return nil
}

func validateTaskTenant(id uuid.UUID) error {
	if id == uuid.Nil {
		return fmt.Errorf("%w: %w", ErrInvalidTaskPayload, ErrMissingTenantID)
	}
	return nil
}

// IngestStatementHandler is the application port for statement ingestion.
type IngestStatementHandler interface {
	IngestStatement(ctx context.Context, payload IngestStatementPayload) error
}

// SyncOpenFinanceHandler is the application port for banking synchronization.
type SyncOpenFinanceHandler interface {
	SyncOpenFinance(ctx context.Context, payload SyncOpenFinancePayload) error
}

// CategorizeTransactionHandler is the application port for batch inference.
type CategorizeTransactionHandler interface {
	CategorizeTransaction(ctx context.Context, payload CategorizeTransactionPayload) error
}
