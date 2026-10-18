package bootstrap

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/luismarquitti/intellifinance-go/internal/adapters/outbound/persistence/postgres"
	"github.com/luismarquitti/intellifinance-go/internal/config"
)

// SeedDevAdmin guarantees that a valid admin user, tenant, system categories,
// and default financial accounts exist for local homelab development.
func SeedDevAdmin(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, logger *slog.Logger) error {
	logger.InfoContext(ctx, "seeding dev admin tenant, user, categories and accounts...",
		"tenant_id", cfg.DevTenantID,
		"user_id", cfg.DevUserID,
	)

	// 1. Upsert Tenant
	const insertTenantSQL = `
		INSERT INTO tenants (id, name, type, currency)
		VALUES ($1, $2, 'HOUSEHOLD', 'BRL')
		ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, updated_at = NOW();
	`
	if _, err := pool.Exec(ctx, insertTenantSQL, cfg.DevTenantID, cfg.DevTenantName); err != nil {
		return fmt.Errorf("failed to seed dev tenant: %w", err)
	}

	// 2. Upsert Admin User (mock password hash for dev)
	_, _ = pool.Exec(ctx, "DELETE FROM users WHERE email = $1 AND id != $2", cfg.DevAdminEmail, cfg.DevUserID)
	const insertUserSQL = `
		INSERT INTO users (id, email, password_hash, full_name, is_active)
		VALUES ($1, $2, '$2a$12$e9Y1iN7q5w8V7kE0Yk2b2.m/8S4T.aE3f2.5E2.5E2.5E2.5E2.5E', $3, TRUE)
		ON CONFLICT (id) DO UPDATE SET email = EXCLUDED.email, full_name = EXCLUDED.full_name, updated_at = NOW();
	`
	if _, err := pool.Exec(ctx, insertUserSQL, cfg.DevUserID, cfg.DevAdminEmail, cfg.DevAdminName); err != nil {
		return fmt.Errorf("failed to seed dev user: %w", err)
	}

	// 3. Upsert Membership
	const insertMemberSQL = `
		INSERT INTO tenant_members (id, tenant_id, user_id, role)
		VALUES (gen_random_uuid(), $1, $2, 'OWNER')
		ON CONFLICT (tenant_id, user_id) DO UPDATE SET role = 'OWNER', updated_at = NOW();
	`
	if _, err := pool.Exec(ctx, insertMemberSQL, cfg.DevTenantID, cfg.DevUserID); err != nil {
		return fmt.Errorf("failed to seed dev membership: %w", err)
	}

	// 4. Categories and Accounts under tenant RLS transaction
	err := postgres.WithTenantTx(ctx, pool, cfg.DevTenantID, cfg.DevUserID, func(tx pgx.Tx) error {
		// Default system categories
		type catSeed struct {
			name     string
			catType  string
			colorHex string
			icon     string
		}

		categories := []catSeed{
			{name: "Alimentação & Mercado", catType: "EXPENSE", colorHex: "#F59E0B", icon: "restaurant"},
			{name: "Transporte & Mobilidade", catType: "EXPENSE", colorHex: "#3B82F6", icon: "directions-car"},
			{name: "Moradia & Contas", catType: "EXPENSE", colorHex: "#EF4444", icon: "home"},
			{name: "Saúde & Farmácia", catType: "EXPENSE", colorHex: "#10B981", icon: "local-hospital"},
			{name: "Lazer & Assinaturas", catType: "EXPENSE", colorHex: "#8B5CF6", icon: "sports-esports"},
			{name: "Educação & Cursos", catType: "EXPENSE", colorHex: "#EC4899", icon: "school"},
			{name: "Outras Despesas", catType: "EXPENSE", colorHex: "#64748B", icon: "tag"},
			{name: "Salário / Prolabore", catType: "INCOME", colorHex: "#22C55E", icon: "payments"},
			{name: "Investimentos & Dividendos", catType: "INCOME", colorHex: "#06B6D4", icon: "trending-up"},
			{name: "Outras Receitas", catType: "INCOME", colorHex: "#14B8A6", icon: "attach-money"},
		}

		const insertCatSQL = `
			INSERT INTO categories (id, tenant_id, name, type, color_hex, icon_name, is_system)
			VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, TRUE)
			ON CONFLICT (tenant_id, name, type) DO NOTHING;
		`
		for _, c := range categories {
			if _, err := tx.Exec(ctx, insertCatSQL, cfg.DevTenantID, c.name, c.catType, c.colorHex, c.icon); err != nil {
				return fmt.Errorf("failed to insert category '%s': %w", c.name, err)
			}
		}

		// Check if any accounts exist
		var count int
		if err := tx.QueryRow(ctx, "SELECT COUNT(*) FROM accounts WHERE tenant_id = $1 AND deleted_at IS NULL", cfg.DevTenantID).Scan(&count); err != nil {
			return err
		}

		if count == 0 {
			const insertAccSQL = `
				INSERT INTO accounts (id, tenant_id, owner_user_id, name, type, currency, balance, visibility, institution_name, color_hex, icon_name, is_active)
				VALUES
				  (gen_random_uuid(), $1, $2, 'Conta Corrente Principal', 'CHECKING', 'BRL', 0.00, 'SHARED', 'Banco Principal', '#10B981', 'account-balance', TRUE),
				  (gen_random_uuid(), $1, $2, 'Cartão de Crédito', 'CREDIT_CARD', 'BRL', 0.00, 'SHARED', 'Cartão Nu', '#6366F1', 'credit-card', TRUE),
				  (gen_random_uuid(), $1, $2, 'Reserva de Emergência', 'SAVINGS', 'BRL', 0.00, 'SHARED', 'Tesouro Direto', '#F59E0B', 'savings', TRUE);
			`
			if _, err := tx.Exec(ctx, insertAccSQL, cfg.DevTenantID, cfg.DevUserID); err != nil {
				return fmt.Errorf("failed to seed initial accounts: %w", err)
			}
			logger.InfoContext(ctx, "seeded initial dev accounts (Checking, Credit Card, Savings)")
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to seed dev categories and accounts: %w", err)
	}

	logger.InfoContext(ctx, "dev admin seeding finished successfully")
	return nil
}
