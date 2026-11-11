package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/app"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

type mockTransferRepo struct {
	confirmCalls int
	createCalls  int
	unlinkCalls  int
}

func (m *mockTransferRepo) ConfirmTransfer(_ context.Context, tenantID, legAID, legBID uuid.UUID) (*domain.Transaction, *domain.Transaction, error) {
	m.confirmCalls++
	t1 := &domain.Transaction{ID: legAID, TenantID: tenantID, Type: domain.TransactionTypeTransfer, TransferPeerTransactionID: &legBID}
	t2 := &domain.Transaction{ID: legBID, TenantID: tenantID, Type: domain.TransactionTypeTransfer, TransferPeerTransactionID: &legAID}
	return t1, t2, nil
}

func (m *mockTransferRepo) CreateTransfer(_ context.Context, _, _ *domain.Transaction) error {
	m.createCalls++
	return nil
}

func (m *mockTransferRepo) UnlinkTransfer(_ context.Context, tenantID, txID uuid.UUID) (*domain.Transaction, *domain.Transaction, error) {
	m.unlinkCalls++
	peerID := uuid.New()
	t1 := &domain.Transaction{ID: txID, TenantID: tenantID, Type: domain.TransactionTypeExpense}
	t2 := &domain.Transaction{ID: peerID, TenantID: tenantID, Type: domain.TransactionTypeIncome}
	return t1, t2, nil
}

func TestTransferService_UseCases(t *testing.T) {
	repo := &mockTransferRepo{}
	svc := app.NewTransferService(repo)
	ctx := context.Background()

	tenantID := uuid.New()
	acc1 := uuid.New()
	acc2 := uuid.New()

	t.Run("CreateTransfer validation and delegation", func(t *testing.T) {
		_, _, err := svc.CreateTransfer(ctx, ports.CreateTransferParams{
			TenantID:             tenantID,
			SourceAccountID:      acc1,
			DestinationAccountID: acc1, // Same account!
			Amount:               decimal.NewFromFloat(100.00),
		})
		require.ErrorIs(t, err, domain.ErrTransferSameAccount)

		_, _, err = svc.CreateTransfer(ctx, ports.CreateTransferParams{
			TenantID:             tenantID,
			SourceAccountID:      acc1,
			DestinationAccountID: acc2,
			Amount:               decimal.NewFromFloat(-100.00), // Negative!
		})
		require.ErrorIs(t, err, domain.ErrInvalidAmount)

		out, in, err := svc.CreateTransfer(ctx, ports.CreateTransferParams{
			TenantID:             tenantID,
			SourceAccountID:      acc1,
			DestinationAccountID: acc2,
			Amount:               decimal.NewFromFloat(100.00),
			Date:                 time.Now().UTC(),
			Description:          "Test transfer",
			IdempotencyKey:       "synthetic-service",
		})
		require.NoError(t, err)
		require.NotNil(t, out)
		require.NotNil(t, in)
		require.Equal(t, 1, repo.createCalls)
	})

	t.Run("ConfirmTransfer validation and delegation", func(t *testing.T) {
		idA := uuid.New()
		idB := uuid.New()

		_, _, err := svc.ConfirmTransfer(ctx, tenantID, idA, idA)
		require.ErrorIs(t, err, domain.ErrInvalidTransferPair)

		legA, legB, err := svc.ConfirmTransfer(ctx, tenantID, idA, idB)
		require.NoError(t, err)
		require.Equal(t, idA, legA.ID)
		require.Equal(t, idB, legB.ID)
		require.Equal(t, 1, repo.confirmCalls)
	})

	t.Run("UnlinkTransfer validation and delegation", func(t *testing.T) {
		idA := uuid.New()
		legA, legB, err := svc.UnlinkTransfer(ctx, tenantID, idA)
		require.NoError(t, err)
		require.Equal(t, idA, legA.ID)
		require.NotNil(t, legB)
		require.Equal(t, 1, repo.unlinkCalls)
	})
}
