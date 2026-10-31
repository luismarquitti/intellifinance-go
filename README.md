# IntelliFinance 🚀

[![CI Pipeline](https://github.com/luismarquitti/intellifinance-go/actions/workflows/ci.yml/badge.svg)](https://github.com/luismarquitti/intellifinance-go/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Architecture: Hexagonal](https://img.shields.io/badge/Architecture-Hexagonal%20Ports%20%26%20Adapters-blueviolet)](docs/adrs/ADR-003-hexagonal-architecture.md)
[![Zero-Leak Security](https://img.shields.io/badge/Security-Gitleaks%20Zero--Leak-success)](docs/adrs/ADR-008-public-showcase-security.md)

> **Enterprise-grade personal and household financial engine written in idiomatic Go 1.22+**, featuring automated Brazilian Open Finance banking sync (Pluggy), resilient offline multi-format ingestion (OFX, CSV, Excel, PDF OCR), multimodal AI transaction categorization via **Gemini 2.0 Flash**, and PostgreSQL Row-Level Security (RLS) multi-tenancy.

---

## Estado de implementação

O MVP validado contém API Go, persistência PostgreSQL com RLS, importação CSV síncrona e dashboard React embutido. **O diagrama e os destaques abaixo descrevem a arquitetura-alvo, não uma confirmação de que todas as integrações já existem.**

- [Estado verificado, correções e próximo ponto de retomada](docs/DEVELOPMENT_STATUS.md)
- [Execução local e testes reproduzíveis](docs/LOCAL_DEVELOPMENT.md)
- O código ativo do MVP está em `dev`; `main` ainda contém apenas o scaffold inicial na inspeção de 2026-10-31.

## 🏛️ Architectural Overview

IntelliFinance follows **Hexagonal Architecture (Ports and Adapters)**, ensuring complete decoupling between pure domain models and infrastructure adapters across both local Homelab and Google Cloud Platform environments.

```
+----------------------------------------------------------------------------------------------------+
|                                      INTELLIFINANCE TOPOLOGY                                       |
+----------------------------------------------------------------------------------------------------+
|                                                                                                    |
|    INBOUND ADAPTERS                                                                                |
|    ├── Chi REST API (OpenAPI 3.0 / RFC 7807)                                                       |
|    ├── Open Finance Webhooks (/open-finance/webhook)                                                |
|    └── Synthetic Fixture Generator (cmd/fixturegen)                                                |
|                                                                                                    |
|                                         │                                                          |
|                                         ▼ (Inbound Ports)                                          |
|    +────────────────────────────────────────────────────────────+                                  |
|    |                      HEXAGONAL DOMAIN                      |                                  |
|    |   • Accounts & Transactions Lifecycle                      |                                  |
|    |   • Deterministic SHA-256 Fingerprinting Deduplication    |                                  |
|    |   • Household Co-Budgeting Engine (50/50 & Proportional)   |                                  |
|    |   • AI Confidence Categorization Scoring                   |                                  |
|    +────────────────────────────────────────────────────────────+                                  |
|                                         ▲                                                          |
|                                         │ (Outbound Ports)                                         |
|                                                                                                    |
|    OUTBOUND ADAPTERS (DUAL-TARGET)                                                                 |
|    ├── Persistence: PostgreSQL 16 via sqlc + pgx/v5 (Row-Level Security)                           |
|    ├── Queueing: Asynq + Redis (Homelab) | Google Cloud Tasks (GCP Serverless)                     |
|    ├── AI Engine: Gemini 2.0 Flash via Vertex AI SDK | LiteLLM & Ollama (Offline fallback)         |
|    ├── Open Finance: Pluggy.ai API (Free Sandbox + Configurable Live)                              |
|    └── Storage: Local Filesystem (/mnt/storage) | Google Cloud Storage (GCS)                       |
+----------------------------------------------------------------------------------------------------+
```

---

## ⚡ Key Highlights & Engineering Features

- **$0.00 Operational Baseline**: Runs locally on bare-metal hardware (Debian 13 / Docker Compose) or in production via Google Cloud Platform serverless free tiers (Cloud Run scale-to-zero, Cloud Tasks, and GenAI App Builder credits).
- **Type-Safe Database Access**: Zero runtime ORM reflection using `sqlc` to compile raw SQL queries directly into type-safe Go structs and queries with `jackc/pgx/v5`.
- **Database-Enforced Multi-Tenancy**: Strict Row-Level Security (RLS) isolating household shared budgets from private personal expenses and business clinic records.
- **Multimodal AI Categorization**: Native PDF and invoice OCR powered by Google's **Gemini 2.0 Flash** via the official `google.golang.org/genai` Go SDK, with automated confidence scoring and human-in-the-loop review queues.
- **Open Finance Integration**: Pluggy.ai integration featuring perpetual free Sandbox bank connectors for public portfolio demonstrations.
- **Zero-Leak DevSecOps**: Enforced Gitleaks CI/CD scanning for Brazilian tax IDs (CPF/CNPJ) and credentials, accompanied by an automated synthetic fixture generator (`cmd/fixturegen`).

---

## 📚 Comprehensive Documentation Suite

| Document | Description |
|---|---|
| [**Product Requirements Document (PRD)**](docs/PRD.md) | Authoritative product requirements, user personas (Luis & Eluma), and 23 Given/When/Then user stories. |
| [**Execution Roadmap**](docs/ROADMAP.md) | 1,800+ lines guided engineering milestones from Phase 0 (Scaffolding) to Phase 5 (Release). |
| [**Architectural Decision Records (ADRs)**](docs/adrs/) | 8 formal ADRs documenting runtime, API, persistence, queueing, Vertex AI, Open Finance, and security. |
| [**Technical Specifications (SPECs)**](docs/specs/) | Detailed specs covering PostgreSQL DDL, REST API endpoints, ingestion parsers, queue state machines, and secrets. |
| [**Legacy Post-Mortem & Security Audit**](docs/legacy_analysis_postmortem.md) | Forensic analysis of legacy Node.js failure modes and git leakage audit mandating this greenfield reboot. |
| [**Agent Guidelines (AGENTS.md)**](AGENTS.md) | Universal engineering standard for autonomous coding agents (Google Jules, Antigravity, Claude Code). |

---

## 📂 Project Structure (Standard Go Layout)

```text
intellifinance-go/
├── cmd/
│   ├── api/          # REST API server entrypoint
│   ├── worker/       # Asynchronous background worker daemon
│   └── fixturegen/   # Synthetic Brazilian financial data generator CLI
├── internal/
│   ├── domain/       # Pure domain models and business entities
│   ├── ports/        # Inbound and outbound Go interface definitions
│   ├── app/          # Use cases and application orchestration services
│   └── adapters/
│       ├── inbound/  # HTTP Chi routes, webhooks, and middlewares
│       └── outbound/ # PostgreSQL sqlc, Asynq, Cloud Tasks, Vertex AI, Pluggy
├── migrations/       # golang-migrate SQL DDL scripts with RLS policies
├── docs/             # Complete architectural documentation, PRD, and ADRs
├── design/stitch/    # UI/UX prototypes and tokens generated via Google Stitch
├── .jules/           # Environment automation scripts for Google Jules
└── .github/          # GitHub Actions CI/CD workflows and security scans
```

---

## 🛠️ Getting Started (Local Development)

### Prerequisites
- Go **1.25+**, a C compiler for `-race`, Node/npm and make.
- PostgreSQL 16 with separate development/test databases and a non-superuser application role.
- golangci-lint **2.15.0** for the checked-in v2 configuration.

### Quickstart on the prepared development host

Use the active `dev` branch (or a working branch based on it), not the initial scaffold on `main`.

```bash
cd ~/workspace/intellifinance-go
source "$HOME/.local/share/intellifinance/tools/env.sh"
make pg-up
make mvp-up
make test
make lint
make smoke
```

The dashboard is served at **http://127.0.0.1:8090** with a synthetic development Admin. It is not exposed to the LAN. The MVP does **not** require Redis or external banking/AI credentials.

See [LOCAL_DEVELOPMENT.md](docs/LOCAL_DEVELOPMENT.md) for private configuration, provisioning on a different host, browser smoke tests, SSH tunneling and service lifecycle. Do not use real financial data in this development environment.

---

## 📄 License
This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
