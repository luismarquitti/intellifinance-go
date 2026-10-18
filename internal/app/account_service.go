package app

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

var _ ports.AccountService = (*AccountService)(nil)

// AccountService implements ports.AccountService application logic.
type AccountService struct {
	repo ports.AccountRepository
}

// NewAccountService creates a new AccountService instance.
func NewAccountService(repo ports.AccountRepository) *AccountService {
	return &AccountService{repo: repo}
}

// Create validates and persists a new account within the tenant boundary.
func (s *AccountService) Create(ctx context.Context, account *domain.Account) error {
	if account == nil {
		return domain.ErrInvalidID
	}
	if err := account.Validate(); err != nil {
		return fmt.Errorf("account validation failed: %w", err)
	}
	return s.repo.Create(ctx, account)
}

// GetByID retrieves a single account by ID within the tenant boundary.
func (s *AccountService) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Account, error) {
	if tenantID == uuid.Nil || id == uuid.Nil {
		return nil, domain.ErrInvalidID
	}
	return s.repo.GetByID(ctx, tenantID, id)
}

// List queries accounts matching the given filter within the tenant boundary.
func (s *AccountService) List(ctx context.Context, tenantID uuid.UUID, filter ports.AccountFilter) ([]*domain.Account, error) {
	if tenantID == uuid.Nil {
		return nil, domain.ErrInvalidID
	}
	return s.repo.List(ctx, tenantID, filter)
}

// Update validates and persists mutations to an existing account.
func (s *AccountService) Update(ctx context.Context, account *domain.Account) error {
	if account == nil {
		return domain.ErrInvalidID
	}
	if err := account.Validate(); err != nil {
		return fmt.Errorf("account validation failed: %w", err)
	}
	return s.repo.Update(ctx, account)
}
