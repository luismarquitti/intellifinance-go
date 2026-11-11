package postgres_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/persistence/postgres"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestReviewTransferConcurrentDeadlockAvoidance(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenant, user := createTestTenantAndUser(t, pool)
	accountsRepo := postgres.NewAccountRepository(pool)
	transferRepo := postgres.NewTransferRepository(pool)
	txRepo := postgres.NewTransactionRepository(pool)

	accA, err := domain.NewAccount(tenant, user, "Synthetic Deadlock Account A", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	require.NoError(t, err)
	require.NoError(t, accountsRepo.Create(ctx, accA))

	accB, err := domain.NewAccount(tenant, user, "Synthetic Deadlock Account B", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	require.NoError(t, err)
	require.NoError(t, accountsRepo.Create(ctx, accB))

	// 1. Test concurrent CreateTransfer in opposite directions
	const concurrency = 8
	var wg sync.WaitGroup
	errCh := make(chan error, concurrency)
	startSignal := make(chan struct{})

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		workerID := i
		go func() {
			defer wg.Done()
			<-startSignal

			var srcID, dstID uuid.UUID
			if workerID%2 == 0 {
				srcID, dstID = accA.ID, accB.ID
			} else {
				srcID, dstID = accB.ID, accA.ID
			}

			amount := decimal.RequireFromString("10.00")
			desc := fmt.Sprintf("Synthetic concurrent create transfer %d", workerID)
			now := time.Date(2026, 2, 1, 10, workerID, 0, 0, time.UTC)

			out, in, newErr := domain.NewInternalTransfer(tenant, srcID, dstID, amount, now, desc, &user)
			if newErr != nil {
				errCh <- newErr
				return
			}
			out.Metadata = map[string]any{"transfer_idempotency_key": fmt.Sprintf("deadlock-avoidance-%d", workerID)}
			in.Metadata = map[string]any{"transfer_idempotency_key": fmt.Sprintf("deadlock-avoidance-%d", workerID)}

			if createErr := transferRepo.CreateTransfer(ctx, out, in); createErr != nil {
				errCh <- fmt.Errorf("worker %d failed: %w", workerID, createErr)
				return
			}
		}()
	}

	close(startSignal)
	wg.Wait()
	close(errCh)

	for e := range errCh {
		require.NoError(t, e, "concurrent CreateTransfer must serialize without deadlocks")
	}

	// 2. Test concurrent ConfirmTransfer in opposite directions
	const confirmPairs = 4
	type pair struct {
		legA *domain.Transaction
		legB *domain.Transaction
	}
	pairs := make([]pair, confirmPairs)

	for i := 0; i < confirmPairs; i++ {
		var srcAcc, dstAcc uuid.UUID
		if i%2 == 0 {
			srcAcc, dstAcc = accA.ID, accB.ID
		} else {
			srcAcc, dstAcc = accB.ID, accA.ID
		}
		amt := decimal.RequireFromString("25.00")
		d := time.Date(2026, 2, 2, 10, i, 0, 0, time.UTC)

		legOut, outErr := domain.NewTransaction(domain.NewTransactionParams{
			TenantID:  tenant,
			AccountID: srcAcc,
			Date:      d,
			Amount:    amt.Neg(),
			Description: fmt.Sprintf("Confirm candidate out %d", i),
			Type:      domain.TransactionTypeExpense,
			Status:    domain.TransactionStatusCompleted,
			Source:    domain.SourceTypeManual,
		})
		require.NoError(t, outErr)
		require.NoError(t, txRepo.Create(ctx, legOut))

		legIn, inErr := domain.NewTransaction(domain.NewTransactionParams{
			TenantID:  tenant,
			AccountID: dstAcc,
			Date:      d,
			Amount:    amt,
			Description: fmt.Sprintf("Confirm candidate in %d", i),
			Type:      domain.TransactionTypeIncome,
			Status:    domain.TransactionStatusCompleted,
			Source:    domain.SourceTypeManual,
		})
		require.NoError(t, inErr)
		require.NoError(t, txRepo.Create(ctx, legIn))

		pairs[i] = pair{legA: legOut, legB: legIn}
	}

	var confirmWg sync.WaitGroup
	confirmErrCh := make(chan error, confirmPairs)
	confirmStart := make(chan struct{})

	for i := 0; i < confirmPairs; i++ {
		confirmWg.Add(1)
		idx := i
		go func() {
			defer confirmWg.Done()
			<-confirmStart

			p := pairs[idx]
			_, _, confirmErr := transferRepo.ConfirmTransfer(ctx, tenant, p.legA.ID, p.legB.ID)
			if confirmErr != nil {
				confirmErrCh <- fmt.Errorf("confirm worker %d failed: %w", idx, confirmErr)
			}
		}()
	}

	close(confirmStart)
	confirmWg.Wait()
	close(confirmErrCh)

	for e := range confirmErrCh {
		require.NoError(t, e, "concurrent ConfirmTransfer must serialize without deadlocks")
	}

	// Verify ledger balance conservation
	finalA, err := accountsRepo.GetByID(ctx, tenant, accA.ID)
	require.NoError(t, err)
	finalB, err := accountsRepo.GetByID(ctx, tenant, accB.ID)
	require.NoError(t, err)

	total := finalA.Balance.Add(finalB.Balance)
	require.True(t, total.IsZero(), "sum of balances between transfer pairs must equal zero")
}
