package ports

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
)

// SyncItemRequest configures an Open Finance synchronization run for one provider connection item.
type SyncItemRequest struct {
	TenantID      uuid.UUID
	UserID        uuid.UUID
	ItemID        string
	ReferenceTime time.Time
}

// SyncItemResult summarizes accounts, snapshots, invoices, transactions, and HITL proposals
// produced during an Open Finance synchronization run.
type SyncItemResult struct {
	AccountsSynced           int                     `json:"accounts_synced"`
	SnapshotsSaved           int                     `json:"snapshots_saved"`
	InvoicesUpserted         int                     `json:"invoices_upserted"`
	InvoicesSettled          int                     `json:"invoices_settled"`
	TransactionsCreated      int                     `json:"transactions_created"`
	TransactionsDeduplicated int                     `json:"transactions_deduplicated"`
	TransferActionsProposed  int                     `json:"transfer_actions_proposed"`
	CategoryActionsProposed  int                     `json:"category_actions_proposed"`
	ProposedActions          []*domain.PendingAction `json:"proposed_actions,omitempty"`
}

// OpenFinanceSyncService orchestrates Open Finance ingestion, Provider Balance Snapshot
// persistence, Invoice cycle projection, Ledger Balance anchoring, Invoice Settlement
// reconciliation, and post-sync Pending Action generation.
type OpenFinanceSyncService interface {
	SyncItem(ctx context.Context, req SyncItemRequest) (*SyncItemResult, error)
}
