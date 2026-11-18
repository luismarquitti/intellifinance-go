# Agent Guidelines — IntelliFinance

Autonomous coding agent entry point for `luismarquitti/intellifinance-go`. Keep this file lean; follow the context pointers below when entering a task.

---

## Context Pointers

- **Review & Design Standards**: Read [CODING_STANDARDS.md](CODING_STANDARDS.md) before `/code-review`, refactoring modules, or adding new `internal/ports` / `internal/app` seams.
- **Domain Vocabulary**: Read [GLOSSARY.md](GLOSSARY.md) before naming domain types, handlers, specs, or issues; avoid its `_Avoid_` synonyms.
- **Architecture Decisions**: Read [docs/adrs/](docs/adrs/) (especially `ADR-003`, `ADR-010`, `ADR-011`, `ADR-012`) and [docs/agents/domain.md](docs/agents/domain.md) when modifying core flows.
- **Parked / Out-of-Scope Code**: Read [.out-of-scope/README.md](.out-of-scope/README.md) before touching unwired domain models (`Split`, `IngestionJob`), unmounted parsers (`OFX`/`Excel`/`PDF`), or `AIClassifier`.
- **Issue Tracker & Triage**: Read [docs/agents/issue-tracker.md](docs/agents/issue-tracker.md) and [docs/agents/triage-labels.md](docs/agents/triage-labels.md) before creating or triaging GitHub issues.

---

## Non-Negotiable Invariants

1. **Hexagonal Isolation**: `internal/domain` must never import `internal/adapters/...` or transport packages; HTTP handlers call `internal/ports`, never repositories or raw SQL directly.
2. **Financial Precision**: Never use `float32`/`float64` for money; use `decimal.Decimal` (`NUMERIC(14,2)`/`NUMERIC(15,2)`) and reject sub-cent inputs via `domain.IsRepresentableInCents`.
3. **Multi-Tenancy & RLS**: Every DB operation is scoped by `tenant_id` inside `postgres.WithTenantTx` (`SET LOCAL app.current_tenant_id`).
4. **Zero-Leak PII & Synthetic Fixtures**: Never commit real financial data, real CPFs, or secrets; use `cmd/fixturegen` or `test/fixtures/`.
5. **Dual-Target Profiles**: `ENVIRONMENT=development` targets homelab (`lm-claw` Postgres/Redis/LiteLLM); `ENVIRONMENT=production` targets GCP (Cloud Run, Cloud Tasks, Vertex AI); `ENVIRONMENT=personal` enforces loopback Basic/bcrypt fail-closed protection.

---

## Verification Gate

- Backend tests (zero skips allowed): `go test -v -race ./...` (or `make test` with isolated `*_test` DB)
- Backend linter: `golangci-lint run` (v2 config in `.golangci.yml`)
- Frontend build & typecheck: `npm --prefix web run build`
- Clean git diff, Conventional Commits (`feat:`, `fix:`, `refactor:`, `docs:`, `test:`), and `Closes #<issue_number>` in PRs.
