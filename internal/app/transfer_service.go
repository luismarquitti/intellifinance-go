package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

var _ ports.TransferService = (*TransferService)(nil)

// TransferService coordinates confirmed internal transfers across own accounts.
type TransferService struct {
	transferRepo ports.TransferRepository
}

// NewTransferService constructs a TransferService.
func NewTransferService(transferRepo ports.TransferRepository) *TransferService {
	return &TransferService{transferRepo: transferRepo}
}

// ConfirmTransfer validates and explicitly links two existing journal transactions into a confirmed transfer pair.
func (s *TransferService) ConfirmTransfer(ctx context.Context, tenantID, legAID, legBID uuid.UUID) (*domain.Transaction, *domain.Transaction, error) {
	if tenantID == uuid.Nil || legAID == uuid.Nil || legBID == uuid.Nil {
		return nil, nil, domain.ErrInvalidID
	}
	if legAID == legBID {
		return nil, nil, domain.ErrInvalidTransferPair
	}
	return s.transferRepo.ConfirmTransfer(ctx, tenantID, legAID, legBID)
}

// CreateTransfer creates a dual-leg confirmed internal transfer with atomic ledger synchronization.
func (s *TransferService) CreateTransfer(ctx context.Context, params ports.CreateTransferParams) (*domain.Transaction, *domain.Transaction, error) {
	if params.TenantID == uuid.Nil || params.SourceAccountID == uuid.Nil || params.DestinationAccountID == uuid.Nil {
		return nil, nil, domain.ErrInvalidID
	}
	if params.SourceAccountID == params.DestinationAccountID {
		return nil, nil, domain.ErrTransferSameAccount
	}
	if !params.Amount.IsPositive() {
		return nil, nil, domain.ErrInvalidAmount
	}
	if !domain.IsRepresentableInCents(params.Amount) || params.Amount.GreaterThan(domain.MaxMonetaryAmount) {
		return nil, nil, domain.ErrInvalidAmount
	}

	outflow, inflow, err := domain.NewInternalTransfer(
		params.TenantID,
		params.SourceAccountID,
		params.DestinationAccountID,
		params.Amount,
		params.Date,
		params.Description,
		params.CreatedByUserID,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to construct transfer pair: %w", err)
	}

	if strings.TrimSpace(params.IdempotencyKey) == "" || len(params.IdempotencyKey) > 200 {
		return nil, nil, domain.ErrInvalidTransferPair
	}
	outflow.Metadata = map[string]any{"transfer_idempotency_key": params.IdempotencyKey}
	inflow.Metadata = map[string]any{"transfer_idempotency_key": params.IdempotencyKey}
	if err := s.transferRepo.CreateTransfer(ctx, outflow, inflow); err != nil {
		return nil, nil, fmt.Errorf("failed to persist transfer pair: %w", err)
	}

	return outflow, inflow, nil
}

// UnlinkTransfer breaks a transfer pairing reversibly without deleting either leg.
func (s *TransferService) UnlinkTransfer(ctx context.Context, tenantID, txID uuid.UUID) (*domain.Transaction, *domain.Transaction, error) {
	if tenantID == uuid.Nil || txID == uuid.Nil {
		return nil, nil, domain.ErrInvalidID
	}
	return s.transferRepo.UnlinkTransfer(ctx, tenantID, txID)
}
