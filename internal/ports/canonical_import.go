package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
)

// CanonicalImportRequest is an explicit, local source mapping, not an approval
// of financial semantics. Supported canonical CSVs contain cash magnitudes with
// explicit Receita/Despesa types; invoices, payroll and obligations need review.
type CanonicalImportRequest struct {
	TenantID   uuid.UUID
	UserID     uuid.UUID
	AccountID  uuid.UUID
	SourceName string
	Content    []byte
}

// CanonicalStatementParser validates a source without skipping malformed records.
type CanonicalStatementParser interface {
	ParseCanonical(context.Context, CanonicalImportRequest) ([]*domain.Transaction, error)
}

// CanonicalImportResult reports committed counts, not preflight guesses.
type CanonicalImportResult struct {
	Parsed     int `json:"parsed"`
	Inserted   int `json:"inserted"`
	Duplicates int `json:"duplicates"`
}

// CanonicalImportRepository must serialize imports to an account, retain
// immutable provenance and reject conflicting original IDs atomically.
type CanonicalImportRepository interface {
	ImportCanonical(context.Context, []*domain.Transaction) (*CanonicalImportResult, error)
}

// SourceArchive retains exact original bytes privately without overwrite.
type SourceArchive interface {
	Retain(context.Context, []byte) (string, error)
}
