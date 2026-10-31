package app

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

var _ ports.TransactionService = (*TransactionService)(nil)

// TransactionService implements transaction use cases. Persistence owns atomic
// ledger/balance synchronization; applying another delta here would double it.
type TransactionService struct {
	txRepo ports.TransactionRepository
}

// NewTransactionService constructs a new TransactionService instance.
func NewTransactionService(txRepo ports.TransactionRepository) *TransactionService {
	return &TransactionService{txRepo: txRepo}
}

// Create persists a transaction and its atomic ledger effect.
func (s *TransactionService) Create(ctx context.Context, tx *domain.Transaction) error {
	if tx == nil {
		return domain.ErrInvalidID
	}
	if err := tx.Validate(); err != nil {
		return fmt.Errorf("transaction validation failed: %w", err)
	}
	if err := s.txRepo.Create(ctx, tx); err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}
	return nil
}

// GetByID retrieves a transaction by ID within the tenant boundary.
func (s *TransactionService) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Transaction, error) {
	if tenantID == uuid.Nil || id == uuid.Nil {
		return nil, domain.ErrInvalidID
	}
	return s.txRepo.GetByID(ctx, tenantID, id)
}

// List queries transactions with filtering and pagination within the tenant boundary.
func (s *TransactionService) List(ctx context.Context, tenantID uuid.UUID, filter ports.TransactionFilter) ([]*domain.Transaction, int64, error) {
	if tenantID == uuid.Nil {
		return nil, 0, domain.ErrInvalidID
	}
	return s.txRepo.List(ctx, tenantID, filter)
}

// Summarize aggregates KPI metrics matching the filter criteria.
func (s *TransactionService) Summarize(ctx context.Context, tenantID uuid.UUID, filter ports.TransactionFilter) (*ports.TransactionSummary, error) {
	if tenantID == uuid.Nil {
		return nil, domain.ErrInvalidID
	}
	return s.txRepo.Summarize(ctx, tenantID, filter)
}

// Update persists mutations and their atomic ledger effect.
func (s *TransactionService) Update(ctx context.Context, tx *domain.Transaction) error {
	if tx == nil {
		return domain.ErrInvalidID
	}
	if err := tx.Validate(); err != nil {
		return fmt.Errorf("transaction validation failed: %w", err)
	}
	if _, err := s.txRepo.GetByID(ctx, tx.TenantID, tx.ID); err != nil {
		return fmt.Errorf("failed to retrieve existing transaction: %w", err)
	}
	if err := s.txRepo.Update(ctx, tx); err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}
	return nil
}

// Delete removes a transaction and atomically reverts its ledger effect.
func (s *TransactionService) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	if tenantID == uuid.Nil || id == uuid.Nil {
		return domain.ErrInvalidID
	}
	if _, err := s.txRepo.GetByID(ctx, tenantID, id); err != nil {
		return fmt.Errorf("failed to find transaction to delete: %w", err)
	}
	if err := s.txRepo.Delete(ctx, tenantID, id); err != nil {
		return fmt.Errorf("failed to delete transaction: %w", err)
	}
	return nil
}

// BatchDelete removes multiple transactions and their atomic ledger effects.
func (s *TransactionService) BatchDelete(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) error {
	if tenantID == uuid.Nil || len(ids) == 0 {
		return nil
	}
	return s.txRepo.BatchDelete(ctx, tenantID, ids)
}

// BatchUpdateCategory sets the category for multiple transactions within the tenant boundary.
func (s *TransactionService) BatchUpdateCategory(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID, categoryID *uuid.UUID) error {
	if tenantID == uuid.Nil || len(ids) == 0 {
		return nil
	}
	return s.txRepo.BatchUpdateCategory(ctx, tenantID, ids, categoryID)
}
