package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

//nolint:misspell // Portuguese banking keyword: "debito automatico"
var (
	stripPrefixes = regexp.MustCompile(`(?i)^(compra\s+cartao|compra\s+com\s+cartao|pix\s+enviado|pix\s+recebido|ted\s+enviada|ted\s+recebida|pagamento\s+boleto|debito\s+automatico|transf\s+eletr\s+ted|doc\s+elet\s+tarifa)\s*[-:]?\s*`)
	stripDates    = regexp.MustCompile(`\b\d{2}/\d{2}(/\d{2,4})?\b`)
	stripNoise    = regexp.MustCompile(`[^\p{L}\p{N}\s]`)
	stripSpaces   = regexp.MustCompile(`\s+`)
)

// CleanDescription strips banking noise, dates, card prefixes, and returns a sanitized uppercase string.
func CleanDescription(raw string) string {
	str := strings.ToUpper(raw)
	str = stripPrefixes.ReplaceAllString(str, "")
	str = stripDates.ReplaceAllString(str, "")
	str = stripNoise.ReplaceAllString(str, " ")
	str = stripSpaces.ReplaceAllString(str, " ")
	return strings.TrimSpace(str)
}

// ComputeFingerprint calculates a deterministic SHA-256 idempotency fingerprint for a transaction.
// Formula: SHA256(tenant_id | account_id | date(YYYY-MM-DD) | amount(cents) | normalized_description | sequence_index)
func ComputeFingerprint(
	tenantID, accountID uuid.UUID,
	date time.Time,
	amount decimal.Decimal,
	cleanDesc string,
	sequenceIndex int,
) string {
	dateStr := date.Format("2006-01-02")
	cents := amount.Mul(decimal.NewFromInt(100)).IntPart()
	centsStr := strconv.FormatInt(cents, 10)

	raw := fmt.Sprintf("%s|%s|%s|%s|%s|%d",
		tenantID.String(),
		accountID.String(),
		dateStr,
		centsStr,
		cleanDesc,
		sequenceIndex,
	)

	hash := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(hash[:])
}

// Transaction represents a financial journal entry with deterministic deduplication and audit metadata.
type Transaction struct {
	ID                         uuid.UUID        `json:"id"`
	TenantID                   uuid.UUID        `json:"tenant_id"`
	AccountID                  uuid.UUID        `json:"account_id"`
	CategoryID                 *uuid.UUID       `json:"category_id,omitempty"`
	CreatedByUserID            *uuid.UUID       `json:"created_by_user_id,omitempty"`
	TransactedAt               time.Time        `json:"transacted_at"`
	Date                       time.Time        `json:"date"`
	Amount                     decimal.Decimal  `json:"amount"` // Signed: negative = expense, positive = income
	Description                string           `json:"description"`
	CleanDescription           string           `json:"clean_description"`
	Type                       TransactionType  `json:"type"`
	Status                     TransactionStatus `json:"status"`
	Visibility                 Visibility       `json:"visibility"`
	Source                     SourceType       `json:"source"`
	ExternalID                 string           `json:"external_id,omitempty"`
	SequenceIndex              int              `json:"sequence_index"`
	Fingerprint                string           `json:"fingerprint"`
	IsAICategorized            bool             `json:"is_ai_categorized"`
	AIConfidence               *decimal.Decimal `json:"ai_confidence,omitempty"`
	AIModelVersion             string           `json:"ai_model_version,omitempty"`
	IsManuallyVerified         bool             `json:"is_manually_verified"`
	TransferPeerTransactionID  *uuid.UUID       `json:"transfer_peer_transaction_id,omitempty"`
	Notes                      string           `json:"notes,omitempty"`
	Metadata                   map[string]any   `json:"metadata"`
	CreatedAt                  time.Time        `json:"created_at"`
	UpdatedAt                  time.Time        `json:"updated_at"`
}

// NewTransactionParams holds arguments for constructing a Transaction.
type NewTransactionParams struct {
	TenantID        uuid.UUID
	AccountID       uuid.UUID
	CategoryID      *uuid.UUID
	CreatedByUserID *uuid.UUID
	TransactedAt    time.Time
	Date            time.Time
	Amount          decimal.Decimal
	Description     string
	Type            TransactionType
	Status          TransactionStatus
	Visibility      Visibility
	Source          SourceType
	ExternalID      string
	SequenceIndex   int
	Notes           string
	Metadata        map[string]any
}

// NewTransaction constructs, normalizes, and validates a new financial Transaction.
func NewTransaction(params NewTransactionParams) (*Transaction, error) {
	if params.TransactedAt.IsZero() {
		params.TransactedAt = time.Now().UTC()
	}
	if params.Date.IsZero() {
		params.Date = params.TransactedAt
	}
	// Truncate Date to day boundary (YYYY-MM-DD)
	y, m, d := params.Date.Date()
	dateOnly := time.Date(y, m, d, 0, 0, 0, 0, params.Date.Location())

	if strings.TrimSpace(string(params.Status)) == "" {
		params.Status = TransactionStatusCompleted
	}
	if strings.TrimSpace(string(params.Visibility)) == "" {
		params.Visibility = VisibilityShared
	}
	if strings.TrimSpace(string(params.Source)) == "" {
		params.Source = SourceTypeManual
	}

	cleanDesc := CleanDescription(params.Description)
	fingerprint := ComputeFingerprint(
		params.TenantID,
		params.AccountID,
		dateOnly,
		params.Amount,
		cleanDesc,
		params.SequenceIndex,
	)

	now := time.Now().UTC()
	meta := params.Metadata
	if meta == nil {
		meta = make(map[string]any)
	}

	tx := &Transaction{
		ID:               uuid.New(),
		TenantID:         params.TenantID,
		AccountID:        params.AccountID,
		CategoryID:       params.CategoryID,
		CreatedByUserID:  params.CreatedByUserID,
		TransactedAt:     params.TransactedAt,
		Date:             dateOnly,
		Amount:           params.Amount,
		Description:      strings.TrimSpace(params.Description),
		CleanDescription: cleanDesc,
		Type:             params.Type,
		Status:           params.Status,
		Visibility:       params.Visibility,
		Source:           params.Source,
		ExternalID:       strings.TrimSpace(params.ExternalID),
		SequenceIndex:    params.SequenceIndex,
		Fingerprint:      fingerprint,
		Notes:            strings.TrimSpace(params.Notes),
		Metadata:         meta,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := tx.Validate(); err != nil {
		return nil, err
	}
	return tx, nil
}

// Validate ensures transaction domain rules and financial invariants are satisfied.
func (t *Transaction) Validate() error {
	if t.ID == uuid.Nil || t.TenantID == uuid.Nil || t.AccountID == uuid.Nil {
		return ErrInvalidID
	}
	if strings.TrimSpace(t.Description) == "" {
		return ErrEmptyDescription
	}
	if t.Date.IsZero() {
		return ErrInvalidDate
	}
	if t.Amount.IsZero() {
		return ErrInvalidAmount
	}
	if !t.Type.IsValid() {
		return ErrInvalidTransactionType
	}
	if !t.Status.IsValid() {
		return ErrInvalidStatus
	}
	if !t.Visibility.IsValid() {
		return ErrInvalidVisibility
	}
	if !t.Source.IsValid() {
		return ErrInvalidSource
	}

	// Sign validation based on transaction type
	switch t.Type {
	case TransactionTypeExpense:
		if t.Amount.IsPositive() {
			return ErrInvalidAmount
		}
	case TransactionTypeIncome:
		if t.Amount.IsNegative() {
			return ErrInvalidAmount
		}
	case TransactionTypeTransfer:
		// Transfer amounts can be signed depending on sender/receiver perspective
		if t.Amount.IsZero() {
			return ErrInvalidAmount
		}
	}

	// AI Confidence validation
	if t.AIConfidence != nil {
		if t.AIConfidence.IsNegative() || t.AIConfidence.GreaterThan(decimal.NewFromInt(1)) {
			return ErrInvalidAmount
		}
	}

	return nil
}

// AssignCategory sets or updates the category classification with optional AI metadata.
func (t *Transaction) AssignCategory(categoryID uuid.UUID, confidence *decimal.Decimal, modelVersion string, isAI bool) error {
	if categoryID == uuid.Nil {
		return ErrInvalidID
	}
	if confidence != nil && (confidence.IsNegative() || confidence.GreaterThan(decimal.NewFromInt(1))) {
		return ErrInvalidAmount
	}

	t.CategoryID = &categoryID
	t.IsAICategorized = isAI
	t.AIConfidence = confidence
	t.AIModelVersion = modelVersion
	t.UpdatedAt = time.Now().UTC()
	return nil
}

// VerifyCategory confirms or overrides the category with manual user verification.
func (t *Transaction) VerifyCategory(categoryID uuid.UUID) error {
	if categoryID == uuid.Nil {
		return ErrInvalidID
	}
	t.CategoryID = &categoryID
	t.IsManuallyVerified = true
	t.UpdatedAt = time.Now().UTC()
	return nil
}

// Void transitions the transaction into VOID status.
func (t *Transaction) Void() {
	t.Status = TransactionStatusVoid
	t.UpdatedAt = time.Now().UTC()
}

// IsSettled returns true if the transaction is posted and settled.
func (t *Transaction) IsSettled() bool {
	return t.Status == TransactionStatusCompleted || t.Status == TransactionStatusReconciled
}
