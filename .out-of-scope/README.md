# Out-of-Scope & Parked Artifacts

Code discovered during Workflow 3 semantic archaeology that exists in the repository with tests or port declarations, but is **not wired into active runtime execution** (`cmd/api`, `cmd/mcp`, `cmd/sync-pluggy`, or `cmd/import-ledger`).

Do not delete these files prematurely without a user decision; keep them out of active refactoring scope unless explicitly scheduled.

| Artifact | Status & Reason Parked | Roadmap Reference |
| --- | --- | --- |
| `internal/domain/split.go` (`Split`, `ValidateSplits`), `ports.TransactionRepository.CreateSplits` / `GetSplits`, `domain.ActionTypeSplitTransaction` | Household co-budgeting splits are implemented at the domain/repository layer but have no application workflow, HTTP handler, or UI consumer yet. | Issue #16 (Co-Orçamento Familiar) |
| `internal/domain/ingestion_job.go` (`IngestionJob`), `internal/ports/ingestion_job_repository.go`, `internal/adapters/outbound/persistence/postgres/db/ingestion_jobs.sql.go` | Asynchronous file ingestion job tracking awaits the background queue worker (`Asynq` / `Cloud Tasks`). | Issue #15 (Worker Assíncrono) |
| `internal/adapters/outbound/parsers/{ofx_parser.go, excel_parser.go, pdf_parser.go}` | Standalone statement parsers tested in `parsers_test.go`, not wired into `cmd/api` (`CSVIngestionService` only wires `NewCSVParser()`). | Phase 2/4 Multi-Format Ingestion |
| `internal/ports/ai_classifier.go` (`AIClassifier`) | Outbound AI classification port with zero concrete adapters; active categorization uses deterministic heuristics in `app.CategoryMatcher` (`ports.CategoryRuleService`). | Issues #21–#24 (Vertex AI / LiteLLM) |
