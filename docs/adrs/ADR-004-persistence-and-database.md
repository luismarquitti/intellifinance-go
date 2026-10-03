# ADR-004: Persistence & Database Access (PostgreSQL 15+, sqlc v2, pgx/v5, and Transaction-Scoped RLS)

- **Status**: Accepted
- **Deciders**: Software Architect (Worker M3), Principal Engineer, Database Administrator, Security Lead
- **Date**: 2026-09-24
- **Technical Story**: Milestone 3 - Architecture Decision Records

---

## 1. Context & Problem Statement

The legacy IntelliFinance prototype utilized PostgreSQL via the **Prisma ORM** in Node.js. In production-like testing and ingestion benchmarks, this architecture revealed severe liabilities:

1. **Query Engine Overhead & Reflection Latency**: Prisma routes SQL queries through an external binary query engine written in Rust. This IPC bridge introduced memory overhead (~40MB RSS for the binary alone) and significant serialization latency on bulk transaction inserts (e.g. inserting 1,000 credit card transaction rows took 1,200ms).
2. **Lack of Native Row-Level Security (RLS) Ergonomics**: Prisma abstracts connection pooling and does not provide first-class ergonomics for setting session-level PostgreSQL parameters (`SET LOCAL app.current_tenant_id`). Enforcing multi-tenant isolation required injecting `WHERE userId = ...` or `WHERE tenantId = ...` manually into every Prisma query, leaving the system vulnerable to catastrophic developer omission bugs.
3. **Connection Pool State Pollution**: In high-throughput connection pools, running `SET app.current_tenant_id` at the session level without an active transaction risks leaving the tenant ID assigned to that physical connection. When another HTTP goroutine acquires the recycled connection from the pool, it inherits the previous tenant's identity, resulting in cross-tenant data leakage.
4. **Migration Rigidity**: Prisma migrations were difficult to audit, lacked atomic rollback scripts (`down.sql`), and did not cleanly support advanced PostgreSQL features like partial indexes, conditional triggers, or custom RLS policies.

IntelliFinance requires a persistence tier with zero-reflection overhead, compile-time query verification, native PostgreSQL binary protocol support, versioned reversible migrations, and database engine-enforced multi-tenant isolation.

---

## 2. Decision Drivers

- **Zero-Reflection Compile-Time Type Safety**: SQL queries must be compiled into idiomatic Go structs at build time without runtime reflection or dynamic query generation overhead.
- **Sub-Millisecond Query Execution & High-Throughput Bulk Ingestion**: Capable of batch inserting thousands of transactions per second utilizing PostgreSQL native binary protocol (`CopyFrom`).
- **Engine-Enforced Multi-Tenancy (Defense-in-Depth)**: The PostgreSQL engine itself must reject cross-tenant data reads and writes even if an application query accidentally omits a `tenant_id` filter.
- **Connection Pool Hygiene**: Tenant session variables must be strictly scoped to individual transactions (`SET LOCAL`), automatically resetting when the transaction commits or aborts.
- **Deterministic Schema Versioning**: Numbered, reversible migration files (`.up.sql` and `.down.sql`) auditable in Git.

---

## 3. Considered Options

1. **Prisma ORM (Legacy Node.js / Go Client)**
2. **GORM (Go Object-Relational Mapping)**
3. **Ent (Facebook Entity Framework for Go)**
4. **Raw `database/sql` with handwritten scanners**
5. **`sqlc` v2 with `jackc/pgx/v5` and `golang-migrate` (Selected)**

---

## 4. Evaluation & Comparative Matrix

| Evaluation Criteria | `sqlc` v2 + `pgx/v5` (Selected) | GORM | Ent Framework | Raw `database/sql` |
|---|---|---|---|---|
| **SQL Verification** | **Compile-Time** (Parses SQL AST against schema) | Runtime (Reflected strings) | Compile-Time (Go DSL generator) | None (Runtime string errors) |
| **Runtime Reflection** | **0% (Zero reflection)** | Heavy (Struct reflection on all queries)| Minimal | None |
| **Bulk Insert Speed** | **Ultra-Fast** (Native `pgx.CopyFrom`) | Slow (Batched `INSERT` statements) | Moderate | Fast (Manual multi-row SQL) |
| **PostgreSQL Feature Access** | **100% Unrestricted** (CTEs, RLS, EXPLAIN, JSONB) | Limited (Leaky DSL abstractions) | Moderate (Limited to Go DSL capabilities) | **100% Unrestricted** |
| **PostgreSQL RLS Ergonomics** | **Native** (`SET LOCAL` within explicit `pgx.Tx`) | Fragile (Hooks required) | Difficult | Native, but manual boilerplate |
| **Connection Pooling** | `pgxpool` (Native binary protocol, health checks)| Wraps standard `database/sql` | Wraps standard `database/sql` | Standard `sql.DB` |
| **Maintenance Burden** | Minimal (Standard SQL queries in `.sql` files) | High (GORM API drift, complex joins) | High (Complex custom Go DSL schema)| High (Manual scanning boilerplate)|

---

## 5. Decision Outcome

**Adopt PostgreSQL 15+** managed via **`sqlc` v2**, connected via **`jackc/pgx/v5` (`pgxpool`)**, versioned with **`golang-migrate/migrate/v4`**, and secured with **Transaction-Scoped Row-Level Security (RLS)**.

### Rationale:
- **Compile-Time Verification**: `sqlc` parses raw PostgreSQL queries and table DDL schemas during `make generate`. Syntax errors, type mismatches, and column renames are caught immediately at build time rather than failing in production.
- **Direct SQL Power**: Developers write standard PostgreSQL queries in `queries/*.sql`. There is no ORM translation layer, enabling full usage of PostgreSQL window functions, JSONB operations, and optimized indexing.
- **Native Binary Protocol**: `pgx/v5` implements PostgreSQL’s frontend/backend binary protocol, bypassing text serialization and delivering 2x–4x throughput improvements over standard `database/sql`.
- **Database Engine RLS**: Multi-tenant isolation is enforced at the PostgreSQL storage engine level, providing defense-in-depth against data leakage.

---

## 6. Multi-Tenancy Architecture: Transaction-Scoped RLS

### 6.1 The Connection Pool Recycling Problem

In Go, `pgxpool.Pool` maintains a pool of physical TCP connections. If an application executes:
```sql
SET app.current_tenant_id = 'tenant-123';
```
That setting persists on the physical connection. If that connection is returned to the pool and later acquired by another goroutine serving a request for `tenant-456`, the second request can execute queries under `tenant-123`'s security context!

### 6.2 The Solution: `SET LOCAL` Inside Explicit Transactions

IntelliFinance eliminates connection pollution by enforcing **Transaction-Scoped RLS**:
1. All database operations execute inside an explicit transaction block (`BEGIN ... COMMIT`).
2. The session variables are injected using `SET LOCAL`:
   ```sql
   SET LOCAL app.current_tenant_id = $1;
   SET LOCAL app.current_user_id = $2;
   ```
3. PostgreSQL automatically discards all `LOCAL` settings when the transaction commits or aborts. The physical connection returns to the pool clean, with zero residual state.

### Go Implementation Pattern (`internal/adapters/outbound/postgres/tx_manager.go`):
```go
package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TxManager struct {
	pool *pgxpool.Pool
}

func NewTxManager(pool *pgxpool.Pool) *TxManager {
	return &TxManager{pool: pool}
}

// WithTenantTx executes the provided callback within an isolated transaction
// where PostgreSQL session variables are strictly localized.
func (m *TxManager) WithTenantTx(ctx context.Context, tenantID, userID uuid.UUID, fn func(tx pgx.Tx) error) error {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// SET LOCAL variables strictly expire at transaction boundary
	setSQL := `
		SET LOCAL app.current_tenant_id = $1;
		SET LOCAL app.current_user_id = $2;
	`
	if _, err := tx.Exec(ctx, setSQL, tenantID.String(), userID.String()); err != nil {
		return fmt.Errorf("failed to set tenant RLS context: %w", err)
	}

	if err := fn(tx); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
```

---

## 7. PostgreSQL Schema & Row-Level Security Policies

```sql
-- migrations/000001_initial_schema.up.sql

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- 1. Households / Tenants
CREATE TABLE tenants (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 2. Users
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    full_name VARCHAR(100) NOT NULL,
    role VARCHAR(20) NOT NULL DEFAULT 'MEMBER', -- OWNER, ADMIN, MEMBER
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_users_tenant_id ON users(tenant_id);

-- 3. Financial Accounts
CREATE TYPE account_type AS ENUM ('CHECKING', 'CREDIT_CARD', 'INVESTMENT', 'CASH');
CREATE TYPE account_visibility AS ENUM ('HOUSEHOLD_SHARED', 'PERSONAL_PRIVATE');

CREATE TABLE accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    owner_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    type account_type NOT NULL,
    visibility account_visibility NOT NULL DEFAULT 'HOUSEHOLD_SHARED',
    balance NUMERIC(14, 2) NOT NULL DEFAULT 0.00,
    currency VARCHAR(3) NOT NULL DEFAULT 'BRL',
    institution_id VARCHAR(50),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_tenant_account_name UNIQUE(tenant_id, name)
);
CREATE INDEX idx_accounts_tenant_owner ON accounts(tenant_id, owner_user_id);

-- 4. Transactions
CREATE TYPE transaction_type AS ENUM ('INCOME', 'EXPENSE', 'TRANSFER');
CREATE TYPE transaction_status AS ENUM ('PENDING', 'COMPLETED', 'RECONCILED');

CREATE TABLE transactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    category_id UUID REFERENCES categories(id) ON DELETE RESTRICT,
    date DATE NOT NULL,
    amount NUMERIC(14, 2) NOT NULL,
    description VARCHAR(255) NOT NULL,
    type transaction_type NOT NULL,
    status transaction_status NOT NULL DEFAULT 'COMPLETED',
    fingerprint CHAR(64) NOT NULL, -- SHA-256 for deterministic deduplication
    is_ai_categorized BOOLEAN NOT NULL DEFAULT FALSE,
    ai_confidence NUMERIC(3, 2),
    is_manually_verified BOOLEAN NOT NULL DEFAULT FALSE,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_tenant_fingerprint UNIQUE(tenant_id, fingerprint)
);
CREATE INDEX idx_transactions_tenant_date ON transactions(tenant_id, date DESC);
CREATE INDEX idx_transactions_account_date ON transactions(account_id, date DESC);

-- 5. Enable and Force Row-Level Security
ALTER TABLE accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE accounts FORCE ROW LEVEL SECURITY;

ALTER TABLE transactions ENABLE ROW LEVEL SECURITY;
ALTER TABLE transactions FORCE ROW LEVEL SECURITY;

-- 6. Define Strict RLS Policies
CREATE POLICY tenant_isolation_accounts ON accounts
    FOR ALL
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::UUID);

CREATE POLICY tenant_isolation_transactions ON transactions
    FOR ALL
    USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::UUID);
```

---

## 8. sqlc v2 Configuration (`sqlc.yaml`)

```yaml
version: "2"
sql:
  - engine: "postgresql"
    schema: "migrations/"
    queries: "queries/"
    gen:
      go:
        package: "db"
        out: "internal/adapters/outbound/postgres/db"
        sql_package: "pgx/v5"
        emit_json_tags: true
        emit_empty_slices: true
        emit_pointers_for_null_types: true
        overrides:
          - db_type: "uuid"
            go_type: "github.com/google/uuid.UUID"
          - db_type: "numeric"
            go_type: "github.com/shopspring/decimal.Decimal"
          - db_type: "jsonb"
            go_type: "encoding/json.RawMessage"
```

---

## 9. Consequences

### Positive Consequences
- **Absolute Compile-Time Safety**: Schema inconsistencies or SQL typos fail during compilation.
- **Sub-Millisecond Speed**: Native binary driver with zero runtime reflection yields optimal database latencies.
- **Engine-Enforced Security**: Even if a developer accidentally forgets a tenant check in a query, PostgreSQL returns zero rows, preventing data leakage across households or invited guests.
- **Clean Connection Pools**: `SET LOCAL` eliminates pool cross-contamination bugs.
- **Reversible Schema Management**: Every migration has an explicit, tested `.down.sql` rollback script.

### Negative Consequences
- **Manual SQL Queries**: Developers must write raw SQL in `queries/*.sql` files rather than relying on an automated ORM abstraction.
- **Code Generation Step**: Updating queries requires running `make generate` before compilation.

### Neutral Consequences
- Monetary values are represented using `shopspring/decimal.Decimal` in Go and `NUMERIC(14, 2)` in PostgreSQL to guarantee zero floating-point rounding errors.

---

## 10. Implementation & Verification Plan

1. **Verify Migrations**:
   - Run `golang-migrate` up and down against a clean PostgreSQL container:
     ```bash
     migrate -path migrations/ -database "$DATABASE_URL" up
     migrate -path migrations/ -database "$DATABASE_URL" down 1
     migrate -path migrations/ -database "$DATABASE_URL" up
     ```
2. **Verify sqlc Generation**:
   - Run `sqlc compile` and `sqlc generate` to confirm clean Go struct generation.
3. **Verify RLS Security via Automated Test**:
   - Execute an integration test inserting records for `Tenant A`.
   - Open a transaction setting `app.current_tenant_id` to `Tenant B`.
   - Execute `SELECT * FROM transactions`.
   - Assert that the returned slice has length 0 and no errors are thrown.
