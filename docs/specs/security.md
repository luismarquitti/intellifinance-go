# IntelliFinance Technical Specification: Security, Secrets & Repository Hygiene

- **Document ID**: SPEC-006-SECURITY
- **Status**: Authoritative / Production-Ready
- **Compliance Standards**: Brazilian LGPD (Lei 13.709/2018), OWASP Top 10 API Security, Zero-Leak Repository Hygiene
- **Secret Management**: Google Cloud Secret Manager (Production) & Gitignored Environment Variables (Homelab Dev)
- **Scanning Gates**: Pre-Commit Gitleaks & GitHub Actions CI Gate

---

## 1. Zero-Leak Secret Architecture

IntelliFinance enforces strict separation between source code and runtime secrets. No credentials, tokens, or encryption keys are ever committed to version control.

### 1.1 Multi-Tier Secret Injection Model

| Tier | Environment | Secret Source | Injection Mechanism | Access Policy |
|---|---|---|---|---|
| **Local Dev / Homelab** | Physical nodes `lm-claw` & `lm-core` | Local `.env` file (strictly gitignored) | `godotenv` or Docker Compose `env_file` | File mode `0600`; local developer ownership |
| **Serverless Prod** | Google Cloud Run | GCP Secret Manager | Cloud Run direct secret env mapping (`projects/$PROJECT_ID/secrets/$NAME/versions/latest`) | Least-privilege IAM service account (`roles/secretmanager.secretAccessor`) |
| **CI / CD Pipeline** | GitHub Actions | GitHub Repository Secrets | Injected into container runner environment | Read-only during workflow execution |

### 1.2 Canonical `.env.example` Template (Safe for Public Repository)

```env
# ==============================================================================
# IntelliFinance Environment Configuration Template
# All production values must be provisioned via Google Cloud Secret Manager.
# ==============================================================================

# Application Runtime
APP_ENV=development                       # development | test | production
PORT=8080
LOG_LEVEL=debug                           # debug | info | warn | error
SERVER_URL=http://localhost:8080

# Database Connectivity (PostgreSQL 15+)
DATABASE_URL=postgres://intellifinance:dev_password@localhost:5432/intellifinance?sslmode=disable
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=5
DB_CONN_MAX_LIFETIME=5m

# Queue Driver & Broker (asynq | cloudtasks)
QUEUE_DRIVER=asynq
REDIS_URL=redis://localhost:6379/0

# Cloud Tasks Configuration (Production only)
GCP_PROJECT_ID=
GCP_LOCATION=us-central1
GCP_TASKS_QUEUE_NAME=intellifinance-tasks
GCP_SERVICE_ACCOUNT_EMAIL=

# Open Finance (Pluggy.ai)
PLUGGY_ENVIRONMENT=sandbox                # sandbox | production
PLUGGY_CLIENT_ID=dev_pluggy_client_id_placeholder
PLUGGY_CLIENT_SECRET=dev_pluggy_client_secret_placeholder
PLUGGY_WEBHOOK_SECRET=dev_webhook_pre_shared_secret_min_32_chars

# AI Categorization & Document OCR Engine (vertexai | litellm | ollama)
AI_PROVIDER=litellm
LITELLM_API_BASE=http://localhost:4000/v1
OLLAMA_API_BASE=http://localhost:11434
VERTEX_PROJECT_ID=
VERTEX_LOCATION=us-central1
AI_CONFIDENCE_THRESHOLD=0.85

# Security, Cryptography & JWT
JWT_SECRET=dev_insecure_jwt_secret_must_be_32_bytes_long_minimum!
JWT_ACCESS_EXPIRATION=1h
JWT_REFRESH_EXPIRATION=720h
TOKEN_ENCRYPTION_KEY=01234567890123456789012345678901 # 32 bytes for AES-256-GCM
```

### 1.3 GCP Secret Manager Integration Contract (`internal/ports/secrets.go`)

```go
package ports

import (
	"context"
	"fmt"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	secretmanagerpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
)

type SecretManager interface {
	GetSecret(ctx context.Context, name string) (string, error)
}

type GCPSecretManagerAdapter struct {
	client    *secretmanager.Client
	projectID string
}

func NewGCPSecretManagerAdapter(ctx context.Context, projectID string) (*GCPSecretManagerAdapter, error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create secret manager client: %w", err)
	}
	return &GCPSecretManagerAdapter{client: client, projectID: projectID}, nil
}

func (s *GCPSecretManagerAdapter) GetSecret(ctx context.Context, name string) (string, error) {
	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: fmt.Sprintf("projects/%s/secrets/%s/versions/latest", s.projectID, name),
	}
	resp, err := s.client.AccessSecretVersion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to access secret %s: %w", name, err)
	}
	return string(resp.Payload.Data), nil
}
```

---

## 2. Repository Hygiene & Gitleaks Protection

To safeguard the public portfolio repository against accidental commits of personal data or credentials, Gitleaks is installed at both local and remote boundaries.

### 2.1 Custom `.gitleaks.toml` Configuration File

```toml
title = "IntelliFinance Gitleaks Security Policy"

[extend]
useDefault = true

# Rule 1: Brazilian Formatted CPF Detection (Unconditional across all files)
[[rules]]
id = "brazilian-cpf-formatted"
description = "Detected Formatted Brazilian CPF Number"
regex = '''\b\d{3}\.\d{3}\.\d{3}-\d{2}\b'''

# Rule 2: Brazilian Unformatted CPF in Keys/Assignments (including Open Finance taxNumber)
[[rules]]
id = "brazilian-cpf-unformatted"
description = "Detected Unformatted 11-digit Brazilian CPF in Assignment"
regex = '''(?i)(cpf|documento|taxNumber|tax_id|doc_number)\s*[:=]\s*["']?(\d{11})["']?'''
keywords = ["cpf", "documento", "taxnumber", "tax_id", "doc_number"]

# Rule 3: Brazilian Formatted CNPJ Detection
[[rules]]
id = "brazilian-cnpj-formatted"
description = "Detected Formatted Brazilian CNPJ Number"
regex = '''\b\d{2}\.\d{3}\.\d{3}/\d{4}-\d{2}\b'''
keywords = ["cnpj", "empresa", "pj", "fornecedor"]

# Rule 4: Pluggy Client Secret Key
[[rules]]
id = "pluggy-client-secret"
description = "Detected Pluggy API Client Secret"
regex = '''(?i)(pluggy[_-]?secret|pluggy[_-]?client[_-]?secret)\s*[:=]\s*["']?([a-f0-9-]{36})["']?'''
keywords = ["pluggy"]

# Rule 5: Pluggy API / Bearer Token
[[rules]]
id = "pluggy-api-key"
description = "Detected Pluggy API Bearer/Header Key"
regex = '''(?i)(x-api-key|pluggy[_-]?api[_-]?key)\s*[:=]\s*["']?([a-zA-Z0-9_-]{32,})["']?'''
keywords = ["pluggy", "x-api-key"]

# Rule 6: Pre-Shared Webhook Secret
[[rules]]
id = "webhook-pre-shared-secret"
description = "Detected IntelliFinance Webhook Secret"
regex = '''(?i)(webhook[_-]?secret|x-intellifinance-webhook-secret)\s*[:=]\s*["']?([a-zA-Z0-9_\-!@#$%^&*]{20,})["']?'''
keywords = ["webhook-secret", "webhook_secret"]

# Rule 7: Brazilian Bank Account Coordinates (Single-line & Multi-line JSON/YAML)
[[rules]]
id = "brazilian-bank-account-leak"
description = "Detected Brazilian Bank Agency and Account Pair"
regex = '''(?i)(agencia|ag[eê]ncia)\s*[:=]\s*["']?\d{4}["']?[\s,;\n]{1,60}(conta|conta[_-]?corrente)\s*[:=]\s*["']?\d{4,9}-?[\dxX]?["']?'''
keywords = ["agencia", "conta"]

# Allowlist: Strictly scoped to environment templates and synthetic test fixtures
[allowlist]
description = "Allow synthetic fixtures and template documentation"
paths = [
    '''\.env\.example$''',
    '''test/fixtures/synthetic/.*'''
]
stopwords = [
    "000.000.000-00",
    "00.000.000/0001-00"
]
```

### 2.2 Pre-Commit Hook Configuration (`.pre-commit-config.yaml`)

```yaml
repos:
  - repo: https://github.com/gitleaks/gitleaks
    rev: v8.18.2
    hooks:
      - id: gitleaks
        name: Detect Hardcoded Secrets & Brazilian PII
        entry: gitleaks protect --staged --verbose --config=.gitleaks.toml
        language: golang
        stages: [commit]

  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.5.0
    hooks:
      - id: check-added-large-files
        args: ['--maxkb=2048'] # Block statements/PDFs over 2MB
      - id: check-merge-conflict
      - id: detect-private-key
      - id: end-of-file-fixer
      - id: trailing-whitespace
```

### 2.3 GitHub Actions Security Workflow (`.github/workflows/security.yml`)

```yaml
name: Security & Secret Audit Gates

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main ]

jobs:
  secret-scan:
    name: Gitleaks Repository Scan
    runs-on: ubuntu-latest
    steps:
      - name: Checkout Code
        uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Run Gitleaks Action
        uses: zricethezav/gitleaks-action@v8
        with:
          config-path: .gitleaks.toml
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

  govulncheck:
    name: Go Dependency Vulnerability Check
    runs-on: ubuntu-latest
    steps:
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Checkout Code
        uses: actions/checkout@v4

      - name: Install govulncheck
        run: go install golang.org/x/vuln/cmd/govulncheck@latest

      - name: Run govulncheck
        run: govulncheck ./...
```

---

## 3. Synthetic Fixture Generation CLI (`cmd/fixturegen`)

To enable realistic automated integration testing, CI validation, and live public demonstrations without exposing real personal transactions, a dedicated CLI binary (`cmd/fixturegen`) creates synthetic Brazilian financial statements.

### 3.1 Design Principles
1. **Zero Real PII**: No real names, account numbers, or merchant addresses are used.
2. **Valid Brazilian Tax Digits**: CPFs are synthesized using standard modulo-11 check algorithms prefixed with test ranges (`000.xxx.xxx-xx`).
3. **Realistic Financial Dynamics**: Inflows mimic Brazilian bi-monthly or monthly salary credits with standard tax deductions (INSS/IRRF). Outflows feature standard Brazilian merchants (supermarkets, bakeries, utilities, streaming).
4. **Multi-Format Generation**: Emits `.ofx`, `.csv`, `.xlsx`, and synthetic PDF statements.

### 3.2 Fixture Generator Implementation Specification

```go
package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

var sampleMerchants = []struct {
	Name     string
	Category string
	MinVal   float64
	MaxVal   float64
}{
	{"SUPERMERCADO PAO DOURADO", "Alimentacao", 45.0, 480.0},
	{"PADARIA SANTO ANTONIO", "Alimentacao", 12.0, 65.0},
	{"DROGARIA SAO PAULO", "Saude", 25.0, 190.0},
	{"POSTO IPIRANGA COMBUSTIVEL", "Transporte", 100.0, 320.0},
	{"ENEL DISTRIBUICAO SP", "Moradia", 140.0, 380.0},
	{"SABESP AGUA E ESGOTO", "Moradia", 80.0, 220.0},
	{"NETFLIX ASSINATURA", "Lazer", 55.90, 55.90},
	{"SPOTIFY BRASIL", "Lazer", 34.90, 34.90},
}

func main() {
	outDir := flag.String("out", "test/fixtures/csv", "Output directory")
	count := flag.Int("count", 50, "Number of transactions to synthesize")
	flag.Parse()

	_ = os.MkdirAll(*outDir, 0755)
	filePath := filepath.Join(*outDir, "itau_checking_synthetic.csv")
	file, err := os.Create(filePath)
	if err != nil {
		fmt.Printf("failed to create fixture: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	writer.Comma = ';'
	defer writer.Flush()

	// Brazilian bank header: Data;Lançamento;Detalhes;Valor;Saldo
	_ = writer.Write([]string{"Data", "Lançamento", "Detalhes", "Valor", "Saldo"})

	currDate := time.Now().AddDate(0, 0, -*count)
	balance := 10500.00

	// Monthly salary entry
	_ = writer.Write([]string{
		currDate.Format("02/01/2006"),
		"TED RECEBIDA - EMPRESA FICTICIA LTDA",
		"DOC 1001",
		"12500,00",
		fmt.Sprintf("%.2f", balance+12500.00),
	})
	balance += 12500.00

	r := rand.New(rand.NewSource(42)) // Deterministic seed

	for i := 0; i < *count; i++ {
		currDate = currDate.AddDate(0, 0, 1)
		merchant := sampleMerchants[r.Intn(len(sampleMerchants))]
		amt := merchant.MinVal + r.Float64()*(merchant.MaxVal-merchant.MinVal)
		balance -= amt

		_ = writer.Write([]string{
			currDate.Format("02/01/2006"),
			fmt.Sprintf("COMPRA CARTAO - %s", merchant.Name),
			fmt.Sprintf("DOC %06d", r.Intn(999999)),
			fmt.Sprintf("-%.2f", amt),
			fmt.Sprintf("%.2f", balance),
		})
	}

	fmt.Printf("Generated %d synthetic transactions at %s\n", *count, filePath)
}
```

---

## 4. LGPD Compliance & Privacy Safeguards

The platform is engineered to comply with Brazil's General Data Protection Law (**LGPD - Lei 13.709/2018**):

### 4.1 Data Minimization (Art. 6, III)
- Bank credentials (passwords, PINs, OTP codes) are **never transmitted to or stored on** IntelliFinance infrastructure.
- All Open Finance authentication is delegated to Pluggy's PCI-DSS compliant infrastructure.
- Raw uploaded files are processed in ephemeral storage and removed following job ingestion.

### 4.2 Right to Erasure & Crypto-Shredding (Art. 18, VI)
When a user or household requests account deletion under LGPD:
1. **Solo Tenant Hard Delete**: A cascading SQL transaction purges all associated transactions, categories, accounts, ingestion records, and tenant memberships.
2. **Shared Household Crypto-Shredding**: If one user leaves a shared household (e.g., divorce or departure of a family member), deleting their shared transaction rows would destroy the mathematical consistency of historical balance sheets. The system applies **Type-Safe Crypto-Shredding & Anonymization**:
   - The user's private accounts and private transactions (`visibility = 'PRIVATE'`) are permanently hard-deleted.
   - For joint shared transactions (`visibility = 'SHARED'`), `created_by_user_id` is updated to the reserved sentinel UUID `00000000-0000-0000-0000-000000000000` (Anonymized Former Member) or set to `NULL` via `ON DELETE SET NULL`, strictly conforming to the PostgreSQL `UUID` column type.
   - All PII text fields (personal notes, custom descriptions, metadata JSON entries) attached to the departing member are redacted/masked.
   - Any Open Finance tokens, OAuth refresh links, and encrypted credentials belonging to the user are crypto-shredded by deleting their KMS/Secret Manager encryption keys.

```sql
-- Transaction-safe LGPD Anonymization Procedure for Departing Household Member
BEGIN;

-- 1. Hard delete departing user's private financial entries
DELETE FROM transactions
WHERE tenant_id = $1 AND created_by_user_id = $2 AND visibility = 'PRIVATE';

DELETE FROM accounts
WHERE tenant_id = $1 AND owner_user_id = $2 AND visibility = 'PRIVATE';

-- 2. Anonymize shared transaction authorship with Sentinel UUID & strip text PII
UPDATE transactions
SET 
    created_by_user_id = '00000000-0000-0000-0000-000000000000'::UUID,
    notes = NULL,
    metadata = metadata - 'user_notes' - 'personal_tag'
WHERE tenant_id = $1 AND created_by_user_id = $2 AND visibility = 'SHARED';

-- 3. Anonymize transaction split attributions
UPDATE transaction_splits
SET 
    user_id = '00000000-0000-0000-0000-000000000000'::UUID,
    notes = '[Anonymized Former Member]'
WHERE tenant_id = $1 AND user_id = $2;

-- 4. Purge tenant membership
DELETE FROM tenant_members WHERE tenant_id = $1 AND user_id = $2;

COMMIT;
```

### 4.3 PII Masking in Structured Observability (`log/slog`)

Logs streamed to stdout or Google Cloud Logging are filtered by a custom `slog.Handler` that redacts Brazilian CPFs, CNPJs, bank account numbers, and email handles:

```go
package logger

import (
	"context"
	"log/slog"
	"regexp"
)

var (
	cpfMaskRegex   = regexp.MustCompile(`\b\d{3}\.\d{3}\.\d{3}-\d{2}\b`)
	emailMaskRegex = regexp.MustCompile(`([a-zA-Z0-9_\-\.]{2})[a-zA-Z0-9_\-\.]+@([a-zA-Z0-9_\-\.]+)`)
)

type RedactingHandler struct {
	slog.Handler
}

func NewRedactingHandler(next slog.Handler) *RedactingHandler {
	return &RedactingHandler{Handler: next}
}

func (h *RedactingHandler) Handle(ctx context.Context, r slog.Record) error {
	newRecord := slog.NewRecord(r.Time, r.Level, maskString(r.Message), r.PC)
	r.Attrs(func(a slog.Attr) bool {
		newRecord.AddAttrs(maskAttr(a))
		return true
	})
	return h.Handler.Handle(ctx, newRecord)
}

func maskString(s string) string {
	s = cpfMaskRegex.ReplaceAllString(s, "***.***.***-**")
	s = emailMaskRegex.ReplaceAllString(s, "${1}***@$2")
	return s
}

func maskAttr(a slog.Attr) slog.Attr {
	if a.Value.Kind() == slog.KindString {
		return slog.String(a.Key, maskString(a.Value.String()))
	}
	return a
}
```
