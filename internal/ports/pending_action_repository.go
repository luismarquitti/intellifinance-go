package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
)

// PendingActionRepository manages Human-in-the-Loop pending actions.
type PendingActionRepository interface {
	Save(ctx context.Context, action *domain.PendingAction) error
	FindByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.PendingAction, error)
	ListPending(ctx context.Context, tenantID uuid.UUID) ([]*domain.PendingAction, error)
	UpdateDecision(ctx context.Context, tenantID, id uuid.UUID, status domain.PendingActionStatus, decidedBy *uuid.UUID, reason *string) error
	MarkExecuted(ctx context.Context, tenantID, id uuid.UUID) error
}
