package domain_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestPendingAction_ValidationAndDecisions(t *testing.T) {
	tenantID := uuid.New()
	targetID := uuid.New()

	// Missing tenant
	_, err := domain.NewPendingAction(uuid.Nil, "mcp-server", domain.ActionTypeCategorizeTransaction, nil)
	require.Error(t, err)

	// Valid creation
	action, err := domain.NewPendingAction(
		tenantID, "mcp-server", domain.ActionTypeCategorizeTransaction,
		map[string]any{"transaction_id": targetID.String(), "suggested_category_id": uuid.New().String()},
	)
	require.NoError(t, err)
	require.Equal(t, domain.ActionStatusPending, action.Status)

	// Approve
	decider := uuid.New()
	require.NoError(t, action.Approve(decider))
	require.Equal(t, domain.ActionStatusApproved, action.Status)
	require.Equal(t, &decider, action.DecidedByUserID)

	// Re-approving fails
	require.Error(t, action.Approve(decider))

	// Mark executed
	require.NoError(t, action.MarkExecuted())
	require.Equal(t, domain.ActionStatusExecuted, action.Status)
	require.NotNil(t, action.ExecutedAt)

	// Rejection workflow
	action2, err := domain.NewPendingAction(
		tenantID, "mcp-server", domain.ActionTypeSplitTransaction, nil,
	)
	require.NoError(t, err)
	require.NoError(t, action2.Reject(decider, "Incorrect proportions"))
	require.Equal(t, domain.ActionStatusRejected, action2.Status)
	require.Equal(t, "Incorrect proportions", *action2.RejectionReason)
}
