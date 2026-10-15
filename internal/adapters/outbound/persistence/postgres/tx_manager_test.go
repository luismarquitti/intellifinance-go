package postgres_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/persistence/postgres"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/persistence/postgres/db"
)

func getTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = os.Getenv("TEST_DATABASE_URL")
	}
	if dbURL == "" {
		// Use dedicated non-superuser intellifinance_app role to ensure RLS is enforced
		dbURL = "postgres://intellifinance_app:CHANGE_IN_PRODUCTION@localhost:5432/intellifinance?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	pool, err := postgres.NewConnectionPool(ctx, dbURL)
	if err != nil {
		t.Skipf("Skipping integration test: PostgreSQL not reachable at %s: %v", dbURL, err)
		return nil
	}

	return pool
}

func createTestUser(ctx context.Context, pool *pgxpool.Pool, t *testing.T, emailPrefix string) *db.User {
	t.Helper()
	queries := db.New(pool)
	u, err := queries.CreateUser(ctx, db.CreateUserParams{
		ID:           uuid.New(),
		Email:        emailPrefix + "-" + uuid.NewString()[:8] + "@example.com",
		PasswordHash: "$2a$12$e80yq9D32aUfK9mP1b...",
		FullName:     "Test User " + emailPrefix,
		IsActive:     true,
		CreatedAt:    pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		UpdatedAt:    pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	return &u
}

func createTestTenant(ctx context.Context, pool *pgxpool.Pool, t *testing.T, name string) *db.Tenant {
	t.Helper()
	queries := db.New(pool)
	tenant, err := queries.CreateTenant(ctx, db.CreateTenantParams{
		ID:        uuid.New(),
		Name:      name + " " + uuid.NewString()[:8],
		Type:      db.TenantTypeHOUSEHOLD,
		Currency:  "BRL",
		Settings:  json.RawMessage(`{}`),
		CreatedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	if err != nil {
		t.Fatalf("failed to create test tenant: %v", err)
	}
	return &tenant
}

func TestWithTenantTx_CrossTenantIsolation(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	ctx := context.Background()
	txManager := postgres.NewTxManager(pool)

	// 1. Create two separate tenants and users
	tenantA := createTestTenant(ctx, pool, t, "Tenant A")
	userA := createTestUser(ctx, pool, t, "user-a")

	tenantB := createTestTenant(ctx, pool, t, "Tenant B")
	userB := createTestUser(ctx, pool, t, "user-b")

	// 2. Insert Category, Account, Transaction, Split and IngestionJob inside Tenant A
	var (
		accountA     db.Account
		transactionA db.Transaction
		categoryA    db.Category
		jobA         db.IngestionJob
	)

	err := txManager.WithTenantQueries(ctx, tenantA.ID, userA.ID, func(q *db.Queries) error {
		var err error
		categoryA, err = q.CreateCategory(ctx, db.CreateCategoryParams{
			ID:        uuid.New(),
			TenantID:  tenantA.ID,
			Name:      "Alimentação " + uuid.NewString()[:8],
			Type:      db.TransactionTypeEXPENSE,
			ColorHex:  "#10B981",
			IconName:  "shopping-cart",
			IsSystem:  false,
			CreatedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
			UpdatedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		})
		if err != nil {
			return err
		}

		accountA, err = q.CreateAccount(ctx, db.CreateAccountParams{
			ID:          uuid.New(),
			TenantID:    tenantA.ID,
			OwnerUserID: userA.ID,
			Name:        "Conta Corrente A " + uuid.NewString()[:8],
			Type:        db.AccountTypeCHECKING,
			Currency:    "BRL",
			Balance:     decimal.NewFromFloat(1500.50),
			Visibility:  db.VisibilityTypeSHARED,
			ColorHex:    "#3B82F6",
			IconName:    "bank",
			IsActive:    true,
			CreatedAt:   pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
			UpdatedAt:   pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		})
		if err != nil {
			return err
		}

		dateOnly := pgtype.Date{Time: time.Now().UTC(), Valid: true}
		transactionA, err = q.CreateTransaction(ctx, db.CreateTransactionParams{
			ID:               uuid.New(),
			TenantID:         tenantA.ID,
			AccountID:        accountA.ID,
			CategoryID:       &categoryA.ID,
			CreatedByUserID:  &userA.ID,
			TransactedAt:     pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
			Date:             dateOnly,
			Amount:           decimal.NewFromFloat(-120.75),
			Description:      "COMPRA SUPERMERCADO A",
			CleanDescription: "SUPERMERCADO A",
			Type:             db.TransactionTypeEXPENSE,
			Status:           db.TransactionStatusCOMPLETED,
			Visibility:       db.VisibilityTypeSHARED,
			Source:           db.SourceTypeMANUAL,
			SequenceIndex:    0,
			Fingerprint:      func() string { h := sha256.Sum256([]byte(uuid.NewString())); return hex.EncodeToString(h[:]) }(),
			IsAiCategorized:  false,
			Metadata:         json.RawMessage(`{}`),
			CreatedAt:        pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
			UpdatedAt:        pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		})
		if err != nil {
			return err
		}

		splitPercentage, _ := decimal.NewFromString("100.00")
		_, err = q.CreateTransactionSplit(ctx, db.CreateTransactionSplitParams{
			ID:            uuid.New(),
			TenantID:      tenantA.ID,
			TransactionID: transactionA.ID,
			UserID:        userA.ID,
			Amount:        decimal.NewFromFloat(120.75),
			Percentage:    splitPercentage,
			CreatedAt:     pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
			UpdatedAt:     pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		})
		if err != nil {
			return err
		}

		jobA, err = q.CreateIngestionJob(ctx, db.CreateIngestionJobParams{
			ID:                uuid.New(),
			TenantID:          tenantA.ID,
			AccountID:         accountA.ID,
			InitiatedByUserID: userA.ID,
			Source:            db.SourceTypeOFX,
			Status:            db.JobStatusPENDING,
			SourceFilename:    "extrato.ofx",
			FileStoragePath:   "/storage/extrato.ofx",
			FileMimeType:      "application/x-ofx",
			FileSizeBytes:     1024,
			TotalRecords:      10,
			InsertedRecords:   0,
			DuplicateRecords:  0,
			FailedRecords:     0,
			CreatedAt:         pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		})
		return err
	})
	if err != nil {
		t.Fatalf("failed to insert Tenant A entities: %v", err)
	}

	// 3. Query from Tenant B context: must return NOTHING from Tenant A
	err = txManager.WithTenantQueries(ctx, tenantB.ID, userB.ID, func(q *db.Queries) error {
		// Accounts list must not contain Account A
		accounts, listAccErr := q.ListAccounts(ctx, tenantB.ID)
		if listAccErr != nil {
			return listAccErr
		}
		if len(accounts) != 0 {
			t.Errorf("expected 0 accounts for Tenant B, got %d", len(accounts))
		}

		// Direct lookup for Account A under Tenant B must fail with ErrNoRows
		_, getAccErr := q.GetAccountByID(ctx, db.GetAccountByIDParams{
			ID:       accountA.ID,
			TenantID: tenantB.ID,
		})
		if !errors.Is(getAccErr, pgx.ErrNoRows) {
			t.Errorf("expected pgx.ErrNoRows when accessing Account A from Tenant B, got: %v", getAccErr)
		}

		// Transactions list must not contain Transaction A
		txs, listTxErr := q.ListTransactions(ctx, db.ListTransactionsParams{
			TenantID: tenantB.ID,
			Limit:    10,
			Offset:   0,
		})
		if listTxErr != nil {
			return listTxErr
		}
		if len(txs) != 0 {
			t.Errorf("expected 0 transactions for Tenant B, got %d", len(txs))
		}

		// Direct lookup for Transaction A must fail
		_, getTxErr := q.GetTransactionByID(ctx, db.GetTransactionByIDParams{
			ID:       transactionA.ID,
			TenantID: tenantB.ID,
		})
		if !errors.Is(getTxErr, pgx.ErrNoRows) {
			t.Errorf("expected pgx.ErrNoRows when accessing Transaction A from Tenant B, got: %v", getTxErr)
		}

		// Categories list must not contain Category A
		categories, listCatErr := q.ListCategories(ctx, tenantB.ID)
		if listCatErr != nil {
			return listCatErr
		}
		if len(categories) != 0 {
			t.Errorf("expected 0 categories for Tenant B, got %d", len(categories))
		}

		// Ingestion jobs list must not contain Job A
		jobs, listJobErr := q.ListIngestionJobs(ctx, tenantB.ID)
		if listJobErr != nil {
			return listJobErr
		}
		if len(jobs) != 0 {
			t.Errorf("expected 0 ingestion jobs for Tenant B, got %d", len(jobs))
		}

		return nil
	})
	if err != nil {
		t.Fatalf("Tenant B query transaction failed: %v", err)
	}

	// 4. Test database engine-level RLS defense:
	// Even if a query omits the WHERE tenant_id filter inside Tenant B's transaction,
	// PostgreSQL RLS policies must return 0 rows.
	err = txManager.WithTenantTx(ctx, tenantB.ID, userB.ID, func(tx pgx.Tx) error {
		var accountCount int
		if scanErr := tx.QueryRow(ctx, "SELECT count(*) FROM accounts WHERE id = $1", accountA.ID).Scan(&accountCount); scanErr != nil {
			return scanErr
		}
		if accountCount != 0 {
			t.Errorf("CRITICAL RLS LEAK: Tenant B can see Account A via unfiltered query! count=%d", accountCount)
		}

		var txCount int
		if scanErr := tx.QueryRow(ctx, "SELECT count(*) FROM transactions WHERE id = $1", transactionA.ID).Scan(&txCount); scanErr != nil {
			return scanErr
		}
		if txCount != 0 {
			t.Errorf("CRITICAL RLS LEAK: Tenant B can see Transaction A via unfiltered query! count=%d", txCount)
		}

		var jobCount int
		if scanErr := tx.QueryRow(ctx, "SELECT count(*) FROM ingestion_jobs WHERE id = $1", jobA.ID).Scan(&jobCount); scanErr != nil {
			return scanErr
		}
		if jobCount != 0 {
			t.Errorf("CRITICAL RLS LEAK: Tenant B can see IngestionJob A via unfiltered query! count=%d", jobCount)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("Tenant B raw RLS test failed: %v", err)
	}
}

func TestWithTenantTx_OutsideTransaction_ZeroVisibilityAndInsertBlocked(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	ctx := context.Background()
	txManager := postgres.NewTxManager(pool)

	tenantA := createTestTenant(ctx, pool, t, "Tenant Outside")
	userA := createTestUser(ctx, pool, t, "user-outside")

	var accountA db.Account
	err := txManager.WithTenantQueries(ctx, tenantA.ID, userA.ID, func(q *db.Queries) error {
		var err error
		accountA, err = q.CreateAccount(ctx, db.CreateAccountParams{
			ID:          uuid.New(),
			TenantID:    tenantA.ID,
			OwnerUserID: userA.ID,
			Name:        "Conta RLS Test " + uuid.NewString()[:8],
			Type:        db.AccountTypeCHECKING,
			Currency:    "BRL",
			Balance:     decimal.NewFromFloat(500.00),
			Visibility:  db.VisibilityTypeSHARED,
			ColorHex:    "#3B82F6",
			IconName:    "bank",
			IsActive:    true,
			CreatedAt:   pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
			UpdatedAt:   pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		})
		return err
	})
	if err != nil {
		t.Fatalf("failed to create account in WithTenantTx: %v", err)
	}

	// 1. Querying accounts directly outside WithTenantTx must return 0 rows
	var count int
	err = pool.QueryRow(ctx, "SELECT count(*) FROM accounts WHERE id = $1", accountA.ID).Scan(&count)
	if err != nil {
		t.Fatalf("query outside WithTenantTx failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 accounts visible outside WithTenantTx, got %d", count)
	}

	// 2. Direct INSERT outside WithTenantTx must be rejected by PostgreSQL RLS policy
	insertSQL := `
		INSERT INTO accounts (id, tenant_id, owner_user_id, name, type, currency, balance, visibility, color_hex, icon_name, is_active)
		VALUES ($1, $2, $3, $4, 'CHECKING', 'BRL', 100.00, 'SHARED', '#000000', 'bank', true)
	`
	_, err = pool.Exec(ctx, insertSQL, uuid.New(), tenantA.ID, userA.ID, "Conta Proibida")
	if err == nil {
		t.Errorf("expected INSERT outside WithTenantTx to fail due to RLS policy, but it succeeded")
	}
}

func TestWithTenantTx_PrivateAccountVisibilityWithinSameTenant(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	ctx := context.Background()
	txManager := postgres.NewTxManager(pool)

	tenant := createTestTenant(ctx, pool, t, "Household Shared")
	userOwner := createTestUser(ctx, pool, t, "owner-user")
	userPartner := createTestUser(ctx, pool, t, "partner-user")

	var privateAccount db.Account
	var sharedAccount db.Account

	// Create one PRIVATE account and one SHARED account owned by userOwner
	err := txManager.WithTenantQueries(ctx, tenant.ID, userOwner.ID, func(q *db.Queries) error {
		var err error
		privateAccount, err = q.CreateAccount(ctx, db.CreateAccountParams{
			ID:          uuid.New(),
			TenantID:    tenant.ID,
			OwnerUserID: userOwner.ID,
			Name:        "Conta Pessoal Privada " + uuid.NewString()[:8],
			Type:        db.AccountTypeINVESTMENT,
			Currency:    "BRL",
			Balance:     decimal.NewFromFloat(50000.00),
			Visibility:  db.VisibilityTypePRIVATE,
			ColorHex:    "#E11D48",
			IconName:    "lock",
			IsActive:    true,
			CreatedAt:   pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
			UpdatedAt:   pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		})
		if err != nil {
			return err
		}

		sharedAccount, err = q.CreateAccount(ctx, db.CreateAccountParams{
			ID:          uuid.New(),
			TenantID:    tenant.ID,
			OwnerUserID: userOwner.ID,
			Name:        "Conta Conjunta Casa " + uuid.NewString()[:8],
			Type:        db.AccountTypeCHECKING,
			Currency:    "BRL",
			Balance:     decimal.NewFromFloat(8000.00),
			Visibility:  db.VisibilityTypeSHARED,
			ColorHex:    "#2563EB",
			IconName:    "home",
			IsActive:    true,
			CreatedAt:   pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
			UpdatedAt:   pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		})
		return err
	})
	if err != nil {
		t.Fatalf("failed to create accounts: %v", err)
	}

	// When partner queries, they should see SHARED account but NOT PRIVATE account
	err = txManager.WithTenantQueries(ctx, tenant.ID, userPartner.ID, func(q *db.Queries) error {
		// Partner sees shared account
		if _, getErr := q.GetAccountByID(ctx, db.GetAccountByIDParams{
			ID:       sharedAccount.ID,
			TenantID: tenant.ID,
		}); getErr != nil {
			t.Errorf("expected partner to see SHARED account, got: %v", getErr)
		}

		// Partner CANNOT see private account
		_, privErr := q.GetAccountByID(ctx, db.GetAccountByIDParams{
			ID:       privateAccount.ID,
			TenantID: tenant.ID,
		})
		if !errors.Is(privErr, pgx.ErrNoRows) {
			t.Errorf("expected pgx.ErrNoRows when partner queries private account, got: %v", privErr)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("partner query transaction failed: %v", err)
	}

	// When owner queries, they should see BOTH accounts
	err = txManager.WithTenantQueries(ctx, tenant.ID, userOwner.ID, func(q *db.Queries) error {
		if _, getErr := q.GetAccountByID(ctx, db.GetAccountByIDParams{
			ID:       privateAccount.ID,
			TenantID: tenant.ID,
		}); getErr != nil {
			t.Errorf("expected owner to see their own PRIVATE account, got: %v", getErr)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("owner query transaction failed: %v", err)
	}
}

func TestWithTenantTx_ConnectionPoolHygiene(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	ctx := context.Background()
	txManager := postgres.NewTxManager(pool)

	tenant := createTestTenant(ctx, pool, t, "Hygiene Tenant")
	user := createTestUser(ctx, pool, t, "hygiene-user")

	// Execute WithTenantTx
	err := txManager.WithTenantTx(ctx, tenant.ID, user.ID, func(tx pgx.Tx) error {
		var currentTenant string
		if err := tx.QueryRow(ctx, "SELECT current_setting('app.current_tenant_id', true)").Scan(&currentTenant); err != nil {
			return err
		}
		if currentTenant != tenant.ID.String() {
			t.Errorf("expected currentTenant to be %s, got %s", tenant.ID.String(), currentTenant)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WithTenantTx failed: %v", err)
	}

	// Immediately acquire a connection outside WithTenantTx
	var remainingTenant string
	err = pool.QueryRow(ctx, "SELECT current_setting('app.current_tenant_id', true)").Scan(&remainingTenant)
	if err != nil {
		t.Fatalf("querying current_setting outside transaction failed: %v", err)
	}
	if remainingTenant != "" {
		t.Errorf("CONNECTION POOL CONTAMINATED: app.current_tenant_id was not cleared! got: %q", remainingTenant)
	}
}
