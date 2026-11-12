package dto

import (
	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
)

// BatchApproveRequest defines payload to approve multiple pending actions.
type BatchApproveRequest struct {
	ActionIDs  []uuid.UUID               `json:"action_ids,omitempty"`
	All        bool                      `json:"all,omitempty"`
	ActionType *domain.PendingActionType `json:"action_type,omitempty"`
}

// BatchRejectRequest defines payload to reject multiple pending actions.
type BatchRejectRequest struct {
	ActionIDs []uuid.UUID `json:"action_ids,omitempty"`
	All       bool        `json:"all,omitempty"`
	Reason    string      `json:"reason,omitempty"`
}

// BatchDecisionResponse reports results of batch operations.
type BatchDecisionResponse struct {
	ProcessedCount int      `json:"processed_count"`
	ApprovedCount  int      `json:"approved_count"`
	RejectedCount  int      `json:"rejected_count"`
	FailedCount    int      `json:"failed_count"`
	Errors         []string `json:"errors,omitempty"`
}
