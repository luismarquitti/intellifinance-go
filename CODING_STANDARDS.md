# Coding Standards — IntelliFinance

Review-time engineering standards enforced during `/code-review` and PR audits.

---

## 1. Hexagonal Seam Discipline

- **Domain Isolation**: `internal/domain` must contain pure Go domain models and invariants (`stdlib`, `google/uuid`, `shopspring/decimal` only). Never import `internal/adapters/...`, `internal/ports`, SQL drivers, or HTTP frameworks.
- **Inbound Handlers**: Handlers under `internal/adapters/inbound/http/handlers` parse HTTP payloads, validate DTOs, invoke application ports in `internal/ports`, and render RFC 7807 `problem` responses. Handlers must never query `*Repository` interfaces or SQL pools directly.
- **Persistence & Provenance Guards**: Outbound PostgreSQL adapters execute inside `postgres.WithTenantTx` so `SET LOCAL app.current_tenant_id` enforces Row-Level Security (RLS). Never bypass repository guards with ad-hoc `pgTx.Exec` updates in `cmd/` or inbound adapters.

---

## 2. Deep Module Design (`/codebase-design`)

- **Prefer Deep Modules over Pass-Throughs**: Concentrate non-trivial orchestration, invariants, and heuristics behind small interfaces. Avoid adding new 1:1 CRUD pass-through wrappers that fail the deletion test.
- **Two-Adapter Rule for Seams**: Introduce a `ports` interface only when at least two concrete adapters exist or are genuinely required across a seam (e.g., production PostgreSQL/Pluggy + test stand-in). Single-implementation pass-through interfaces add indirection without leverage.
- **Canonical Vocabulary**: Name domain concepts strictly using [GLOSSARY.md](GLOSSARY.md) and architecture elements using `module`, `interface`, `implementation`, `depth`, `seam`, `adapter`, `leverage`, and `locality`.

---

## 3. Financial Precision, Determinism & Concurrency

- **Zero Floats for Currency**: Use `github.com/shopspring/decimal` (`decimal.Decimal`) in Go and `NUMERIC(14, 2)` / `NUMERIC(15, 2)` in PostgreSQL. Reject sub-cent values (`!domain.IsRepresentableInCents(amount)` -> HTTP 422 / `domain.ErrInvalidAmount`).
- **Brazil Timezone Anchoring**: Format transaction calendar dates through `domain.StandardizeDateToBrazil` (`America/Sao_Paulo`) before deduplication fingerprinting or cycle matching.
- **Anti-Deadlock Lock Ordering (ADR-010)**: When locking multiple accounts or transaction legs inside a PostgreSQL transaction, always acquire row locks in deterministic ascending UUID order (`ORDER BY id ASC FOR NO KEY UPDATE` or `FOR UPDATE`).
- **Immutable Provenance (ADR-011)**: Canonical CSV ingestion must retain exact original bytes before ledger insertion and preserve `metadata.provenance` against generic updates.

---

## 4. HITL Governance & Error Handling

- **No Autonomous Ledger Mutation (ADR-012)**: AI/MCP tools (`cmd/mcp`) and automated scanners (`CategoryMatcher`, `TransferReconciliationService`) must propose changes as `domain.PendingAction` (`PENDING`) rather than mutating balances, categories, or transfers directly.
- **Explicit Error Propagation**: Never discard persistence or lookup errors (`_ = ...`) on state-changing execution paths (such as `PendingAction` approval, invoice settlement, or balance anchoring).
- **Consistent Domain Error Mapping**: Map domain sentinel errors (`domain.ErrNotFound`, `domain.ErrInvalidAmount`, transfer conflict errors) to RFC 7807 HTTP status codes via `internal/adapters/inbound/http/problem`.

---

## 5. Verification & Test Hygiene

- **Zero Skipped Tests**: Never call `t.Skip` / `t.Skipf`. The CI and `make test` pipeline enforces zero skipped tests via `scripts/check-test-results.py`.
- **Test Across Public Seams**: Write tests against the module's public interface using real invariants or synthetic fixtures (`cmd/fixturegen`, `test/fixtures/`). Avoid testing internal private helpers when behavior is observable through the module interface.
- **Zero-Leak PII**: Never commit real statements, real CPFs, real account numbers, or credentials; every test fixture must be synthetic.
