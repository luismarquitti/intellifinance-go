package ports

import (
	"context"

	"github.com/google/uuid"
)

// CategorizationScanResult holds the summary metrics from an auto-categorization scan.
type CategorizationScanResult struct {
	ScannedTransactionsCount int `json:"scanned_transactions_count"`
	ProposedActionsCount     int `json:"proposed_actions_count"`
	AlreadyCategorizedCount  int `json:"already_categorized_count"`
}

// CategoryRuleService defines automated taxonomy and classification evaluation for uncategorized transactions.
type CategoryRuleService interface {
	ScanAndPropose(ctx context.Context, tenantID uuid.UUID) (*CategorizationScanResult, error)
}
