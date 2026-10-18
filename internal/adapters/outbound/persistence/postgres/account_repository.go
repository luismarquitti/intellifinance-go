package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/persistence/postgres/db"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
	"github.com/shopspring/decimal"
)

var _ ports.AccountRepository = (*AccountRepository)(nil)

// AccountRepository implements ports.AccountRepository using PostgreSQL via sqlc.
type AccountRepository struct {
	pool *pgxpool.Pool
	tx   pgx.Tx
}

// NewAccountRepository creates a new AccountRepository instance.
func NewAccountRepository(pool *pgxpool.Pool) *AccountRepository {
	return &AccountRepository{pool: pool}
}

// WithTx returns a new AccountRepository bound to a transaction.
func (r *AccountRepository) WithTx(tx pgx.Tx) *AccountRepository {
	return &AccountRepository{
		pool: r.pool,
		tx:   tx,
	}
}

func (r *AccountRepository) getQueries(ctx context.Context) *db.Queries {
	if r.tx != nil {
		return db.New(r.tx)
	}
	if tx, ok := TxFromContext(ctx); ok {
		return db.New(tx)
	}
	return db.New(r.pool)
}

// Create persists a new Account.
func (r *AccountRepository) Create(ctx context.Context, account *domain.Account) error {
	if err := account.Validate(); err != nil {
		return fmt.Errorf("invalid account: %w", err)
	}

	if r.tx == nil {
		if _, ok := TxFromContext(ctx); !ok && r.pool != nil {
			return WithTenantTx(ctx, r.pool, account.TenantID, account.OwnerUserID, func(tx pgx.Tx) error {
				return r.WithTx(tx).Create(ctx, account)
			})
		}
	}

	q := r.getQueries(ctx)

	var instID, instName, accNum *string
	if account.InstitutionID != "" {
		instID = &account.InstitutionID
	}
	if account.InstitutionName != "" {
		instName = &account.InstitutionName
	}
	if account.AccountNumberMasked != "" {
		accNum = &account.AccountNumberMasked
	}

	createdAt := pgtype.Timestamptz{Time: account.CreatedAt, Valid: !account.CreatedAt.IsZero()}
	updatedAt := pgtype.Timestamptz{Time: account.UpdatedAt, Valid: !account.UpdatedAt.IsZero()}

	_, err := q.CreateAccount(ctx, db.CreateAccountParams{
		ID:                  account.ID,
		TenantID:            account.TenantID,
		OwnerUserID:         account.OwnerUserID,
		Name:                account.Name,
		Type:                db.AccountType(account.Type),
		Currency:            account.Currency,
		Balance:             account.Balance,
		Visibility:          db.VisibilityType(account.Visibility),
		InstitutionID:       instID,
		InstitutionName:     instName,
		AccountNumberMasked: accNum,
		ColorHex:            account.ColorHex,
		IconName:            account.IconName,
		IsActive:            account.IsActive,
		CreatedAt:           createdAt,
		UpdatedAt:           updatedAt,
	})
	if err != nil {
		return fmt.Errorf("failed to create account: %w", err)
	}
	return nil
}

// GetByID retrieves an account by ID within a tenant boundary.
func (r *AccountRepository) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Account, error) {
	if r.tx == nil {
		if _, ok := TxFromContext(ctx); !ok && r.pool != nil {
			var acc *domain.Account
			err := WithTenantTx(ctx, r.pool, tenantID, uuid.Nil, func(tx pgx.Tx) error {
				var err error
				acc, err = r.WithTx(tx).GetByID(ctx, tenantID, id)
				return err
			})
			return acc, err
		}
	}

	q := r.getQueries(ctx)
	row, err := q.GetAccountByID(ctx, db.GetAccountByIDParams{
		ID:       id,
		TenantID: tenantID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get account by id: %w", err)
	}

	return dbToDomainAccount(&row), nil
}

// List queries accounts matching filter criteria within a tenant boundary.
func (r *AccountRepository) List(ctx context.Context, tenantID uuid.UUID, filter ports.AccountFilter) ([]*domain.Account, error) {
	if r.tx == nil {
		if _, ok := TxFromContext(ctx); !ok && r.pool != nil {
			var accs []*domain.Account
			err := WithTenantTx(ctx, r.pool, tenantID, uuid.Nil, func(tx pgx.Tx) error {
				var err error
				accs, err = r.WithTx(tx).List(ctx, tenantID, filter)
				return err
			})
			return accs, err
		}
	}

	q := r.getQueries(ctx)

	var accType db.NullAccountType
	if filter.Type != nil {
		accType = db.NullAccountType{AccountType: db.AccountType(*filter.Type), Valid: true}
	}

	var visType db.NullVisibilityType
	if filter.Visibility != nil {
		visType = db.NullVisibilityType{VisibilityType: db.VisibilityType(*filter.Visibility), Valid: true}
	}

	rows, err := q.ListAccountsFiltered(ctx, db.ListAccountsFilteredParams{
		TenantID:    tenantID,
		OwnerUserID: filter.OwnerUserID,
		Type:        accType,
		Visibility:  visType,
		IsActive:    filter.IsActive,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list accounts: %w", err)
	}

	items := make([]*domain.Account, len(rows))
	for i := range rows {
		items[i] = dbToDomainAccount(&rows[i])
	}
	return items, nil
}

// Update persists mutations to an existing account.
func (r *AccountRepository) Update(ctx context.Context, account *domain.Account) error {
	if err := account.Validate(); err != nil {
		return fmt.Errorf("invalid account: %w", err)
	}

	if r.tx == nil {
		if _, ok := TxFromContext(ctx); !ok && r.pool != nil {
			return WithTenantTx(ctx, r.pool, account.TenantID, account.OwnerUserID, func(tx pgx.Tx) error {
				return r.WithTx(tx).Update(ctx, account)
			})
		}
	}

	q := r.getQueries(ctx)

	var instID, instName, accNum *string
	if account.InstitutionID != "" {
		instID = &account.InstitutionID
	}
	if account.InstitutionName != "" {
		instName = &account.InstitutionName
	}
	if account.AccountNumberMasked != "" {
		accNum = &account.AccountNumberMasked
	}

	_, err := q.UpdateAccount(ctx, db.UpdateAccountParams{
		ID:                  account.ID,
		TenantID:            account.TenantID,
		Name:                account.Name,
		Type:                db.AccountType(account.Type),
		Visibility:          db.VisibilityType(account.Visibility),
		InstitutionID:       instID,
		InstitutionName:     instName,
		AccountNumberMasked: accNum,
		ColorHex:            account.ColorHex,
		IconName:            account.IconName,
		IsActive:            account.IsActive,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("failed to update account: %w", err)
	}
	return nil
}

// Delete soft-deletes or removes an account by ID.
func (r *AccountRepository) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	if r.tx == nil {
		if _, ok := TxFromContext(ctx); !ok && r.pool != nil {
			return WithTenantTx(ctx, r.pool, tenantID, uuid.Nil, func(tx pgx.Tx) error {
				return r.WithTx(tx).Delete(ctx, tenantID, id)
			})
		}
	}

	q := r.getQueries(ctx)
	if err := q.SoftDeleteAccount(ctx, db.SoftDeleteAccountParams{
		ID:       id,
		TenantID: tenantID,
	}); err != nil {
		return fmt.Errorf("failed to soft delete account: %w", err)
	}
	return nil
}

// UpdateBalance atomically adjusts the account balance by delta.
func (r *AccountRepository) UpdateBalance(ctx context.Context, tenantID, id uuid.UUID, delta decimal.Decimal) error {
	if r.tx == nil {
		if _, ok := TxFromContext(ctx); !ok && r.pool != nil {
			return WithTenantTx(ctx, r.pool, tenantID, uuid.Nil, func(tx pgx.Tx) error {
				return r.WithTx(tx).UpdateBalance(ctx, tenantID, id, delta)
			})
		}
	}

	q := r.getQueries(ctx)
	_, err := q.UpdateAccountBalance(ctx, db.UpdateAccountBalanceParams{
		ID:       id,
		TenantID: tenantID,
		Balance:  delta,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("failed to update account balance: %w", err)
	}
	return nil
}

func dbToDomainAccount(row *db.Account) *domain.Account {
	var instID, instName, accNum string
	if row.InstitutionID != nil {
		instID = *row.InstitutionID
	}
	if row.InstitutionName != nil {
		instName = *row.InstitutionName
	}
	if row.AccountNumberMasked != nil {
		accNum = *row.AccountNumberMasked
	}

	var deletedAt *time.Time
	if row.DeletedAt.Valid {
		t := row.DeletedAt.Time
		deletedAt = &t
	}

	return &domain.Account{
		ID:                  row.ID,
		TenantID:            row.TenantID,
		OwnerUserID:         row.OwnerUserID,
		Name:                row.Name,
		Type:                domain.AccountType(row.Type),
		Currency:            row.Currency,
		Balance:             row.Balance,
		Visibility:          domain.Visibility(row.Visibility),
		InstitutionID:       instID,
		InstitutionName:     instName,
		AccountNumberMasked: accNum,
		ColorHex:            row.ColorHex,
		IconName:            row.IconName,
		IsActive:            row.IsActive,
		DeletedAt:           deletedAt,
		CreatedAt:           row.CreatedAt.Time,
		UpdatedAt:           row.UpdatedAt.Time,
	}
}
