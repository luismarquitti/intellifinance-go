package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ProviderBalanceSnapshot records an external bank balance observation for reconciliation
// without ever directly mutating accounts.balance (which remains strictly managed by double-entry ledger).
type ProviderBalanceSnapshot struct {
	ID         uuid.UUID       `json:"id"`
	TenantID   uuid.UUID       `json:"tenant_id"`
	AccountID  uuid.UUID       `json:"account_id"`
	Provider   string          `json:"provider"`
	Balance    decimal.Decimal `json:"balance"`
	Currency   string          `json:"currency"`
	ObservedAt time.Time       `json:"observed_at"`
	Metadata   map[string]any  `json:"metadata"`
	CreatedAt  time.Time       `json:"created_at"`
}

// NewProviderBalanceSnapshot constructs and validates a provider balance snapshot.
func NewProviderBalanceSnapshot(
	tenantID, accountID uuid.UUID,
	provider string,
	balance decimal.Decimal,
	currency string,
	observedAt time.Time,
) (*ProviderBalanceSnapshot, error) {
	if tenantID == uuid.Nil {
		return nil, errors.New("tenant_id is required")
	}
	if accountID == uuid.Nil {
		return nil, errors.New("account_id is required")
	}
	if strings.TrimSpace(provider) == "" {
		return nil, errors.New("provider is required")
	}
	if strings.TrimSpace(currency) == "" {
		currency = "BRL"
	}
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}

	return &ProviderBalanceSnapshot{
		ID:         uuid.New(),
		TenantID:   tenantID,
		AccountID:  accountID,
		Provider:   strings.ToUpper(strings.TrimSpace(provider)),
		Balance:    balance.Round(2),
		Currency:   strings.ToUpper(strings.TrimSpace(currency)),
		ObservedAt: observedAt,
		Metadata:   make(map[string]any),
		CreatedAt:  time.Now().UTC(),
	}, nil
}
