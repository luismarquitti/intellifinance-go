package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
)

// ProviderBalanceSnapshotRepository manages historical external balance snapshots.
type ProviderBalanceSnapshotRepository interface {
	Save(ctx context.Context, snapshot *domain.ProviderBalanceSnapshot) error
	GetLatestByAccount(ctx context.Context, tenantID, accountID uuid.UUID) (*domain.ProviderBalanceSnapshot, error)
	ListByAccount(ctx context.Context, tenantID, accountID uuid.UUID, limit int) ([]*domain.ProviderBalanceSnapshot, error)
}
