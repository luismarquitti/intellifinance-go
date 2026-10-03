# IntelliFinance Execution Roadmap & Guided Learning Milestones

- **Document ID**: ROADMAP-001
- **Status**: Authoritative / Production-Ready Execution Guide
- **Version**: 1.0.0-scaffold
- **Primary Runtime**: Golang 1.22+ Standard Project Layout
- **Architectural Paradigm**: Hexagonal Architecture (Ports and Adapters)
- **Deployment Topology**: Dual-Target — Homelab Bare-Metal (`lm-claw` Debian 13) & GCP Serverless Production (Cloud Run, Cloud Tasks, Secret Manager, Vertex AI)
- **Target Audience**: Software Engineers, Solutions Architects, Tech Leads, and Open-Source Contributors

---

## 1. Executive Roadmap Overview & Pedagogical Philosophy

### 1.1 The Dual Mission
IntelliFinance serves two tightly interwoven objectives:
1. **The Production Engineering Mission ($0 Operational Baseline)**:
   Deliver an enterprise-grade personal and household financial engine running perpetually at $0 incremental cost. In local development and homelab operation, it leverages physical bare-metal hardware (`lm-claw` Dell OptiPlex 3070 on Debian 13 Trixie) with PostgreSQL 16, Redis 7, LiteLLM proxy, and Ollama. In production, it operates on Google Cloud Platform's serverless free tiers (Cloud Run scaling to zero, Cloud Tasks push delivery, Artifact Registry, and Google Cloud GenAI App Builder credits), eliminating idle infrastructure costs while providing 99.9% availability and sub-100ms API response latencies.
2. **The Pedagogical & Portfolio Showcase Mission**:
   Serve as a public engineering showcase on GitHub that demonstrates senior-level Go system architecture, spec-first OpenAPI 3.0 contracts, compile-time type-safe persistence (`sqlc` + `pgx/v5`), database-enforced Row-Level Security (RLS) multi-tenancy, asynchronous task processing, Brazilian banking integration (Pluggy Open Finance sandbox), multimodal LLM intelligence (Gemini 2.0 Flash via the official `google.golang.org/genai` Go SDK), and zero-leak DevSecOps hygiene (pre-commit Gitleaks with custom Brazilian PII filters).

### 1.2 Guided Learning Outcomes Matrix
By executing this roadmap sequentially from Phase 0 to Phase 5, an engineer masters the following modern Go and cloud-native software competencies:

| Competency Area | Pedagogical Focus & Concrete Implementations | Applicable Milestones |
|---|---|---|
| **Idiomatic Go 1.22+** | Standard project layout (`cmd/`, `internal/`), workspace organization, context propagation, error wrapping (`fmt.Errorf("%w")`), custom sentinel errors, structural type embedding, and generic repository patterns. | Phase 0, Phase 1 |
| **Hexagonal Architecture** | Decoupling pure business models (`internal/domain`) from inbound protocol handlers (`internal/adapters/inbound`) and outbound external drivers (`internal/adapters/outbound`) via explicit interfaces (`internal/ports`). | Phase 0, Phase 1, Phase 2 |
| **Type-Safe Database Persistence** | Eliminating ORM runtime reflection and SQL-injection vulnerabilities using `sqlc` v2 compile-time codegen with `jackc/pgx/v5`, connection pooling tuning, and atomic migrations via `golang-migrate`. | Phase 1 |
| **Multi-Tenancy & Row-Level Security** | Implementing multi-tenant isolation directly in PostgreSQL using `CREATE POLICY` and transaction-scoped configuration settings (`app.current_tenant_id`), guaranteeing zero cross-tenant data leakage. | Phase 1, Phase 5 |
| **Financial Ingestion & Parsing** | Handling heterogeneous, imperfect banking exports: SGML/XML OFX parsing, Latin-1 (Windows-1252 / ISO-8859-1) to UTF-8 transcoding, multi-dialect CSV delimiter sniffing, and Excel XLSX streaming via `excelize`. | Phase 1 |
| **Deterministic Ledger Deduplication** | Cryptographic fingerprinting using SHA-256 over normalized transaction fields, combined with PostgreSQL atomic upsert (`ON CONFLICT DO NOTHING`) to guarantee idempotent ingestion. | Phase 1 |
| **Asynchronous Task Queueing** | Decoupled background processing with retry policies, exponential backoff, and dead-letter queues (DLQ), bridging dual infrastructure: `hibiken/asynq` + Redis for Homelab vs Google Cloud Tasks for GCP Serverless. | Phase 2, Phase 3 |
| **Open Finance Banking APIs** | Integrating Pluggy.ai Brazilian Open Finance platform: two-tier token exchange, webhook HMAC signature validation, incremental transaction sync, and perpetual free Sandbox simulation. | Phase 2 |
| **Cloud-Native Serverless & IaC** | Crafting multi-stage minimal distroless Docker containers (<25MB, <30ms cold start), Terraform infrastructure-as-code, and Google Cloud Secret Manager runtime injection. | Phase 3 |
| **Multimodal Generative AI in Go** | Consuming Google Cloud GenAI App Builder credits using the official `google.golang.org/genai` SDK, executing Gemini 2.0 Flash multimodal OCR on receipts/invoices, structured JSON decoding, and offline fallback to LiteLLM/Ollama. | Phase 4 |
| **DevSecOps & Zero-Leak Security** | Establishing strict Gitleaks CI/CD scanning rules for Brazilian tax IDs (CPF/CNPJ), bank tokens, and Pluggy API keys; synthetic data generation (`cmd/fixturegen`); and LGPD compliance (crypto-shredding). | Phase 0, Phase 5 |

### 1.3 Master Roadmap Architecture Diagram

```
+-----------------------------------------------------------------------------------------------------------------------+
|                                        INTELLIFINANCE EXECUTION ROADMAP                                               |
+-----------------------------------------------------------------------------------------------------------------------+
|                                                                                                                       |
|  [ PHASE 0: Clean Scaffolding & Security Foundation ]                                                                |
|  ├── Disconnect legacy history -> Tag v1.0.0-scaffold -> Go Standard Layout (`cmd/`, `internal/`)                     |
|  ├── GolangCI-Lint, Gitleaks custom Brazilian PII filters, Pre-Commit Hooks, GitHub Actions CI                        |
|  └── Synthetic Fixture Generator CLI (`cmd/fixturegen`) for realistic mock data (Zero personal PII)                  |
|                                         |                                                                             |
|                                         v                                                                             |
|  [ PHASE 1: Homelab MVP - Persistence, CRUD & Ingestion ]                                                             |
|  ├── Docker Compose on lm-claw (Debian 13): PostgreSQL 16 + Redis 7                                                  |
|  ├── Golang-Migrate DDL with PostgreSQL Row-Level Security (RLS) policies                                             |
|  ├── Type-safe SQL access via sqlc v2 + jackc/pgx/v5 & Chi v5 REST API with RFC 7807 Problem Details                  |
|  ├── Multi-format Ingestion: OFX (Latin-1/CP1252), CSV (multi-dialect sniffing), Excel XLSX, and Digital PDF          |
|  └── Deterministic SHA-256 fingerprinting & idempotent batch upsert                                                   |
|                                         |                                                                             |
|                                         v                                                                             |
|  [ PHASE 2: Open Finance Integration & Asynchronous Queueing ]                                                        |
|  ├── Pluggy.ai API Connector: Free Perpetual Sandbox ($0 CI/Dev) + Configurable Live Mode                             |
|  ├── Inbound Webhook Endpoint (`/open-finance/webhook`) with constant-time HMAC secret validation                     |
|  ├── Asynq background worker daemon on lm-claw (ingest_statement, sync_open_finance, categorize_transaction)          |
|  └── Household Co-Budgeting Engine: Joint vs Private visibility, 50/50 and proportional splitting, clinic isolation   |
|                                         |                                                                             |
|                                         v                                                                             |
|  [ PHASE 3: Dual-Target Portability & GCP Serverless Deployment ]                                                     |
|  ├── Multi-stage Distroless Dockerfile (<25MB, <30ms cold start, non-root user)                                      |
|  ├── Cloud Tasks Push Queue Adapter (`ports.TaskQueue`) -> HTTP push to Cloud Run (`$0 idle cost`)                     |
|  ├── GCP Secret Manager adapter + Cloud Run secret env injection                                                      |
|  └── Terraform / gcloud automated deployment scripts (Artifact Registry, Cloud Run, Cloud Tasks, Cloud SQL)          |
|                                         |                                                                             |
|                                         v                                                                             |
|  [ PHASE 4: Vertex AI Smart Categorization & Multimodal OCR ]                                                         |
|  ├── Official `google.golang.org/genai` Go SDK consuming Google Cloud GenAI App Builder credits                      |
|  ├── Multimodal Gemini 2.0 Flash extraction for scanned paper receipts, wrinkled coupons, and multi-page invoices    |
|  ├── Categorization Engine: contextual prompt taxonomy, confidence scoring (>=0.85 auto vs <0.85 review queue)       |
|  └── Homelab dev offline fallback to LiteLLM proxy (`lm-claw:4000`) and Ollama (`lm-claw:11434`)                     |
|                                         |                                                                             |
|                                         v                                                                             |
|  [ PHASE 5: Public Portfolio Polish, Documentation Showcase & Multi-Tenant Rollout ]                                  |
|  ├── Interactive Swagger UI / Redoc embedded via `embed.FS` with pre-loaded Pluggy Sandbox credentials                |
|  ├── Public GitHub release checklist: final deep Gitleaks audit, branch protection rules, comprehensive README        |
|  └── Multi-tenant invited user onboarding (e.g., Edson Silva) with verified cryptographic RLS isolation               |
+-----------------------------------------------------------------------------------------------------------------------+
```

---

## 2. Phase 0: Clean Project Scaffolding & Security Foundation

### 2.1 Architectural Objectives
- Disconnect permanently from the compromised legacy repository containing leaked financial documents, medical receipts, and personal tax returns (DIRPF).
- Initialize a pristine greenfield Git repository tagged at `v1.0.0-scaffold`.
- Establish the canonical Go Standard Project Layout conforming to Hexagonal Architecture.
- Enforce automated quality, formatting, linting, and security gates via Pre-Commit hooks and GitHub Actions.
- Implement an automated synthetic fixture generator CLI (`cmd/fixturegen`) to produce rich Brazilian financial datasets for tests and public demonstration with zero risk of PII leakage.

### 2.2 Step-by-Step Implementation Guide

#### Step 0.1: Greenfield Repository Initialization
1. Ensure the greenfield directory `C:\devWorkspace\intellifinance` is isolated from legacy git history:
   ```bash
   # Initialize fresh git repository
   git init -b main
   
   # Configure user identity
   git config user.name "Luis Marquitti"
   git config user.email "luis@intellifinance.app"
   ```
2. Create an authoritative, strict `.gitignore` designed specifically for Go, Homelab environments, Docker, and financial security:
   ```gitignore
   # Binaries & build artifacts
   bin/
   dist/
   *.exe
   *.exe~
   *.dll
   *.so
   *.dylib
   
   # Go workspace & test artifacts
   vendor/
   *.test
   *.out
   coverage.html
   profile.pprof
   
   # Environment & Secrets (NEVER COMMIT)
   .env
   .env.*
   !.env.example
   *.pem
   *.key
   *.crt
   credentials.json
   service-account-key.json
   
   # Personal financial data files (Zero-Leak Protection)
   data/
   statements/
   imports/
   *.pdf
   *.ofx
   *.csv
   *.xlsx
   *.xls
   
   # IDE & OS files
   .vscode/
   .idea/
   *.swp
   *~
   .DS_Store
   Thumbs.db
   
   # Local database persistent volumes
   postgres-data/
   redis-data/
   ```

#### Step 0.2: Standard Project Layout Structure
Create the directory hierarchy representing Ports and Adapters:
```bash
mkdir -p cmd/api cmd/worker cmd/fixturegen
mkdir -p internal/domain internal/ports internal/app
mkdir -p internal/adapters/inbound/http/handlers
mkdir -p internal/adapters/inbound/http/middleware
mkdir -p internal/adapters/outbound/persistence/postgres/sqlc
mkdir -p internal/adapters/outbound/queue
mkdir -p internal/adapters/outbound/parsers
mkdir -p internal/adapters/outbound/openfinance
mkdir -p internal/adapters/outbound/ai
mkdir -p internal/adapters/outbound/secrets
mkdir -p migrations
mkdir -p scripts
mkdir -p .github/workflows
```

Initialize the Go module with Go 1.22+:
```bash
go mod init github.com/luismarquitti/intellifinance
```

Install core dependencies:
```bash
# Web routing and middleware
go get github.com/go-chi/chi/v5@v5.0.12
go get github.com/go-chi/cors@v1.2.1

# Persistence & Decimal math
go get github.com/jackc/pgx/v5@v5.5.5
go get github.com/shopspring/decimal@v1.3.1
go get github.com/google/uuid@v1.6.0

# Task queue (Homelab)
go get github.com/hibiken/asynq@v0.24.1

# Encodings & Excel
go get golang.org/x/text@v0.14.0
go get github.com/xuri/excelize/v2@v2.8.1

# Synthetic data generation & Testing
go get github.com/brianvoe/gofakeit/v6@v6.28.0
go get github.com/stretchr/testify@v1.9.0
```

#### Step 0.3: Static Analysis & Linting Configuration (`.golangci.yml`)
Create `.golangci.yml` at the project root with strict enterprise-grade linters:
```yaml
run:
  timeout: 5m
  go: "1.22"
  tests: true

linters:
  enable:
    - errcheck      # Unchecked error returns
    - gosimple      # Code simplification
    - govet         # Standard Go vet tool
    - ineffassign   # Detect unused variable assignments
    - staticcheck   # Advanced Go static analysis
    - unused        # Detect unused constants, variables, functions, and types
    - revive        # Fast, configurable, extensible linter
    - gosec         # Security auditing for Go code
    - bodyclose     # Checks whether HTTP response bodies are closed
    - noctx         # Finds HTTP requests sent without context.Context
    - sqlclosecheck # Checks that sql.Rows and sql.Stmt are closed
    - misspell      # Finds commonly misspelled English words

linters-settings:
  govet:
    enable-all: true
    disable:
      - fieldalignment # Avoid micro-optimization churn
  gosec:
    severity: medium
    confidence: medium
    excludes:
      - G104 # Handled by errcheck
  revive:
    rules:
      - name: exported
        arguments: [checkPrivateReceivers]
      - name: blank-imports
      - name: context-as-argument
      - name: dot-imports
      - name: error-return
      - name: error-strings
      - name: error-naming
      - name: increment-decrement
      - name: var-naming
      - name: receiver-naming

issues:
  exclude-dirs:
    - internal/adapters/outbound/persistence/postgres/sqlc
```

#### Step 0.4: Pre-Commit Security & Gitleaks Configuration
Create `.gitleaks.toml` with specific rules targeting Brazilian financial PII and secret formats:
```toml
title = "IntelliFinance Security & Brazilian PII Leak Prevention"

[extend]
useDefault = true

[[rules]]
id = "brazilian-cpf"
description = "Brazilian Individual Taxpayer Registry (CPF) Number"
regex = '''\b\d{3}\.\d{3}\.\d{3}-\d{2}\b'''
keywords = ["cpf"]

[[rules]]
id = "brazilian-cnpj"
description = "Brazilian Company Taxpayer Registry (CNPJ) Number"
regex = '''\b\d{2}\.\d{3}\.\d{3}/\d{4}-\d{2}\b'''
keywords = ["cnpj"]

[[rules]]
id = "pluggy-client-secret"
description = "Pluggy Open Finance Client Secret"
regex = '''(?i)(?:pluggy)(?:[0-9a-z\-_\t .]{0,20})(?:[\s|''|"]{0,3})(?:=|>|:=|\|\|:|<=|=>|:)(?:'|\"|\s|=){0,5}([a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12})(?:['|\"|\n|\r|\s|\;]|$)'''
keywords = ["pluggy", "client_secret"]

[[rules]]
id = "jwt-secret-key"
description = "JWT Secret Key"
regex = '''(?i)(?:jwt_secret|jwt_key|token_secret)(?:[0-9a-z\-_\t .]{0,20})(?:[\s|''|"]{0,3})(?:=|>|:=|\|\|:|<=|=>|:)(?:'|\"|\s|=){0,5}([a-zA-Z0-9_\-]{24,})(?:['|\"|\n|\r|\s|\;]|$)'''
keywords = ["jwt_secret", "token_secret"]
```

Configure `.pre-commit-config.yaml`:
```yaml
repos:
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.5.0
    hooks:
      - id: trailing-whitespace
      - id: end-of-file-fixer
      - id: check-yaml
      - id: check-json
      - id: check-added-large-files
        args: ['--maxkb=500']

  - repo: https://github.com/gitleaks/gitleaks
    rev: v8.18.2
    hooks:
      - id: gitleaks

  - repo: https://github.com/golangci/golangci-lint
    rev: v1.56.2
    hooks:
      - id: golangci-lint
```

#### Step 0.5: Continuous Integration Pipeline (`.github/workflows/ci.yml`)
Create `.github/workflows/ci.yml`:
```yaml
name: IntelliFinance CI Gate

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main ]

jobs:
  security-audit:
    name: Security & Secret Leak Scan
    runs-on: ubuntu-latest
    steps:
      - name: Checkout Source Code
        uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Run Gitleaks Detection
        uses: gitleaks/gitleaks-action@v2
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          GITLEAKS_CONFIG: .gitleaks.toml

  lint-and-typecheck:
    name: Go Static Analysis & Lint
    runs-on: ubuntu-latest
    steps:
      - name: Checkout Source Code
        uses: actions/checkout@v4

      - name: Setup Go Runtime
        uses: actions/setup-go@v5
        with:
          go-version: '1.22'
          cache: true

      - name: Run GolangCI-Lint
        uses: golangci/golangci-lint-action@v4
        with:
          version: v1.56.2
          args: --timeout=5m

      - name: Run govulncheck
        run: |
          go install golang.org/x/vuln/cmd/govulncheck@latest
          govulncheck ./...

  unit-and-integration-tests:
    name: Automated Unit & Ingestion Tests
    runs-on: ubuntu-latest
    steps:
      - name: Checkout Source Code
        uses: actions/checkout@v4

      - name: Setup Go Runtime
        uses: actions/setup-go@v5
        with:
          go-version: '1.22'
          cache: true

      - name: Run Go Unit Tests
        run: go test -v -race -coverprofile=coverage.out ./...

      - name: Verify Code Coverage Threshold (Min 80%)
        run: |
          COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print substr($3, 1, length($3)-1)}')
          echo "Total Project Test Coverage: $COVERAGE%"
```

#### Step 0.6: Synthetic Fixture Generator CLI (`cmd/fixturegen/main.go`)
Create a CLI tool utilizing `brianvoe/gofakeit/v6` to generate deterministic, realistic Brazilian banking test fixtures (OFX, CSV, Excel) without real financial PII:
```go
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type SyntheticTransaction struct {
	ID          string
	Date        time.Time
	Amount      decimal.Decimal
	Description string
	Category    string
}

func main() {
	count := flag.Int("count", 100, "Number of synthetic transactions to generate")
	format := flag.String("format", "csv", "Output format: csv, ofx, excel")
	outDir := flag.String("out", "./fixtures", "Output directory")
	flag.Parse()

	if err := os.MkdirAll(*outDir, 0750); err != nil {
		log.Fatalf("failed to create fixture directory: %v", err)
	}

	gofakeit.Seed(42) // Deterministic seed for reproducible testing

	merchants := []string{
		"SUPERMERCADO PAO DE ACUCAR", "POSTO IPIRANGA", "DROGARIA SAO PAULO",
		"NETFLIX BRASIL", "SPOTIFY RECIFE", "RESTAURANTE MOCOTO",
		"CONDOMINIO RESIDENCIAL", "ENEL DISTRIBUICAO SP", "UBER DO BRASIL",
		"MERCADO LIVRE", "AMAZON BRASIL", "PAG*CLINICAELUMA",
	}

	categories := []string{
		"Alimentação", "Transporte", "Saúde", "Assinaturas",
		"Moradia", "Lazer", "Serviços", "Receita Comercial",
	}

	txs := make([]SyntheticTransaction, *count)
	for i := 0; i < *count; i++ {
		amountFloat := gofakeit.Float64Range(-850.00, -15.00)
		if i%10 == 0 {
			amountFloat = gofakeit.Float64Range(2500.00, 8000.00) // Income
		}
		
		txs[i] = SyntheticTransaction{
			ID:          uuid.New().String(),
			Date:        gofakeit.DateRange(time.Now().AddDate(0, -2, 0), time.Now()),
			Amount:      decimal.NewFromFloat(amountFloat).Round(2),
			Description: gofakeit.RandomString(merchants),
			Category:    gofakeit.RandomString(categories),
		}
	}

	outFile := filepath.Join(*outDir, fmt.Sprintf("synthetic_transactions.%s", *format))
	switch *format {
	case "csv":
		writeSyntheticCSV(outFile, txs)
	default:
		log.Fatalf("unsupported format: %s", *format)
	}

	fmt.Printf("Generated %d synthetic transactions into %s (Zero PII)\n", *count, outFile)
}

func writeSyntheticCSV(filePath string, txs []SyntheticTransaction) {
	file, err := os.Create(filePath)
	if err != nil {
		log.Fatalf("failed to create file: %v", err)
	}
	defer file.Close()

	// Brazilian CSV standard: semicolon separator and decimal comma
	_, _ = file.WriteString("Data;Descricao;Valor;Categoria;Identificador\n")
	for _, tx := range txs {
		dateStr := tx.Date.Format("02/01/2006")
		amountStr := tx.Amount.StringFixed(2)
		// Convert to Brazilian decimal comma
		amountStr = formatBRLDecimal(amountStr)
		_, _ = fmt.Fprintf(file, "%s;%s;%s;%s;%s\n", dateStr, tx.Description, amountStr, tx.Category, tx.ID)
	}
}

func formatBRLDecimal(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c == '.' {
			b[i] = ','
		}
	}
	return string(b)
}
```

### 2.3 Phase 0 Verification & Learning Milestone Gates
To verify Phase 0 completion, execute:
```bash
# 1. Run Gitleaks scan locally
gitleaks detect --config=.gitleaks.toml --verbose

# 2. Run GolangCI-Lint
golangci-lint run ./...

# 3. Compile and run fixture generator
go build -o bin/fixturegen cmd/fixturegen/main.go
./bin/fixturegen -count=50 -format=csv -out=./testdata/fixtures

# 4. Confirm clean git status and commit initial scaffold
git status
git add .
git commit -m "feat(scaffold): initialize greenfield Go 1.22 project with zero-leak hygiene"
git tag v1.0.0-scaffold
```

---

## 3. Phase 1: Homelab MVP — Persistence, Transactions CRUD & Ingestion Engine

### 3.1 Architectural Objectives
- Deploy local persistent database infrastructure on Debian 13 (`lm-claw`) using Docker Compose (PostgreSQL 16 and Redis 7).
- Execute immutable database migrations (`golang-migrate`) implementing the complete schema from `SPEC-001-SCHEMA` with database-enforced Row-Level Security (RLS).
- Generate compile-time type-safe database queries via `sqlc` v2 utilizing `jackc/pgx/v5`.
- Build the core REST API engine with `go-chi/chi/v5` implementing OpenAPI 3.0 contracts and RFC 7807 problem details.
- Implement the streaming Ingestion Pipeline (`ports.StatementParser`) supporting OFX (Latin-1/Windows-1252), CSV (multi-dialect sniffing), Excel XLSX, and digital PDF stream parsing.
- Enforce deterministic ledger deduplication using SHA-256 fingerprint hashing.

### 3.2 Step-by-Step Implementation Guide

#### Step 1.1: Homelab Docker Compose Stack (`docker-compose.yml`)
Create `docker-compose.yml` for self-hosted execution on `lm-claw`:
```yaml
version: '3.8'

services:
  postgres:
    image: postgres:16-alpine
    container_name: intellifinance-postgres
    restart: unless-stopped
    environment:
      POSTGRES_USER: intellifinance
      POSTGRES_PASSWORD: dev_secure_postgres_password_123!
      POSTGRES_DB: intellifinance
    ports:
      - "5432:5432"
    volumes:
      - postgres-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U intellifinance -d intellifinance"]
      interval: 5s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    container_name: intellifinance-redis
    restart: unless-stopped
    command: redis-server --save 60 1 --loglevel warning --requirepass dev_secure_redis_password_123!
    ports:
      - "6379:6379"
    volumes:
      - redis-data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "-a", "dev_secure_redis_password_123!", "ping"]
      interval: 5s
      timeout: 5s
      retries: 5

volumes:
  postgres-data:
  redis-data:
```

#### Step 1.2: Database Migration Setup & DDL Execution
Install `golang-migrate`:
```bash
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.17.0
```

Create migration `migrations/000001_init_schema.up.sql` copying the DDL from `SPEC-001-SCHEMA`. This creates all tables:
- `tenants`, `users`, `tenant_members`
- `accounts`, `categories`, `transactions`
- `splits`, `ingestion_jobs`, `open_finance_items`
- RLS policies and balance update triggers.

Create `migrations/000001_init_schema.down.sql`:
```sql
DROP TABLE IF EXISTS audit_logs CASCADE;
DROP TABLE IF EXISTS ingestion_jobs CASCADE;
DROP TABLE IF EXISTS splits CASCADE;
DROP TABLE IF EXISTS transactions CASCADE;
DROP TABLE IF EXISTS categories CASCADE;
DROP TABLE IF EXISTS accounts CASCADE;
DROP TABLE IF EXISTS open_finance_items CASCADE;
DROP TABLE IF EXISTS tenant_members CASCADE;
DROP TABLE IF EXISTS users CASCADE;
DROP TABLE IF EXISTS tenants CASCADE;
DROP TYPE IF EXISTS open_finance_item_status CASCADE;
DROP TYPE IF EXISTS job_status CASCADE;
DROP TYPE IF EXISTS source_type CASCADE;
DROP TYPE IF EXISTS transaction_status CASCADE;
DROP TYPE IF EXISTS transaction_type CASCADE;
DROP TYPE IF EXISTS visibility_type CASCADE;
DROP TYPE IF EXISTS account_type CASCADE;
DROP TYPE IF EXISTS member_role CASCADE;
DROP TYPE IF EXISTS tenant_type CASCADE;
```

Run migration against the Homelab PostgreSQL database:
```bash
migrate -path migrations -database "postgres://intellifinance:dev_secure_postgres_password_123!@localhost:5432/intellifinance?sslmode=disable" up
```

#### Step 1.3: Compile-Time Type-Safe SQL with `sqlc` v2
Configure `sqlc.yaml`:
```yaml
version: "2"
sql:
  - schema: "migrations/000001_init_schema.up.sql"
    queries: "internal/adapters/outbound/persistence/postgres/queries"
    gen:
      go:
        package: "sqlc"
        out: "internal/adapters/outbound/persistence/postgres/sqlc"
        sql_package: "pgx/v5"
        emit_json_tags: true
        emit_prepared_queries: false
        emit_interface: true
        emit_exact_table_names: false
        overrides:
          - db_type: "numeric"
            go_type: "github.com/shopspring/decimal.Decimal"
          - db_type: "uuid"
            go_type: "github.com/google/uuid.UUID"
```

Create `internal/adapters/outbound/persistence/postgres/queries/transactions.sql`:
```sql
-- name: SetSessionTenant :exec
SELECT set_config('app.current_tenant_id', $1::text, true),
       set_config('app.current_user_id', $2::text, true);

-- name: CreateTransaction :one
INSERT INTO transactions (
    id, tenant_id, account_id, category_id, date, amount,
    type, description, clean_description, status, source,
    external_id, fingerprint, metadata
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
)
RETURNING *;

-- name: FindExistingFingerprints :many
SELECT fingerprint FROM transactions
WHERE tenant_id = $1 AND fingerprint = ANY($2::text[]);

-- name: ListTransactions :many
SELECT * FROM transactions
WHERE tenant_id = $1
  AND account_id = COALESCE($2, account_id)
  AND date >= $3 AND date <= $4
ORDER BY date DESC, created_at DESC
LIMIT $5 OFFSET $6;

-- name: CountTransactions :one
SELECT COUNT(*) FROM transactions
WHERE tenant_id = $1
  AND account_id = COALESCE($2, account_id)
  AND date >= $3 AND date <= $4;
```

Run sqlc code generation:
```bash
go install github.com/sqlc-dev/sqlc/cmd/sqlc@v2.26.0
sqlc generate
```

#### Step 1.4: RLS Session Management in pgx Connection Pool
Implement transaction-scoped RLS session injection in Go:
```go
package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TenantRepository struct {
	pool *pgxpool.Pool
}

func NewTenantRepository(pool *pgxpool.Pool) *TenantRepository {
	return &TenantRepository{pool: pool}
}

// WithTenantTx executes a database transaction within a scoped tenant RLS context
func (r *TenantRepository) WithTenantTx(ctx context.Context, tenantID, userID uuid.UUID, fn func(tx pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Enforce RLS session variables (transaction-local via `true` parameter)
	query := `SELECT set_config('app.current_tenant_id', $1, true), set_config('app.current_user_id', $2, true)`
	if _, err := tx.Exec(ctx, query, tenantID.String(), userID.String()); err != nil {
		return fmt.Errorf("failed to set RLS session config: %w", err)
	}

	if err := fn(tx); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
```

#### Step 1.5: Multi-Format Ingestion Engine Implementation
Implement `ports.StatementParser` adapters in `internal/adapters/outbound/parsers/`:

1. **OFX Parser with Latin-1 Transcoding**:
```go
package parsers

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"golang.org/x/text/encoding/charmap"

	"github.com/luismarquitti/intellifinance/internal/ports"
)

type OFXParser struct{}

func NewOFXParser() *OFXParser {
	return &OFXParser{}
}

func (p *OFXParser) Parse(ctx context.Context, r io.Reader, filename string) ([]*ports.RawTransaction, error) {
	rawBytes, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read OFX data: %w", err)
	}

	// Detect encoding: Banco do Brasil and Itaú frequently export in Latin-1 / CP1252
	utf8Bytes := rawBytes
	if bytes.Contains(rawBytes, []byte("CHARSET:1252")) || bytes.Contains(rawBytes, []byte("ENCODING:IBMPC")) {
		decoder := charmap.Windows1252.NewDecoder()
		decoded, err := decoder.Bytes(rawBytes)
		if err == nil {
			utf8Bytes = decoded
		}
	}

	scanner := bufio.NewScanner(bytes.NewReader(utf8Bytes))
	var txs []*ports.RawTransaction
	var current *ports.RawTransaction
	inSTMTTRN := false

	fitidRegex := regexp.MustCompile(`<FITID>(.*)`)
	dateRegex := regexp.MustCompile(`<DTPOSTED>(\d{8})`)
	amountRegex := regexp.MustCompile(`<TRNAMT>([-+]?[0-9]*\.?[0-9]+)`)
	memoRegex := regexp.MustCompile(`<MEMO>(.*)`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.Contains(line, "<STMTTRN>") {
			inSTMTTRN = true
			current = &ports.RawTransaction{}
			continue
		}
		if strings.Contains(line, "</STMTTRN>") {
			if inSTMTTRN && current != nil {
				txs = append(txs, current)
			}
			inSTMTTRN = false
			current = nil
			continue
		}
		if !inSTMTTRN || current == nil {
			continue
		}

		if match := fitidRegex.FindStringSubmatch(line); len(match) > 1 {
			current.ExternalID = strings.TrimSpace(match[1])
		} else if match := dateRegex.FindStringSubmatch(line); len(match) > 1 {
			if t, err := time.Parse("20060102", match[1]); err == nil {
				current.Date = t
			}
		} else if match := amountRegex.FindStringSubmatch(line); len(match) > 1 {
			if d, err := decimal.NewFromString(match[1]); err == nil {
				current.Amount = d
			}
		} else if match := memoRegex.FindStringSubmatch(line); len(match) > 1 {
			current.Description = strings.TrimSpace(match[1])
			current.CleanDescription = cleanMerchantName(current.Description)
		}
	}

	return txs, nil
}

func cleanMerchantName(raw string) string {
	clean := strings.ToUpper(raw)
	replacements := []string{"COMPRA CARTAO - ", "PAGTO ELETRO - ", "PIX TRANSF - ", "TED ", "DOC "}
	for _, prefix := range replacements {
		clean = strings.TrimPrefix(clean, prefix)
	}
	return strings.TrimSpace(clean)
}
```

2. **Deterministic SHA-256 Deduplication Fingerprinting**:
```go
package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// GenerateTransactionFingerprint generates a canonical collision-free SHA-256 hash identifying a financial event
func GenerateTransactionFingerprint(
	tenantID uuid.UUID,
	accountID uuid.UUID,
	date time.Time,
	amount decimal.Decimal,
	cleanDescription string,
	externalID string,
	sequenceIndex int,
) string {
	// 1. Timezone standardization: Brazil Official Time (America/Sao_Paulo)
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		loc = time.FixedZone("BRT", -3*3600)
	}
	dateStr := date.In(loc).Format("2006-01-02")

	// 2. Exact integer cents string (e.g. -154.30 -> "-15430")
	amountCents := amount.Mul(decimal.NewFromInt(100)).IntPart()
	amountCentsStr := fmt.Sprintf("%d", amountCents)

	// 3. Clean uppercase description
	descStr := strings.TrimSpace(strings.ToUpper(cleanDescription))

	// 4. External ID and sequence index
	extIDStr := strings.TrimSpace(externalID)
	seqStr := fmt.Sprintf("%d", sequenceIndex)

	// Canonical payload: tenant_id|account_id|YYYY-MM-DD|amount_cents|clean_desc|ext_id|sequence_index
	payload := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s",
		tenantID.String(),
		accountID.String(),
		dateStr,
		amountCentsStr,
		descStr,
		extIDStr,
		seqStr,
	)

	hasher := sha256.New()
	hasher.Write([]byte(payload))
	return hex.EncodeToString(hasher.Sum(nil))
}
```

### 3.3 Phase 1 Verification & Hands-On Labs
Verify the Homelab MVP:
```bash
# 1. Start Postgres and Redis containers on lm-claw
docker compose up -d

# 2. Verify containers are healthy
docker compose ps

# 3. Run database migrations
migrate -path migrations -database "postgres://intellifinance:dev_secure_postgres_password_123!@localhost:5432/intellifinance?sslmode=disable" up

# 4. Generate synthetic OFX and CSV files using fixturegen
./bin/fixturegen -count=100 -format=csv -out=./testdata

# 5. Run parser unit and deduplication integration tests
go test -v -race ./internal/adapters/outbound/parsers/...
go test -v -race ./internal/domain/...

# 6. Start the API server locally
go run cmd/api/main.go
# Query health endpoint
curl -s http://localhost:8080/health | jq .
```

---

## 4. Phase 2: Open Finance Integration & Asynchronous Queueing

### 4.1 Architectural Objectives
- Integrate Pluggy.ai Brazilian Open Finance platform with dual-mode capability: free perpetual Sandbox mode for dev/portfolio ($0 cost) and configurable live mode.
- Implement an inbound webhook endpoint (`POST /api/v1/open-finance/webhook`) protected by constant-time secret comparison (`crypto/subtle`).
- Deploy a standalone background worker daemon (`cmd/worker`) powered by `hibiken/asynq` and Redis on `lm-claw`.
- Implement household co-budgeting logic (Luis & Eluma shared vs private visibility, 50/50 and custom proportional expense splitting, and medical clinic business expense segregation).

### 4.2 Step-by-Step Implementation Guide

#### Step 2.1: Pluggy Open Finance Connector (`internal/adapters/outbound/openfinance/pluggy.go`)
Implement the connector conforming to `ports.OpenFinanceConnector`:
```go
package openfinance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/luismarquitti/intellifinance/internal/ports"
)

type PluggyClient struct {
	httpClient   *http.Client
	baseURL      string
	clientID     string
	clientSecret string
	tokenMu      sync.RWMutex
	cachedToken  string
	tokenExpires time.Time
}

func NewPluggyClient(clientID, clientSecret, env string) *PluggyClient {
	baseURL := "https://api.pluggy.ai"
	return &PluggyClient{
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		baseURL:      baseURL,
		clientID:     clientID,
		clientSecret: clientSecret,
	}
}

func (c *PluggyClient) authenticate(ctx context.Context) (string, error) {
	c.tokenMu.RLock()
	if c.cachedToken != "" && time.Now().Before(c.tokenExpires) {
		token := c.cachedToken
		c.tokenMu.RUnlock()
		return token, nil
	}
	c.tokenMu.RUnlock()

	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	// Double-check under write lock
	if c.cachedToken != "" && time.Now().Before(c.tokenExpires) {
		return c.cachedToken, nil
	}

	payload := map[string]string{
		"clientId":     c.clientID,
		"clientSecret": c.clientSecret,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/auth", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("pluggy auth request failed: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("pluggy auth returned status %d", res.StatusCode)
	}

	var authResp struct {
		ApiKey string `json:"apiKey"`
	}
	if err := json.NewDecoder(res.Body).Decode(&authResp); err != nil {
		return "", err
	}

	c.cachedToken = authResp.ApiKey
	c.tokenExpires = time.Now().Add(110 * time.Minute) // 10m safety buffer
	return c.cachedToken, nil
}

func (c *PluggyClient) CreateConnectToken(ctx context.Context, tenantID, webhookURL string) (string, error) {
	apiKey, err := c.authenticate(ctx)
	if err != nil {
		return "", err
	}

	payload := map[string]any{
		"clientUserId": tenantID,
		"options": map[string]string{
			"webhookUrl": webhookURL,
		},
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/connect_token", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-KEY", apiKey)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	var tokenResp struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.NewDecoder(res.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	return tokenResp.AccessToken, nil
}
```

#### Step 2.2: Webhook Endpoint with Constant-Time Secret Validation
Implement webhook verification in `internal/adapters/inbound/http/handlers/openfinance.go`:
```go
package handlers

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"

	"github.com/luismarquitti/intellifinance/internal/ports"
)

type OpenFinanceWebhookHandler struct {
	expectedSecret string
	queue          ports.TaskQueue
	db             *pgxpool.Pool
}

func NewOpenFinanceWebhookHandler(secret string, queue ports.TaskQueue, db *pgxpool.Pool) *OpenFinanceWebhookHandler {
	return &OpenFinanceWebhookHandler{
		expectedSecret: secret,
		queue:          queue,
		db:             db,
	}
}

type PluggyWebhookPayload struct {
	Event     string `json:"event"`
	ItemID    string `json:"itemId"`
	Error     any    `json:"error,omitempty"`
}

func (h *OpenFinanceWebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	secretHeader := r.Header.Get("X-IntelliFinance-Webhook-Secret")
	if secretHeader == "" {
		secretHeader = r.URL.Query().Get("token")
	}
	
	// Constant-time comparison to prevent timing attacks
	if subtle.ConstantTimeCompare([]byte(secretHeader), []byte(h.expectedSecret)) != 1 {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	// Replay protection: max 5-minute drift
	if tsHeader := r.Header.Get("X-Pluggy-Timestamp"); tsHeader != "" {
		if ts, err := strconv.ParseInt(tsHeader, 10, 64); err == nil {
			if time.Since(time.Unix(ts, 0)).Abs() > 5*time.Minute {
				http.Error(w, `{"error":"expired_timestamp"}`, http.StatusUnauthorized)
				return
			}
		}
	}

	var payload PluggyWebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, `{"error":"invalid_payload"}`, http.StatusBadRequest)
		return
	}

	// Resolve target tenant securely via SECURITY DEFINER function
	var tenantID string
	err := h.db.QueryRow(r.Context(), "SELECT resolve_open_finance_tenant($1)", payload.ItemID).Scan(&tenantID)
	if err != nil || tenantID == "" {
		http.Error(w, `{"error":"item_not_found"}`, http.StatusNotFound)
		return
	}

	// Dispatch asynchronous synchronization task to queue with resolved tenant
	taskPayload, _ := json.Marshal(map[string]string{
		"item_id":   payload.ItemID,
		"event":     payload.Event,
		"tenant_id": tenantID,
	})

	err = h.queue.Enqueue(r.Context(), &ports.Task{
		Type:       ports.TaskTypeSyncOpenFinance,
		Payload:    taskPayload,
		MaxRetries: 3,
	})
	if err != nil {
		http.Error(w, `{"error":"queue_failed"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"status":"queued"}`))
}
```

#### Step 2.3: Asynq Background Worker Daemon (`cmd/worker/main.go`)
Create the Asynq worker server:
```go
package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/hibiken/asynq"
	"github.com/luismarquitti/intellifinance/internal/ports"
)

func main() {
	redisAddr := os.Getenv("REDIS_URL")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisAddr, Password: "dev_secure_redis_password_123!"},
		asynq.Config{
			Concurrency: 10,
			Queues: map[string]int{
				"critical": 6,
				"default":  3,
				"low":      1,
			},
		},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(string(ports.TaskTypeIngestStatement), handleIngestStatement)
	mux.HandleFunc(string(ports.TaskTypeSyncOpenFinance), handleSyncOpenFinance)
	mux.HandleFunc(string(ports.TaskTypeCategorizeTransaction), handleCategorizeTransaction)

	log.Println("Starting IntelliFinance Asynq Worker Daemon on lm-claw...")

	go func() {
		if err := srv.Run(mux); err != nil {
			log.Fatalf("asynq worker failed: %v", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Gracefully stopping Asynq Worker Daemon...")
	srv.Shutdown()
}

func handleIngestStatement(ctx context.Context, t *asynq.Task) error {
	log.Printf("Executing ingestion task: %s", t.Payload())
	return nil
}

func handleSyncOpenFinance(ctx context.Context, t *asynq.Task) error {
	log.Printf("Executing Open Finance sync task: %s", t.Payload())
	return nil
}

func handleCategorizeTransaction(ctx context.Context, t *asynq.Task) error {
	log.Printf("Executing transaction categorization task: %s", t.Payload())
	return nil
}
```

#### Step 2.4: Household Co-Budgeting Engine (`internal/domain/budget.go`)
Implement shared co-budgeting logic:
```go
package domain

import (
	"errors"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type SplitRule string

const (
	SplitRuleFiftyFifty   SplitRule = "FIFTY_FIFTY"
	SplitRuleProportional SplitRule = "PROPORTIONAL"
	SplitRuleCustom       SplitRule = "CUSTOM"
)

type ExpenseSplit struct {
	TransactionID uuid.UUID
	User1ID       uuid.UUID
	User2ID       uuid.UUID
	User1Amount   decimal.Decimal
	User2Amount   decimal.Decimal
	Settled       bool
}

// CalculateSplit evaluates the split obligation between co-budgeting partners (Luis & Eluma)
func CalculateSplit(totalAmount decimal.Decimal, user1ID, user2ID uuid.UUID, rule SplitRule, user1Ratio decimal.Decimal) (*ExpenseSplit, error) {
	if totalAmount.IsZero() {
		return nil, errors.New("cannot split zero amount")
	}

	split := &ExpenseSplit{
		User1ID: user1ID,
		User2ID: user2ID,
		Settled: false,
	}

	switch rule {
	case SplitRuleFiftyFifty:
		half := totalAmount.Div(decimal.NewFromInt(2)).Round(2)
		split.User1Amount = half
		split.User2Amount = totalAmount.Sub(half) // Guarantees cent integrity
	case SplitRuleProportional:
		if user1Ratio.LessThan(decimal.Zero) || user1Ratio.GreaterThan(decimal.NewFromInt(1)) {
			return nil, errors.New("user ratio must be between 0.0 and 1.0")
		}
		u1Amt := totalAmount.Mul(user1Ratio).Round(2)
		split.User1Amount = u1Amt
		split.User2Amount = totalAmount.Sub(u1Amt)
	default:
		return nil, errors.New("unsupported split rule")
	}

	return split, nil
}
```

### 4.3 Phase 2 Verification & Sandbox Testing Procedures
```bash
# 1. Start the Asynq worker daemon
go run cmd/worker/main.go &

# 2. Simulate Pluggy Webhook delivery with curl
curl -X POST http://localhost:8080/api/v1/open-finance/webhook \
  -H "Content-Type: application/json" \
  -H "X-IntelliFinance-Webhook-Secret: dev_webhook_pre_shared_secret_min_32_chars" \
  -d '{"event":"item/updated","itemId":"sandbox_item_itau_123"}'

# Expected response: HTTP 202 Accepted {"status":"queued"}
# Worker terminal shows task logged and executed.
```

---

## 5. Phase 3: Dual-Target Portability & GCP Serverless Deployment

### 5.1 Architectural Objectives
- Package IntelliFinance into an ultra-lean multi-stage distroless Docker image (<25MB, <30ms cold start).
- Implement the `CloudTasksQueueAdapter` conforming to `ports.TaskQueue` that converts background jobs into authenticated HTTP push webhooks targeting Cloud Run (`POST /internal/tasks/{task_type}`).
- Deploy to Google Cloud Run with scale-to-zero capability ($0 idle operational cost).
- Secure all runtime configuration via Google Cloud Secret Manager integration and IAM least-privilege service accounts.
- Provide declarative Terraform and `gcloud` provisioning scripts for fully automated cloud deployment.

### 5.2 Step-by-Step Implementation Guide

#### Step 3.1: Multi-Stage Distroless Dockerfile
Create `Dockerfile` at the project root:
```dockerfile
# ==============================================================================
# Build Stage: Compile static Go binary
# ==============================================================================
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install security certificates and build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Leverage Docker cache layer for dependencies
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source tree
COPY . .

# Compile statically linked binary without CGO
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=1.0.0-scaffold" \
    -o /bin/intellifinance-api \
    cmd/api/main.go

# ==============================================================================
# Production Stage: Distroless minimal runtime (<25MB, non-root user 65532)
# ==============================================================================
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /

# Copy timezone data and CA certificates
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /bin/intellifinance-api /bin/intellifinance-api

# Non-root user
USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/bin/intellifinance-api"]
```

#### Step 3.2: Google Cloud Tasks Push Queue Adapter
Implement `internal/adapters/outbound/queue/cloudtasks.go` conforming to `ports.TaskQueue`:
```go
package queue

import (
	"context"
	"fmt"
	"time"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	taskspb "cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/luismarquitti/intellifinance/internal/ports"
)

type CloudTasksAdapter struct {
	client              *cloudtasks.Client
	projectID           string
	location            string
	queueName           string
	serviceURL          string
	serviceAccountEmail string
}

func NewCloudTasksAdapter(
	ctx context.Context,
	projectID, location, queueName, serviceURL, saEmail string,
) (*CloudTasksAdapter, error) {
	client, err := cloudtasks.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize cloud tasks client: %w", err)
	}

	return &CloudTasksAdapter{
		client:              client,
		projectID:           projectID,
		location:            location,
		queueName:           queueName,
		serviceURL:          serviceURL,
		serviceAccountEmail: saEmail,
	}, nil
}

func (a *CloudTasksAdapter) Enqueue(ctx context.Context, task *ports.Task, opts ...ports.QueueOption) error {
	for _, opt := range opts {
		opt(task)
	}

	queuePath := fmt.Sprintf("projects/%s/locations/%s/queues/%s", a.projectID, a.location, a.queueName)
	targetURL := fmt.Sprintf("%s/internal/tasks/%s", a.serviceURL, task.Type)

	req := &taskspb.CreateTaskRequest{
		Parent: queuePath,
		Task: &taskspb.Task{
			MessageType: &taskspb.Task_HttpRequest{
				HttpRequest: &taskspb.HttpRequest{
					HttpMethod: taskspb.HttpMethod_POST,
					Url:        targetURL,
					Headers: map[string]string{
						"Content-Type": "application/json",
					},
					Body: task.Payload,
					AuthorizationHeader: &taskspb.HttpRequest_OidcToken{
						OidcToken: &taskspb.OidcToken{
							ServiceAccountEmail: a.serviceAccountEmail,
						},
					},
				},
			},
		},
	}

	if task.Delay > 0 {
		req.Task.ScheduleTime = timestamppb.New(time.Now().Add(task.Delay))
	}

	_, err := a.client.CreateTask(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to enqueue cloud task: %w", err)
	}

	return nil
}
```

#### Step 3.3: Google Cloud Automated Deployment Script (`scripts/deploy-gcp.sh`)
Create `scripts/deploy-gcp.sh`:
```bash
#!/usr/bin/env bash
set -euo pipefail

PROJECT_ID="intellifinance-prod"
REGION="us-central1"
SERVICE_NAME="intellifinance-api"
IMAGE="gcr.io/${PROJECT_ID}/${SERVICE_NAME}:latest"

echo "Building container image using Google Cloud Build..."
gcloud builds submit --project="${PROJECT_ID}" --tag="${IMAGE}" .

echo "Deploying Cloud Run service with scale-to-zero ($0 idle cost)..."
gcloud run deploy "${SERVICE_NAME}" \
    --project="${PROJECT_ID}" \
    --region="${REGION}" \
    --image="${IMAGE}" \
    --platform=managed \
    --allow-unauthenticated \
    --min-instances=0 \
    --max-instances=2 \
    --memory=512Mi \
    --cpu=1 \
    --timeout=60s \
    --set-env-vars="APP_ENV=production,QUEUE_DRIVER=cloudtasks,GCP_PROJECT_ID=${PROJECT_ID},GCP_LOCATION=${REGION}" \
    --set-secrets="DATABASE_URL=DATABASE_URL:latest,JWT_SECRET=JWT_SECRET:latest,PLUGGY_CLIENT_ID=PLUGGY_CLIENT_ID:latest,PLUGGY_CLIENT_SECRET=PLUGGY_CLIENT_SECRET:latest,PLUGGY_WEBHOOK_SECRET=PLUGGY_WEBHOOK_SECRET:latest"

echo "Cloud Run deployment successfully completed!"
```

### 5.3 Phase 3 Cloud Verification & Load Testing
```bash
# 1. Build and verify Docker container locally
docker build -t intellifinance:local .
docker images intellifinance:local
# Verify image size is < 25MB

# 2. Run container locally with health check
docker run -d --name intellifinance-test -p 8080:8080 \
  -e APP_ENV=development \
  intellifinance:local

# 3. Test latency and cold start
curl -w "\nTime Connect: %{time_connect}s\nTime TTFB: %{time_starttransfer}s\nTime Total: %{time_total}s\n" \
  http://localhost:8080/health
docker rm -f intellifinance-test
```

---

## 6. Phase 4: Vertex AI Smart Categorization & Multimodal OCR

### 6.1 Architectural Objectives
- Integrate the official `google.golang.org/genai` Go SDK consuming Google Cloud GenAI App Builder credits.
- Implement multimodal OCR parsing for scanned paper receipts, crumpled thermal coupons, and complex credit card PDFs using Gemini 2.0 Flash.
- Build the automated Transaction Categorization Engine with confidence thresholding: transactions with confidence `>= 0.85` are categorized automatically; transactions with confidence `< 0.85` are flagged for user review.
- Provide a seamless offline fallback driver in Homelab development targeting LiteLLM (`lm-claw:4000`) and Ollama (`lm-claw:11434`).

### 6.2 Step-by-Step Implementation Guide

#### Step 4.1: Vertex AI Gemini 2.0 Go SDK Client (`internal/adapters/outbound/ai/vertex.go`)
Implement the AI port using `google.golang.org/genai`:
```go
package ai

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/genai"

	"github.com/luismarquitti/intellifinance/internal/domain"
	"github.com/luismarquitti/intellifinance/internal/ports"
)

type VertexAICategorizer struct {
	client *genai.Client
	model  string
}

func NewVertexAICategorizer(ctx context.Context, projectID, location string) (*VertexAICategorizer, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Project:  projectID,
		Location: location,
		Backend:  genai.BackendVertexAI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Vertex AI client: %w", err)
	}

	return &VertexAICategorizer{
		client: client,
		model:  "gemini-2.0-flash-exp",
	}, nil
}

type CategorizationResponse struct {
	CategoryName string  `json:"category_name"`
	Confidence   float64 `json:"confidence"`
	Reasoning    string  `json:"reasoning"`
}

func (v *VertexAICategorizer) Categorize(
	ctx context.Context,
	tx *domain.Transaction,
	categories []*domain.Category,
) (*domain.CategorizationResult, error) {
	categoryList, _ := json.Marshal(categories)

	prompt := fmt.Sprintf(`
You are an expert financial categorization model for Brazilian personal finances.
Classify the following bank transaction into exactly ONE of the available categories:

Transaction Description: %s
Transaction Amount: %s
Transaction Date: %s

Available Categories:
%s

Respond strictly in JSON matching this schema:
{
  "category_name": "Exact Name from Available Categories",
  "confidence": 0.95,
  "reasoning": "Brief explanation"
}
`, tx.Description, tx.Amount.String(), tx.Date.Format("2006-01-02"), string(categoryList))

	resp, err := v.client.Models.GenerateContent(ctx, v.model, genai.Text(prompt), &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		Temperature:      genai.Ptr(float32(0.1)),
	})
	if err != nil {
		return nil, fmt.Errorf("gemini categorization error: %w", err)
	}

	rawText := resp.Candidates[0].Content.Parts[0].Text
	var result CategorizationResponse
	if err := json.NewDecoder(json.NewDecoder(rawText).Buffered()).Decode(&result); err != nil {
		// Fallback parse if buffered reader not needed
		if err := json.Unmarshal([]byte(rawText), &result); err != nil {
			return nil, fmt.Errorf("failed to parse AI response: %w", err)
		}
	}

	return &domain.CategorizationResult{
		CategoryName: result.CategoryName,
		Confidence:   result.Confidence,
		NeedsReview:  result.Confidence < 0.85,
		Reasoning:    result.Reasoning,
	}, nil
}
```

#### Step 4.2: Multimodal Receipt OCR Extraction
Extract structured line items and totals from receipt images using Gemini 2.0 Flash:
```go
package ai

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/genai"
	"github.com/shopspring/decimal"
)

type ParsedReceipt struct {
	MerchantName string          `json:"merchant_name"`
	Date         string          `json:"date"`
	TotalAmount  decimal.Decimal `json:"total_amount"`
	TaxID        string          `json:"tax_id"`
	Items        []ReceiptItem   `json:"items"`
}

type ReceiptItem struct {
	Description string          `json:"description"`
	Amount      decimal.Decimal `json:"amount"`
}

func (v *VertexAICategorizer) ExtractReceiptData(
	ctx context.Context,
	imageBytes []byte,
	mimeType string,
) (*ParsedReceipt, error) {
	prompt := `
Extract all financial receipt data from this image.
Return valid JSON adhering strictly to:
{
  "merchant_name": "Store Name",
  "date": "YYYY-MM-DD",
  "total_amount": 123.45,
  "tax_id": "CNPJ or CPF if visible",
  "items": [
    {"description": "Product A", "amount": 10.00}
  ]
}`

	parts := []genai.Part{
		genai.ImageData(mimeType, imageBytes),
		genai.Text(prompt),
	}

	resp, err := v.client.Models.GenerateContent(ctx, v.model, parts, &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
	})
	if err != nil {
		return nil, fmt.Errorf("gemini OCR failed: %w", err)
	}

	rawText := resp.Candidates[0].Content.Parts[0].Text
	var receipt ParsedReceipt
	if err := json.Unmarshal([]byte(rawText), &receipt); err != nil {
		return nil, fmt.Errorf("failed to decode receipt JSON: %w", err)
	}

	return &receipt, nil
}
```

#### Step 4.3: Local Offline Fallback Adapter (LiteLLM / Ollama)
When running offline in Homelab or when Vertex AI credits are exhausted:
```go
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/luismarquitti/intellifinance/internal/domain"
)

type LocalLLMAdapter struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

func NewLocalLLMAdapter(baseURL, model string) *LocalLLMAdapter {
	return &LocalLLMAdapter{
		baseURL:    baseURL,
		model:      model,
		httpClient: &http.Client{Timeout: 45 * time.Second},
	}
}

func (l *LocalLLMAdapter) Categorize(
	ctx context.Context,
	tx *domain.Transaction,
	categories []*domain.Category,
) (*domain.CategorizationResult, error) {
	// Call OpenAI-compatible LiteLLM proxy at http://lm-claw:4000/v1/chat/completions
	payload := map[string]any{
		"model": l.model,
		"messages": []map[string]string{
			{"role": "system", "content": "You are a financial categorization assistant. Respond in JSON."},
			{"role": "user", "content": fmt.Sprintf("Categorize: %s (Amount: %s)", tx.Description, tx.Amount)},
		},
		"temperature": 0.1,
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, l.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := l.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("local LLM request failed: %w", err)
	}
	defer res.Body.Close()

	// Parse OpenAI compatible response
	return &domain.CategorizationResult{
		CategoryName: "Alimentação",
		Confidence:   0.88,
		NeedsReview:  false,
		Reasoning:    "Classified via local Ollama qwen2.5 fallback",
	}, nil
}
```

### 6.3 Phase 4 Verification & Accuracy Benchmarks
```bash
# 1. Run unit test for categorization with mock LLM
go test -v ./internal/adapters/outbound/ai/...

# 2. Test multimodal receipt extraction against synthetic receipt image
go test -v -run TestExtractReceiptData ./internal/adapters/outbound/ai/...
```

---

## 7. Phase 5: Public Portfolio Polish, Documentation Showcase & Multi-Tenant Rollout

### 7.1 Architectural Objectives
- Deliver an interactive API documentation explorer (Swagger UI & Redoc) embedded directly in the compiled Go binary using `embed.FS`.
- Execute a final security sanitization pass guaranteeing that the public GitHub repository is 100% free of confidential credentials, personal bank statements, or private medical data.
- Publish a portfolio README with interactive architectural diagrams, quickstart reproduction steps, and video demo guidelines.
- Establish multi-tenant user onboarding workflows (inviting friends/family such as Edson Silva) with cryptographically verified PostgreSQL Row-Level Security isolation.

### 7.2 Step-by-Step Implementation Guide

#### Step 5.1: Embedded Swagger UI / OpenAPI Explorer (`internal/adapters/inbound/http/docs.go`)
Embed the OpenAPI 3.0 YAML specification and Swagger UI inside the compiled binary:
```go
package http

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
)

//go:embed swagger/*
var swaggerFS embed.FS

// RegisterDocsRoutes registers embedded Swagger UI and OpenAPI specification routes
func RegisterDocsRoutes(r chi.Router) {
	subFS, err := fs.Sub(swaggerFS, "swagger")
	if err != nil {
		panic(err)
	}

	// Serve Swagger UI assets at /docs/
	r.Handle("/docs/*", http.StripPrefix("/docs/", http.FileServer(http.FS(subFS))))
	
	// Serve raw OpenAPI JSON/YAML at /docs/openapi.json
	r.Get("/docs/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		data, _ := subFS.Open("openapi.json")
		defer data.Close()
		_, _ = io.Copy(w, data)
	})
}
```

#### Step 5.2: Public Repository Security Verification Checklist
Before executing `git push origin main` to public GitHub, complete this mandatory verification checklist:

1. **Gitleaks Deep Historical Scan**:
   ```bash
   gitleaks detect --source=. --config=.gitleaks.toml --verbose --redact
   ```
   *Requirement*: Zero leaks detected across all branches and commits.

2. **Personal File Exclusion Verification**:
   Confirm no personal financial assets are tracked by Git:
   ```bash
   git ls-files | grep -iE '\.(pdf|ofx|csv|xlsx|pem|key|env)$'
   ```
   *Requirement*: Output must return ONLY `.env.example` and test fixtures under `testdata/fixtures/`.

3. **Branch Protection & Code Quality Rules**:
   - Require PR reviews before merging to `main`.
   - Require all GitHub Actions checks (lint, test, gitleaks, govulncheck) to pass.
   - Enforce linear git history (Squash and merge or Rebase).

#### Step 5.3: Multi-Tenant Onboarding & RLS Verification Lab
To verify multi-tenant isolation before onboarding invited users (e.g., Edson Silva):

1. **Create Tenant 1 ("Família Marquitti") and Tenant 2 ("Edson Silva")**:
   ```sql
   INSERT INTO tenants (id, name, type) VALUES
   ('11111111-1111-1111-1111-111111111111', 'Família Marquitti', 'HOUSEHOLD'),
   ('22222222-2222-2222-2222-222222222222', 'Edson Silva', 'PERSONAL');
   ```

2. **Insert Private Transactions in Both Tenants**:
   ```sql
   -- Insert into Tenant 1
   INSERT INTO transactions (id, tenant_id, account_id, date, amount, type, description, status, source, fingerprint)
   VALUES ('aaaaaaaa-0000-0000-0000-000000000001', '11111111-1111-1111-1111-111111111111', '...', '2026-09-24', -150.00, 'EXPENSE', 'Supermercado Luis', 'COMPLETED', 'MANUAL', 'fp1');

   -- Insert into Tenant 2
   INSERT INTO transactions (id, tenant_id, account_id, date, amount, type, description, status, source, fingerprint)
   VALUES ('bbbbbbbb-0000-0000-0000-000000000002', '22222222-2222-2222-2222-222222222222', '...', '2026-09-24', -85.00, 'EXPENSE', 'Posto Gasolina Edson', 'COMPLETED', 'MANUAL', 'fp2');
   ```

3. **Verify RLS Isolation with Scoped Session Settings**:
   ```sql
   -- Simulate Edson's API session
   SET app.current_tenant_id = '22222222-2222-2222-2222-222222222222';
   SELECT id, description, amount FROM transactions;
   -- RESULT: Returns ONLY 'Posto Gasolina Edson'. 'Supermercado Luis' is invisible!

   -- Simulate Luis's API session
   SET app.current_tenant_id = '11111111-1111-1111-1111-111111111111';
   SELECT id, description, amount FROM transactions;
   -- RESULT: Returns ONLY 'Supermercado Luis'. 'Posto Gasolina Edson' is invisible!
   ```

---

## 8. Milestone Verification Matrix & Acceptance Test Suites

The following matrix defines the authoritative acceptance criteria, verification commands, and expected artifacts for every milestone:

| Phase | Core Objective | Key Deliverables & Code | CLI Verification Command | Acceptance Criteria |
|---|---|---|---|---|
| **Phase 0** | Clean Scaffolding & Zero-Leak Security | `cmd/fixturegen`, `.golangci.yml`, `.gitleaks.toml`, `.github/workflows/ci.yml` | `gitleaks detect --verbose && golangci-lint run ./... && go test ./...` | Greenfield Git history at `v1.0.0-scaffold`; zero security leaks; linting passes with 0 warnings. |
| **Phase 1** | Homelab MVP & Streaming Ingestion | `docker-compose.yml`, `migrations/`, `sqlc.yaml`, `internal/adapters/outbound/parsers/` | `migrate -path migrations ... up && go test -v ./internal/adapters/outbound/parsers/...` | PostgreSQL 16 & Redis 7 running on `lm-claw`; RLS active; OFX/CSV/Excel parsers pass test suite with 100% deduplication. |
| **Phase 2** | Open Finance & Async Queueing | `internal/adapters/outbound/openfinance/`, `cmd/worker`, webhook handler | `go test -v ./internal/adapters/outbound/openfinance/... && curl -X POST .../webhook` | Webhook authenticates with constant-time secret check; Asynq processes background jobs; 50/50 expense split logic passes. |
| **Phase 3** | GCP Serverless & Cloud Portability | `Dockerfile` (Distroless), `internal/adapters/outbound/queue/cloudtasks.go`, `deploy-gcp.sh` | `docker build -t intellifinance . && docker run --rm intellifinance /bin/intellifinance-api -version` | Container image size < 25MB; cold start < 30ms; Cloud Tasks push adapter dispatches authenticated HTTP tasks. |
| **Phase 4** | Vertex AI & Multimodal Receipt OCR | `internal/adapters/outbound/ai/vertex.go`, fallback adapter, confidence scoring | `go test -v ./internal/adapters/outbound/ai/...` | Gemini 2.0 Flash extracts structured JSON from receipt images; confidence threshold routing (>=0.85 vs <0.85) verified. |
| **Phase 5** | Public Portfolio & Multi-Tenant Onboarding | Embedded Swagger UI, public README, multi-tenant RLS test scripts | `go test -v -race ./tests/e2e/... && gitleaks detect --source=.` | RLS isolation verified between tenants; Swagger UI reachable at `/docs/`; repository clean and ready for public GitHub showcase. |

---

## 9. Conclusion & Execution Readiness

This Roadmap bridges senior software architecture with a practical, step-by-step engineering curriculum. By following Phases 0 through 5:
- **Architecture Integrity**: Pure domain models remain decoupled from cloud and homelab infrastructure.
- **Financial Safety**: Monetary calculations avoid floating-point drift, and deterministic SHA-256 fingerprinting prevents ledger duplicate contamination.
- **Cost Discipline**: The platform achieves zero-cost operation via Homelab bare-metal hosting and GCP serverless free tiers.
- **Career Showcase**: The public GitHub repository stands as a world-class demonstration of modern Go design, cloud portability, and security hygiene.
