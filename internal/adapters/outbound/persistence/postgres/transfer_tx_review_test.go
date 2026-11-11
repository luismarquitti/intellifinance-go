package postgres_test

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/persistence/postgres"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestReviewTransferExternalTransactionRollback(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()
	ctx := context.Background()
	tenant, user := createTestTenantAndUser(t, pool)
	accounts := postgres.NewAccountRepository(pool)
	a, _ := domain.NewAccount(tenant, user, "Synthetic rollback A", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	b, _ := domain.NewAccount(tenant, user, "Synthetic rollback B", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	require.NoError(t, accounts.Create(ctx, a))
	require.NoError(t, accounts.Create(ctx, b))
	ledger := postgres.NewTransactionRepository(pool)
	repo := postgres.NewTransferRepository(pool)
	rollback := errors.New("synthetic outer rollback")
	for _, binding := range []string{"WithTx", "ContextWithTx"} {
		for _, operation := range []string{"Create", "Confirm", "Unlink"} {
			t.Run(binding+"/"+operation, func(t *testing.T) {
				out, in, err := domain.NewInternalTransfer(tenant, a.ID, b.ID, decimal.NewFromInt(10), time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), "Synthetic "+binding+operation, &user)
				require.NoError(t, err)
				if operation == "Confirm" {
					out.TransferPeerTransactionID = nil
					in.TransferPeerTransactionID = nil
					out.Type = domain.TransactionTypeExpense
					in.Type = domain.TransactionTypeIncome
					require.NoError(t, ledger.Create(ctx, out))
					require.NoError(t, ledger.Create(ctx, in))
				} else {
					out.Metadata = map[string]any{"transfer_idempotency_key": binding + operation}
					in.Metadata = map[string]any{"transfer_idempotency_key": binding + operation}
					if operation == "Unlink" {
						require.NoError(t, repo.CreateTransfer(ctx, out, in))
					}
				}
				err = postgres.WithTenantTx(ctx, pool, tenant, user, func(tx pgx.Tx) error {
					bound := repo.WithTx(tx)
					boundCtx := ctx
					if binding == "ContextWithTx" {
						bound = repo
						boundCtx = postgres.ContextWithTx(ctx, tx)
					}
					switch operation {
					case "Create":
						err = bound.CreateTransfer(boundCtx, out, in)
					case "Confirm":
						_, _, err = bound.ConfirmTransfer(boundCtx, tenant, out.ID, in.ID)
					case "Unlink":
						_, _, err = bound.UnlinkTransfer(boundCtx, tenant, out.ID)
					}
					if err != nil {
						return err
					}
					return rollback
				})
				require.ErrorIs(t, err, rollback)
				stored, err := ledger.GetByID(ctx, tenant, out.ID)
				if operation == "Create" {
					require.ErrorIs(t, err, domain.ErrNotFound)
					return
				}
				require.NoError(t, err)
				if operation == "Confirm" {
					require.Nil(t, stored.TransferPeerTransactionID)
					require.Equal(t, domain.TransactionTypeExpense, stored.Type)
				} else {
					require.Equal(t, &in.ID, stored.TransferPeerTransactionID)
					require.Equal(t, domain.TransactionTypeTransfer, stored.Type)
				}
			})
		}
	}
}
