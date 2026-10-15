package ports

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
)

// TransactionFilter defines querying and pagination criteria for transactions.
type TransactionFilter struct {
	AccountID        *uuid.UUID
	CategoryID       *uuid.UUID
	Type             *domain.TransactionType
	Status           *domain.TransactionStatus
	Visibility       *domain.Visibility
	FromDate         *time.Time
	ToDate           *time.Time
	Search           string
	AIReviewRequired *bool
	Page             int
	Limit            int
}

// TransactionRepository defines the persistence contract for transactions and splits.
type TransactionRepository interface {
	// Create persists a single transaction.
	Create(ctx context.Context, tx *domain.Transaction) error

	// BatchCreate persists multiple transactions atomically or in high-throughput batches.
	BatchCreate(ctx context.Context, txs []*domain.Transaction) error

	// GetByID retrieves a transaction by ID within a tenant boundary.
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Transaction, error)

	// List queries paginated transactions and returns items and total count.
	List(ctx context.Context, tenantID uuid.UUID, filter TransactionFilter) ([]*domain.Transaction, int64, error)

	// Update persists mutations to an existing transaction.
	Update(ctx context.Context, tx *domain.Transaction) error

	// Delete removes a transaction by ID.
	Delete(ctx context.Context, tenantID, id uuid.UUID) error

	// FindExistingFingerprints returns a map of fingerprints already stored for the tenant.
	FindExistingFingerprints(ctx context.Context, tenantID uuid.UUID, fingerprints []string) (map[string]bool, error)

	// CreateSplits persists attribution splits for a transaction.
	CreateSplits(ctx context.Context, tenantID, txID uuid.UUID, splits []domain.Split) error

	// GetSplits retrieves all attribution splits for a transaction.
	GetSplits(ctx context.Context, tenantID, txID uuid.UUID) ([]domain.Split, error)
}
