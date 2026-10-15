package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/shopspring/decimal"
)

// AccountFilter defines querying criteria for listing accounts.
type AccountFilter struct {
	OwnerUserID *uuid.UUID
	Type        *domain.AccountType
	Visibility  *domain.Visibility
	IsActive    *bool
}

// AccountRepository defines the persistence contract for Account entities.
type AccountRepository interface {
	// Create persists a new Account.
	Create(ctx context.Context, account *domain.Account) error

	// GetByID retrieves an account by ID within a tenant boundary.
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Account, error)

	// List queries accounts matching filter criteria within a tenant boundary.
	List(ctx context.Context, tenantID uuid.UUID, filter AccountFilter) ([]*domain.Account, error)

	// Update persists mutations to an existing account.
	Update(ctx context.Context, account *domain.Account) error

	// Delete soft-deletes or removes an account by ID.
	Delete(ctx context.Context, tenantID, id uuid.UUID) error

	// UpdateBalance atomically adjusts the account balance by delta.
	UpdateBalance(ctx context.Context, tenantID, id uuid.UUID, delta decimal.Decimal) error
}
