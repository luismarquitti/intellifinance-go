package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/persistence/postgres/db"
	"github.com/luismarquitti/intellifinance-go/internal/domain"
	"github.com/luismarquitti/intellifinance-go/internal/ports"
)

var _ ports.CategoryRepository = (*CategoryRepository)(nil)

// CategoryRepository implements ports.CategoryRepository using PostgreSQL via sqlc.
type CategoryRepository struct {
	pool *pgxpool.Pool
	tx   pgx.Tx
}

// NewCategoryRepository creates a new CategoryRepository instance.
func NewCategoryRepository(pool *pgxpool.Pool) *CategoryRepository {
	return &CategoryRepository{pool: pool}
}

// WithTx returns a new CategoryRepository bound to a transaction.
func (r *CategoryRepository) WithTx(tx pgx.Tx) *CategoryRepository {
	return &CategoryRepository{
		pool: r.pool,
		tx:   tx,
	}
}

func (r *CategoryRepository) getQueries(ctx context.Context) *db.Queries {
	if r.tx != nil {
		return db.New(r.tx)
	}
	if tx, ok := TxFromContext(ctx); ok {
		return db.New(tx)
	}
	return db.New(r.pool)
}

// Create persists a new budget category.
func (r *CategoryRepository) Create(ctx context.Context, category *domain.Category) error {
	if err := category.Validate(); err != nil {
		return fmt.Errorf("invalid category: %w", err)
	}

	if r.tx == nil {
		if _, ok := TxFromContext(ctx); !ok && r.pool != nil {
			return WithTenantTx(ctx, r.pool, category.TenantID, uuid.Nil, func(tx pgx.Tx) error {
				return r.WithTx(tx).Create(ctx, category)
			})
		}
	}

	q := r.getQueries(ctx)

	createdAt := pgtype.Timestamptz{Time: category.CreatedAt, Valid: !category.CreatedAt.IsZero()}
	updatedAt := pgtype.Timestamptz{Time: category.UpdatedAt, Valid: !category.UpdatedAt.IsZero()}

	_, err := q.CreateCategory(ctx, db.CreateCategoryParams{
		ID:        category.ID,
		TenantID:  category.TenantID,
		ParentID:  category.ParentID,
		Name:      category.Name,
		Type:      db.TransactionType(category.Type),
		ColorHex:  category.ColorHex,
		IconName:  category.IconName,
		IsSystem:  category.IsSystem,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	})
	if err != nil {
		return fmt.Errorf("failed to create category: %w", err)
	}
	return nil
}

// GetByID retrieves a category by ID within a tenant boundary.
func (r *CategoryRepository) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Category, error) {
	if r.tx == nil {
		if _, ok := TxFromContext(ctx); !ok && r.pool != nil {
			var cat *domain.Category
			err := WithTenantTx(ctx, r.pool, tenantID, uuid.Nil, func(tx pgx.Tx) error {
				var err error
				cat, err = r.WithTx(tx).GetByID(ctx, tenantID, id)
				return err
			})
			return cat, err
		}
	}

	q := r.getQueries(ctx)
	row, err := q.GetCategoryByID(ctx, db.GetCategoryByIDParams{
		ID:       id,
		TenantID: tenantID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get category by id: %w", err)
	}

	return dbToDomainCategory(&row), nil
}

// List retrieves categories matching filter criteria.
func (r *CategoryRepository) List(ctx context.Context, tenantID uuid.UUID, filter ports.CategoryFilter) ([]*domain.Category, error) {
	if r.tx == nil {
		if _, ok := TxFromContext(ctx); !ok && r.pool != nil {
			var cats []*domain.Category
			err := WithTenantTx(ctx, r.pool, tenantID, uuid.Nil, func(tx pgx.Tx) error {
				var err error
				cats, err = r.WithTx(tx).List(ctx, tenantID, filter)
				return err
			})
			return cats, err
		}
	}

	q := r.getQueries(ctx)

	var catType db.NullTransactionType
	if filter.Type != nil {
		catType = db.NullTransactionType{TransactionType: db.TransactionType(*filter.Type), Valid: true}
	}

	rows, err := q.ListCategoriesFiltered(ctx, db.ListCategoriesFilteredParams{
		TenantID:      tenantID,
		Type:          catType,
		ParentID:      filter.ParentID,
		IncludeSystem: filter.IncludeSystem,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list categories: %w", err)
	}

	items := make([]*domain.Category, len(rows))
	for i := range rows {
		items[i] = dbToDomainCategory(&rows[i])
	}
	return items, nil
}

// Update persists mutations to an existing category.
func (r *CategoryRepository) Update(ctx context.Context, category *domain.Category) error {
	if err := category.Validate(); err != nil {
		return fmt.Errorf("invalid category: %w", err)
	}

	if r.tx == nil {
		if _, ok := TxFromContext(ctx); !ok && r.pool != nil {
			return WithTenantTx(ctx, r.pool, category.TenantID, uuid.Nil, func(tx pgx.Tx) error {
				return r.WithTx(tx).Update(ctx, category)
			})
		}
	}

	q := r.getQueries(ctx)
	_, err := q.UpdateCategory(ctx, db.UpdateCategoryParams{
		ID:       category.ID,
		TenantID: category.TenantID,
		ParentID: category.ParentID,
		Name:     category.Name,
		Type:     db.TransactionType(category.Type),
		ColorHex: category.ColorHex,
		IconName: category.IconName,
		IsSystem: category.IsSystem,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("failed to update category: %w", err)
	}
	return nil
}

// Delete removes a category by ID.
func (r *CategoryRepository) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	if r.tx == nil {
		if _, ok := TxFromContext(ctx); !ok && r.pool != nil {
			return WithTenantTx(ctx, r.pool, tenantID, uuid.Nil, func(tx pgx.Tx) error {
				return r.WithTx(tx).Delete(ctx, tenantID, id)
			})
		}
	}

	q := r.getQueries(ctx)
	if err := q.DeleteCategory(ctx, db.DeleteCategoryParams{
		ID:       id,
		TenantID: tenantID,
	}); err != nil {
		return fmt.Errorf("failed to delete category: %w", err)
	}
	return nil
}

// CountTransactionsByCategory checks whether existing transactions reference the category.
func (r *CategoryRepository) CountTransactionsByCategory(ctx context.Context, tenantID, categoryID uuid.UUID) (int64, error) {
	if r.tx == nil {
		if _, ok := TxFromContext(ctx); !ok && r.pool != nil {
			var count int64
			err := WithTenantTx(ctx, r.pool, tenantID, uuid.Nil, func(tx pgx.Tx) error {
				var err error
				count, err = r.WithTx(tx).CountTransactionsByCategory(ctx, tenantID, categoryID)
				return err
			})
			return count, err
		}
	}

	q := r.getQueries(ctx)
	count, err := q.CountTransactionsByCategory(ctx, db.CountTransactionsByCategoryParams{
		CategoryID: &categoryID,
		TenantID:   tenantID,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to count transactions by category: %w", err)
	}
	return count, nil
}

func dbToDomainCategory(row *db.Category) *domain.Category {
	return &domain.Category{
		ID:        row.ID,
		TenantID:  row.TenantID,
		ParentID:  row.ParentID,
		Name:      row.Name,
		Type:      domain.TransactionType(row.Type),
		ColorHex:  row.ColorHex,
		IconName:  row.IconName,
		IsSystem:  row.IsSystem,
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}
}
