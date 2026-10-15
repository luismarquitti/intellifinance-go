package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
)

// CategoryFilter defines criteria for querying categories within a tenant.
type CategoryFilter struct {
	Type          *domain.TransactionType
	ParentID      *uuid.UUID
	IncludeSystem *bool
}

// CategoryRepository defines the persistence contract for budget taxonomy categories.
type CategoryRepository interface {
	// Create persists a new budget category.
	Create(ctx context.Context, category *domain.Category) error

	// GetByID retrieves a category by ID within a tenant boundary.
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Category, error)

	// List retrieves categories matching filter criteria.
	List(ctx context.Context, tenantID uuid.UUID, filter CategoryFilter) ([]*domain.Category, error)

	// Update persists mutations to an existing category.
	Update(ctx context.Context, category *domain.Category) error

	// Delete removes a category by ID.
	Delete(ctx context.Context, tenantID, id uuid.UUID) error

	// CountTransactionsByCategory checks whether existing transactions reference the category.
	CountTransactionsByCategory(ctx context.Context, tenantID, categoryID uuid.UUID) (int64, error)
}
