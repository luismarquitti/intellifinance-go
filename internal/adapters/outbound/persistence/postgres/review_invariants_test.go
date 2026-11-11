package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/parsers"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/persistence/postgres"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestReviewImportedSourceFactsCannotChange(t *testing.T) {
	mutations := map[string]func(*domain.Transaction, *domain.Account){
		"account":           func(row *domain.Transaction, target *domain.Account) { row.AccountID = target.ID },
		"amount":            func(row *domain.Transaction, _ *domain.Account) { row.Amount = decimal.RequireFromString("-20.50") },
		"date":              func(row *domain.Transaction, _ *domain.Account) { row.Date = row.Date.AddDate(0, 0, 1) },
		"description":       func(row *domain.Transaction, _ *domain.Account) { row.Description = "Changed financial source" },
		"clean_description": func(row *domain.Transaction, _ *domain.Account) { row.CleanDescription = "CHANGED FINANCIAL SOURCE" },
		"source":            func(row *domain.Transaction, _ *domain.Account) { row.Source = domain.SourceTypeManual },
		"external_id":       func(row *domain.Transaction, _ *domain.Account) { row.ExternalID = "FORGED-IDENTITY" },
		"sequence":          func(row *domain.Transaction, _ *domain.Account) { row.SequenceIndex++ },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			pool := getTestPool(t)
			defer pool.Close()
			ctx := context.Background()
			tenant, user := createTestTenantAndUser(t, pool)
			accounts := postgres.NewAccountRepository(pool)
			account, err := domain.NewAccount(tenant, user, "Synthetic retained source", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
			require.NoError(t, err)
			require.NoError(t, accounts.Create(ctx, account))
			target, err := domain.NewAccount(tenant, user, "Synthetic target", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
			require.NoError(t, err)
			require.NoError(t, accounts.Create(ctx, target))
			req := ports.CanonicalImportRequest{TenantID: tenant, UserID: user, AccountID: account.ID, SourceName: "synthetic.csv", Content: []byte("ID,Data,Descrição,Valor,Tipo\nSYN-X,01/02/2026,Repeated source,10.50,Despesa\nSYN-Y,01/02/2026,Repeated source,10.50,Despesa\n")}
			parser := parsers.NewCanonicalCSVParser()
			rows, err := parser.ParseCanonical(ctx, req)
			require.NoError(t, err)
			repo := postgres.NewTransactionRepository(pool)
			_, err = repo.ImportCanonical(ctx, rows)
			require.NoError(t, err)
			before, err := repo.GetByID(ctx, tenant, rows[0].ID)
			require.NoError(t, err)
			changed, err := repo.GetByID(ctx, tenant, rows[0].ID)
			require.NoError(t, err)
			mutate(changed, target)
			require.Error(t, repo.Update(ctx, changed), "original facts must remain immutable; use classification/notes rather than changing the statement")
			after, err := repo.GetByID(ctx, tenant, rows[0].ID)
			require.NoError(t, err)
			require.Equal(t, before, after)
			again, err := parser.ParseCanonical(ctx, req)
			require.NoError(t, err)
			report, err := repo.ImportCanonical(ctx, again)
			require.NoError(t, err)
			require.Zero(t, report.Inserted)
			require.Equal(t, 2, report.Duplicates)
			req.Content = []byte("ID,Data,Descrição,Valor,Tipo\nSYN-Z,01/02/2026,Repeated source,10.50,Despesa\n")
			extra, err := parser.ParseCanonical(ctx, req)
			require.NoError(t, err)
			report, err = repo.ImportCanonical(ctx, extra)
			require.NoError(t, err)
			require.Equal(t, 1, report.Inserted)
			balance, err := accounts.GetByID(ctx, tenant, account.ID)
			require.NoError(t, err)
			require.Equal(t, "-31.50", balance.Balance.StringFixed(2))
			moved, err := accounts.GetByID(ctx, tenant, target.ID)
			require.NoError(t, err)
			require.Equal(t, "0.00", moved.Balance.StringFixed(2))
		})
	}
}

func TestReviewLinkedPairRejectsGenericMutation(t *testing.T) {
	mutations := map[string]func(*domain.Transaction, *domain.Transaction){
		"amount":       func(row, _ *domain.Transaction) { row.Amount = decimal.RequireFromString("-200.00") },
		"account":      func(row, peer *domain.Transaction) { row.AccountID = peer.AccountID },
		"type":         func(row, _ *domain.Transaction) { row.Type = domain.TransactionTypeExpense },
		"status":       func(row, _ *domain.Transaction) { row.Status = domain.TransactionStatusVoid },
		"remove_peer":  func(row, _ *domain.Transaction) { row.TransferPeerTransactionID = nil },
		"replace_peer": func(row, _ *domain.Transaction) { id := uuid.New(); row.TransferPeerTransactionID = &id },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			f := newReviewPairFixture(t)
			ctx := context.Background()
			before, err := f.repo.GetByID(ctx, f.tenant, f.out.ID)
			require.NoError(t, err)
			changed, err := f.repo.GetByID(ctx, f.tenant, f.out.ID)
			require.NoError(t, err)
			mutate(changed, f.in)
			require.ErrorIs(t, f.repo.Update(ctx, changed), domain.ErrTransferAlreadyLinked)
			after, err := f.repo.GetByID(ctx, f.tenant, f.out.ID)
			require.NoError(t, err)
			require.Equal(t, before, after)
			requireReviewPairIntact(t, f)
		})
	}
	t.Run("stale_snapshot_cannot_unlink", func(t *testing.T) {
		f := newReviewPairFixture(t)
		f.stale.Notes = "Note from before confirmation"
		require.ErrorIs(t, f.repo.Update(context.Background(), f.stale), domain.ErrTransferAlreadyLinked)
		requireReviewPairIntact(t, f)
	})
	t.Run("notes_and_category_remain_editable", func(t *testing.T) {
		f := newReviewPairFixture(t)
		ctx := context.Background()
		stored, err := f.repo.GetByID(ctx, f.tenant, f.out.ID)
		require.NoError(t, err)
		stored.Notes = "Synthetic pair note"
		require.NoError(t, f.repo.Update(ctx, stored))
		requireReviewPairIntact(t, f)
	})
}

func TestReviewLinkedPairCannotBeDeletedGenerically(t *testing.T) {
	for _, op := range []string{"delete", "batch_one", "batch_both"} {
		t.Run(op, func(t *testing.T) {
			f := newReviewPairFixture(t)
			ctx := context.Background()
			var err error
			switch op {
			case "delete":
				err = f.repo.Delete(ctx, f.tenant, f.out.ID)
			case "batch_one":
				err = f.repo.BatchDelete(ctx, f.tenant, []uuid.UUID{f.out.ID})
			case "batch_both":
				err = f.repo.BatchDelete(ctx, f.tenant, []uuid.UUID{f.out.ID, f.in.ID})
			}
			require.ErrorIs(t, err, domain.ErrTransferAlreadyLinked)
			requireReviewPairIntact(t, f)
		})
	}
}

type reviewPairFixture struct {
	pool           *pgxpool.Pool
	tenant         uuid.UUID
	repo           *postgres.TransactionRepository
	accounts       *postgres.AccountRepository
	out, in, stale *domain.Transaction
}

func newReviewPairFixture(t *testing.T) *reviewPairFixture {
	t.Helper()
	pool := getTestPool(t)
	t.Cleanup(pool.Close)
	ctx := context.Background()
	tenant, user := createTestTenantAndUser(t, pool)
	accounts := postgres.NewAccountRepository(pool)
	repo := postgres.NewTransactionRepository(pool)
	origin, err := domain.NewAccount(tenant, user, "Synthetic pair origin", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	require.NoError(t, err)
	require.NoError(t, accounts.Create(ctx, origin))
	target, err := domain.NewAccount(tenant, user, "Synthetic pair target", domain.AccountTypeSavings, "BRL", domain.VisibilityShared)
	require.NoError(t, err)
	require.NoError(t, accounts.Create(ctx, target))
	makeLeg := func(account uuid.UUID, amount string, kind domain.TransactionType) *domain.Transaction {
		row, e := domain.NewTransaction(domain.NewTransactionParams{TenantID: tenant, AccountID: account, Date: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), Amount: decimal.RequireFromString(amount), Description: "Synthetic pair source", Type: kind, Status: domain.TransactionStatusCompleted, Source: domain.SourceTypeManual})
		require.NoError(t, e)
		require.NoError(t, repo.Create(ctx, row))
		return row
	}
	out := makeLeg(origin.ID, "-100.00", domain.TransactionTypeExpense)
	in := makeLeg(target.ID, "100.00", domain.TransactionTypeIncome)
	stale, err := repo.GetByID(ctx, tenant, out.ID)
	require.NoError(t, err)
	out, in, err = postgres.NewTransferRepository(pool).ConfirmTransfer(ctx, tenant, out.ID, in.ID)
	require.NoError(t, err)
	return &reviewPairFixture{pool: pool, tenant: tenant, repo: repo, accounts: accounts, out: out, in: in, stale: stale}
}
func requireReviewPairIntact(t *testing.T, f *reviewPairFixture) {
	t.Helper()
	ctx := context.Background()
	out, err := f.repo.GetByID(ctx, f.tenant, f.out.ID)
	require.NoError(t, err)
	in, err := f.repo.GetByID(ctx, f.tenant, f.in.ID)
	require.NoError(t, err)
	require.Equal(t, &in.ID, out.TransferPeerTransactionID)
	require.Equal(t, &out.ID, in.TransferPeerTransactionID)
	require.Equal(t, domain.TransactionTypeTransfer, out.Type)
	require.Equal(t, domain.TransactionTypeTransfer, in.Type)
	account, err := f.accounts.GetByID(ctx, f.tenant, out.AccountID)
	require.NoError(t, err)
	require.Equal(t, "-100.00", account.Balance.StringFixed(2))
	account, err = f.accounts.GetByID(ctx, f.tenant, in.AccountID)
	require.NoError(t, err)
	require.Equal(t, "100.00", account.Balance.StringFixed(2))
	summary, err := f.repo.Summarize(ctx, f.tenant, ports.TransactionFilter{})
	require.NoError(t, err)
	require.Equal(t, "0.00", summary.TotalIncome.StringFixed(2))
	require.Equal(t, "0.00", summary.TotalExpense.StringFixed(2))
}

func TestReviewGenericWriterRechecksAfterConcurrentConfirmation(t *testing.T) {
	for _, operation := range []string{"update", "delete", "batch_delete"} {
		t.Run(operation, func(t *testing.T) {
			f := newReviewPairFixture(t)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			transfers := postgres.NewTransferRepository(f.pool)
			_, _, err := transfers.UnlinkTransfer(ctx, f.tenant, f.out.ID)
			require.NoError(t, err)
			stale, err := f.repo.GetByID(ctx, f.tenant, f.out.ID)
			require.NoError(t, err)
			stale.Notes = "Synthetic concurrent note"
			confirming, err := f.pool.Begin(ctx)
			require.NoError(t, err)
			defer func() { _ = confirming.Rollback(context.Background()) }()
			_, err = confirming.Exec(ctx, `SELECT set_config('app.current_tenant_id',$1,true)`, f.tenant.String())
			require.NoError(t, err)
			_, _, err = transfers.WithTx(confirming).ConfirmTransfer(ctx, f.tenant, f.out.ID, f.in.ID)
			require.NoError(t, err)
			mutating, err := f.pool.Begin(ctx)
			require.NoError(t, err)
			defer func() { _ = mutating.Rollback(context.Background()) }()
			_, err = mutating.Exec(ctx, `SELECT set_config('app.current_tenant_id',$1,true)`, f.tenant.String())
			require.NoError(t, err)
			var pid int
			require.NoError(t, mutating.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid))
			done := make(chan error, 1)
			go func() {
				repo := f.repo.WithTx(mutating)
				switch operation {
				case "update":
					done <- repo.Update(ctx, stale)
				case "delete":
					done <- repo.Delete(ctx, f.tenant, f.out.ID)
				case "batch_delete":
					done <- repo.BatchDelete(ctx, f.tenant, []uuid.UUID{f.out.ID})
				}
			}()
			// Observe a real PostgreSQL lock wait before allowing confirmation to commit.
			require.Eventually(t, func() bool {
				var waiting bool
				e := f.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE pid=$1 AND wait_event_type='Lock')`, pid).Scan(&waiting)
				return e == nil && waiting
			}, 3*time.Second, 10*time.Millisecond)
			require.NoError(t, confirming.Commit(ctx))
			select {
			case err = <-done:
				require.ErrorIs(t, err, domain.ErrTransferAlreadyLinked)
			case <-ctx.Done():
				t.Fatal("generic writer did not finish after confirmation commit")
			}
			require.NoError(t, mutating.Rollback(ctx))
			requireReviewPairIntact(t, f)
		})
	}
}

func TestReviewGenericWritersCannotMintTransferPeer(t *testing.T) {
	for _, operation := range []string{"create", "batch_create", "update"} {
		t.Run(operation, func(t *testing.T) {
			f := newReviewPairFixture(t)
			ctx := context.Background()
			row, err := domain.NewTransaction(domain.NewTransactionParams{TenantID: f.tenant, AccountID: f.out.AccountID, Date: f.out.Date, Amount: decimal.RequireFromString("-3.00"), Description: "Synthetic third leg", Type: domain.TransactionTypeExpense, Status: domain.TransactionStatusCompleted, Source: domain.SourceTypeManual})
			require.NoError(t, err)
			if operation == "update" {
				require.NoError(t, f.repo.Create(ctx, row))
			}
			row.TransferPeerTransactionID = &f.in.ID
			switch operation {
			case "create":
				err = f.repo.Create(ctx, row)
			case "batch_create":
				err = f.repo.BatchCreate(ctx, []*domain.Transaction{row})
			case "update":
				err = f.repo.Update(ctx, row)
			}
			require.ErrorIs(t, err, domain.ErrTransferAlreadyLinked)
			if operation == "update" {
				stored, e := f.repo.GetByID(ctx, f.tenant, row.ID)
				require.NoError(t, e)
				require.Nil(t, stored.TransferPeerTransactionID)
			} else {
				_, e := f.repo.GetByID(ctx, f.tenant, row.ID)
				require.ErrorIs(t, e, domain.ErrNotFound)
			}
		})
	}
}

func TestReviewGenericWritersCannotForgeProvenance(t *testing.T) {
	for _, operation := range []string{"create", "batch_create", "update"} {
		t.Run(operation, func(t *testing.T) {
			pool := getTestPool(t)
			defer pool.Close()
			ctx := context.Background()
			tenant, user := createTestTenantAndUser(t, pool)
			accounts := postgres.NewAccountRepository(pool)
			account, err := domain.NewAccount(tenant, user, "Synthetic untrusted metadata", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
			require.NoError(t, err)
			require.NoError(t, accounts.Create(ctx, account))
			row, err := domain.NewTransaction(domain.NewTransactionParams{TenantID: tenant, AccountID: account.ID, Date: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), Amount: decimal.RequireFromString("-7.00"), Description: "Synthetic ordinary transaction", Type: domain.TransactionTypeExpense, Status: domain.TransactionStatusCompleted, Source: domain.SourceTypeManual})
			require.NoError(t, err)
			repo := postgres.NewTransactionRepository(pool)
			if operation == "update" {
				require.NoError(t, repo.Create(ctx, row))
			}
			row.Metadata = map[string]any{"provenance": map[string]any{"row_sha256": "forged-source-identity"}}
			switch operation {
			case "create":
				err = repo.Create(ctx, row)
			case "batch_create":
				err = repo.BatchCreate(ctx, []*domain.Transaction{row})
			case "update":
				err = repo.Update(ctx, row)
			}
			require.Error(t, err, "generic writers must not mint a retained-source identity")
			stored, count, err := repo.List(ctx, tenant, ports.TransactionFilter{AccountID: &account.ID, Limit: 100, Page: 1})
			require.NoError(t, err)
			if operation == "update" {
				require.EqualValues(t, 1, count)
				require.NotContains(t, stored[0].Metadata, "provenance")
			} else {
				require.Zero(t, count)
			}
		})
	}
}
