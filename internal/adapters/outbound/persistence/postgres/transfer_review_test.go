package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/parsers"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/persistence/postgres"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"runtime"
	"testing"
	"time"
)

func TestReviewTransferStableReplay(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()
	ctx := context.Background()
	tenant, user := createTestTenantAndUser(t, pool)
	accounts := postgres.NewAccountRepository(pool)
	a, _ := domain.NewAccount(tenant, user, "Synthetic source", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	b, _ := domain.NewAccount(tenant, user, "Synthetic destination", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	require.NoError(t, accounts.Create(ctx, a))
	require.NoError(t, accounts.Create(ctx, b))
	repo := postgres.NewTransferRepository(pool)
	ledger := postgres.NewTransactionRepository(pool)
	create := func(key string, amount int64) (*domain.Transaction, *domain.Transaction, error) {
		out, in, err := domain.NewInternalTransfer(tenant, a.ID, b.ID, decimal.NewFromInt(amount), time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), "Synthetic repeat", &user)
		require.NoError(t, err)
		out.Metadata = map[string]any{"transfer_idempotency_key": key}
		in.Metadata = map[string]any{"transfer_idempotency_key": key}
		out.CreatedAt = time.Date(2026, 2, 1, 0, 0, 0, 123456789, time.UTC)
		in.CreatedAt = out.CreatedAt
		err = repo.CreateTransfer(ctx, out, in)
		return out, in, err
	}
	out, in, err := create("key-one", 10)
	require.NoError(t, err)
	persisted, err := ledger.GetByID(ctx, tenant, out.ID)
	require.NoError(t, err)
	require.True(t, persisted.CreatedAt.Equal(out.CreatedAt), "creation response must be the persisted representation")
	replayOut, replayIn, err := create("key-one", 10)
	require.NoError(t, err)
	require.Equal(t, out.ID, replayOut.ID, "replay must return persisted IDs")
	require.Equal(t, in.ID, replayIn.ID)
	repeatOut, repeatIn, err := create("key-two", 10)
	require.NoError(t, err)
	require.NotEqual(t, out.ID, repeatOut.ID)
	for _, row := range []*domain.Transaction{out, in, repeatOut, repeatIn} {
		stored, getErr := ledger.GetByID(ctx, tenant, row.ID)
		require.NoError(t, getErr)
		require.Equal(t, domain.ComputeFingerprint(row.TenantID, row.AccountID, row.Date, row.Amount, row.CleanDescription, row.SequenceIndex), stored.Fingerprint)
	}
	_, _, err = create("key-one", 11)
	require.Error(t, err, "same key divergent payload must conflict")
	balance, err := accounts.GetByID(ctx, tenant, a.ID)
	require.NoError(t, err)
	require.Equal(t, "-20.00", balance.Balance.StringFixed(2))
}

func TestReviewTransferRejectsCrossCurrency(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()
	ctx := context.Background()
	tenant, user := createTestTenantAndUser(t, pool)
	accounts := postgres.NewAccountRepository(pool)
	ids := []uuid.UUID{}
	for _, currency := range []string{"BRL", "USD"} {
		a, _ := domain.NewAccount(tenant, user, "Synthetic "+currency, domain.AccountTypeChecking, currency, domain.VisibilityShared)
		require.NoError(t, accounts.Create(ctx, a))
		ids = append(ids, a.ID)
	}
	out, in, err := domain.NewInternalTransfer(tenant, ids[0], ids[1], decimal.NewFromInt(10), time.Now(), "Synthetic currency", &user)
	require.NoError(t, err)
	out.Metadata = map[string]any{"transfer_idempotency_key": "currency"}
	in.Metadata = map[string]any{"transfer_idempotency_key": "currency"}
	require.ErrorIs(t, postgres.NewTransferRepository(pool).CreateTransfer(ctx, out, in), domain.ErrInvalidCurrency)
	out.TransferPeerTransactionID = nil
	in.TransferPeerTransactionID = nil
	out.Type = domain.TransactionTypeExpense
	in.Type = domain.TransactionTypeIncome
	out.Metadata = nil
	in.Metadata = nil
	ledger := postgres.NewTransactionRepository(pool)
	require.NoError(t, ledger.Create(ctx, out))
	require.NoError(t, ledger.Create(ctx, in))
	_, _, err = postgres.NewTransferRepository(pool).ConfirmTransfer(ctx, tenant, out.ID, in.ID)
	require.ErrorIs(t, err, domain.ErrInvalidCurrency)
	stored, err := ledger.GetByID(ctx, tenant, out.ID)
	require.NoError(t, err)
	require.Nil(t, stored.TransferPeerTransactionID)
	require.Equal(t, domain.TransactionTypeExpense, stored.Type)
}

func TestReviewTransferCreateRequiresSettledMutualPair(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()
	ctx := context.Background()
	tenant, user := createTestTenantAndUser(t, pool)
	accounts := postgres.NewAccountRepository(pool)
	a, _ := domain.NewAccount(tenant, user, "Synthetic invalid A", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	b, _ := domain.NewAccount(tenant, user, "Synthetic invalid B", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	require.NoError(t, accounts.Create(ctx, a))
	require.NoError(t, accounts.Create(ctx, b))
	for _, scenario := range []string{"pending", "income_type", "missing_peer", "swapped_direction"} {
		t.Run(scenario, func(t *testing.T) {
			out, in, err := domain.NewInternalTransfer(tenant, a.ID, b.ID, decimal.NewFromInt(10), time.Now(), "Synthetic invalid "+scenario, &user)
			require.NoError(t, err)
			out.Metadata = map[string]any{"transfer_idempotency_key": scenario}
			in.Metadata = map[string]any{"transfer_idempotency_key": scenario}
			switch scenario {
			case "pending":
				out.Status = domain.TransactionStatusPending
			case "income_type":
				in.Type = domain.TransactionTypeIncome
			case "missing_peer":
				out.TransferPeerTransactionID = nil
			case "swapped_direction":
				out, in = in, out
			}
			require.Error(t, postgres.NewTransferRepository(pool).CreateTransfer(ctx, out, in))
		})
	}
	balance, err := accounts.GetByID(ctx, tenant, a.ID)
	require.NoError(t, err)
	require.True(t, balance.Balance.IsZero())
}

func TestReviewTransferConcurrentReplayAndRepetitions(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	tenant, user := createTestTenantAndUser(t, pool)
	accounts := postgres.NewAccountRepository(pool)
	a, _ := domain.NewAccount(tenant, user, "Synthetic concurrent A", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	b, _ := domain.NewAccount(tenant, user, "Synthetic concurrent B", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	require.NoError(t, accounts.Create(ctx, a))
	require.NoError(t, accounts.Create(ctx, b))
	type result struct {
		key     string
		out, in *domain.Transaction
		err     error
	}
	start := make(chan struct{})
	done := make(chan result, 8)
	for i := 0; i < 8; i++ {
		key := "replay"
		source, dest := a.ID, b.ID
		if i >= 4 {
			key = fmt.Sprintf("distinct-%d", i)
			if i%2 == 0 {
				source, dest = dest, source
			}
		}
		out, in, err := domain.NewInternalTransfer(tenant, source, dest, decimal.NewFromInt(10), time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), "Synthetic concurrent", &user)
		require.NoError(t, err)
		out.Metadata = map[string]any{"transfer_idempotency_key": key}
		in.Metadata = map[string]any{"transfer_idempotency_key": key}
		go func(key string, out, in *domain.Transaction) {
			<-start
			err := postgres.NewTransferRepository(pool).CreateTransfer(ctx, out, in)
			done <- result{key, out, in, err}
		}(key, out, in)
	}
	close(start)
	replayID := uuid.Nil
	ids := map[uuid.UUID]bool{}
	for i := 0; i < 8; i++ {
		r := <-done
		require.NoError(t, r.err)
		ids[r.out.ID] = true
		ids[r.in.ID] = true
		if r.key == "replay" {
			if replayID == uuid.Nil {
				replayID = r.out.ID
			}
			require.Equal(t, replayID, r.out.ID)
		}
	}
	require.Len(t, ids, 10)
	ledger := postgres.NewTransactionRepository(pool)
	for id := range ids {
		stored, err := ledger.GetByID(ctx, tenant, id)
		require.NoError(t, err)
		require.Equal(t, domain.ComputeFingerprint(stored.TenantID, stored.AccountID, stored.Date, stored.Amount, stored.CleanDescription, stored.SequenceIndex), stored.Fingerprint)
	}
	balance, err := accounts.GetByID(ctx, tenant, a.ID)
	require.NoError(t, err)
	require.Equal(t, "-10.00", balance.Balance.StringFixed(2))
}

func TestReviewTransferSharesCanonicalAllocationLock(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	tenant, user := createTestTenantAndUser(t, pool)
	accounts := postgres.NewAccountRepository(pool)
	a, _ := domain.NewAccount(tenant, user, "Synthetic shared A", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	b, _ := domain.NewAccount(tenant, user, "Synthetic shared B", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	require.NoError(t, accounts.Create(ctx, a))
	require.NoError(t, accounts.Create(ctx, b))
	if a.ID.String() > b.ID.String() {
		a, b = b, a
	}
	rows, err := parsers.NewCanonicalCSVParser().ParseCanonical(ctx, ports.CanonicalImportRequest{TenantID: tenant, UserID: user, AccountID: a.ID, SourceName: "synthetic-shared.csv", Content: []byte("ID,Data,Descrição,Valor,Tipo\nSHARED-1,01/02/2026,Synthetic shared,10.00,Despesa\n")})
	require.NoError(t, err)
	out, in, err := domain.NewInternalTransfer(tenant, a.ID, b.ID, decimal.NewFromInt(10), time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), "Synthetic shared", &user)
	require.NoError(t, err)
	require.Equal(t, rows[0].CleanDescription, out.CleanDescription)
	out.Metadata = map[string]any{"transfer_idempotency_key": "shared"}
	in.Metadata = map[string]any{"transfer_idempotency_key": "shared"}
	blocker, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback(context.Background()) }()
	var pid int
	require.NoError(t, blocker.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid))
	_, err = blocker.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, tenant.String()+"|"+a.ID.String())
	require.NoError(t, err)
	done := make(chan error, 2)
	go func() {
		_, importErr := postgres.NewTransactionRepository(pool).ImportCanonical(ctx, rows)
		done <- importErr
	}()
	go func() { done <- postgres.NewTransferRepository(pool).CreateTransfer(ctx, out, in) }()
	for {
		var waiters int
		err = pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid))`, pid).Scan(&waiters)
		require.NoError(t, err)
		if waiters == 2 {
			break
		}
		runtime.Gosched()
	}
	require.NoError(t, blocker.Commit(ctx))
	require.NoError(t, <-done)
	require.NoError(t, <-done)
	ledger := postgres.NewTransactionRepository(pool)
	stored, err := ledger.GetByID(ctx, tenant, rows[0].ID)
	require.NoError(t, err)
	manual, err := ledger.GetByID(ctx, tenant, out.ID)
	require.NoError(t, err)
	require.NotEqual(t, stored.SequenceIndex, manual.SequenceIndex)
	require.NotEqual(t, stored.Fingerprint, manual.Fingerprint)
	extra, err := parsers.NewCanonicalCSVParser().ParseCanonical(ctx, ports.CanonicalImportRequest{TenantID: tenant, UserID: user, AccountID: a.ID, SourceName: "synthetic-shared.csv", Content: []byte("ID,Data,Descrição,Valor,Tipo\nSHARED-2,01/02/2026,Synthetic shared,10.00,Despesa\n")})
	require.NoError(t, err)
	result, err := ledger.ImportCanonical(ctx, extra)
	require.NoError(t, err)
	require.Equal(t, 1, result.Inserted)
	require.NotEqual(t, stored.SequenceIndex, extra[0].SequenceIndex)
	require.NotEqual(t, manual.SequenceIndex, extra[0].SequenceIndex)
	balance, err := accounts.GetByID(ctx, tenant, a.ID)
	require.NoError(t, err)
	require.Equal(t, "-30.00", balance.Balance.StringFixed(2))
}

func TestReviewTransferConfirmationRejectsUnsettledAndStandalone(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()
	ctx := context.Background()
	tenant, user := createTestTenantAndUser(t, pool)
	accounts := postgres.NewAccountRepository(pool)
	a, _ := domain.NewAccount(tenant, user, "Synthetic confirm A", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	b, _ := domain.NewAccount(tenant, user, "Synthetic confirm B", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	require.NoError(t, accounts.Create(ctx, a))
	require.NoError(t, accounts.Create(ctx, b))
	ledger := postgres.NewTransactionRepository(pool)
	repo := postgres.NewTransferRepository(pool)
	for _, scenario := range []string{"pending_out", "pending_in", "void_out", "void_in", "standalone_transfer"} {
		t.Run(scenario, func(t *testing.T) {
			out, in, err := domain.NewInternalTransfer(tenant, a.ID, b.ID, decimal.NewFromInt(10), time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), "Synthetic confirm "+scenario, &user)
			require.NoError(t, err)
			out.TransferPeerTransactionID = nil
			in.TransferPeerTransactionID = nil
			out.Type = domain.TransactionTypeExpense
			in.Type = domain.TransactionTypeIncome
			switch scenario {
			case "pending_out":
				out.Status = domain.TransactionStatusPending
			case "pending_in":
				in.Status = domain.TransactionStatusPending
			case "void_out":
				out.Status = domain.TransactionStatusVoid
			case "void_in":
				in.Status = domain.TransactionStatusVoid
			case "standalone_transfer":
				out.Type = domain.TransactionTypeTransfer
				in.Type = domain.TransactionTypeTransfer
			}
			require.NoError(t, ledger.Create(ctx, out))
			require.NoError(t, ledger.Create(ctx, in))
			_, _, err = repo.ConfirmTransfer(ctx, tenant, out.ID, in.ID)
			require.Error(t, err)
			for _, leg := range []*domain.Transaction{out, in} {
				stored, err := ledger.GetByID(ctx, tenant, leg.ID)
				require.NoError(t, err)
				require.Nil(t, stored.TransferPeerTransactionID)
				require.Equal(t, leg.Status, stored.Status)
				require.Equal(t, leg.Type, stored.Type)
			}
		})
	}
}

func TestReviewTransferUnlinkOrderedLocks(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tenant, user := createTestTenantAndUser(t, pool)
	accounts := postgres.NewAccountRepository(pool)
	a, _ := domain.NewAccount(tenant, user, "Synthetic lock A", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	b, _ := domain.NewAccount(tenant, user, "Synthetic lock B", domain.AccountTypeChecking, "BRL", domain.VisibilityShared)
	require.NoError(t, accounts.Create(ctx, a))
	require.NoError(t, accounts.Create(ctx, b))
	out, in, err := domain.NewInternalTransfer(tenant, a.ID, b.ID, decimal.NewFromInt(10), time.Now(), "Synthetic lock", &user)
	require.NoError(t, err)
	out.Metadata = map[string]any{"transfer_idempotency_key": "lock"}
	in.Metadata = map[string]any{"transfer_idempotency_key": "lock"}
	repo := postgres.NewTransferRepository(pool)
	require.NoError(t, repo.CreateTransfer(ctx, out, in))
	low, high := out.ID, in.ID
	if low.String() > high.String() {
		low, high = high, low
	}
	blocker, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback(context.Background()) }()
	_, err = blocker.Exec(ctx, `SELECT set_config('app.current_tenant_id',$1,true)`, tenant.String())
	require.NoError(t, err)
	var blockerPID int
	require.NoError(t, blocker.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&blockerPID))
	_, err = blocker.Exec(ctx, `SELECT id FROM transactions WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, tenant, low)
	require.NoError(t, err)
	done := make(chan error, 2)
	go func() { _, _, unlinkErr := repo.UnlinkTransfer(ctx, tenant, high); done <- unlinkErr }()
	// Observe the real waiter before probing the high leg: no timing sleep.
	for {
		var waiting bool
		err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)) AND query LIKE '%ORDER BY id ASC%')`, blockerPID).Scan(&waiting)
		require.NoError(t, err)
		if waiting {
			break
		}
		runtime.Gosched()
	}
	probe, err := pool.Begin(ctx)
	require.NoError(t, err)
	_, err = probe.Exec(ctx, `SELECT set_config('app.current_tenant_id',$1,true)`, tenant.String())
	require.NoError(t, err)
	_, probeErr := probe.Exec(ctx, `SELECT id FROM transactions WHERE tenant_id=$1 AND id=$2 FOR UPDATE NOWAIT`, tenant, high)
	require.NoError(t, probe.Rollback(ctx))
	// Queue the opposite endpoint, then release the low row.
	go func() { _, _, unlinkErr := repo.UnlinkTransfer(ctx, tenant, low); done <- unlinkErr }()
	require.NoError(t, blocker.Commit(ctx))
	first, second := <-done, <-done
	require.NoError(t, probeErr, "unlink must not lock the high endpoint before low")
	require.True(t, (first == nil && errors.Is(second, domain.ErrTransferNotLinked)) || (second == nil && errors.Is(first, domain.ErrTransferNotLinked)), "results: %v / %v", first, second)
	stored, err := postgres.NewTransactionRepository(pool).GetByID(ctx, tenant, low)
	require.NoError(t, err)
	require.Nil(t, stored.TransferPeerTransactionID)
}
