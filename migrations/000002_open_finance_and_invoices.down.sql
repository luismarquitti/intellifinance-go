-- Migration: 000002_open_finance_and_invoices.down.sql
DROP TABLE IF EXISTS provider_balance_snapshots CASCADE;
ALTER TABLE transactions DROP COLUMN IF EXISTS bill_id;
DROP TABLE IF EXISTS credit_card_invoices CASCADE;