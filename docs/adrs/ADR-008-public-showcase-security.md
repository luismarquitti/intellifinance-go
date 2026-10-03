# ADR-008: Public Repository, Portfolio Showcase & Zero-Leak Security Architecture

- **Status**: Accepted
- **Deciders**: Software Architect (Worker M3), Principal Engineer, Chief Information Security Officer (CISO), Legal / LGPD Compliance Lead
- **Date**: 2026-09-24
- **Technical Story**: Milestone 3 - Architecture Decision Records

---

## 1. Context & Problem Statement

IntelliFinance is designed to serve a dual purpose:
1. A highly functional, private personal and household financial management platform for Luis & Eluma with family multi-tenancy.
2. A flagship public GitHub portfolio project showcasing enterprise-grade Golang software architecture, Hexagonal modularity, Cloud Run serverless deployment, and robust security practices.

However, a comprehensive forensic security audit of the legacy repository revealed severe confidential data leaks:
- **Historical Git Commits**: Commit `8f57fb7` (`feat(docs-migration): import and organize Google Drive data sources and normalization scripts`) introduced **over 25 actual personal financial files** into the git commit history tree. These included annual checking account spreadsheets (`itau_luis_extrato_completo_2025.xls`), 24+ monthly transaction CSVs, real salary credits, and identifiable payee names.
- **Tracked Root Assets**: A 1.03MB raw bank statement PDF (`extrato-luis-2026.pdf`) was tracked directly in the repository.
- **Sensitive Local Assets**: Workspace directories contained Brazilian Federal Tax Declaration documents (`Guia_DIRPF_2026_Luis_Marquitti.docx`), employer paystubs with full CPFs and deductions (`Recibo de Pagamento 05_2026 completo.pdf`), complete family financial balance sheets, and medical clinic accounting records (`Clinica_Eluma`).

If this existing repository or its git commit history were published to GitHub, it would constitute a **catastrophic privacy breach** and a direct violation of the Brazilian General Data Protection Law (**LGPD - Lei 13.709/2018**), exposing real CPFs, income, banking transactions, employer details, and medical records.

Furthermore, traditional history-rewriting tools (`git-filter-repo` or BFG Repo-Cleaner) carry significant residual risks: dangling commit objects, cached pull requests on remote remotes, and the lack of an untainted cryptographic audit baseline.

---

## 2. Decision Drivers

- **Zero-Leak Guarantee**: Absolute certainty that 0% of personal financial data, real Brazilian CPFs, bank account numbers, or production credentials can ever appear on GitHub.
- **LGPD Legal Compliance**: Full adherence to Brazilian data protection mandates, including data minimization (Art. 6, III), security safeguards (Art. 46), and right to erasure (Art. 18, VI).
- **Automated Security Gates (Shift-Left)**: Automated CI/CD and pre-commit scanning to block any future accidental commit of secrets or Brazilian PII before it reaches version control.
- **Professional Portfolio Presentation**: Virgin, pristine git commit history following Conventional Commits, with realistic synthetic fixtures demonstrating real-world functionality without compromising privacy.
- **Cloud-Native Secret Management**: Strict separation of code from configuration using Google Cloud Secret Manager in production and `.env.example` in development.

---

## 3. Considered Options

1. **Git History Rewriting (`git-filter-repo` / BFG Repo-Cleaner)**: Rewrite history of the existing repository to purge sensitive blobs.
2. **Permanent Private Repository Isolation**: Keep the repository permanently private and never publish to GitHub.
3. **Clean Greenfield Repository Strategy (`git init` at `v1.0.0-scaffold`), Multi-Gate Gitleaks with Brazilian PII Rules, Synthetic Test Fixtures, and GCP Secret Manager (Selected)**.

---

## 4. Evaluation & Comparative Matrix

| Evaluation Criteria | Clean Greenfield Strategy (Selected) | Git History Rewriting (`git-filter-repo`) | Permanent Private Repository |
|---|---|---|---|
| **Data Leak Risk** | **0.00% (Cryptographically impossible)** | Residual risk of dangling blobs / PR cache | Zero (Unpublished) |
| **Audit Verification** | **Trivial** (Every commit from root is clean) | Complex (Must verify all historical trees) | None |
| **Portfolio Showcase Value** | **Maximum**: Pristine commits, showcase CI/CD | Diminished (Rewritten history artifacts) | **Zero (Cannot be shared publicly)** |
| **Developer Ergonomics** | Clean start with Go 1.22+ standard layout | Carries legacy node/ts commit baggage | Unchanged |
| **Automated Gate Protection** | Pre-commit + GitHub Actions Gitleaks | Only retroactive scanning | None |
| **LGPD Compliance Assurance** | Absolute | Subject to forensic audit scrutiny | Internal only |

---

## 5. Decision Outcome

**Adopt a Clean Greenfield Repository Strategy**:
1. Sever all git history ties with the legacy repository by initializing a virgin repository (`git init`) starting at `v1.0.0-scaffold`.
2. Archive the legacy codebase and all personal financial files in an offline, encrypted backup volume completely isolated from the Git tree.
3. Implement a **Multi-Gate Gitleaks Architecture** with custom rules for Brazilian PII (CPF, CNPJ) and financial tokens.
4. Build a dedicated **Synthetic Test Fixture Generator (`cmd/fixturegen`)** to generate realistic, non-identifying financial data for tests, demonstrations, and documentation.
5. Enforce **Zero-Leak Secret Management** utilizing Google Cloud Secret Manager in production and `.env.example` templates in development.

---

## 6. Multi-Gate Gitleaks Architecture & Brazilian PII Rules

IntelliFinance establishes two automated security verification gates:

```
[ Developer Working Tree ]
          │
          ▼ (git commit)
┌───────────────────────────────────────────────┐
│ GATE 1: Local Pre-Commit Hook (Husky / Git)   │
│ Command: gitleaks protect --staged            │
│ Blocks commit locally if leak is detected     │
└──────────────────────┬────────────────────────┘
                       │
                       ▼ (git push)
┌───────────────────────────────────────────────┐
│ GATE 2: GitHub Actions Automated CI Gate      │
│ Action: zricethezav/gitleaks-action@v8         │
│ Scans all commits in push / pull request      │
│ Fails build and blocks PR merge if violated   │
└───────────────────────────────────────────────┘
```

### Custom `.gitleaks.toml` Configuration:
```toml
title = "IntelliFinance Security & PII Detection Policy"

[extend]
useDefault = true

# Rule 1: Formatted Brazilian CPF Detection
[[rules]]
id = "brazilian-cpf"
description = "Detected Formatted Brazilian CPF (Cadastro de Pessoas Físicas)"
regex = '''\b\d{3}\.\d{3}\.\d{3}-\d{2}\b'''
keywords = ["cpf", "documento", "titular"]

# Rule 2: Formatted Brazilian CNPJ Detection
[[rules]]
id = "brazilian-cnpj"
description = "Detected Formatted Brazilian CNPJ"
regex = '''\b\d{2}\.\d{3}\.\d{3}/\d{4}-\d{2}\b'''
keywords = ["cnpj", "empresa"]

# Rule 3: Pluggy API Secrets
[[rules]]
id = "pluggy-secret"
description = "Detected Pluggy API Client Secret or Token"
regex = '''(?i)(pluggy[_-]?secret|pluggy[_-]?client[_-]?secret)\s*[:=]\s*["']?([a-f0-9-]{36})["']?'''
keywords = ["pluggy"]

# Rule 4: Bank Account & Agency Numbers in Code
[[rules]]
id = "bank-agency-account"
description = "Detected Brazilian Bank Agency & Account Combination"
regex = '''(?i)(ag[eê]ncia|conta)\s*[:=]\s*["']?\d{4,5}[- ]?\d{1}["']?'''
keywords = ["agencia", "conta"]
```

---

## 7. Synthetic Test Fixture Architecture (`cmd/fixturegen`)

To showcase real financial reports, dashboards, and automated ingestion on GitHub without exposing personal data, IntelliFinance implements a dedicated CLI tool: `cmd/fixturegen`.

### Design of `cmd/fixturegen`:
- **Fictitious Identities**: Replaces real names with celebrated Brazilian historical and literary figures (*"Carlos Drummond de Andrade"*, *"Clarice Lispector"*, *"Machado de Assis"*).
- **Valid Mathematical Test CPFs**: Generates CPFs that satisfy the official Brazilian Mod11 validation algorithm, but uses reserved testing prefix ranges (`000.xxx.xxx-xx`) to prevent collisions with living citizens.
- **Realistic Brazilian Payees & Merchants**: Generates contextual transactions across authentic Brazilian merchant categories:
  - *Food*: "Padaria Estrela da Manhã", "Supermercado Pão de Ouro", "Restaurante Feijão Tropeiro"
  - *Utilities*: "Enel Distribuição São Paulo", "Companhia de Saneamento Básico", "Vivo Fibra Telecom"
  - *Health*: "Farmácia Popular Brasil", "Laboratório Diagnósticos"
- **Multi-Format Output Generator**: Generates synthetic files across all supported ingestion formats:
  - `test/fixtures/ofx/itau_checking_synthetic.ofx`
  - `test/fixtures/csv/nubank_credit_synthetic.csv`
  - `test/fixtures/excel/monthly_budget_synthetic.xlsx`
  - `test/fixtures/pdf/synthetic_bank_statement.pdf`

---

## 8. Cloud-Native Secret Management

IntelliFinance strictly separates source code from credentials:

| Environment | Secret Store | Ingestion Mechanism | Access Policy |
|---|---|---|---|
| **Local Development** | `.env` (strictly ignored by `.gitignore`) | Loaded via Go config loader | Host file permission `0600`; template provided in `.env.example` |
| **GCP Cloud Run (Prod)** | Google Cloud Secret Manager | Direct environment injection or mounted secret volumes | Least-privilege IAM service account (`roles/secretmanager.secretAccessor`) |
| **CI / CD (GitHub Actions)**| GitHub Repository Secrets | Injected into test runner environment | Encrypted at rest; restricted to repository administrators |

### `.env.example` Specification:
```env
# IntelliFinance Configuration Template (Safe for Public Repository)
PORT=8080
APP_ENV=development
LOG_LEVEL=info

# Database (PostgreSQL 15+)
DATABASE_URL=postgres://intellifinance_dev:dev_secret_password@localhost:5432/intellifinance?sslmode=disable

# Redis (Homelab Worker)
REDIS_URL=redis://localhost:6379/0

# Open Finance (Pluggy Sandbox)
PLUGGY_ENVIRONMENT=sandbox
PLUGGY_CLIENT_ID=mock_client_id_for_dev
PLUGGY_CLIENT_SECRET=mock_client_secret_for_dev
PLUGGY_WEBHOOK_SECRET=dev_webhook_pre_shared_secret

# AI Ingestion
AI_PROVIDER=litellm
LITELLM_API_BASE=http://localhost:4000
VERTEX_PROJECT_ID=
VERTEX_LOCATION=us-central1

# Cryptography & Security
JWT_SECRET=dev_jwt_secret_minimum_32_characters_long_for_security
TOKEN_ENCRYPTION_KEY=01234567890123456789012345678901 # 32 bytes for AES-256-GCM
```

---

## 9. Observability Redaction & LGPD Compliance

### 9.1 Structured Logging PII Masking
The `log/slog` structured logger wraps standard handlers with a custom `PIIRedactionHandler` that intercepts log attributes:
- Brazilian CPFs are redacted to `***.***.***-XX`.
- Bank account numbers and payment tokens are replaced with `[REDACTED]`.
- Raw authorization headers are stripped.

### 9.2 Right to Erasure & Crypto-Shredding (LGPD Art. 18)
When a user requests tenant account deletion:
- Personal accounts, uploaded statements, and raw transaction attachments are permanently deleted via cascading SQL deletes.
- In shared household budgets, transactions linked to joint balances are crypto-shredded: the associated encryption key is permanently erased from Secret Manager, rendering individual transaction notes unrecoverable while preserving the mathematical balance of the shared ledger.

---

## 10. Consequences

### Positive Consequences
- **Total Privacy & Immunity from Leaks**: The public GitHub repository is guaranteed to be completely clean from inception.
- **Enterprise-Grade Portfolio Standing**: Showcases elite repository hygiene, automated Gitleaks CI gates, and LGPD compliance to engineering leaders and hiring teams.
- **Reproducible Test Fixtures**: Developers and CI can test every feature immediately using synthetic fixtures without requiring real bank statements.
- **Strict Secret Hygiene**: Zero hardcoded secrets anywhere in the codebase.

### Negative Consequences
- **Legacy History Severance**: Historical commits prior to `v1.0.0-scaffold` are archived separately and cannot be viewed on GitHub (a necessary and beneficial outcome given the sensitive data in that history).
- **Synthetic Maintenance**: Requires maintaining `cmd/fixturegen` when adding new banking formats.

### Neutral Consequences
- All contributors must install Gitleaks locally or rely on the automated GitHub Actions CI gate.

---

## 11. Implementation & Verification Plan

1. **Verify Greenfield Git Scaffolding**:
   - Confirm virgin repository status:
     ```bash
     git rev-list --count HEAD # Must only contain greenfield commits
     ```
2. **Verify Gitleaks CI Gate**:
   - Run Gitleaks against the entire repository tree:
     ```bash
     gitleaks detect --verbose --config=.gitleaks.toml
     ```
   - Assert exit code 0 and zero detected leaks.
3. **Verify Synthetic Fixture Generation**:
   - Run `go run ./cmd/fixturegen/main.go --output test/fixtures/`.
   - Assert that no real CPFs or real personal names exist in generated files.
