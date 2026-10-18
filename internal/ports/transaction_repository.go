package ports

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/shopspring/decimal"
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
	Uncategorized    *bool
	SortBy           string
	SortOrder        string
	Page             int
	Limit            int
}

// TransactionSummary contains aggregated totals for the dashboard KPI cards.
type TransactionSummary struct {
	TotalIncome  decimal.Decimal `json:"total_income"`
	TotalExpense decimal.Decimal `json:"total_expense"`
	NetBalance   decimal.Decimal `json:"net_balance"`
	Count        int64           `json:"count"`
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

	// Summarize computes total income, expense, net balance and count matching filter criteria.
	Summarize(ctx context.Context, tenantID uuid.UUID, filter TransactionFilter) (*TransactionSummary, error)

	// Update persists mutations to an existing transaction.
	Update(ctx context.Context, tx *domain.Transaction) error

	// Delete removes a transaction by ID.
	Delete(ctx context.Context, tenantID, id uuid.UUID) error

	// BatchDelete removes multiple transactions by ID within a tenant boundary.
	BatchDelete(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) error

	// BatchUpdateCategory sets the category for multiple transactions within a tenant boundary.
	BatchUpdateCategory(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID, categoryID *uuid.UUID) error

	// FindExistingFingerprints returns a map of fingerprints already stored for the tenant.
	FindExistingFingerprints(ctx context.Context, tenantID uuid.UUID, fingerprints []string) (map[string]bool, error)

	// CreateSplits persists attribution splits for a transaction.
	CreateSplits(ctx context.Context, tenantID, txID uuid.UUID, splits []domain.Split) error

	// GetSplits retrieves all attribution splits for a transaction.
	GetSplits(ctx context.Context, tenantID, txID uuid.UUID) ([]domain.Split, error)
}
