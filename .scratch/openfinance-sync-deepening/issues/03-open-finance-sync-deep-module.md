# 03: Deep OpenFinanceSyncService Module

**What to build:** Introduce the `OpenFinanceSyncService` application port and deep module orchestrating account matching, Provider Balance Snapshots, Invoice cycle upserts, transaction ingestion/deduplication, Ledger Balance anchoring, Invoice Settlement reconciliation, and post-sync HITL Pending Action scans.

**Blocked by:** 01, 02

**Status:** completed

- [x] Application port exposes a minimal `SyncItem` interface returning structured sync and reconciliation metrics
- [x] Implementation orchestrates all sync steps deterministically using domain rules and injected repository/connector ports with explicit error handling
- [x] Behavioral tests exercise `SyncItem` through its public interface using a fake connector and in-memory repositories (covering initial sync, settlement matching, idempotent re-sync `PENDING -> COMPLETED`, and HITL Pending Action emission)
