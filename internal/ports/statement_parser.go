package ports

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// SourceFormat identifies the file format of a financial statement.
type SourceFormat string

const (
	// SourceFormatOFX represents Open Financial Exchange statement files (.ofx).
	SourceFormatOFX SourceFormat = "OFX"
	// SourceFormatCSV represents Comma-Separated Values files (.csv).
	SourceFormatCSV SourceFormat = "CSV"
	// SourceFormatExcel represents Microsoft Excel spreadsheets (.xlsx).
	SourceFormatExcel SourceFormat = "EXCEL"
	// SourceFormatPDF represents Adobe Portable Document Format files (.pdf).
	SourceFormatPDF SourceFormat = "PDF"
)

// RawTransaction represents an uncommitted, parsed transaction entry extracted from a statement.
type RawTransaction struct {
	ExternalID       string          `json:"external_id,omitempty"`
	Date             time.Time       `json:"date"`
	Amount           decimal.Decimal `json:"amount"` // Negative = expense, positive = income
	Description      string          `json:"description"`
	CleanDescription string          `json:"clean_description"`
	CategoryHint     string          `json:"category_hint,omitempty"`
	AccountHint      string          `json:"account_hint,omitempty"`
	Metadata         map[string]any  `json:"metadata,omitempty"`
}

// ParsedStatement aggregates the results of parsing a statement document.
type ParsedStatement struct {
	Format          SourceFormat     `json:"format"`
	AccountID       string           `json:"account_id,omitempty"`
	BankID          string           `json:"bank_id,omitempty"`
	Currency        string           `json:"currency,omitempty"`
	PeriodStart     time.Time        `json:"period_start,omitempty"`
	PeriodEnd       time.Time        `json:"period_end,omitempty"`
	HeaderBalance   *decimal.Decimal `json:"header_balance,omitempty"`
	Transactions    []RawTransaction `json:"transactions"`
	SkippedRows     int              `json:"skipped_rows"`
	ValidationWarns []string         `json:"validation_warns,omitempty"`
}

// StatementParser defines the contract implemented by file format parsing adapters.
type StatementParser interface {
	// CanParse sniffs whether the parser can handle the given file or stream.
	CanParse(filename string, r io.Reader) (bool, error)

	// Parse processes the file stream and returns structured raw transactions.
	Parse(ctx context.Context, r io.Reader, defaultAccountID uuid.UUID) (*ParsedStatement, error)

	// Format returns the specific source format handled by this parser.
	Format() SourceFormat
}
