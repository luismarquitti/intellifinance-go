# 04: Migrate Invoice, Reconciliation, and CLI Callers (Contract Duplication)

**What to build:** Migrate `InvoiceService`, `TransferReconciliationService`, and the Open Finance sync CLI entrypoint onto the new domain and application seams, deleting duplicated settlement heuristics and raw CLI SQL.

**Blocked by:** 03 (Deep OpenFinanceSyncService Module)

**Status:** completed

- [x] `InvoiceService` delegates settlement detection, Liquid Account selection, settlement matching, and metadata clearing to the unified domain module; redundant private helpers are deleted
- [x] `TransferReconciliationService` aligns credit card Invoice Settlement matching with the unified domain rules
- [x] `cmd/sync-pluggy` delegates synchronization and reconciliation to `OpenFinanceSyncService.SyncItem` with zero raw SQL or `pgx` transaction queries
- [x] Full unit, build, and `go vet` verification passes alongside two-axis `/code-review`
