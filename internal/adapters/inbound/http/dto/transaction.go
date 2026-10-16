package dto

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/inbound/http/problem"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/shopspring/decimal"
)

// CreateTransactionRequest defines the payload for creating a single transaction.
type CreateTransactionRequest struct {
	AccountID   string         `json:"account_id"`
	CategoryID  *string        `json:"category_id,omitempty"`
	Date        string         `json:"date"`
	Amount      string         `json:"amount"`
	Description string         `json:"description"`
	Type        string         `json:"type"`
	Status      string         `json:"status,omitempty"`
	Visibility  string         `json:"visibility,omitempty"`
	Notes       string         `json:"notes,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// Validate validates input fields of CreateTransactionRequest.
func (r *CreateTransactionRequest) Validate() []problem.InvalidParam {
	var errs []problem.InvalidParam

	if strings.TrimSpace(r.AccountID) == "" {
		errs = append(errs, problem.InvalidParam{Name: "account_id", Reason: "cannot be empty"})
	} else if id, err := uuid.Parse(strings.TrimSpace(r.AccountID)); err != nil || id == uuid.Nil {
		errs = append(errs, problem.InvalidParam{Name: "account_id", Reason: "must be a valid non-nil UUID"})
	}

	if r.CategoryID != nil && strings.TrimSpace(*r.CategoryID) != "" {
		if id, err := uuid.Parse(strings.TrimSpace(*r.CategoryID)); err != nil || id == uuid.Nil {
			errs = append(errs, problem.InvalidParam{Name: "category_id", Reason: "must be a valid non-nil UUID"})
		}
	}

	if strings.TrimSpace(r.Description) == "" {
		errs = append(errs, problem.InvalidParam{Name: "description", Reason: "cannot be empty"})
	}

	txType := domain.TransactionType(strings.ToUpper(strings.TrimSpace(r.Type)))
	if !txType.IsValid() {
		errs = append(errs, problem.InvalidParam{Name: "type", Reason: "must be one of INCOME, EXPENSE, TRANSFER"})
	}

	if strings.TrimSpace(r.Date) == "" {
		errs = append(errs, problem.InvalidParam{Name: "date", Reason: "cannot be empty"})
	} else if _, err := ParseDate(r.Date); err != nil {
		errs = append(errs, problem.InvalidParam{Name: "date", Reason: "must be formatted as YYYY-MM-DD or RFC3339"})
	}

	if strings.TrimSpace(r.Amount) == "" {
		errs = append(errs, problem.InvalidParam{Name: "amount", Reason: "cannot be empty"})
	} else {
		amt, err := decimal.NewFromString(strings.TrimSpace(r.Amount))
		if err != nil {
			errs = append(errs, problem.InvalidParam{Name: "amount", Reason: "must be a valid decimal number"})
		} else if amt.IsZero() {
			errs = append(errs, problem.InvalidParam{Name: "amount", Reason: "must be a non-zero decimal value"})
		} else if txType == domain.TransactionTypeExpense && amt.IsPositive() {
			errs = append(errs, problem.InvalidParam{Name: "amount", Reason: "amount for EXPENSE must be negative"})
		} else if txType == domain.TransactionTypeIncome && amt.IsNegative() {
			errs = append(errs, problem.InvalidParam{Name: "amount", Reason: "amount for INCOME must be positive"})
		}
	}

	if r.Visibility != "" {
		vis := domain.Visibility(strings.ToUpper(strings.TrimSpace(r.Visibility)))
		if !vis.IsValid() {
			errs = append(errs, problem.InvalidParam{Name: "visibility", Reason: "must be one of SHARED, PRIVATE, RESTRICTED"})
		}
	}

	if r.Status != "" {
		st := domain.TransactionStatus(strings.ToUpper(strings.TrimSpace(r.Status)))
		if !st.IsValid() {
			errs = append(errs, problem.InvalidParam{Name: "status", Reason: "must be one of PENDING, COMPLETED, RECONCILED, VOID"})
		}
	}

	return errs
}

// UpdateTransactionRequest defines the payload for updating transaction fields.
type UpdateTransactionRequest struct {
	CategoryID  *string        `json:"category_id,omitempty"`
	Description *string        `json:"description,omitempty"`
	Visibility  *string        `json:"visibility,omitempty"`
	Notes       *string        `json:"notes,omitempty"`
	Status      *string        `json:"status,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// Validate validates input fields of UpdateTransactionRequest.
func (r *UpdateTransactionRequest) Validate() []problem.InvalidParam {
	var errs []problem.InvalidParam
	if r.CategoryID != nil && strings.TrimSpace(*r.CategoryID) != "" {
		if id, err := uuid.Parse(strings.TrimSpace(*r.CategoryID)); err != nil || id == uuid.Nil {
			errs = append(errs, problem.InvalidParam{Name: "category_id", Reason: "must be a valid non-nil UUID"})
		}
	}
	if r.Description != nil && strings.TrimSpace(*r.Description) == "" {
		errs = append(errs, problem.InvalidParam{Name: "description", Reason: "cannot be empty"})
	}
	if r.Visibility != nil {
		vis := domain.Visibility(strings.ToUpper(strings.TrimSpace(*r.Visibility)))
		if !vis.IsValid() {
			errs = append(errs, problem.InvalidParam{Name: "visibility", Reason: "must be one of SHARED, PRIVATE, RESTRICTED"})
		}
	}
	if r.Status != nil {
		st := domain.TransactionStatus(strings.ToUpper(strings.TrimSpace(*r.Status)))
		if !st.IsValid() {
			errs = append(errs, problem.InvalidParam{Name: "status", Reason: "must be one of PENDING, COMPLETED, RECONCILED, VOID"})
		}
	}
	return errs
}

// TransactionResponse defines the serializable response for a transaction.
type TransactionResponse struct {
	ID                 uuid.UUID       `json:"id"`
	TenantID           uuid.UUID       `json:"tenant_id"`
	AccountID          uuid.UUID       `json:"account_id"`
	CategoryID         *uuid.UUID      `json:"category_id"`
	CreatedByUserID    *uuid.UUID      `json:"created_by_user_id,omitempty"`
	Date               string          `json:"date"`
	Amount             string          `json:"amount"`
	Description        string          `json:"description"`
	CleanDescription   string          `json:"clean_description"`
	Type               string          `json:"type"`
	Status             string          `json:"status"`
	Visibility         string          `json:"visibility"`
	Source             string          `json:"source"`
	ExternalID         string          `json:"external_id,omitempty"`
	Fingerprint        string          `json:"fingerprint"`
	IsAICategorized    bool            `json:"is_ai_categorized"`
	AIConfidence       *float64        `json:"ai_confidence,omitempty"`
	IsManuallyVerified bool            `json:"is_manually_verified"`
	Notes              string          `json:"notes,omitempty"`
	Metadata           map[string]any  `json:"metadata"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

// TransactionListResponse defines the paginated collection envelope for transactions.
type TransactionListResponse struct {
	Items      []TransactionResponse `json:"items"`
	Pagination PaginationResponse    `json:"pagination"`
}

// ParseDate parses date string formatted as YYYY-MM-DD or RFC3339.
func ParseDate(val string) (time.Time, error) {
	val = strings.TrimSpace(val)
	if t, err := time.Parse("2006-01-02", val); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, val)
}

// ToTransactionResponse transforms a domain.Transaction into TransactionResponse.
func ToTransactionResponse(tx *domain.Transaction) TransactionResponse {
	var confidence *float64
	if tx.AIConfidence != nil {
		f, _ := tx.AIConfidence.Float64()
		confidence = &f
	}

	return TransactionResponse{
		ID:                 tx.ID,
		TenantID:           tx.TenantID,
		AccountID:          tx.AccountID,
		CategoryID:         tx.CategoryID,
		CreatedByUserID:    tx.CreatedByUserID,
		Date:               tx.Date.Format("2006-01-02"),
		Amount:             tx.Amount.StringFixed(2),
		Description:        tx.Description,
		CleanDescription:   tx.CleanDescription,
		Type:               string(tx.Type),
		Status:             string(tx.Status),
		Visibility:         string(tx.Visibility),
		Source:             string(tx.Source),
		ExternalID:         tx.ExternalID,
		Fingerprint:        tx.Fingerprint,
		IsAICategorized:    tx.IsAICategorized,
		AIConfidence:       confidence,
		IsManuallyVerified: tx.IsManuallyVerified,
		Notes:              tx.Notes,
		Metadata:           tx.Metadata,
		CreatedAt:          tx.CreatedAt,
		UpdatedAt:          tx.UpdatedAt,
	}
}

// ToTransactionListResponse transforms slice and pagination into TransactionListResponse.
func ToTransactionListResponse(txs []*domain.Transaction, pagination PaginationResponse) TransactionListResponse {
	items := make([]TransactionResponse, 0, len(txs))
	for _, tx := range txs {
		items = append(items, ToTransactionResponse(tx))
	}
	return TransactionListResponse{
		Items:      items,
		Pagination: pagination,
	}
}
