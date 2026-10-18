package app

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

var _ ports.TransactionService = (*TransactionService)(nil)

// TransactionService implements ports.TransactionService application logic.
type TransactionService struct {
	txRepo  ports.TransactionRepository
	accRepo ports.AccountRepository
}

// NewTransactionService constructs a new TransactionService instance.
func NewTransactionService(
	txRepo ports.TransactionRepository,
	accRepo ports.AccountRepository,
) *TransactionService {
	return &TransactionService{
		txRepo:  txRepo,
		accRepo: accRepo,
	}
}

// Create persists a transaction and adjusts the corresponding account balance.
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

	if tx.Status.AffectsBalance() && s.accRepo != nil {
		if err := s.accRepo.UpdateBalance(ctx, tx.TenantID, tx.AccountID, tx.Amount); err != nil {
			return fmt.Errorf("failed to update account balance: %w", err)
		}
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

// Update persists mutations to a transaction and synchronizes account balance deltas.
func (s *TransactionService) Update(ctx context.Context, tx *domain.Transaction) error {
	if tx == nil {
		return domain.ErrInvalidID
	}
	if err := tx.Validate(); err != nil {
		return fmt.Errorf("transaction validation failed: %w", err)
	}

	oldTx, err := s.txRepo.GetByID(ctx, tx.TenantID, tx.ID)
	if err != nil {
		return fmt.Errorf("failed to retrieve existing transaction: %w", err)
	}

	if err := s.txRepo.Update(ctx, tx); err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	if s.accRepo != nil {
		// If account changed, reverse from old and apply to new
		if oldTx.AccountID != tx.AccountID {
			if oldTx.Status.AffectsBalance() {
				_ = s.accRepo.UpdateBalance(ctx, oldTx.TenantID, oldTx.AccountID, oldTx.Amount.Neg())
			}
			if tx.Status.AffectsBalance() {
				_ = s.accRepo.UpdateBalance(ctx, tx.TenantID, tx.AccountID, tx.Amount)
			}
		} else {
			// Same account: calculate delta if balance-affecting
			oldAffects := oldTx.Status.AffectsBalance()
			newAffects := tx.Status.AffectsBalance()

			if oldAffects && newAffects {
				delta := tx.Amount.Sub(oldTx.Amount)
				if !delta.IsZero() {
					_ = s.accRepo.UpdateBalance(ctx, tx.TenantID, tx.AccountID, delta)
				}
			} else if oldAffects && !newAffects {
				_ = s.accRepo.UpdateBalance(ctx, tx.TenantID, tx.AccountID, oldTx.Amount.Neg())
			} else if !oldAffects && newAffects {
				_ = s.accRepo.UpdateBalance(ctx, tx.TenantID, tx.AccountID, tx.Amount)
			}
		}
	}

	return nil
}

// Delete removes a transaction and reverts its impact on account balance.
func (s *TransactionService) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	if tenantID == uuid.Nil || id == uuid.Nil {
		return domain.ErrInvalidID
	}

	tx, err := s.txRepo.GetByID(ctx, tenantID, id)
	if err != nil {
		return fmt.Errorf("failed to find transaction to delete: %w", err)
	}

	if err := s.txRepo.Delete(ctx, tenantID, id); err != nil {
		return fmt.Errorf("failed to delete transaction: %w", err)
	}

	if tx.Status.AffectsBalance() && s.accRepo != nil {
		_ = s.accRepo.UpdateBalance(ctx, tenantID, tx.AccountID, tx.Amount.Neg())
	}

	return nil
}

// BatchDelete removes multiple transactions and reverts their account balances.
func (s *TransactionService) BatchDelete(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) error {
	if tenantID == uuid.Nil || len(ids) == 0 {
		return nil
	}

	for _, id := range ids {
		tx, err := s.txRepo.GetByID(ctx, tenantID, id)
		if err == nil && tx != nil && tx.Status.AffectsBalance() && s.accRepo != nil {
			_ = s.accRepo.UpdateBalance(ctx, tenantID, tx.AccountID, tx.Amount.Neg())
		}
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
