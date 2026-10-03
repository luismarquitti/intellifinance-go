# Agent Guidelines & Engineering Standards — IntelliFinance

This document governs autonomous coding agents (including Google Jules, Google Antigravity, Claude Code, and GitHub Copilot) working in this repository.

---

## 1. Architectural Invariants (Hexagonal Architecture)

This repository follows **Hexagonal Architecture (Ports and Adapters)** in Go 1.22+:

```
Inbound Adapters (HTTP Chi, Webhooks, CLI)
        │
        ▼ (Calls methods on)
Application & Ports (internal/ports)
        ▲
        │ (Implements domain interfaces)
Outbound Adapters (PostgreSQL sqlc, Asynq, Cloud Tasks, Vertex AI, Pluggy)
```

### Core Architecture Rules:
1. **Domain Isolation**: `internal/domain` contains pure business logic and models. It must NOT import any adapter package (`internal/adapters/...`) or transport framework.
2. **Ports as Contracts**: All external interactions (Database, Message Queues, AI Engines, Storage, Open Finance) are defined as Go interfaces in `internal/ports`.
3. **HTTP Handlers**: Located in `internal/adapters/inbound/http/handlers`. Handlers parse HTTP requests, validate DTOs, call domain services via Ports, and format RFC 7807 problem details on error. **Handlers must NEVER query the database directly.**
4. **Outbound Adapters**: Located in `internal/adapters/outbound/...`. Each adapter implements a port interface.

---

## 2. Financial Precision & Data Integrity

1. **Zero Floats for Currency**: `float32` and `float64` are strictly forbidden for monetary amounts.
   * Use `github.com/shopspring/decimal` (`decimal.Decimal`) for domain financial calculations.
   * In PostgreSQL DDL, use `NUMERIC(14, 2)`.
2. **Deduplication Fingerprint**: Every imported transaction is fingerprinted using canonical SHA-256 over:
   `SHA256(tenant_id | account_id | date(YYYY-MM-DD) | amount(cents) | normalized_description | sequence_index)`
3. **Multi-Tenancy & Row-Level Security (RLS)**:
   * Every database table is scoped by `tenant_id`.
   * Queries executed through `sqlc` rely on transaction-scoped session parameters: `SET LOCAL app.current_tenant_id = '...'`.
   * Cross-tenant data leakage is a critical security vulnerability.

---

## 3. DevSecOps & Zero-Leak PII Policy

1. **Zero Real Financial Data**: Never commit real personal banking statements, paystubs (*holerites*), tax returns, names, or real CPFs to this repository.
2. **Synthetic Fixtures**: All unit, integration, and E2E tests must use synthetic fixtures generated via `cmd/fixturegen` or mock files in `test/fixtures/`.
3. **Secrets Management**:
   * No hardcoded API keys, JWT secrets, or tokens.
   * Local development uses `.env` (ignored by git; documented in `.env.example`).
   * Production loads secrets via Google Cloud Secret Manager.

---

## 4. Dual-Target Environment Awareness

The application supports two execution profiles:
* **`ENVIRONMENT=development` (Homelab MQT_Home)**:
  * Database: PostgreSQL 16 on node `lm-claw` (`192.168.3.10:5432`)
  * Queue: Asynq with Redis on `lm-claw` (`192.168.3.10:6379`)
  * AI: LiteLLM proxy (`lm-claw:4000`) and Ollama (`lm-claw:11434`)
* **`ENVIRONMENT=production` (Google Cloud Platform)**:
  * Runtime: Google Cloud Run (Serverless, scale-to-zero)
  * Queue: Google Cloud Tasks (Push delivery to internal endpoints)
  * AI: Google Vertex AI (Gemini 2.0 Flash via official Go SDK)

---

## 5. Agent Verification Checklist Before Completing Tasks

Before opening a PR or marking an issue complete, every agent must execute and verify:
1. **Tests Pass**: `go test -v -race ./...`
2. **Linting Passes**: `golangci-lint run` (zero warnings allowed)
3. **Clean Diffs**: Ensure no temporary test files, binaries, or credentials were created.
4. **Conventional Commits**: Commit messages must follow `feat:`, `fix:`, `refactor:`, `docs:`, or `test:`.
5. **Issue Reference**: Include `Closes #<issue_number>` in PR descriptions.
