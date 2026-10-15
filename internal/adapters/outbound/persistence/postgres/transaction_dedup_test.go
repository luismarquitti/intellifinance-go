package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"

	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/persistence/postgres"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/persistence/postgres/db"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

func createTestAccount(ctx context.Context, pool *db.Queries, tenantID, userID uuid.UUID, name string) (*db.Account, error) {
	acc, err := pool.CreateAccount(ctx, db.CreateAccountParams{
		ID:          uuid.New(),
		TenantID:    tenantID,
		OwnerUserID: userID,
		Name:        name + " " + uuid.NewString()[:8],
		Type:        db.AccountTypeCHECKING,
		Currency:    "BRL",
		Balance:     decimal.NewFromFloat(5000.00),
		Visibility:  db.VisibilityTypeSHARED,
		ColorHex:    "#10B981",
		IconName:    "wallet",
		IsActive:    true,
		CreatedAt:   pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		UpdatedAt:   pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	if err != nil {
		return nil, err
	}
	return &acc, nil
}

func TestTransactionRepository_FindExistingFingerprints(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	ctx := context.Background()
	txManager := postgres.NewTxManager(pool)
	repo := postgres.NewTransactionRepository(pool)

	tenant := createTestTenant(ctx, pool, t, "Tenant Fingerprints")
	user := createTestUser(ctx, pool, t, "user-fp")

	var account *db.Account
	err := txManager.WithTenantTx(ctx, tenant.ID, user.ID, func(tx pgx.Tx) error {
		q := db.New(tx)
		var err error
		account, err = createTestAccount(ctx, q, tenant.ID, user.ID, "Conta Teste")
		return err
	})
	if err != nil {
		t.Fatalf("failed to create test account: %v", err)
	}

	// 1. Empty fingerprints slice returns empty map without DB round-trip error
	emptyResult, err := repo.FindExistingFingerprints(ctx, tenant.ID, []string{})
	if err != nil {
		t.Fatalf("unexpected error on empty fingerprints: %v", err)
	}
	if len(emptyResult) != 0 {
		t.Errorf("expected empty map, got len %d", len(emptyResult))
	}

	// 2. Create and insert a real transaction
	tx1, err := domain.NewTransaction(domain.NewTransactionParams{
		TenantID:        tenant.ID,
		AccountID:       account.ID,
		CreatedByUserID: &user.ID,
		TransactedAt:    time.Now().In(domain.BrazilLocation),
		Amount:          decimal.NewFromFloat(-79.90),
		Description:     "MERCADO SILVA",
		Type:            domain.TransactionTypeExpense,
		Status:          domain.TransactionStatusCompleted,
		Visibility:      domain.VisibilityShared,
		Source:          domain.SourceTypeOFX,
		ExternalID:      "FITID-SILVA-1",
		SequenceIndex:   0,
	})
	if err != nil {
		t.Fatalf("failed to construct transaction: %v", err)
	}

	err = txManager.WithTenantTx(ctx, tenant.ID, user.ID, func(tx pgx.Tx) error {
		return repo.WithTx(tx).Create(ctx, tx1)
	})
	if err != nil {
		t.Fatalf("failed to insert transaction: %v", err)
	}

	// 3. Query existing and non-existent fingerprints
	nonExistentFP1 := "0000000000000000000000000000000000000000000000000000000000000001"
	nonExistentFP2 := "0000000000000000000000000000000000000000000000000000000000000002"

	queryFPs := []string{tx1.Fingerprint, nonExistentFP1, nonExistentFP2}
	foundMap, err := repo.FindExistingFingerprints(ctx, tenant.ID, queryFPs)
	if err != nil {
		t.Fatalf("failed to find existing fingerprints: %v", err)
	}

	if len(foundMap) != 1 {
		t.Errorf("expected exactly 1 found fingerprint, got %d", len(foundMap))
	}
	if !foundMap[tx1.Fingerprint] {
		t.Errorf("expected fingerprint %s to be found", tx1.Fingerprint)
	}
	if foundMap[nonExistentFP1] || foundMap[nonExistentFP2] {
		t.Error("non-existent fingerprints should not be found in map")
	}

	// 4. Cross-tenant isolation: querying under another tenant returns nothing
	otherTenant := createTestTenant(ctx, pool, t, "Other Tenant")
	otherResult, err := repo.FindExistingFingerprints(ctx, otherTenant.ID, queryFPs)
	if err != nil {
		t.Fatalf("failed to query fingerprints for other tenant: %v", err)
	}
	if len(otherResult) != 0 {
		t.Errorf("cross-tenant leakage: found %d fingerprints in other tenant", len(otherResult))
	}
}

func TestTransactionRepository_IdempotentUpsert_Single(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	ctx := context.Background()
	txManager := postgres.NewTxManager(pool)
	repo := postgres.NewTransactionRepository(pool)

	tenant := createTestTenant(ctx, pool, t, "Tenant Idempotent Single")
	user := createTestUser(ctx, pool, t, "user-single")

	var account *db.Account
	err := txManager.WithTenantTx(ctx, tenant.ID, user.ID, func(tx pgx.Tx) error {
		q := db.New(tx)
		var err error
		account, err = createTestAccount(ctx, q, tenant.ID, user.ID, "Conta Idempotente")
		return err
	})
	if err != nil {
		t.Fatalf("failed to create test account: %v", err)
	}

	domTx, err := domain.NewTransaction(domain.NewTransactionParams{
		TenantID:        tenant.ID,
		AccountID:       account.ID,
		CreatedByUserID: &user.ID,
		TransactedAt:    time.Date(2026, 10, 15, 14, 0, 0, 0, domain.BrazilLocation),
		Amount:          decimal.NewFromFloat(-120.00),
		Description:     "RESTAURANTE FOGO DE CHAO",
		Type:            domain.TransactionTypeExpense,
		Status:          domain.TransactionStatusCompleted,
		Visibility:      domain.VisibilityShared,
		Source:          domain.SourceTypeCSV,
		ExternalID:      "REST-FOGO-120",
		SequenceIndex:   0,
	})
	if err != nil {
		t.Fatalf("failed to construct transaction: %v", err)
	}

	// First insertion
	err = txManager.WithTenantTx(ctx, tenant.ID, user.ID, func(tx pgx.Tx) error {
		return repo.WithTx(tx).Create(ctx, domTx)
	})
	if err != nil {
		t.Fatalf("first transaction creation failed: %v", err)
	}

	// Second insertion of the EXACT same transaction (idempotent upsert via ON CONFLICT DO NOTHING)
	err = txManager.WithTenantTx(ctx, tenant.ID, user.ID, func(tx pgx.Tx) error {
		return repo.WithTx(tx).Create(ctx, domTx)
	})
	if err != nil {
		t.Fatalf("duplicate transaction creation returned unexpected error: %v", err)
	}

	// Verify table still contains exactly 1 transaction
	err = txManager.WithTenantTx(ctx, tenant.ID, user.ID, func(tx pgx.Tx) error {
		items, total, listErr := repo.WithTx(tx).List(ctx, tenant.ID, ports.TransactionFilter{})
		if listErr != nil {
			return listErr
		}
		if total != 1 {
			t.Errorf("expected total count 1, got %d", total)
		}
		if len(items) != 1 {
			t.Errorf("expected 1 item in list, got %d", len(items))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to list transactions: %v", err)
	}
}

func TestTransactionRepository_BatchCreate_PartialOverlapAndReimport(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	ctx := context.Background()
	txManager := postgres.NewTxManager(pool)
	repo := postgres.NewTransactionRepository(pool)

	tenant := createTestTenant(ctx, pool, t, "Tenant Batch Ingestion")
	user := createTestUser(ctx, pool, t, "user-batch")

	var account *db.Account
	err := txManager.WithTenantTx(ctx, tenant.ID, user.ID, func(tx pgx.Tx) error {
		q := db.New(tx)
		var err error
		account, err = createTestAccount(ctx, q, tenant.ID, user.ID, "Conta Extrato")
		return err
	})
	if err != nil {
		t.Fatalf("failed to create test account: %v", err)
	}

	// Prepare Statement Batch 1: tx1 and tx2
	tx1, err := domain.NewTransaction(domain.NewTransactionParams{
		TenantID:        tenant.ID,
		AccountID:       account.ID,
		CreatedByUserID: &user.ID,
		TransactedAt:    time.Date(2026, 10, 1, 10, 0, 0, 0, domain.BrazilLocation),
		Amount:          decimal.NewFromFloat(-35.50),
		Description:     "PADARIA CENTRAL",
		Type:            domain.TransactionTypeExpense,
		Status:          domain.TransactionStatusCompleted,
		Visibility:      domain.VisibilityShared,
		Source:          domain.SourceTypeOFX,
		ExternalID:      "OFX-TRN-001",
		SequenceIndex:   0,
	})
	if err != nil {
		t.Fatalf("failed to construct tx1: %v", err)
	}

	tx2, err := domain.NewTransaction(domain.NewTransactionParams{
		TenantID:        tenant.ID,
		AccountID:       account.ID,
		CreatedByUserID: &user.ID,
		TransactedAt:    time.Date(2026, 10, 2, 11, 30, 0, 0, domain.BrazilLocation),
		Amount:          decimal.NewFromFloat(-210.00),
		Description:     "FARMACIA RAIA",
		Type:            domain.TransactionTypeExpense,
		Status:          domain.TransactionStatusCompleted,
		Visibility:      domain.VisibilityShared,
		Source:          domain.SourceTypeOFX,
		ExternalID:      "OFX-TRN-002",
		SequenceIndex:   0,
	})
	if err != nil {
		t.Fatalf("failed to construct tx2: %v", err)
	}

	// 1. Initial statement upload: Batch 1 (tx1, tx2)
	err = txManager.WithTenantTx(ctx, tenant.ID, user.ID, func(tx pgx.Tx) error {
		return repo.WithTx(tx).BatchCreate(ctx, []*domain.Transaction{tx1, tx2})
	})
	if err != nil {
		t.Fatalf("batch 1 insert failed: %v", err)
	}

	// Verify count is 2
	err = txManager.WithTenantTx(ctx, tenant.ID, user.ID, func(tx pgx.Tx) error {
		_, total, listErr := repo.WithTx(tx).List(ctx, tenant.ID, ports.TransactionFilter{})
		if listErr != nil {
			return listErr
		}
		if total != 2 {
			t.Errorf("expected total 2 after batch 1, got %d", total)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed checking count: %v", err)
	}

	// 2. Re-importing exact same file/batch does NOT duplicate any rows
	err = txManager.WithTenantTx(ctx, tenant.ID, user.ID, func(tx pgx.Tx) error {
		return repo.WithTx(tx).BatchCreate(ctx, []*domain.Transaction{tx1, tx2})
	})
	if err != nil {
		t.Fatalf("re-import of batch 1 failed: %v", err)
	}

	err = txManager.WithTenantTx(ctx, tenant.ID, user.ID, func(tx pgx.Tx) error {
		_, total, listErr := repo.WithTx(tx).List(ctx, tenant.ID, ports.TransactionFilter{})
		if listErr != nil {
			return listErr
		}
		if total != 2 {
			t.Errorf("expected total still 2 after re-import, got %d", total)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed checking count after re-import: %v", err)
	}

	// 3. New statement upload with overlapping date range:
	// Batch 2 contains tx1 (duplicate), tx2 (duplicate), and tx3 (INÉDITA / NEW!)
	tx3, err := domain.NewTransaction(domain.NewTransactionParams{
		TenantID:        tenant.ID,
		AccountID:       account.ID,
		CreatedByUserID: &user.ID,
		TransactedAt:    time.Date(2026, 10, 3, 16, 45, 0, 0, domain.BrazilLocation),
		Amount:          decimal.NewFromFloat(-89.90),
		Description:     "POSTO IPIRANGA",
		Type:            domain.TransactionTypeExpense,
		Status:          domain.TransactionStatusCompleted,
		Visibility:      domain.VisibilityShared,
		Source:          domain.SourceTypeOFX,
		ExternalID:      "OFX-TRN-003",
		SequenceIndex:   0,
	})
	if err != nil {
		t.Fatalf("failed to construct tx3: %v", err)
	}

	// Pre-check via FindExistingFingerprints
	allFPs := []string{tx1.Fingerprint, tx2.Fingerprint, tx3.Fingerprint}
	existingMap, err := repo.FindExistingFingerprints(ctx, tenant.ID, allFPs)
	if err != nil {
		t.Fatalf("FindExistingFingerprints failed: %v", err)
	}

	if !existingMap[tx1.Fingerprint] {
		t.Errorf("tx1 should be identified as existing duplicate")
	}
	if !existingMap[tx2.Fingerprint] {
		t.Errorf("tx2 should be identified as existing duplicate")
	}
	if existingMap[tx3.Fingerprint] {
		t.Errorf("tx3 should be recognized as brand new (not existing)")
	}

	// Ingest Batch 2 (tx1, tx2, tx3)
	err = txManager.WithTenantTx(ctx, tenant.ID, user.ID, func(tx pgx.Tx) error {
		return repo.WithTx(tx).BatchCreate(ctx, []*domain.Transaction{tx1, tx2, tx3})
	})
	if err != nil {
		t.Fatalf("Batch 2 insert with partial new data failed: %v", err)
	}

	// Verify count is now 3 (tx1 and tx2 were skipped, only tx3 was inserted)
	err = txManager.WithTenantTx(ctx, tenant.ID, user.ID, func(tx pgx.Tx) error {
		items, total, listErr := repo.WithTx(tx).List(ctx, tenant.ID, ports.TransactionFilter{})
		if listErr != nil {
			return listErr
		}
		if total != 3 {
			t.Errorf("expected total 3 after batch 2, got %d", total)
		}
		if len(items) != 3 {
			t.Errorf("expected 3 items in list, got %d", len(items))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed checking count after batch 2: %v", err)
	}

	// Re-import Batch 2 once again -> count remains 3
	err = txManager.WithTenantTx(ctx, tenant.ID, user.ID, func(tx pgx.Tx) error {
		return repo.WithTx(tx).BatchCreate(ctx, []*domain.Transaction{tx1, tx2, tx3})
	})
	if err != nil {
		t.Fatalf("re-import of batch 2 failed: %v", err)
	}

	err = txManager.WithTenantTx(ctx, tenant.ID, user.ID, func(tx pgx.Tx) error {
		_, total, listErr := repo.WithTx(tx).List(ctx, tenant.ID, ports.TransactionFilter{})
		if listErr != nil {
			return listErr
		}
		if total != 3 {
			t.Errorf("expected total still 3 after re-import of batch 2, got %d", total)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed checking final count: %v", err)
	}
}

func TestTransactionRepository_TimezoneAndWhitespaceImmunityDeduplication(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	ctx := context.Background()
	txManager := postgres.NewTxManager(pool)
	repo := postgres.NewTransactionRepository(pool)

	tenant := createTestTenant(ctx, pool, t, "Tenant Timezone Dedup")
	user := createTestUser(ctx, pool, t, "user-tz")

	var account *db.Account
	err := txManager.WithTenantTx(ctx, tenant.ID, user.ID, func(tx pgx.Tx) error {
		q := db.New(tx)
		var err error
		account, err = createTestAccount(ctx, q, tenant.ID, user.ID, "Conta TZ")
		return err
	})
	if err != nil {
		t.Fatalf("failed to create test account: %v", err)
	}

	// Transaction 1: Generated in Cloud Run worker running in UTC
	// Time: 2026-10-15 01:30:00 UTC (which is 22:30:00 BRT on 2026-10-14)
	// Description has noisy spaces and banking prefixes
	txUTC, err := domain.NewTransaction(domain.NewTransactionParams{
		TenantID:        tenant.ID,
		AccountID:       account.ID,
		CreatedByUserID: &user.ID,
		TransactedAt:    time.Date(2026, 10, 15, 1, 30, 0, 0, time.UTC),
		Amount:          decimal.NewFromFloat(-150.00),
		Description:     "  COMPRA   CARTAO   -   PÃO   DE   AÇÚCAR   14/10  ",
		Type:            domain.TransactionTypeExpense,
		Status:          domain.TransactionStatusCompleted,
		Visibility:      domain.VisibilityShared,
		Source:          domain.SourceTypeCSV,
		ExternalID:      " FIT-ITAU-001 ",
		SequenceIndex:   0,
	})
	if err != nil {
		t.Fatalf("failed to construct txUTC: %v", err)
	}

	// Transaction 2: Generated in Homelab worker running in America/Sao_Paulo
	// Time: 2026-10-14 22:30:00 -03:00 (exact same instant in time)
	// Clean description with single spaces
	txBRT, err := domain.NewTransaction(domain.NewTransactionParams{
		TenantID:        tenant.ID,
		AccountID:       account.ID,
		CreatedByUserID: &user.ID,
		TransactedAt:    time.Date(2026, 10, 14, 22, 30, 0, 0, domain.BrazilLocation),
		Amount:          decimal.NewFromFloat(-150.00),
		Description:     "PÃO DE AÇÚCAR",
		Type:            domain.TransactionTypeExpense,
		Status:          domain.TransactionStatusCompleted,
		Visibility:      domain.VisibilityShared,
		Source:          domain.SourceTypeCSV,
		ExternalID:      "FIT-ITAU-001",
		SequenceIndex:   0,
	})
	if err != nil {
		t.Fatalf("failed to construct txBRT: %v", err)
	}

	// Verify both transactions yielded identical fingerprints despite timezone and whitespace differences
	if txUTC.Fingerprint != txBRT.Fingerprint {
		t.Fatalf("fingerprint mismatch: txUTC (%s) != txBRT (%s)", txUTC.Fingerprint, txBRT.Fingerprint)
	}

	// Insert txUTC first
	err = txManager.WithTenantTx(ctx, tenant.ID, user.ID, func(tx pgx.Tx) error {
		return repo.WithTx(tx).Create(ctx, txUTC)
	})
	if err != nil {
		t.Fatalf("failed to insert txUTC: %v", err)
	}

	// Check FindExistingFingerprints with txBRT's fingerprint -> must find it!
	foundMap, err := repo.FindExistingFingerprints(ctx, tenant.ID, []string{txBRT.Fingerprint})
	if err != nil {
		t.Fatalf("FindExistingFingerprints failed: %v", err)
	}
	if !foundMap[txBRT.Fingerprint] {
		t.Errorf("expected txBRT fingerprint to be detected as existing duplicate")
	}

	// Attempt to insert txBRT via repo.Create -> idempotent upsert DO NOTHING succeeds
	err = txManager.WithTenantTx(ctx, tenant.ID, user.ID, func(tx pgx.Tx) error {
		return repo.WithTx(tx).Create(ctx, txBRT)
	})
	if err != nil {
		t.Fatalf("idempotent insert of txBRT returned error: %v", err)
	}

	// Verify ledger contains exactly 1 row
	err = txManager.WithTenantTx(ctx, tenant.ID, user.ID, func(tx pgx.Tx) error {
		_, total, listErr := repo.WithTx(tx).List(ctx, tenant.ID, ports.TransactionFilter{})
		if listErr != nil {
			return listErr
		}
		if total != 1 {
			t.Errorf("expected total count 1, got %d", total)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed checking count: %v", err)
	}
}
