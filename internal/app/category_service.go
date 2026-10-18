package app

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

var _ ports.CategoryService = (*CategoryService)(nil)

// CategoryService implements ports.CategoryService application logic.
type CategoryService struct {
	repo ports.CategoryRepository
}

// NewCategoryService creates a new CategoryService instance.
func NewCategoryService(repo ports.CategoryRepository) *CategoryService {
	return &CategoryService{repo: repo}
}

// Create validates and persists a new budget category within the tenant boundary.
func (s *CategoryService) Create(ctx context.Context, category *domain.Category) error {
	if category == nil {
		return domain.ErrInvalidID
	}
	if err := category.Validate(); err != nil {
		return fmt.Errorf("category validation failed: %w", err)
	}
	return s.repo.Create(ctx, category)
}

// GetByID retrieves a single category by ID within the tenant boundary.
func (s *CategoryService) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Category, error) {
	if tenantID == uuid.Nil || id == uuid.Nil {
		return nil, domain.ErrInvalidID
	}
	return s.repo.GetByID(ctx, tenantID, id)
}

// List queries categories matching the given filter within the tenant boundary.
func (s *CategoryService) List(ctx context.Context, tenantID uuid.UUID, filter ports.CategoryFilter) ([]*domain.Category, error) {
	if tenantID == uuid.Nil {
		return nil, domain.ErrInvalidID
	}
	return s.repo.List(ctx, tenantID, filter)
}

// Update validates and persists mutations to an existing category.
func (s *CategoryService) Update(ctx context.Context, category *domain.Category) error {
	if category == nil {
		return domain.ErrInvalidID
	}
	if err := category.Validate(); err != nil {
		return fmt.Errorf("category validation failed: %w", err)
	}
	return s.repo.Update(ctx, category)
}
