package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// PendingActionType defines the category of mutation requiring human approval.
type PendingActionType string

// Pending action type definitions.
const (
	// ActionTypeCategorizeTransaction proposes setting category and metadata on a transaction.
	ActionTypeCategorizeTransaction PendingActionType = "CATEGORIZE_TRANSACTION"
	// ActionTypeSplitTransaction proposes dividing a transaction into sub-splits.
	ActionTypeSplitTransaction PendingActionType = "SPLIT_TRANSACTION"
	// ActionTypeReconcileTransfer proposes linking two peer transactions into an internal transfer.
	ActionTypeReconcileTransfer PendingActionType = "RECONCILE_TRANSFER"
	// ActionTypeBalanceAdjustment proposes reconciling external and internal account balances.
	ActionTypeBalanceAdjustment PendingActionType = "BALANCE_ADJUSTMENT"
)

// PendingActionStatus defines the decision state.
type PendingActionStatus string

// Pending action status definitions.
const (
	// ActionStatusPending indicates the action awaits human review and approval.
	ActionStatusPending PendingActionStatus = "PENDING"
	// ActionStatusApproved indicates the action was authorized by an authenticated user.
	ActionStatusApproved PendingActionStatus = "APPROVED"
	// ActionStatusRejected indicates the action was declined by an authenticated user.
	ActionStatusRejected PendingActionStatus = "REJECTED"
	// ActionStatusExecuted indicates the mutation has been committed to the ledger.
	ActionStatusExecuted PendingActionStatus = "EXECUTED"
)

// PendingAction enforces Human-in-the-Loop governance for MCP and AI operations.
type PendingAction struct {
	ID               uuid.UUID           `json:"id"`
	TenantID         uuid.UUID           `json:"tenant_id"`
	CreatedByAgent   string              `json:"created_by_agent"`
	ActionType       PendingActionType   `json:"action_type"`
	Payload          map[string]any      `json:"payload"`
	Status           PendingActionStatus `json:"status"`
	RejectionReason  *string             `json:"rejection_reason,omitempty"`
	DecidedByUserID  *uuid.UUID          `json:"decided_by_user_id,omitempty"`
	DecidedAt        *time.Time          `json:"decided_at,omitempty"`
	ExecutedAt       *time.Time          `json:"executed_at,omitempty"`
	Metadata         map[string]any      `json:"metadata"`
	CreatedAt        time.Time           `json:"created_at"`
	UpdatedAt        time.Time           `json:"updated_at"`
}

// NewPendingAction creates a proposed action in PENDING status.
func NewPendingAction(
	tenantID uuid.UUID,
	createdByAgent string,
	actionType PendingActionType,
	payload map[string]any,
) (*PendingAction, error) {
	if tenantID == uuid.Nil {
		return nil, errors.New("tenant_id is required")
	}
	if strings.TrimSpace(createdByAgent) == "" {
		createdByAgent = "system"
	}
	if strings.TrimSpace(string(actionType)) == "" {
		return nil, errors.New("action_type is required")
	}
	if payload == nil {
		payload = make(map[string]any)
	}

	now := time.Now().UTC()
	return &PendingAction{
		ID:             uuid.New(),
		TenantID:       tenantID,
		CreatedByAgent: createdByAgent,
		ActionType:     actionType,
		Payload:        payload,
		Status:         ActionStatusPending,
		Metadata:       make(map[string]any),
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

// Approve transitions the action to APPROVED state with the deciding user ID.
func (p *PendingAction) Approve(decidedBy uuid.UUID) error {
	if p.Status != ActionStatusPending {
		return errors.New("cannot approve action that is not pending")
	}
	now := time.Now().UTC()
	p.Status = ActionStatusApproved
	p.DecidedAt = &now
	p.DecidedByUserID = &decidedBy
	p.UpdatedAt = now
	return nil
}

// Reject transitions the action to REJECTED state with reason.
func (p *PendingAction) Reject(decidedBy uuid.UUID, reason string) error {
	if p.Status != ActionStatusPending {
		return errors.New("cannot reject action that is not pending")
	}
	now := time.Now().UTC()
	p.Status = ActionStatusRejected
	p.DecidedAt = &now
	p.DecidedByUserID = &decidedBy
	p.RejectionReason = &reason
	p.UpdatedAt = now
	return nil
}

// MarkExecuted marks an approved action as executed.
func (p *PendingAction) MarkExecuted() error {
	if p.Status != ActionStatusApproved {
		return errors.New("cannot execute action that is not approved")
	}
	now := time.Now().UTC()
	p.Status = ActionStatusExecuted
	p.ExecutedAt = &now
	p.UpdatedAt = now
	return nil
}
