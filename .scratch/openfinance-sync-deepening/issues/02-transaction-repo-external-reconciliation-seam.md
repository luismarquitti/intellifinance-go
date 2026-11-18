# 02: Transaction Repository External Reconciliation Seam

**What to build:** Extend the transaction persistence port and PostgreSQL/in-memory adapters so external transactions persist their Invoice reference on creation and reconcile Invoice links and `PENDING` ↔ `COMPLETED` transitions without raw SQL in callers.

**Blocked by:** 01 (Domain Invoice Settlement, Normalization, and Cycle Rules)

**Status:** completed

- [x] Transaction repository port exposes `ReconcileExternalTransaction` scoped by tenant and external identifier
- [x] PostgreSQL adapter persists `bill_id` when creating transactions and executes `ReconcileExternalTransaction` inside `WithTenantTx`
- [x] In-memory test transaction repository adapter implements `ReconcileExternalTransaction` with matching balance delta semantics on status transitions
