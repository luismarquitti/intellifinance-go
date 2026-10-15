-- ============================================================================
-- IntelliFinance Migration: 000001_init_schema.down.sql
-- Description: Complete and idempotent rollback of 000001_init_schema.up.sql
-- Reverses in exact reverse order: role, policies, RLS, triggers, functions,
-- indexes, tables, types, and extensions.
-- ============================================================================

-- 1. ROW-LEVEL SECURITY (RLS) POLICIES
DROP POLICY IF EXISTS rls_audit_logs_insert_policy ON audit_logs;
DROP POLICY IF EXISTS rls_audit_logs_select_policy ON audit_logs;
DROP POLICY IF EXISTS rls_open_finance_items_tenant_isolation ON open_finance_items;
DROP POLICY IF EXISTS rls_ingestion_jobs_tenant_isolation ON ingestion_jobs;
DROP POLICY IF EXISTS rls_transaction_splits_isolation ON transaction_splits;
DROP POLICY IF EXISTS rls_transactions_tenant_isolation ON transactions;
DROP POLICY IF EXISTS rls_categories_tenant_isolation ON categories;
DROP POLICY IF EXISTS rls_accounts_tenant_isolation ON accounts;
DROP POLICY IF EXISTS rls_households_tenant_isolation ON households;

-- 3. DISABLE ROW-LEVEL SECURITY
ALTER TABLE IF EXISTS audit_logs NO FORCE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS audit_logs DISABLE ROW LEVEL SECURITY;

ALTER TABLE IF EXISTS open_finance_items NO FORCE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS open_finance_items DISABLE ROW LEVEL SECURITY;

ALTER TABLE IF EXISTS ingestion_jobs NO FORCE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS ingestion_jobs DISABLE ROW LEVEL SECURITY;

ALTER TABLE IF EXISTS transaction_splits NO FORCE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS transaction_splits DISABLE ROW LEVEL SECURITY;

ALTER TABLE IF EXISTS transactions NO FORCE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS transactions DISABLE ROW LEVEL SECURITY;

ALTER TABLE IF EXISTS categories NO FORCE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS categories DISABLE ROW LEVEL SECURITY;

ALTER TABLE IF EXISTS accounts NO FORCE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS accounts DISABLE ROW LEVEL SECURITY;

ALTER TABLE IF EXISTS households NO FORCE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS households DISABLE ROW LEVEL SECURITY;

-- 4. TRIGGERS
DROP TRIGGER IF EXISTS trg_immutable_audit_logs ON audit_logs;
DROP TRIGGER IF EXISTS trg_sync_balance ON transactions;
DROP TRIGGER IF EXISTS set_timestamp_open_finance_items ON open_finance_items;
DROP TRIGGER IF EXISTS set_timestamp_transaction_splits ON transaction_splits;
DROP TRIGGER IF EXISTS set_timestamp_transactions ON transactions;
DROP TRIGGER IF EXISTS set_timestamp_categories ON categories;
DROP TRIGGER IF EXISTS set_timestamp_accounts ON accounts;
DROP TRIGGER IF EXISTS set_timestamp_households ON households;
DROP TRIGGER IF EXISTS set_timestamp_tenant_members ON tenant_members;
DROP TRIGGER IF EXISTS set_timestamp_users ON users;
DROP TRIGGER IF EXISTS set_timestamp_tenants ON tenants;

-- 5. FUNCTIONS
DROP FUNCTION IF EXISTS resolve_open_finance_tenant(VARCHAR);
DROP FUNCTION IF EXISTS trigger_prevent_audit_log_mutation();
DROP FUNCTION IF EXISTS trigger_sync_account_balance();
DROP FUNCTION IF EXISTS trigger_set_timestamp();

-- 6. PERFORMANCE INDEXES
DROP INDEX IF EXISTS idx_audit_payload_gin;
DROP INDEX IF EXISTS idx_audit_resource;
DROP INDEX IF EXISTS idx_audit_tenant_created;
DROP INDEX IF EXISTS idx_open_finance_tenant_status;
DROP INDEX IF EXISTS idx_ingestion_account;
DROP INDEX IF EXISTS idx_ingestion_tenant_status;
DROP INDEX IF EXISTS idx_splits_tenant;
DROP INDEX IF EXISTS idx_splits_user;
DROP INDEX IF EXISTS idx_splits_tx;
DROP INDEX IF EXISTS idx_transactions_metadata_gin;
DROP INDEX IF EXISTS idx_transactions_ai_review;
DROP INDEX IF EXISTS idx_transactions_fingerprint;
DROP INDEX IF EXISTS idx_transactions_category_date;
DROP INDEX IF EXISTS idx_transactions_account_date;
DROP INDEX IF EXISTS idx_transactions_tenant_date;
DROP INDEX IF EXISTS idx_categories_tenant_parent;
DROP INDEX IF EXISTS idx_accounts_tenant_active_name;
DROP INDEX IF EXISTS idx_accounts_tenant_active;
DROP INDEX IF EXISTS idx_accounts_tenant_owner;
DROP INDEX IF EXISTS idx_tenant_members_tenant;
DROP INDEX IF EXISTS idx_tenant_members_user;

-- 7. CANONICAL TABLES
DROP TABLE IF EXISTS audit_logs CASCADE;
DROP TABLE IF EXISTS open_finance_items CASCADE;
DROP TABLE IF EXISTS ingestion_jobs CASCADE;
DROP TABLE IF EXISTS transaction_splits CASCADE;
DROP TABLE IF EXISTS transactions CASCADE;
DROP TABLE IF EXISTS categories CASCADE;
DROP TABLE IF EXISTS accounts CASCADE;
DROP TABLE IF EXISTS households CASCADE;
DROP TABLE IF EXISTS tenant_members CASCADE;
DROP TABLE IF EXISTS users CASCADE;
DROP TABLE IF EXISTS tenants CASCADE;

-- 8. ENUM TYPES
DROP TYPE IF EXISTS open_finance_item_status CASCADE;
DROP TYPE IF EXISTS job_status CASCADE;
DROP TYPE IF EXISTS source_type CASCADE;
DROP TYPE IF EXISTS transaction_status CASCADE;
DROP TYPE IF EXISTS transaction_type CASCADE;
DROP TYPE IF EXISTS visibility_type CASCADE;
DROP TYPE IF EXISTS account_type CASCADE;
DROP TYPE IF EXISTS member_role CASCADE;
DROP TYPE IF EXISTS tenant_type CASCADE;

-- 9. EXTENSIONS
DROP EXTENSION IF EXISTS "pgcrypto";
DROP EXTENSION IF EXISTS "uuid-ossp";

-- 10. APPLICATION ROLE (Cluster-wide cleanup with dependency check)
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'intellifinance_app') THEN
        BEGIN
            DROP OWNED BY intellifinance_app;
            DROP ROLE intellifinance_app;
        EXCEPTION
            WHEN dependent_objects_still_exist OR undefined_object THEN
                -- Safe fallback if role is still referenced in other databases
                NULL;
        END;
    END IF;
END
$$;

