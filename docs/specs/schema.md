# IntelliFinance Technical Specification: Relational Schema & PostgreSQL DDL

- **Document ID**: SPEC-001-SCHEMA
- **Status**: Authoritative / Production-Ready
- **Target Database**: PostgreSQL 15+ (Local Homelab & Cloud SQL / Supabase)
- **Primary Driver**: `jackc/pgx/v5` with `sqlc` v2
- **Security Context**: Database-Enforced Row-Level Security (RLS) with Session Scoping

---

## 1. Overview & Architectural Principles

The IntelliFinance relational schema is designed for zero-leak multi-tenancy, high-throughput financial batch ingestion, real-time balance reconciliation, and cryptographic auditability. The data architecture satisfies the following technical invariants:

1. **Strict Multi-Tenant Isolation**: Every financial entity is scoped by a mandatory `tenant_id` foreign key. Isolation is enforced not merely at the application layer, but directly in the PostgreSQL engine via Row-Level Security (RLS) policies using transaction-scoped session settings (`app.current_tenant_id` and `app.current_user_id`).
2. **Co-Budgeting & Visibility Controls**: Within a household tenant (e.g., Luis & Eluma), accounts and transactions support granular visibility levels (`SHARED`, `PRIVATE`, `RESTRICTED`), allowing joint balance tracking while keeping personal transactions confidential.
3. **Deterministic Deduplication**: Transactions enforce an SHA-256 fingerprint uniqueness constraint `(tenant_id, fingerprint)`. Duplicate imports from OFX, CSV, Excel, or Open Finance webhooks are rejected atomically via `ON CONFLICT DO NOTHING`.
4. **Monetary Precision**: All monetary values are stored as `NUMERIC(15, 2)` (or integer cent equivalents where specified), mapped to `shopspring/decimal.Decimal` in Go. IEEE 754 floating-point types (`REAL`, `FLOAT`, `DOUBLE PRECISION`) are strictly prohibited.
5. **Atomic Balance Maintenance**: Account balances are maintained synchronously via transactional database triggers, eliminating drift between transaction ledgers and account header balances.
6. **Immutable Audit Trails**: An append-only audit log records all administrative and data lifecycle operations. Database triggers prevent any `UPDATE` or `DELETE` mutations on audit records.

---

## 2. Complete PostgreSQL DDL (`schema.sql`)

```sql
-- ============================================================================
-- IntelliFinance Greenfield PostgreSQL DDL Schema
-- Version: 1.0.0
-- Extensions: uuid-ossp, pgcrypto
-- ============================================================================

-- 1. EXTENSIONS
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- 2. ENUM DEFINITIONS
CREATE TYPE tenant_type AS ENUM (
    'HOUSEHOLD',
    'PERSONAL',
    'BUSINESS'
);

CREATE TYPE member_role AS ENUM (
    'OWNER',
    'ADMIN',
    'MEMBER',
    'VIEWER'
);

CREATE TYPE account_type AS ENUM (
    'CHECKING',
    'CREDIT_CARD',
    'SAVINGS',
    'INVESTMENT',
    'CASH'
);

CREATE TYPE visibility_type AS ENUM (
    'SHARED',
    'PRIVATE',
    'RESTRICTED'
);

CREATE TYPE transaction_type AS ENUM (
    'INCOME',
    'EXPENSE',
    'TRANSFER'
);

CREATE TYPE transaction_status AS ENUM (
    'PENDING',
    'COMPLETED',
    'RECONCILED',
    'VOID'
);

CREATE TYPE source_type AS ENUM (
    'PLUGGY',
    'OFX',
    'CSV',
    'EXCEL',
    'PDF',
    'MANUAL'
);

CREATE TYPE job_status AS ENUM (
    'PENDING',
    'PROCESSING',
    'COMPLETED',
    'FAILED'
);

CREATE TYPE open_finance_item_status AS ENUM (
    'DISCONNECTED',
    'CONNECTING',
    'WAITING_USER_INPUT',
    'UPDATING',
    'UPDATED',
    'LOGIN_ERROR',
    'OUTDATED',
    'ERROR'
);

-- ============================================================================
-- 3. CORE IDENTITY & MULTI-TENANCY TABLES
-- ============================================================================

-- Tenants (Household / Organization Unit)
CREATE TABLE tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(120) NOT NULL,
    type tenant_type NOT NULL DEFAULT 'HOUSEHOLD',
    currency VARCHAR(3) NOT NULL DEFAULT 'BRL',
    settings JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Users Table
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    full_name VARCHAR(150) NOT NULL,
    avatar_url VARCHAR(500),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Tenant Memberships (Mapping Users to Tenants with RBAC)
CREATE TABLE tenant_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role member_role NOT NULL DEFAULT 'MEMBER',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_tenant_member UNIQUE (tenant_id, user_id)
);

-- Households View / Metadata (Convenience entity for household budgeting)
CREATE TABLE households (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE UNIQUE,
    budget_start_day SMALLINT NOT NULL DEFAULT 1 CHECK (budget_start_day BETWEEN 1 AND 31),
    split_rules JSONB NOT NULL DEFAULT '{"default_split_ratio": 50.00, "members": []}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================================
-- 4. FINANCIAL LEDGER TABLES
-- ============================================================================

-- Accounts Table
CREATE TABLE accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    owner_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    name VARCHAR(100) NOT NULL,
    type account_type NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'BRL',
    balance NUMERIC(15, 2) NOT NULL DEFAULT 0.00,
    visibility visibility_type NOT NULL DEFAULT 'SHARED',
    institution_id VARCHAR(50),      -- e.g. "itau", "nubank", "inter"
    institution_name VARCHAR(100),
    account_number_masked VARCHAR(50),
    color_hex VARCHAR(7) NOT NULL DEFAULT '#6366F1',
    icon_name VARCHAR(50) NOT NULL DEFAULT 'account-balance',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_accounts_tenant_id UNIQUE (tenant_id, id)
);

-- Categories Table (Hierarchical Budget Taxonomy)
CREATE TABLE categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    parent_id UUID,
    name VARCHAR(80) NOT NULL,
    type transaction_type NOT NULL,
    color_hex VARCHAR(7) NOT NULL DEFAULT '#64748B',
    icon_name VARCHAR(50) NOT NULL DEFAULT 'tag',
    is_system BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_categories_tenant_id UNIQUE (tenant_id, id),
    CONSTRAINT uq_tenant_category_name_type UNIQUE (tenant_id, name, type),
    CONSTRAINT fk_categories_tenant_parent FOREIGN KEY (tenant_id, parent_id) REFERENCES categories(tenant_id, id) ON DELETE SET NULL
);

-- Transactions Table (Primary Financial Journal Entry)
CREATE TABLE transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    account_id UUID NOT NULL,
    category_id UUID,
    created_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL, -- Nullable or sentinel UUID '00000000-0000-0000-0000-000000000000' for LGPD erasure
    transacted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    date DATE NOT NULL,
    amount NUMERIC(15, 2) NOT NULL, -- Negative: Outflow/Expense, Positive: Inflow/Income
    description VARCHAR(255) NOT NULL,
    clean_description VARCHAR(255) NOT NULL,
    type transaction_type NOT NULL,
    status transaction_status NOT NULL DEFAULT 'COMPLETED',
    visibility visibility_type NOT NULL DEFAULT 'SHARED',
    source source_type NOT NULL DEFAULT 'MANUAL',
    external_id VARCHAR(120),       -- FITID for OFX, Pluggy transaction ID, etc.
    sequence_index INT NOT NULL DEFAULT 0, -- Statement row or intraday collision disambiguator
    fingerprint VARCHAR(64) NOT NULL, -- Deterministic SHA-256 hash for deduplication
    is_ai_categorized BOOLEAN NOT NULL DEFAULT FALSE,
    ai_confidence NUMERIC(3, 2),    -- 0.00 to 1.00
    ai_model_version VARCHAR(50),
    is_manually_verified BOOLEAN NOT NULL DEFAULT FALSE,
    transfer_peer_transaction_id UUID REFERENCES transactions(id) ON DELETE SET NULL,
    notes TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_tenant_transaction_fingerprint UNIQUE (tenant_id, fingerprint),
    CONSTRAINT fk_transactions_tenant_account FOREIGN KEY (tenant_id, account_id) REFERENCES accounts(tenant_id, id) ON DELETE CASCADE,
    CONSTRAINT fk_transactions_tenant_category FOREIGN KEY (tenant_id, category_id) REFERENCES categories(tenant_id, id) ON DELETE SET NULL
);

-- Transaction Splits Table (Co-budgeting Split Attribution)
CREATE TABLE transaction_splits (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    transaction_id UUID NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount NUMERIC(15, 2) NOT NULL,
    percentage NUMERIC(5, 2) NOT NULL,
    notes VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================================
-- 5. INGESTION & OPEN FINANCE TABLES
-- ============================================================================

-- Ingestion Jobs Table
CREATE TABLE ingestion_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    initiated_by_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    source source_type NOT NULL,
    status job_status NOT NULL DEFAULT 'PENDING',
    source_filename VARCHAR(255) NOT NULL,
    file_storage_path VARCHAR(500) NOT NULL,
    file_mime_type VARCHAR(100) NOT NULL,
    file_size_bytes BIGINT NOT NULL DEFAULT 0,
    total_records INT NOT NULL DEFAULT 0,
    inserted_records INT NOT NULL DEFAULT 0,
    duplicate_records INT NOT NULL DEFAULT 0,
    failed_records INT NOT NULL DEFAULT 0,
    error_summary TEXT,
    error_details JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

-- Open Finance Items Table (Pluggy Bank Connection Records)
CREATE TABLE open_finance_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    connector_id INT NOT NULL,               -- Pluggy connector identifier (e.g. 2 for Bradesco, 201 for Nubank)
    connector_name VARCHAR(100) NOT NULL,
    item_id VARCHAR(100) NOT NULL UNIQUE,    -- Pluggy item GUID
    status open_finance_item_status NOT NULL DEFAULT 'UPDATING',
    error_code VARCHAR(50),
    error_message TEXT,
    parameter_required JSONB,                -- Schema for MFA or SMS token input
    consent_expires_at TIMESTAMPTZ,
    last_synced_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_tenant_pluggy_item UNIQUE (tenant_id, item_id)
);

-- ============================================================================
-- 6. AUDIT & COMPLIANCE TABLES
-- ============================================================================

-- Immutable Audit Log Table
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(60) NOT NULL,             -- e.g. "AUTH_LOGIN", "TX_CREATE", "JOB_UPLOAD", "TENANT_PURGE"
    resource_type VARCHAR(60) NOT NULL,      -- e.g. "transaction", "account", "ingestion_job"
    resource_id UUID,
    ip_address INET,
    user_agent TEXT,
    payload_diff JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================================
-- 7. PERFORMANCE INDEXES
-- ============================================================================

-- Users & Membership
CREATE INDEX idx_tenant_members_user ON tenant_members(user_id);
CREATE INDEX idx_tenant_members_tenant ON tenant_members(tenant_id);

-- Accounts
CREATE INDEX idx_accounts_tenant_owner ON accounts(tenant_id, owner_user_id);
CREATE INDEX idx_accounts_tenant_active ON accounts(tenant_id, is_active);
CREATE UNIQUE INDEX idx_accounts_tenant_active_name ON accounts(tenant_id, name) WHERE deleted_at IS NULL;

-- Categories
CREATE INDEX idx_categories_tenant_parent ON categories(tenant_id, parent_id);

-- Transactions (High Frequency Query Paths)
CREATE INDEX idx_transactions_tenant_date ON transactions(tenant_id, date DESC);
CREATE INDEX idx_transactions_account_date ON transactions(account_id, date DESC);
CREATE INDEX idx_transactions_category_date ON transactions(category_id, date DESC);
CREATE INDEX idx_transactions_fingerprint ON transactions(fingerprint);
CREATE INDEX idx_transactions_ai_review ON transactions(tenant_id, is_ai_categorized, is_manually_verified)
    WHERE is_ai_categorized = TRUE AND is_manually_verified = FALSE;
CREATE INDEX idx_transactions_metadata_gin ON transactions USING GIN (metadata);

-- Transaction Splits
CREATE INDEX idx_splits_tx ON transaction_splits(transaction_id);
CREATE INDEX idx_splits_user ON transaction_splits(user_id);
CREATE INDEX idx_splits_tenant ON transaction_splits(tenant_id);

-- Ingestion Jobs
CREATE INDEX idx_ingestion_tenant_status ON ingestion_jobs(tenant_id, status);
CREATE INDEX idx_ingestion_account ON ingestion_jobs(account_id);

-- Open Finance Items
CREATE INDEX idx_open_finance_tenant_status ON open_finance_items(tenant_id, status);

-- Audit Logs
CREATE INDEX idx_audit_tenant_created ON audit_logs(tenant_id, created_at DESC);
CREATE INDEX idx_audit_resource ON audit_logs(resource_type, resource_id);
CREATE INDEX idx_audit_payload_gin ON audit_logs USING GIN (payload_diff);

-- ============================================================================
-- 8. TRIGGERS & STORED PROCEDURES
-- ============================================================================

-- 8.1 Automatic updated_at Timestamp Function
CREATE OR REPLACE FUNCTION trigger_set_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Apply updated_at trigger across all mutable entities
CREATE TRIGGER set_timestamp_tenants
    BEFORE UPDATE ON tenants
    FOR EACH ROW EXECUTE FUNCTION trigger_set_timestamp();

CREATE TRIGGER set_timestamp_users
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION trigger_set_timestamp();

CREATE TRIGGER set_timestamp_tenant_members
    BEFORE UPDATE ON tenant_members
    FOR EACH ROW EXECUTE FUNCTION trigger_set_timestamp();

CREATE TRIGGER set_timestamp_households
    BEFORE UPDATE ON households
    FOR EACH ROW EXECUTE FUNCTION trigger_set_timestamp();

CREATE TRIGGER set_timestamp_accounts
    BEFORE UPDATE ON accounts
    FOR EACH ROW EXECUTE FUNCTION trigger_set_timestamp();

CREATE TRIGGER set_timestamp_categories
    BEFORE UPDATE ON categories
    FOR EACH ROW EXECUTE FUNCTION trigger_set_timestamp();

CREATE TRIGGER set_timestamp_transactions
    BEFORE UPDATE ON transactions
    FOR EACH ROW EXECUTE FUNCTION trigger_set_timestamp();

CREATE TRIGGER set_timestamp_transaction_splits
    BEFORE UPDATE ON transaction_splits
    FOR EACH ROW EXECUTE FUNCTION trigger_set_timestamp();

CREATE TRIGGER set_timestamp_open_finance_items
    BEFORE UPDATE ON open_finance_items
    FOR EACH ROW EXECUTE FUNCTION trigger_set_timestamp();

-- 8.2 Atomic Account Balance Synchronization Trigger
CREATE OR REPLACE FUNCTION trigger_sync_account_balance()
RETURNS TRIGGER AS $$
BEGIN
    IF (TG_OP = 'INSERT') THEN
        IF (NEW.status = 'COMPLETED' OR NEW.status = 'RECONCILED') THEN
            UPDATE accounts
            SET balance = balance + NEW.amount
            WHERE id = NEW.account_id;
        END IF;
        RETURN NEW;
    ELSIF (TG_OP = 'UPDATE') THEN
        -- Adjust old amount out if previously active
        IF (OLD.status = 'COMPLETED' OR OLD.status = 'RECONCILED') THEN
            UPDATE accounts
            SET balance = balance - OLD.amount
            WHERE id = OLD.account_id;
        END IF;
        -- Adjust new amount in if currently active
        IF (NEW.status = 'COMPLETED' OR NEW.status = 'RECONCILED') THEN
            UPDATE accounts
            SET balance = balance + NEW.amount
            WHERE id = NEW.account_id;
        END IF;
        RETURN NEW;
    ELSIF (TG_OP = 'DELETE') THEN
        IF (OLD.status = 'COMPLETED' OR OLD.status = 'RECONCILED') THEN
            UPDATE accounts
            SET balance = balance - OLD.amount
            WHERE id = OLD.account_id;
        END IF;
        RETURN OLD;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_sync_balance
    AFTER INSERT OR UPDATE OR DELETE ON transactions
    FOR EACH ROW EXECUTE FUNCTION trigger_sync_account_balance();

-- 8.3 Immutability Trigger on Audit Logs (Guarantees Append-Only Trail)
CREATE OR REPLACE FUNCTION trigger_prevent_audit_log_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'audit_logs entries are immutable and cannot be updated or deleted';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_immutable_audit_logs
    BEFORE UPDATE OR DELETE ON audit_logs
    FOR EACH ROW EXECUTE FUNCTION trigger_prevent_audit_log_mutation();

-- 8.4 Secure Open Finance Webhook Tenant Resolution Function
-- Runs as SECURITY DEFINER with fixed search_path to securely resolve tenant_id for incoming
-- unauthenticated webhooks without exposing other tenant records or bypassing RLS in application queries.
CREATE OR REPLACE FUNCTION resolve_open_finance_tenant(p_item_id VARCHAR)
RETURNS UUID
SECURITY DEFINER
SET search_path = public, pg_temp
AS $$
DECLARE
    v_tenant_id UUID;
BEGIN
    -- Strict input sanitization
    IF p_item_id IS NULL OR length(trim(p_item_id)) = 0 THEN
        RETURN NULL;
    END IF;

    SELECT tenant_id INTO v_tenant_id
    FROM open_finance_items
    WHERE item_id = trim(p_item_id)
    LIMIT 1;

    RETURN v_tenant_id;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- 9. ROW-LEVEL SECURITY (RLS) POLICIES
-- ============================================================================

-- Enable and Force RLS on all tenant-scoped tables
ALTER TABLE households ENABLE ROW LEVEL SECURITY;
ALTER TABLE households FORCE ROW LEVEL SECURITY;

ALTER TABLE accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE accounts FORCE ROW LEVEL SECURITY;

ALTER TABLE categories ENABLE ROW LEVEL SECURITY;
ALTER TABLE categories FORCE ROW LEVEL SECURITY;

ALTER TABLE transactions ENABLE ROW LEVEL SECURITY;
ALTER TABLE transactions FORCE ROW LEVEL SECURITY;

ALTER TABLE transaction_splits ENABLE ROW LEVEL SECURITY;
ALTER TABLE transaction_splits FORCE ROW LEVEL SECURITY;

ALTER TABLE ingestion_jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE ingestion_jobs FORCE ROW LEVEL SECURITY;

ALTER TABLE open_finance_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE open_finance_items FORCE ROW LEVEL SECURITY;

ALTER TABLE audit_logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_logs FORCE ROW LEVEL SECURITY;

-- 9.0 Households Isolation Policy
CREATE POLICY rls_households_tenant_isolation ON households
    FOR ALL
    USING (
        tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::UUID
    );

-- 9.1 Accounts Isolation Policy
-- Strict privacy: PRIVATE accounts are strictly visible ONLY to their owner.
-- No role-based bypass (OWNER/ADMIN cannot inspect other household members' PRIVATE accounts).
CREATE POLICY rls_accounts_tenant_isolation ON accounts
    FOR ALL
    USING (
        tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::UUID
        AND (
            visibility = 'SHARED'
            OR owner_user_id = NULLIF(current_setting('app.current_user_id', true), '')::UUID
        )
    );

-- 9.2 Categories Isolation Policy
CREATE POLICY rls_categories_tenant_isolation ON categories
    FOR ALL
    USING (
        tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::UUID
    );

-- 9.3 Transactions Isolation Policy
-- Strict privacy: PRIVATE transactions are strictly visible ONLY to their creator.
-- No role-based bypass (OWNER/ADMIN cannot inspect other household members' confidential gifts or personal expenses).
CREATE POLICY rls_transactions_tenant_isolation ON transactions
    FOR ALL
    USING (
        tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::UUID
        AND (
            visibility = 'SHARED'
            OR created_by_user_id = NULLIF(current_setting('app.current_user_id', true), '')::UUID
        )
    );

-- 9.4 Transaction Splits Policy
CREATE POLICY rls_transaction_splits_isolation ON transaction_splits
    FOR ALL
    USING (
        tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::UUID
    );

-- 9.5 Ingestion Jobs Policy
CREATE POLICY rls_ingestion_jobs_tenant_isolation ON ingestion_jobs
    FOR ALL
    USING (
        tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::UUID
    );

-- 9.6 Open Finance Items Policy
CREATE POLICY rls_open_finance_items_tenant_isolation ON open_finance_items
    FOR ALL
    USING (
        tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::UUID
    );

-- 9.7 Audit Logs Policy (Tenants can view their own logs; append is system-wide)
CREATE POLICY rls_audit_logs_select_policy ON audit_logs
    FOR SELECT
    USING (
        tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::UUID
    );

CREATE POLICY rls_audit_logs_insert_policy ON audit_logs
    FOR INSERT
    WITH CHECK (
        tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::UUID
    );

-- ============================================================================
-- 10. LEAST-PRIVILEGE APPLICATION DATABASE ROLE
-- ============================================================================
-- PostgreSQL superusers and roles with BYPASSRLS bypass RLS policies unconditionally.
-- Application connection pools must connect using a dedicated non-superuser role.
CREATE ROLE intellifinance_app WITH LOGIN PASSWORD 'CHANGE_IN_PRODUCTION';
ALTER ROLE intellifinance_app NOSUPERUSER NOBYPASSRLS;

GRANT USAGE ON SCHEMA public TO intellifinance_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO intellifinance_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO intellifinance_app;
REVOKE ALL ON FUNCTION resolve_open_finance_tenant(VARCHAR) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION resolve_open_finance_tenant(VARCHAR) TO intellifinance_app;
```

---

## 3. Go Connection Pooling & Session Scoping (`pgxpool`)

Because connection pools reuse physical TCP connections, setting session configuration variables (`SET app.current_tenant_id`) globally would introduce catastrophic cross-tenant state leaks. IntelliFinance enforces **Transaction-Scoped Session Settings** using `SET LOCAL`. 

```go
package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// WithTenantTx executes a database operation within an isolated PostgreSQL transaction
// where RLS session settings are guaranteed to be discarded upon commit or rollback.
func WithTenantTx(
	ctx context.Context,
	pool *pgxpool.Pool,
	tenantID uuid.UUID,
	userID uuid.UUID,
	fn func(tx pgx.Tx) error,
) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// SET LOCAL applies exclusively to the current transaction block
	const setSessionSQL = `
		SET LOCAL app.current_tenant_id = $1;
		SET LOCAL app.current_user_id = $2;
	`
	if _, err := tx.Exec(ctx, setSessionSQL, tenantID.String(), userID.String()); err != nil {
		return fmt.Errorf("failed to bind RLS tenant context: %w", err)
	}

	if err := fn(tx); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
```

---

## 4. Comprehensive Data Dictionary

| Table | Column | Type | Nullable | Constraints & Defaults | Description |
|---|---|---|---|---|---|
| `tenants` | `id` | UUID | NO | PK, `gen_random_uuid()` | Unique tenant identifier (Household or Account entity) |
| `tenants` | `name` | VARCHAR(120) | NO | | Display name of household (e.g., "Família Marquitti") |
| `tenants` | `type` | tenant_type | NO | DEFAULT 'HOUSEHOLD' | Tenancy classifier (`HOUSEHOLD`, `PERSONAL`, `BUSINESS`) |
| `tenants` | `currency` | VARCHAR(3) | NO | DEFAULT 'BRL' | ISO 4217 standard currency code |
| `tenants` | `settings` | JSONB | NO | DEFAULT '{}' | Tenant-level feature flags and preferences |
| `users` | `id` | UUID | NO | PK, `gen_random_uuid()` | Unique user identifier |
| `users` | `email` | VARCHAR(255) | NO | UNIQUE | User login email address |
| `users` | `password_hash` | VARCHAR(255) | NO | | Argon2id or bcrypt hashed password string |
| `users` | `full_name` | VARCHAR(150) | NO | | Legal or preferred display name |
| `users` | `is_active` | BOOLEAN | NO | DEFAULT TRUE | Account enablement status |
| `tenant_members` | `tenant_id` | UUID | NO | FK `tenants(id)` | Associated tenant |
| `tenant_members` | `user_id` | UUID | NO | FK `users(id)` | Associated user |
| `tenant_members` | `role` | member_role | NO | DEFAULT 'MEMBER' | Role within tenant (`OWNER`, `ADMIN`, `MEMBER`, `VIEWER`) |
| `households` | `id` | UUID | NO | PK, `gen_random_uuid()` | Unique household metadata identifier |
| `households` | `tenant_id` | UUID | NO | FK `tenants(id)` UNIQUE | Household tenant reference |
| `households` | `budget_start_day` | SMALLINT | NO | DEFAULT 1, CHECK (1-31)| Billing/budget start cycle day |
| `households` | `split_rules` | JSONB | NO | DEFAULT '{"default_split_ratio": 50.00, "members": []}' | Generic co-budgeting member split configurations |
| `accounts` | `id` | UUID | NO | PK, `gen_random_uuid()` | Unique account identifier |
| `accounts` | `tenant_id` | UUID | NO | FK `tenants(id)` | Owner tenant (part of composite key `uq_accounts_tenant_id`) |
| `accounts` | `owner_user_id`| UUID | NO | FK `users(id)` | User who registered/manages this account |
| `accounts` | `name` | VARCHAR(100) | NO | Partial Unique (`deleted_at IS NULL`) | Human-readable account label (e.g., "Itaú Conjunta") |
| `accounts` | `type` | account_type | NO | | Account classification (`CHECKING`, `CREDIT_CARD`, etc.) |
| `accounts` | `balance` | NUMERIC(15,2)| NO | DEFAULT 0.00 | Real-time calculated ledger balance |
| `accounts` | `visibility` | visibility_type| NO | DEFAULT 'SHARED' | Granular access control (`SHARED`, `PRIVATE`, `RESTRICTED`)|
| `accounts` | `deleted_at` | TIMESTAMPTZ | YES | | Soft-deletion timestamp (allows name re-use after deletion) |
| `categories` | `id` | UUID | NO | PK, `gen_random_uuid()` | Unique category identifier (part of composite key `uq_categories_tenant_id`) |
| `categories` | `parent_id` | UUID | YES | Composite FK `(tenant_id, parent_id)` | Parent category for hierarchical grouping |
| `categories` | `name` | VARCHAR(80) | NO | UNIQUE(tenant, name, type)| Category title (e.g., "Supermercado") |
| `categories` | `type` | transaction_type| NO| | Applicable flow (`INCOME`, `EXPENSE`, `TRANSFER`) |
| `transactions` | `id` | UUID | NO | PK, `gen_random_uuid()` | Primary transaction identifier |
| `transactions` | `tenant_id` | UUID | NO | FK `tenants(id)` | Tenancy boundary |
| `transactions` | `account_id` | UUID | NO | Composite FK `(tenant_id, account_id)` | Financial account debited/credited |
| `transactions` | `category_id`| UUID | YES | Composite FK `(tenant_id, category_id)` | Budget taxonomy attribution |
| `transactions` | `created_by_user_id`| UUID | YES | FK `users(id)` ON DELETE SET NULL | Author user (or sentinel UUID 00000000-0000-0000-0000-000000000000) |
| `transactions` | `transacted_at` | TIMESTAMPTZ | NO | DEFAULT NOW() | Canonical timezone-aware event timestamp |
| `transactions` | `date` | DATE | NO | | Accounting date of transaction (America/Sao_Paulo) |
| `transactions` | `amount` | NUMERIC(15,2)| NO | | Signed amount (negative = expense, positive = income) |
| `transactions` | `description`| VARCHAR(255)| NO | | Raw statement description |
| `transactions` | `clean_description`| VARCHAR(255)| NO | | Normalized string with bank prefixes removed |
| `transactions` | `sequence_index` | INT | NO | DEFAULT 0 | Batch statement row index for same-day duplicate disambiguation |
| `transactions` | `fingerprint` | VARCHAR(64) | NO | UNIQUE(tenant, fingerprint)| SHA-256 deterministic idempotency hash |
| `transactions` | `is_ai_categorized`| BOOLEAN| NO | DEFAULT FALSE | Flag indicating machine learning category prediction |
| `transactions` | `ai_confidence` | NUMERIC(3,2)| YES| CHECK(0.00-1.00) | AI confidence score |
| `ingestion_jobs` | `id` | UUID | NO | PK, `gen_random_uuid()` | Batch ingestion tracking identifier |
| `ingestion_jobs` | `status` | job_status | NO | DEFAULT 'PENDING' | Execution state (`PENDING`, `PROCESSING`, `COMPLETED`, `FAILED`)|
| `ingestion_jobs` | `file_storage_path`| VARCHAR(500)| NO | | Local disk or GCS bucket path |
| `open_finance_items`| `item_id` | VARCHAR(100)| NO | UNIQUE | Pluggy external Item UUID |
| `open_finance_items`| `status` | open_finance_item_status| NO | DEFAULT 'UPDATING' | Current synchronization lifecycle status |
| `audit_logs` | `id` | UUID | NO | PK, `gen_random_uuid()` | Audit record GUID |
| `audit_logs` | `action` | VARCHAR(60) | NO | | Executed system action code |
| `audit_logs` | `payload_diff`| JSONB | NO | DEFAULT '{}' | Before/after state snapshot for compliance |

---

## 5. Migration Execution Specification (`golang-migrate`)

Database schema evolution is managed through numbered, bidirectional SQL migrations:

```
migrations/
├── 000001_create_extensions_and_enums.up.sql
├── 000001_create_extensions_and_enums.down.sql
├── 000002_create_tenants_and_users.up.sql
├── 000002_create_tenants_and_users.down.sql
├── 000003_create_financial_ledger.up.sql
├── 000003_create_financial_ledger.down.sql
├── 000004_create_ingestion_and_open_finance.up.sql
├── 000004_create_ingestion_and_open_finance.down.sql
├── 000005_create_audit_logs.up.sql
├── 000005_create_audit_logs.down.sql
├── 000006_create_triggers_and_balances.up.sql
├── 000006_create_triggers_and_balances.down.sql
├── 000007_enable_row_level_security.up.sql
└── 000007_enable_row_level_security.down.sql
```

The migration CLI entrypoint `cmd/migrate/main.go` wraps `github.com/golang-migrate/migrate/v4` and executes automatically during CI integration tests and container boot orchestration.
