package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
)

// AccountService defines the inbound application service port for managing financial accounts.
type AccountService interface {
	// Create persists a new account within the tenant boundary.
	Create(ctx context.Context, account *domain.Account) error

	// GetByID retrieves a single account by ID within the tenant boundary.
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Account, error)

	// List queries accounts matching the given filter within the tenant boundary.
	List(ctx context.Context, tenantID uuid.UUID, filter AccountFilter) ([]*domain.Account, error)

	// Update updates an existing account.
	Update(ctx context.Context, account *domain.Account) error
}

// CategoryService defines the inbound application service port for budget taxonomy categories.
type CategoryService interface {
	// Create persists a new budget category within the tenant boundary.
	Create(ctx context.Context, category *domain.Category) error

	// GetByID retrieves a single category by ID within the tenant boundary.
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Category, error)

	// List queries categories matching the given filter within the tenant boundary.
	List(ctx context.Context, tenantID uuid.UUID, filter CategoryFilter) ([]*domain.Category, error)

	// Update updates an existing category.
	Update(ctx context.Context, category *domain.Category) error
}

// TransactionService defines the inbound application service port for financial transactions.
type TransactionService interface {
	// Create persists a new transaction.
	Create(ctx context.Context, tx *domain.Transaction) error

	// GetByID retrieves a transaction by ID within the tenant boundary.
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Transaction, error)

	// List queries transactions with filtering and pagination within the tenant boundary.
	List(ctx context.Context, tenantID uuid.UUID, filter TransactionFilter) ([]*domain.Transaction, int64, error)

	// Update updates an existing transaction.
	Update(ctx context.Context, tx *domain.Transaction) error

	// Delete removes a transaction by ID within the tenant boundary.
	Delete(ctx context.Context, tenantID, id uuid.UUID) error
}
