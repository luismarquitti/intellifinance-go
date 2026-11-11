package ports

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/shopspring/decimal"
)

// CreateTransferParams defines parameters for creating an internal transfer with both legs.
type CreateTransferParams struct {
	// IdempotencyKey is required, tenant-scoped, opaque and at most 200 bytes.
	// Same key/payload replays persisted legs; divergence (or an unlinked original) conflicts.
	// Distinct keys deliberately permit identical financial movements.
	IdempotencyKey       string
	TenantID             uuid.UUID
	SourceAccountID      uuid.UUID
	DestinationAccountID uuid.UUID
	Amount               decimal.Decimal
	Date                 time.Time
	Description          string
	CreatedByUserID      *uuid.UUID
}

// TransferRepository defines persistence contracts for confirmed internal transfers.
// Ledger mutations and mutual linking must execute within an isolated atomic transaction.
type TransferRepository interface {
	// ConfirmTransfer atomically links two existing transactions as an internal transfer pair,
	// locking both rows FOR UPDATE, enforcing tenant/amount/account invariants, and updating both.
	ConfirmTransfer(ctx context.Context, tenantID, legAID, legBID uuid.UUID) (*domain.Transaction, *domain.Transaction, error)

	// CreateTransfer atomically persists both outbound and inbound legs of an internal transfer.
	CreateTransfer(ctx context.Context, outflow, inflow *domain.Transaction) error

	// UnlinkTransfer atomically breaks a transfer pairing without deleting either leg.
	UnlinkTransfer(ctx context.Context, tenantID, txID uuid.UUID) (*domain.Transaction, *domain.Transaction, error)
}

// TransferService defines inbound use cases for confirmed internal transfers.
type TransferService interface {
	// ConfirmTransfer validates and explicitly links two existing journal transactions as a transfer pair.
	ConfirmTransfer(ctx context.Context, tenantID, legAID, legBID uuid.UUID) (*domain.Transaction, *domain.Transaction, error)

	// CreateTransfer creates and persists a dual-leg internal transfer between own accounts.
	CreateTransfer(ctx context.Context, params CreateTransferParams) (*domain.Transaction, *domain.Transaction, error)

	// UnlinkTransfer unlinks an existing transfer pair, reverting legs to their signed categories.
	UnlinkTransfer(ctx context.Context, tenantID, txID uuid.UUID) (*domain.Transaction, *domain.Transaction, error)
}
