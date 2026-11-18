# Spec: Deepen Open Finance Sync & Invoice Settlement

**Status:** completed

## Problem Statement

Synchronizing Open Finance accounts, credit card Invoices, and Transactions is currently embedded directly inside a procedural CLI entrypoint that mixes flag parsing with over 500 lines of untested domain calculations, raw PostgreSQL SQL updates bypassing repository seams, and a third copy of Invoice Settlement matching. Because Invoice Settlement rules (date windows, keyword affinity, card-payment detection, and Liquid Account fallback) are duplicated with divergent tolerances across Open Finance sync, manual Invoice settlement, and Internal Transfer reconciliation scanning, fixes to one flow silently drift from the others and cannot be tested through a single interface.

## Solution

Extract all pure financial normalization, Invoice billing-cycle routing, unified Invoice Settlement matching, and Ledger Balance anchoring rules into an in-process domain core, and consolidate Open Finance synchronization orchestration behind one high-leverage application port (`OpenFinanceSyncService`). Eliminate raw SQL leakage from the CLI by adding an external transaction reconciliation method to `TransactionRepository`, allowing the CLI to collapse into thin configuration wiring while Open Finance sync, manual Invoice settlement, and transfer reconciliation share one canonical source of truth.

## User Stories

1. As a tenant operator, I want Open Finance synchronization to fetch connected accounts and persist a Provider Balance Snapshot separately from each account's Ledger Balance, so that bank observations never overwrite atomic ledger accounting.
2. As a tenant operator, I want credit card purchases from Open Finance to be normalized into negative expense transactions and refunds into positive income transactions, so that ledger sums reflect real liabilities.
3. As a tenant operator, I want credit card bill payment entries from Open Finance to be classified as neutral transfer inflows rather than spending or income, so that consolidated cashflow is never double-counted.
4. As a tenant operator, I want unbilled credit card transactions without a provider bill identifier to be routed deterministically into monthly Invoice cycles based on closing and due days in Brazil time (`America/Sao_Paulo`), so that upcoming liabilities are accurate.
5. As a tenant operator, I want future-dated Liquid Account transactions from Open Finance to be recorded as `PENDING` until their settlement date arrives in Brazil time, and automatically promoted to `COMPLETED` on subsequent sync runs, so that the current Ledger Balance excludes unposted items.
6. As a tenant operator, I want newly connected Liquid Accounts to have their initial Ledger Balance anchored against the Provider Balance Snapshot adjusted for post-today scheduled movements, so that starting balances reconcile cleanly.
7. As a tenant operator, I want Invoice Settlement detection during Open Finance sync, manual Invoice payment, and HITL transfer reconciliation scanning to apply the exact same date window and keyword rules, so that paid Invoices are reconciled consistently across every entry point.
8. As a tenant operator, I want credit card Ledger Balances to reflect the negative sum of unpaid Invoices after sync and settlement, so that total net worth reflects outstanding card debt.
9. As a tenant operator, I want post-sync reconciliation and category matching to emit `PENDING` Pending Actions rather than mutating categories or linking transfers autonomously, so that human approval remains enforced.
10. As a maintainer, I want Open Finance synchronization and Invoice Settlement behavior to be tested end-to-end against fake external connectors and repository adapters without requiring raw SQL or live banking credentials.

## Implementation Decisions

- **Pure In-Process Domain Core**:
  - Consolidate pure Open Finance normalization (`NormalizeProviderTransaction`), Invoice cycle calculation (`ResolveInvoiceCycle`), provider Invoice status resolution (`ResolveProviderInvoiceStatus`), unified Invoice Settlement matching (`MatchInvoiceSettlement`, `ApplySettlement`, `ClearSettlement`), `IsLiquidReserve`, `IsInvoiceSettlement`, `IsCheckingInvoicePaymentDescription`, `ComputeLiquidAccountAnchorDelta`, and `ComputeCreditCardLiabilityDelta` inside the domain package with zero I/O or port imports.
  - Standardize the canonical Invoice Settlement matching window via `DefaultSettlementWindow` (`[-10, +20]` days around `DueDate`, expanded around an explicit payment date when supplied) and canonical payment keyword affinity (`FATURA`, `ITAU BLACK`, `ITAUCARD`, `UNICLASS`).
- **Repository Seam Extension**:
  - Extend `TransactionRepository` with `ReconcileExternalTransaction(ctx, tenantID, externalID, billID, status)` executed inside `WithTenantTx` so duplicate external transactions update their Invoice link (`bill_id`) and transition `PENDING` ↔ `COMPLETED` with `metadata.is_future` through the persistence port instead of raw `pgTx.Exec` SQL in the CLI.
  - Persist `bill_id` during initial transaction creation in the PostgreSQL repository adapter.
- **Deep Application Seam (`OpenFinanceSyncService`)**:
  - Define a minimal `OpenFinanceSyncService` port exposing `SyncItem(ctx, SyncItemRequest) (*SyncItemResult, error)`.
  - Implement `OpenFinanceSyncService` in the application module orchestrating: account matching/creation, Provider Balance Snapshot recording, Invoice upserts preserving prior `PAID` state, transaction normalization and cycle rollup, duplicate reconciliation via `FindExistingFingerprints` + `ReconcileExternalTransaction`, initial Liquid Account Ledger Balance anchoring, Invoice Settlement matching and credit card liability balance sync, and optional post-sync transfer/category Pending Action scanners.
- **Caller Migration & Contraction**:
  - Migrate `InvoiceService` (`GetInvoiceDetails`, `GetCashflowProjection`, `PayInvoice`, `UnpayInvoice`) and `TransferReconciliationService` to use the shared domain settlement functions and delete duplicated private settlement helpers.
  - Reduce the Open Finance sync CLI entrypoint to flag/env parsing, tenant/user discovery, adapter wiring, and invoking `OpenFinanceSyncService.SyncItem`.
  - Keep existing HTTP/JSON response field names (`bill_id`, `upcoming_bills`, `is_bill_payment`) intact to preserve backward compatibility while standardizing internal code and new interfaces on `Invoice`.

## Testing Decisions

- **Unit Tests (In-Process Seam)**: Test pure normalization, month-end Invoice cycle clamping, Brazil timezone date boundaries, settlement window matching with keyword prioritization, and balance delta calculations directly against domain functions using table-driven tests and zero mocks.
- **Integration-Style Behavioral Tests (Application Seam)**: Test `OpenFinanceSyncService.SyncItem` across its public port interface using an in-memory fake `OpenFinanceConnector` and the existing in-memory repository test adapters, verifying observable outputs (`AccountsSynced`, `InvoicesSettled`, account Ledger Balances, Invoice metadata, `PENDING -> COMPLETED` re-sync promotion, and emitted Pending Actions).
- **Regression Verification**: Retain and re-run existing `InvoiceService`, `TransferReconciliationService`, and CLI tests after migrating callers; assert zero skipped tests.

## Out of Scope

- Adding new non-Pluggy external banking HTTP adapters.
- Changing webhook transport or background task queue wiring (`Asynq` / `Cloud Tasks`).
- Touching parked modules listed in `.out-of-scope/README.md` (`Split`, `IngestionJob`, `OFX`/`Excel`/`PDF` parsers, `AIClassifier`).
- Renaming database columns or breaking existing JSON API payloads (`bill_id`, `upcoming_bills`).

## Further Notes

- Aligns with `ADR-010` (confirmed internal transfers & decimal precision) and `ADR-012` (Provider Balance Snapshot segregation & HITL Pending Action governance).
- Follows Workflow 3 Stage 6 (`Expand → Migrate → Contract` and *Replace, don't layer*).
