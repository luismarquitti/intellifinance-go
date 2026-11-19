package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
)

// BatchDecisionResult summarizes the outcomes of a batch pending action approval or rejection.
type BatchDecisionResult struct {
	ProcessedCount int      `json:"processed_count"`
	ApprovedCount  int      `json:"approved_count"`
	RejectedCount  int      `json:"rejected_count"`
	FailedCount    int      `json:"failed_count"`
	Errors         []string `json:"errors,omitempty"`
}

// PendingActionService defines inbound use cases for Human-in-the-Loop pending action governance.
type PendingActionService interface {
	// ListPending retrieves all PENDING actions awaiting review within a tenant.
	ListPending(ctx context.Context, tenantID uuid.UUID) ([]*domain.PendingAction, error)

	// LoadPendingIndex builds an O(1) deduplication index from active PENDING actions within a tenant.
	LoadPendingIndex(ctx context.Context, tenantID uuid.UUID) (*domain.PendingActionIndex, error)

	// ProposeCategorization validates, optionally hydrates, deduplicates, and persists a categorization proposal.
	ProposeCategorization(ctx context.Context, tenantID uuid.UUID, createdByAgent string, proposal domain.CategorizeTransactionProposal) (*domain.PendingAction, error)

	// ProposeTransferReconciliation validates, deduplicates, and persists a transfer reconciliation proposal.
	ProposeTransferReconciliation(ctx context.Context, tenantID uuid.UUID, createdByAgent string, proposal domain.ReconcileTransferProposal) (*domain.PendingAction, error)

	// Approve executes a single pending action and marks it APPROVED and EXECUTED only if execution succeeds.
	Approve(ctx context.Context, tenantID, actionID, decidedBy uuid.UUID) (*domain.PendingAction, error)

	// Reject marks a single pending action as REJECTED with an optional trimmed reason.
	Reject(ctx context.Context, tenantID, actionID, decidedBy uuid.UUID, reason string) (*domain.PendingAction, error)

	// ApproveBatch applies the selector to pending actions and approves each one with per-item failure isolation.
	ApproveBatch(ctx context.Context, tenantID, decidedBy uuid.UUID, selector domain.PendingActionSelector) (*BatchDecisionResult, error)

	// RejectBatch applies the selector to pending actions and rejects each one with per-item failure isolation.
	RejectBatch(ctx context.Context, tenantID, decidedBy uuid.UUID, selector domain.PendingActionSelector, reason string) (*BatchDecisionResult, error)
}
