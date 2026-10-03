# ADR-003: Hexagonal Architecture (Ports and Adapters) for Dual-Target Infrastructure

- **Status**: Accepted
- **Deciders**: Software Architect (Worker M3), Principal Engineer, Systems Architect
- **Date**: 2026-09-24
- **Technical Story**: Milestone 3 - Architecture Decision Records

---

## 1. Context & Problem Statement

IntelliFinance is required to execute seamlessly across two fundamentally divergent deployment environments:
1. **Homelab Bare-Metal Dev (`lm-claw` / `lm-core`)**: Self-hosted on local physical hardware (Dell OptiPlex 3070, Debian 13 Trixie). Operates with local PostgreSQL 15, persistent Redis 7 for Asynq workers, and local AI engines (LiteLLM proxy on `:4000` and Ollama on `:11434`). Runs 24/7 with zero cloud dependency and zero incremental dollar cost.
2. **Google Cloud Platform (GCP) Serverless Production**: Cloud Run containerized deployment, managed serverless PostgreSQL, Google Cloud Tasks push webhooks ($0 idle cost scaling to zero), Google Secret Manager, and Vertex AI Gemini 2.0 Flash consuming GenAI App Builder credits.

In traditional N-tier or framework-centric architectures (such as the legacy NestJS/Prisma prototype), business logic is tightly bound to database ORM models, framework decorators, and specific messaging drivers. If domain logic directly imports the GCP Cloud Tasks SDK, it cannot run in the local Homelab without complex emulation. Conversely, if business use cases are tightly coupled to a persistent Redis connection, the application cannot scale to zero on Google Cloud Run without incurring expensive managed Redis charges ($35–$50/month).

IntelliFinance requires an architectural pattern that completely isolates enterprise financial business rules from delivery mechanisms and cloud infrastructure, enabling zero-code-change driver swapping driven purely by runtime configuration.

---

## 2. Decision Drivers

- **Zero-Coupling Domain Invariant**: Core financial domain models (`Account`, `Transaction`, `Category`, `IngestionJob`) and business rules must have zero dependencies on external frameworks, databases, or cloud SDKs.
- **Dual-Target Pluggability**: Infrastructure drivers (PostgreSQL vs SQLite, Asynq/Redis vs GCP Cloud Tasks, Vertex AI vs LiteLLM/Ollama, Pluggy vs Mock banking) must be interchangeable via configuration flags without touching domain code.
- **Independent Testability**: Business rules must be 100% unit-testable in memory using simple mock or fake implementations of ports, requiring neither Docker containers nor live cloud credentials.
- **Clear Separation of Driving vs Driven Actors**: Distinct architectural boundaries separating inbound delivery triggers (HTTP requests, queue consumers, CLI commands) from outbound external service interactions (databases, cloud tasks, AI gateways).

---

## 3. Considered Options

1. **Layered / N-Tier Architecture (Controller -> Service -> Repository)**: Traditional enterprise structure where services directly instantiate or depend on concrete repository and client implementations.
2. **Clean / Onion Architecture**: Concentric rings with domain at center and outer dependency rules.
3. **Hexagonal Architecture (Ports and Adapters / Cockburn)**: Decouples inside (application domain) from outside (infrastructure) via explicitly declared primary (driving) and secondary (driven) interfaces. (Selected)
4. **Framework-Coupled / Active Record**: Direct coupling of business entities to persistence frameworks (e.g. GORM or Prisma models acting as domain entities).

---

## 4. Evaluation & Comparative Matrix

| Evaluation Criteria | Hexagonal Architecture (Selected) | Layered / N-Tier | Clean / Onion | Active Record (Framework) |
|---|---|---|---|---|
| **Domain Purity** | **100% Pure Go** (No SDK imports) | Leaky (DB tags, ORM models) | **100% Pure** | Poor (Tightly coupled to ORM) |
| **Dual-Target Swapping** | **Trivial** (Swap concrete adapters at startup) | Difficult (Services coupled to drivers) | **Trivial** | Impossible without refactoring |
| **Testability without DB** | **Instant** (Mock ports via pure Go) | Requires DB mock or test containers | **Instant** | Requires in-memory DB or SQLite |
| **Architectural Clarity** | Explicit Driving vs Driven boundary | Ambiguous boundary definitions | Can be overly abstract | High initial speed, zero long-term modularity |
| **Boilerplate Overhead** | Low-Moderate (Port interfaces + mapper) | Low | Moderate-High | Minimal initially, heavy tech debt later |
| **Enforcement via Go Compiler**| Strict (Package boundaries prevent leaks) | Leaky package imports common | Strict | Leaky |

---

## 5. Decision Outcome

**Adopt Hexagonal Architecture (Ports and Adapters)** as the governing design paradigm for the IntelliFinance greenfield platform.

### Architectural Blueprint:

```
                      OUTSIDE (Infrastructure & Delivery)
 +-----------------------------------------------------------------------------+
 |                                                                             |
 |   [ DRIVING ADAPTERS (Inbound) ]                                            |
 |   - Chi HTTP REST API Handlers                                              |
 |   - Cloud Tasks Webhook Receiver (/internal/tasks/*)                        |
 |   - Homelab Asynq Worker Daemon Consumer                                    |
 |   - CLI Fixture Generator (cmd/fixturegen)                                  |
 |                                                                             |
 +--------------------------------------┬--------------------------------------+
                                        │ (invokes)
                                        ▼
 +-----------------------------------------------------------------------------+
 |   PRIMARY / DRIVING PORTS (Application Use Cases)                           |
 |   - AccountUseCase (CreateAccount, GetBalance, Reconcile)                   |
 |   - TransactionUseCase (RecordTransaction, ListTransactions, SplitExpense)  |
 |   - IngestionUseCase (SubmitFile, ExecuteJob, RetryJob)                     |
 |   - CategorizationUseCase (PredictCategory, AuditCorrection)                |
 +--------------------------------------┬--------------------------------------+
                                        │
                                        ▼
 +-----------------------------------------------------------------------------+
 |   CORE APPLICATION & DOMAIN LAYER (internal/domain & internal/app)          |
 |   - Pure Entities: Account, Transaction, Category, IngestionJob, Tenant     |
 |   - Value Objects: Money (Decimal), Currency, ConfidenceScore               |
 |   - Domain Rules: Balance calculations, Deduplication Fingerprinting        |
 |   - Zero External Dependencies (Only stdlib, uuid, decimal)                 |
 +--------------------------------------┬--------------------------------------+
                                        │
                                        ▼
 +-----------------------------------------------------------------------------+
 |   SECONDARY / DRIVEN PORTS (Outbound Contracts)                             |
 |   - ports.Repository (Persistence operations with RLS context)              |
 |   - ports.TaskQueue (Asynchronous background task dispatching)              |
 |   - ports.CategorizationEngine (AI category prediction & reasoning)         |
 |   - ports.DocumentParserEngine (Raw file stream to transactions)            |
 |   - ports.OpenFinanceConnector (Bank connection & transaction sync)         |
 |   - ports.SecretManager (Secure credential retrieval)                       |
 +--------------------------------------┬--------------------------------------+
                                        │ (implemented by)
                                        ▼
 +-----------------------------------------------------------------------------+
 |                                                                             |
 |   [ DRIVEN ADAPTERS (Outbound) ]                                            |
 |   - PostgreSQL Adapter (sqlc v2 + pgx/v5 with RLS)                          |
 |   - Queue Adapter: Asynq (Redis/Homelab) OR Cloud Tasks (GCP Prod)          |
 |   - AI Adapter: Vertex AI Gemini 2.0 OR LiteLLM/Ollama (Homelab)            |
 |   - Ingestion Parsers: OFX (SGML/XML), CSV (Multi-Dialect), PDF (Multimodal)|
 |   - Open Finance Adapter: PluggyClient (Sandbox / Live)                     |
 |                                                                             |
 +-----------------------------------------------------------------------------+
                      OUTSIDE (Infrastructure & External Systems)
```

---

## 6. Port Interface Contracts (`internal/ports/`)

The application defines clean interface contracts separating intent from implementation:

### 6.1 Primary / Driving Ports (Use Cases in `internal/ports/inbound.go`)
```go
package ports

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
	"intellifinance/internal/domain"
)

type AccountUseCase interface {
	CreateAccount(ctx context.Context, tenantID, userID uuid.UUID, name string, accType domain.AccountType, visibility domain.Visibility) (*domain.Account, error)
	ListAccounts(ctx context.Context, tenantID, userID uuid.UUID) ([]*domain.Account, error)
	GetAccountByID(ctx context.Context, tenantID, accountID uuid.UUID) (*domain.Account, error)
}

type IngestionUseCase interface {
	SubmitStatement(ctx context.Context, tenantID, accountID uuid.UUID, filename, mimeType string, content io.Reader) (*domain.IngestionJob, error)
	ProcessJob(ctx context.Context, jobID uuid.UUID) error
	GetJobStatus(ctx context.Context, tenantID, jobID uuid.UUID) (*domain.IngestionJob, error)
}

type TransactionUseCase interface {
	RecordManual(ctx context.Context, tenantID, userID uuid.UUID, tx *domain.Transaction) (*domain.Transaction, error)
	ListTransactions(ctx context.Context, tenantID, userID uuid.UUID, filter domain.TransactionFilter) ([]*domain.Transaction, int64, error)
	OverrideCategory(ctx context.Context, tenantID, userID, txID, categoryID uuid.UUID) error
}
```

### 6.2 Secondary / Driven Ports (`internal/ports/repository.go` & `queue.go`)
```go
package ports

import (
	"context"
	"time"

	"github.com/google/uuid"
	"intellifinance/internal/domain"
)

// Repository defines database operations. Adapters implement this using sqlc and pgx.
type Repository interface {
	WithTx(ctx context.Context, fn func(r Repository) error) error
	SetTenantContext(ctx context.Context, tenantID, userID uuid.UUID) context.Context

	CreateAccount(ctx context.Context, acc *domain.Account) error
	GetAccountByID(ctx context.Context, id uuid.UUID) (*domain.Account, error)
	ListAccountsByTenant(ctx context.Context, tenantID uuid.UUID) ([]*domain.Account, error)

	BatchCreateTransactions(ctx context.Context, txs []*domain.Transaction) error
	FindExistingFingerprints(ctx context.Context, tenantID uuid.UUID, fingerprints []string) (map[string]bool, error)
	ListTransactions(ctx context.Context, filter domain.TransactionFilter) ([]*domain.Transaction, int64, error)

	CreateIngestionJob(ctx context.Context, job *domain.IngestionJob) error
	GetIngestionJobByID(ctx context.Context, id uuid.UUID) (*domain.IngestionJob, error)
	UpdateIngestionJobStatus(ctx context.Context, id uuid.UUID, status domain.IngestionStatus, summary *string, errDetails *domain.ErrorDetails) error
}

// TaskQueue abstracts asynchronous dispatching.
type TaskQueue interface {
	Enqueue(ctx context.Context, task Task) error
}

type Task struct {
	ID        string
	Type      string
	Payload   []byte
	Headers   map[string]string
	Delay     time.Duration
	MaxRetry  int
}
```

---

## 7. Zero Framework Pollution Invariant

To guarantee architectural integrity:
1. **Rule 1**: Packages under `internal/domain` must NEVER import `net/http`, `github.com/go-chi/chi`, `github.com/jackc/pgx`, `cloud.google.com/go`, or any database/cloud library.
2. **Rule 2**: Only primitive types, standard library packages (`time`, `context`, `errors`), and approved mathematical/UUID value object libraries (`github.com/google/uuid`, `github.com/shopspring/decimal`) are permitted in the domain layer.
3. **Rule 3**: Inward Dependency Direction — Adapters depend on Ports; Ports depend on Domain; Domain depends on nothing.

---

## 8. Composition Root & Dependency Injection

The application uses pure Go composition roots without dynamic reflection-based DI containers (such as Uber Dig or Fx). Dependencies are assembled explicitly at startup in `cmd/api/main.go`:

```go
package main

import (
	"context"
	"log/slog"
	"os"

	"intellifinance/internal/adapters/inbound/http"
	"intellifinance/internal/adapters/outbound/ai"
	"intellifinance/internal/adapters/outbound/postgres"
	"intellifinance/internal/adapters/outbound/queue"
	"intellifinance/internal/app"
	"intellifinance/internal/ports"
)

func main() {
	ctx := context.Background()
	cfg := loadConfig()

	// 1. Initialize Persistence Adapter (PostgreSQL with pgxpool)
	repo := postgres.NewRepository(cfg.DatabaseURL)

	// 2. Initialize Queue Adapter based on environment
	var taskQueue ports.TaskQueue
	if cfg.Environment == "production" {
		taskQueue = queue.NewCloudTasksAdapter(cfg.GCPProjectID, cfg.GCPLocation, cfg.CloudTasksQueueID)
	} else {
		taskQueue = queue.NewAsynqQueueAdapter(cfg.RedisURL)
	}

	// 3. Initialize AI Adapter based on configuration
	var aiEngine ports.CategorizationEngine
	if cfg.AIProvider == "vertexai" {
		aiEngine = ai.NewVertexAIAdapter(ctx, cfg.GCPProjectID, cfg.GCPLocation)
	} else {
		aiEngine = ai.NewLiteLLMAdapter(cfg.LiteLLMBaseURL)
	}

	// 4. Assemble Application Services (Use Cases)
	ingestionService := app.NewIngestionService(repo, taskQueue, aiEngine)
	accountService := app.NewAccountService(repo)
	transactionService := app.NewTransactionService(repo, aiEngine)

	// 5. Mount Inbound HTTP Transport (Chi Router)
	router := http.NewRouter(accountService, transactionService, ingestionService)
	http.Serve(cfg.Port, router)
}
```

---

## 9. Consequences

### Positive Consequences
- **Total Infrastructure Agnosticism**: Switching from Homelab bare-metal to GCP Serverless requires zero modifications to core domain logic.
- **Superior Testability**: Unit tests run in milliseconds using in-memory mock implementations of `ports.Repository` and `ports.TaskQueue`.
- **Architectural Boundary Enforcement**: Package boundaries prevent accidental database or HTTP calls from leaking into business calculations.
- **Resilience Against Upstream Changes**: When third-party APIs (e.g. Pluggy or Vertex AI) evolve their SDKs, only the respective adapter in `internal/adapters/outbound/` requires updates.

### Negative Consequences
- **Adapter Boilerplate**: Requires defining explicit interfaces in `internal/ports/` and translation structs between domain models and database models.
- **Initial Setup Investment**: Slightly more upfront structure compared to building rapid prototypes directly coupled to an ORM.

### Neutral Consequences
- All team members must adhere to dependency flow rules enforced via code reviews and linting checks.

---

## 10. Implementation & Verification Plan

1. **Verify Dependency Direction**:
   - Enforce package import boundaries using `go-cleanarch` or custom AST linter in CI:
     ```bash
     # Verify internal/domain contains no adapter or third-party cloud imports
     go list -f '{{.ImportPath}} -> {{.Imports}}' ./internal/domain/...
     ```
2. **Unit Test Verification**:
   - Create unit tests for `app.IngestionService` using mocked `ports.Repository` and `ports.TaskQueue` without running PostgreSQL or Redis.
3. **Dual-Driver Verification**:
   - Verify that setting `QUEUE_DRIVER=asynq` initializes the Redis driver, while `QUEUE_DRIVER=cloudtasks` initializes the Google Cloud Tasks driver.
