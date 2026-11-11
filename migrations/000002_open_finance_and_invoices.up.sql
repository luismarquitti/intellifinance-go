-- Migration: 000002_open_finance_and_invoices.up.sql
-- Description: Credit card invoices, provider balance snapshots, and bill linking for Open Finance

-- 1. Provider Balance Snapshots (Segregates external reported balances from reconstructed ledger balance)
CREATE TABLE IF NOT EXISTS provider_balance_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    provider VARCHAR(50) NOT NULL DEFAULT 'PLUGGY',
    balance NUMERIC(15, 2) NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'BRL',
    observed_at TIMESTAMPTZ NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_provider_balance_snapshots_account_observed
    ON provider_balance_snapshots (tenant_id, account_id, observed_at DESC);

ALTER TABLE provider_balance_snapshots ENABLE ROW LEVEL SECURITY;
ALTER TABLE provider_balance_snapshots FORCE ROW LEVEL SECURITY;

CREATE POLICY rls_provider_balance_snapshots_policy ON provider_balance_snapshots
    FOR ALL
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::UUID)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::UUID);

-- 2. Credit Card Invoices (Bills)
CREATE TABLE IF NOT EXISTS credit_card_invoices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    external_id VARCHAR(255) NULL,
    due_date DATE NOT NULL,
    closing_date DATE NULL,
    total_amount NUMERIC(15, 2) NOT NULL DEFAULT 0.00,
    minimum_payment NUMERIC(15, 2) NOT NULL DEFAULT 0.00,
    currency VARCHAR(3) NOT NULL DEFAULT 'BRL',
    status VARCHAR(20) NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN', 'CLOSED', 'OVERDUE', 'PAID')),
    allows_installments BOOLEAN NOT NULL DEFAULT false,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_credit_card_invoices_external
    ON credit_card_invoices (tenant_id, external_id)
    WHERE external_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_credit_card_invoices_period
    ON credit_card_invoices (tenant_id, account_id, due_date);

CREATE INDEX IF NOT EXISTS idx_credit_card_invoices_tenant_account
    ON credit_card_invoices (tenant_id, account_id, due_date DESC);

ALTER TABLE credit_card_invoices ENABLE ROW LEVEL SECURITY;
ALTER TABLE credit_card_invoices FORCE ROW LEVEL SECURITY;

CREATE POLICY rls_credit_card_invoices_policy ON credit_card_invoices
    FOR ALL
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::UUID)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::UUID);

-- 3. Link Transactions to Invoices (bill_id)
ALTER TABLE transactions
    ADD COLUMN IF NOT EXISTS bill_id UUID REFERENCES credit_card_invoices(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_transactions_bill_id
    ON transactions (tenant_id, bill_id)
    WHERE bill_id IS NOT NULL;

-- 4. Grants on new tables to app user
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'intellifinance_app') THEN
        GRANT SELECT, INSERT, UPDATE, DELETE ON provider_balance_snapshots TO intellifinance_app;
        GRANT SELECT, INSERT, UPDATE, DELETE ON credit_card_invoices TO intellifinance_app;
    END IF;
END
$$;