package postgres_test

import (
	"context"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/persistence/postgres"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestReviewGenericWritersCannotForgeTransferOperation(t *testing.T) {
	for _, op := range []string{"create", "batch", "update"} {
		t.Run(op, func(t *testing.T) {
			f := newReviewPairFixture(t)
			ctx := context.Background()
			row, err := domain.NewTransaction(domain.NewTransactionParams{TenantID: f.tenant, AccountID: f.out.AccountID, Date: f.out.Date, Amount: decimal.RequireFromString("-7.00"), Description: "Synthetic reserved operation", Type: domain.TransactionTypeExpense, Status: domain.TransactionStatusCompleted, Source: domain.SourceTypeManual})
			require.NoError(t, err)
			if op == "update" {
				require.NoError(t, f.repo.Create(ctx, row))
			}
			row.Metadata = map[string]any{"transfer_idempotency_key": "synthetic-forged-operation"}
			switch op {
			case "create":
				err = f.repo.Create(ctx, row)
			case "batch":
				err = f.repo.BatchCreate(ctx, []*domain.Transaction{row})
			case "update":
				err = f.repo.Update(ctx, row)
			}
			require.ErrorIs(t, err, domain.ErrReservedProvenance)
		})
	}
}

func TestReviewTransferOperationIdentityIsImmutable(t *testing.T) {
	for _, state := range []string{"linked", "unlinked"} {
		for _, mutation := range []string{"remove", "replace"} {
			t.Run(state+"/"+mutation, func(t *testing.T) {
				f := newReviewPairFixture(t)
				ctx := context.Background()
				transfers := postgres.NewTransferRepository(f.pool)
				out, in, err := domain.NewInternalTransfer(f.tenant, f.out.AccountID, f.in.AccountID, decimal.RequireFromString("3.00"), f.out.Date, "Synthetic immutable operation", nil)
				require.NoError(t, err)
				out.Metadata = map[string]any{"transfer_idempotency_key": "synthetic-immutable-operation"}
				in.Metadata = map[string]any{"transfer_idempotency_key": "synthetic-immutable-operation"}
				require.NoError(t, transfers.CreateTransfer(ctx, out, in))
				if state == "unlinked" {
					_, _, err = transfers.UnlinkTransfer(ctx, f.tenant, out.ID)
					require.NoError(t, err)
				}
				stored, err := f.repo.GetByID(ctx, f.tenant, out.ID)
				require.NoError(t, err)
				stored.Notes = "Synthetic editable note"
				require.NoError(t, f.repo.Update(ctx, stored))
				if mutation == "remove" {
					delete(stored.Metadata, "transfer_idempotency_key")
				} else {
					stored.Metadata["transfer_idempotency_key"] = "synthetic-replaced-operation"
				}
				require.ErrorIs(t, f.repo.Update(ctx, stored), domain.ErrReservedProvenance)
				actual, err := f.repo.GetByID(ctx, f.tenant, out.ID)
				require.NoError(t, err)
				require.Equal(t, "synthetic-immutable-operation", actual.Metadata["transfer_idempotency_key"])
				require.Equal(t, "Synthetic editable note", actual.Notes)
			})
		}
	}
}
