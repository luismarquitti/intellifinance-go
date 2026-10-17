# Especificação Técnica Formal — [Execução Local] Bootstrap de Execução Local Ponta a Ponta (API + Worker + Migrations + Seed)

- **Task ID**: `gh-35` (Issue #35)
- **Título**: `[Execução Local] Bootstrap de execução local ponta a ponta (API + worker + migrations + seed)`
- **Fase**: `Phase 1 / Phase 2 — Homelab MVP & Local Runtime Orchestration`
- **Arquiteto Responsável**: Archon (`01-spec-architect`)
- **Status**: Revisado e Aprovado pelo Comitê Técnico (`05-review-committee`)
- **Documentos de Referência**:
  - [`docs/PRD.md`](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/docs/PRD.md) (§1.3 Dual-Target Paradigm, §3 Product Architecture, §5 NFRs, §7 Schema Contracts)
  - [`docs/ROADMAP.md`](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/docs/ROADMAP.md) (§2 Phase 0, §3 Phase 1, §4 Phase 2)
  - [`AGENTS.md`](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/AGENTS.md) (Invariantes arquiteturais e checklist de verificação)
  - [`docs/adrs/ADR-005-asynchronous-queueing.md`](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/docs/adrs/ADR-005-asynchronous-queueing.md) (Contrato do dual-driver de mensageria)
  - [`docs/adrs/ADR-008-public-showcase-security.md`](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/docs/adrs/ADR-008-public-showcase-security.md) (§7 Synthetic Data Generation)

---

## 1. Visão Geral, Objetivo e Diagnóstico Forense do Repositório

### 1.1 Objetivo da Especificação
Estabelecer a arquitetura técnica, contratos de interfaces, requisitos funcionais formais no padrão EARS e critérios de verificação determinísticos para entregar a **execução local de comando único (`make dev-up`)** da plataforma **IntelliFinance**. A solução abrange a orquestração do contêiner PostgreSQL 16 e Redis 7, execução programática e idempotente de migrações DDL, carga inicial de dados sintéticos determinísticos (seed) em conformidade com RLS, inicialização do servidor HTTP REST (`cmd/api`) com endpoints de verificação de saúde e prontidão, e inicialização do daemon de processamento em segundo plano Asynq (`cmd/worker`) com dreno gracioso de conexões e sinais.

### 1.2 Diagnóstico do Estado Real do Repositório (Evidências por Arquivo e Linha)
A inspeção forense da árvore de código no branch `origin/dev` atesta os seguintes fatos:

1. [`cmd/api/main.go`](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/cmd/api/main.go#L1-L8): Contém apenas um stub não-funcional (`fmt.Println("IntelliFinance API Engine Initializing...")`), sem servidor HTTP, sem escuta de porta, sem conexão com banco de dados e sem tratamento de sinais de encerramento (`SIGINT`/`SIGTERM`).
2. `cmd/worker`: **Inexistente** na árvore de código. O daemon de processamento assíncrono em segundo plano via Redis/Asynq previsto no `docs/ROADMAP.md` §4 e `docs/adrs/ADR-005-asynchronous-queueing.md` não possui ponto de entrada.
3. [`internal/adapters/inbound/http/router.go`](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/internal/adapters/inbound/http/router.go#L15-L21): Define `RouterConfig` exigindo `ports.AccountService`, `ports.CategoryService` e `ports.TransactionService`. O construtor `NewRouter` [L25-L94] define rotas `/healthz` [L41-L45] e `/readyz` [L47-L51], porém `/readyz` responde estaticamente `{"status":"READY"}` com HTTP 200 sem consultar nem pingar o PostgreSQL ou o Redis.
4. [`internal/adapters/inbound/http/middleware/tenant.go`](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/internal/adapters/inbound/http/middleware/tenant.go#L55-L88): Middleware `TenantContextMiddleware` intercepta `X-Tenant-ID` e `X-User-ID`, retornando HTTP 401 Unauthorized se o header de tenant estiver ausente.
5. [`internal/adapters/outbound/persistence/postgres/connection.go`](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/internal/adapters/outbound/persistence/postgres/connection.go#L10-L28): Expõe `NewConnectionPool(ctx context.Context, connString string) (*pgxpool.Pool, error)` que configura e valida a conectividade com o banco via `pool.Ping(ctx)`.
6. [`internal/adapters/outbound/persistence/postgres/tx_manager.go`](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/internal/adapters/outbound/persistence/postgres/tx_manager.go#L14-L85): Expõe `TxManager` e `WithTenantTx(ctx, pool, tenantID, userID, fn)`, injetando o contexto de tenant no PostgreSQL via `SELECT set_config('app.current_tenant_id', $1, true), set_config('app.current_user_id', $2, true)`.
7. [`migrations/000001_init_schema.up.sql`](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/migrations/000001_init_schema.up.sql#L139-L210): Cria tabelas fundamentais com colunas monetárias em `NUMERIC(15, 2)` e [L450-L473] força PostgreSQL Row-Level Security via `ALTER TABLE ... FORCE ROW LEVEL SECURITY;`.
8. [`docker-compose.yml`](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/docker-compose.yml#L1-L40): Declara serviços `postgres` (porta `${POSTGRES_PORT:-5432}:5432`) e `redis` (porta `${REDIS_PORT:-6379}:6379`), com healthchecks de contêiner configurados.
9. [`.env.example`](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/.env.example#L1-L38): Define template de ambiente com credenciais de desenvolvimento não-sensíveis.
10. [`scripts/infra-up.sh`](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/scripts/infra-up.sh#L1-L9): Executa apenas `docker compose up -d` e `docker compose ps`, sem orquestração de migrations, seed ou inicialização de binários.
11. `Makefile`: **Inexistente** no repositório.

---

## 2. Invariantes Inegociáveis (Red Lines)

1. **Zero PII Real (LGPD Art. 6 & ADR-008 §7)**:
   - Todo dado persistido no banco de desenvolvimento local ou em demonstração pública DEVE ser sintético, derivado exclusivamente do gerador determinístico [`cmd/fixturegen`](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/cmd/fixturegen/main.go) ou de personas literárias fictícias pré-definidas ("Luis Marquitti", "Dra. Eluma Marquitti", "Edson Silva").
   - CPFs devem respeitar rigorosamente a máscara sintética reservada `000.xxx.xxx-xx` calculada via Mod11, sem correspondência com pessoas físicas reais.
2. **Zero Segredos Hardcoded**:
   - É terminantemente proibido inserir senhas, tokens JWT, chaves de API ou segredos criptográficos no código-fonte Go. Toda configuração DEVE ser lida de variáveis de ambiente gerenciadas via arquivo `.env` (ignorado no git) com base no template `.env.example`.
3. **Isolamento de Domínio Hexagonal (Hexagonal Architecture Invariant)**:
   - Os pacotes `internal/domain` e `internal/ports` não devem importar nenhum adaptador de entrada ou saída (`internal/adapters/...`), frameworks web (`chi`), drivers de banco de dados (`pgx`) ou clientes de mensageria (`asynq`).
4. **Precisão Financeira Estrita (`Zero Floats`)**:
   - É estritamente vedado o uso de tipos de ponto flutuante (`float32`, `float64`) para quantias monetárias, saldos de contas ou percentuais de rateio. Toda operação monetária deve utilizar [`github.com/shopspring/decimal`](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/go.mod#L13) (`decimal.Decimal`) e no banco de dados o tipo `NUMERIC(15, 2)`.
5. **Preservação de Volumes em `make dev-down`**:
   - O comando de parada local `make dev-down` deve suspender graciosamente os processos `cmd/api` e `cmd/worker` e parar os contêineres Docker **sem destruir os volumes de dados** (`intellifinance-postgres-data` e `intellifinance-redis-data`). A limpeza destrutiva de volumes é uma ação deliberada e isolada no comando `make dev-reset`.
6. **Idempotência Estrita no Bootstrap**:
   - Executar sucessivamente `make dev-up` ou `go run cmd/seed/main.go` não pode resultar em falhas de integridade referencial, violação de chaves únicas (`uq_tenant_fingerprint`, `tenants_slug_key`) ou duplicação de entidades de demonstração.
7. **Conformidade Obrigatória com `FORCE ROW LEVEL SECURITY` no Seeder**:
   - Dado que a DDL impõe `FORCE ROW LEVEL SECURITY` nas tabelas `accounts`, `categories` e `transactions`, o seeder DEVE obrigatoriamente executar todas as mutações no contexto de uma transação scoped via [`postgres.WithTenantTx`](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/internal/adapters/outbound/persistence/postgres/tx_manager.go#L44-L85), sob pena de bloqueio pelo motor de políticas do PostgreSQL.

---

## 3. Arquitetura da Solução e Ciclo de Vida Local

### 3.1 Diagrama de Bootstrap e Orquestração Local

```
                                  [ make dev-up ]
                                         │
                 ┌───────────────────────┴───────────────────────┐
                 ▼                                               ▼
     [ 1. Docker Compose Up ]                         [ 2. Validate Env ]
  (postgres:16 & redis:7 Alpine)                     (internal/config/config.go)
                 │                                               │
                 ▼                                               │
     [ Wait For Healthchecks ]                                   │
      - pg_isready (5432)                                        │
      - redis-cli ping (6379)                                    │
                 │                                               │
                 ├───────────────────────────────────────────────┘
                 ▼
     [ 3. Run Migrations ]
  (internal/bootstrap/migrate.go)
  - golang-migrate programmatic API
  - migrations/*.sql applied in order
                 │
                 ▼
     [ 4. Run Synthetic Seed ]
        (cmd/seed/main.go)
  - Resolves Demo Tenant & Users
  - Runs inside WithTenantTx (RLS-safe)
  - Creates Accounts, Categories, Txs
                 │
                 ▼
     [ 5. Spawn Daemons in Background ]
                 │
        ┌────────┴────────────────────────┐
        ▼                                 ▼
   [ cmd/api ]                       [ cmd/worker ]
  - HTTP Server on :8080            - Asynq Server on :6379
  - Chi v5 Router                   - Queues: critical/default/low
  - GET /healthz (Liveness)         - Handlers: ingest, categorize, sync
  - GET /readyz (DB+Redis Ping)     - Signal trap: SIGINT / SIGTERM
  - Stores PID in .pids/api.pid     - Stores PID in .pids/worker.pid
```

### 3.2 Componentes e Responsabilidades

1. **Configuração Tipada (`internal/config/config.go`)**:
   - Carrega variáveis de ambiente, lendo opcionalmente o arquivo `.env` do diretório raiz.
   - Aplica valores padrão para ambiente local (`ENVIRONMENT=development`, `PORT=8080`, `POSTGRES_PORT=5432`, `REDIS_PORT=6379`).
   - Valida parâmetros mandatórios (`DATABASE_URL`, `REDIS_ADDR`).
   - Fornece mecanismo *fail-fast* com relatório detalhado de variáveis ausentes sem imprimir valores de segredos em texto plano.

2. **Runner Programático de Migrações (`internal/bootstrap/migrate.go`)**:
   - Executa migrações de banco utilizando a biblioteca `github.com/golang-migrate/migrate/v4`.
   - Lê os arquivos de migração presentes em `migrations/`.
   - Suporta conexão direta com PostgreSQL via driver de conexão padrão, operando de forma atômica e idempotente (`migrate.Up`). Em caso de versão já aplicada (`migrate.ErrNoChange`), trata o retorno como sucesso.

3. **Bootstrap de Conexões e Healthcheck (`internal/bootstrap/bootstrap.go`)**:
   - Inicializa e disponibiliza o pool de conexões PostgreSQL [`*pgxpool.Pool`](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/internal/adapters/outbound/persistence/postgres/connection.go#L10) e a conexão Redis (`*redis.Client` ou via `asynq.Client`).
   - Implementa o verificador de prontidão `ReadinessChecker`, executando queries de ping com timeout contextual defensivo (2 segundos) contra PostgreSQL (`pool.Ping(ctx)`) e Redis (`rdb.Ping(ctx)`).

4. **Servidor HTTP REST (`cmd/api/main.go`)**:
   - Inicializa o `*slog.Logger` estruturado (JSON em produção, texto colorido/legível em desenvolvimento).
   - Instancia o `RouterConfig` e monta o roteador Chi v5 via [`router.NewRouter`](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/internal/adapters/inbound/http/router.go#L25).
   - Registra o endpoint dinâmico `/readyz` que consulta o `ReadinessChecker`:
     - Retorna `HTTP 200 OK` com payload `{"status":"UP","checks":{"postgres":"UP","redis":"UP"}}` quando ambos estão operacionais.
     - Retorna `HTTP 503 Service Unavailable` formatado em conformidade com a RFC 7807 (Problem Details) quando qualquer dependência externa falhar.
   - Inicia o `http.Server` escutando na porta configurada (`:8080`).
   - Intercepta `SIGINT` e `SIGTERM`, executando encerramento gracioso com `server.Shutdown(ctx)` com timeout limite de 15 segundos.

5. **Daemon de Fila em Segundo Plano (`cmd/worker/main.go`)**:
   - Inicializa o `asynq.Server` conectando-se ao Redis configurado.
   - Configura prioridades estritas de filas: `critical` (peso 6), `default` (peso 3), `low` (peso 1) e concorrência máxima de 10 workers simultâneos.
   - Registra os três tipos de tarefas formais declarados em [`internal/ports/task_queue.go`](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/internal/ports/task_queue.go#L11-L18):
     - `task:ingest_statement`
     - `task:categorize_transaction`
     - `task:sync_open_finance`
   - Os handlers iniciais operam com registro estruturado de log (no-op seguro), desserialização de payload e validação de contexto de tenant, retornando sem pânico.
   - Intercepta `SIGINT` e `SIGTERM`, executando `asynqServer.Shutdown()` e aguardando o encerramento das tarefas em voo.

6. **Carga Inicial de Demonstração (`cmd/seed/main.go`)**:
   - Conecta-se ao PostgreSQL utilizando o pool `pgxpool`.
   - Executa mutações dentro do envelope [`postgres.WithTenantTx`](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/internal/adapters/outbound/persistence/postgres/tx_manager.go#L44-L85), ativando as variáveis de sessão `app.current_tenant_id` e `app.current_user_id`.
   - Garante idempotência através de cláusulas `ON CONFLICT DO NOTHING` ou verificação prévia de existência de registros.
   - Popula a seguinte topologia sintética (conforme PRD §2.4):
     - **Tenant 1**: Família Marquitti (`slug: familia-marquitti`)
     - **Usuários**: Luis Marquitti (Role: `OWNER`), Dra. Eluma Marquitti (Role: `ADMIN`)
     - **Contas**:
       - Itaú Conta Conjunta (`CHECKING`, visibilidade `SHARED`, saldo R$ 5.000,00)
       - Nubank Pessoal Eluma (`CREDIT_CARD`, visibilidade `PERSONAL`, saldo R$ 0,00)
     - **Categorias Base**: Alimentação, Transporte, Saúde, Moradia, Lazer, Salário, Receita Comercial.
     - **Transações Sintéticas**: 20 transações sintéticas geradas via regras de `cmd/fixturegen`, com cálculos de saldo atualizados pelo trigger `trg_sync_balance`.

7. **Orquestração Automatizada (`Makefile`)**:
   - `make dev-up`: Executa em sequência: subida dos contêineres Docker, espera ativa de prontidão (healthcheck), aplicação das migrações SQL, execução do seed de dados sintéticos e inicialização em segundo plano dos binários `cmd/api` e `cmd/worker`, registrando seus PIDs em `.pids/api.pid` e `.pids/worker.pid`.
   - `make dev-down`: Envia `SIGTERM` para os processos registrados nos arquivos PID, aguarda o encerramento gracioso e executa `docker compose down` (preservando volumes).
   - `make dev-reset`: Executa `make dev-down`, remove os volumes Docker persistentes (`docker compose down -v`) e limpa a pasta `.pids/`.
   - `make dev-seed`: Executa isoladamente o seeder sintético (`go run cmd/seed/main.go`).
   - `make dev-status`: Exibe o status consolidado dos contêineres Docker e dos processos locais `cmd/api` e `cmd/worker`.
   - `make dev-logs`: Exibe logs dos processos em execução ou dos contêineres.
   - `make test-local`: Executa a suíte de testes unitários e de integração local com verificação de corrida (`go test -race ./...`).

---

## 4. Contratos e Assinaturas Go

### 4.1 Estrutura de Configuração (`internal/config/config.go`)

```go
package config

import (
	"time"
)

type Config struct {
	Environment string         `json:"environment"`
	Port        string         `json:"port"`
	Database    DatabaseConfig `json:"database"`
	Redis       RedisConfig    `json:"redis"`
	App         AppConfig      `json:"app"`
}

type DatabaseConfig struct {
	URL             string        `json:"-"` // Omitido em serialização para evitar vazamento
	MaxOpenConns    int           `json:"max_open_conns"`
	MaxIdleConns    int           `json:"max_idle_conns"`
	ConnMaxLifetime time.Duration `json:"conn_max_lifetime"`
}

type RedisConfig struct {
	Addr     string `json:"addr"`
	Password string `json:"-"` // Omitido em serialização
	DB       int    `json:"db"`
}

type AppConfig struct {
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`
	LogLevel        string        `json:"log_level"`
}

// Load lê variáveis de ambiente e arquivo .env opcional, aplicando defaults e validando campos mandatórios.
func Load() (*Config, error)
```

### 4.2 Verificação de Saúde e Prontidão (`internal/bootstrap/readiness.go`)

```go
package bootstrap

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type ComponentStatus string

const (
	StatusUp   ComponentStatus = "UP"
	StatusDown ComponentStatus = "DOWN"
)

type ReadinessReport struct {
	Status ComponentStatus            `json:"status"`
	Checks map[string]ComponentStatus `json:"checks"`
	Errors map[string]string          `json:"errors,omitempty"`
}

type ReadinessChecker interface {
	Check(ctx context.Context) ReadinessReport
}

func NewReadinessChecker(pool *pgxpool.Pool, rdb *redis.Client) ReadinessChecker
```

### 4.3 Runner de Migrações (`internal/bootstrap/migrate.go`)

```go
package bootstrap

// RunMigrations executa as migrações up de forma idempotente utilizando a biblioteca golang-migrate.
// Se as migrações já estiverem na versão mais recente, retorna nil sem erro.
func RunMigrations(databaseURL, migrationsPath string) error
```

### 4.4 Roteador com Readiness Dinâmico (`internal/adapters/inbound/http/router.go`)

A assinatura atual de `RouterConfig` em [`router.go:15`](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/internal/adapters/inbound/http/router.go#L15-L21) é preservada e estendida com o campo opcional `ReadinessChecker`:

```go
type RouterConfig struct {
	AccountService     ports.AccountService
	CategoryService    ports.CategoryService
	TransactionService ports.TransactionService
	ReadinessChecker   bootstrap.ReadinessChecker // Injetado para verificação dinâmica de /readyz
	Logger             *slog.Logger
	CORSOptions        *cors.Options
}

func NewRouter(cfg RouterConfig) *chi.Mux
```

---

## 5. Requisitos Formais no Padrão EARS

### 5.1 Inicialização e Carregamento de Configuração
- **REQ-EXEC-01**: **Quando** o módulo `internal/config` for executado, o sistema **DEVE** carregar as variáveis declaradas no arquivo `.env` do diretório raiz se o arquivo estiver presente, preservando variáveis já exportadas no ambiente do sistema operacional.
- **REQ-EXEC-02**: **Quando** a configuração for inicializada, **se** a variável de ambiente `DATABASE_URL` não estiver definida ou for uma string vazia, o sistema **DEVE** abortar a inicialização com erro descritivo identificando a variável faltante sem exibir nenhum valor sensível no console.
- **REQ-EXEC-03**: **Quando** a variável `PORT` não estiver definida no ambiente, o sistema **DEVE** assumir o valor padrão `"8080"`.
- **REQ-EXEC-04**: **Quando** a variável `ENVIRONMENT` não estiver definida no ambiente, o sistema **DEVE** assumir o valor padrão `"development"`.
- **REQ-EXEC-05**: **Quando** o método `config.Load()` for chamado em testes unitários sem arquivo `.env`, o sistema **DEVE** validar as variáveis a partir dos valores do ambiente do processo.

### 5.2 Execução de Migrações do Banco de Dados
- **REQ-EXEC-06**: **Quando** a rotina `bootstrap.RunMigrations` for invocada, o sistema **DEVE** aplicar sequencialmente todos os scripts SQL pendentes no diretório `migrations/` até a versão mais recente registrada.
- **REQ-EXEC-07**: **Quando** `bootstrap.RunMigrations` for executada sobre uma base de dados cujas migrações já foram aplicadas na totalidade, o sistema **DEVE** retornar sucesso (`nil`) de forma idempotente, sem lançar erro de migração suja ou `ErrNoChange`.
- **REQ-EXEC-08**: **Quando** uma instrução DDL de migração falhar durante a execução, o sistema **DEVE** retornar o erro encapsulado com contexto sem deixar a tabela de controle de versão de migrações em estado inconsistente.

### 5.3 Carga Inicial de Dados Sintéticos (Seed) e Respeito ao RLS
- **REQ-EXEC-09**: **Quando** o utilitário `cmd/seed/main.go` for executado, o sistema **DEVE** conectar-se ao PostgreSQL e criar o tenant padrão de demonstração (`slug: "familia-marquitti"`, `name: "Família Marquitti"`) se ele ainda não existir.
- **REQ-EXEC-10**: **Quando** o seeder registrar contas e categorias, o sistema **DEVE** executar as operações obrigatoriamente dentro de uma transação scoped via `postgres.WithTenantTx`, definindo `app.current_tenant_id` e `app.current_user_id` na sessão da transação para respeitar as políticas de `FORCE ROW LEVEL SECURITY`.
- **REQ-EXEC-11**: **Quando** o seeder criar usuários, o sistema **DEVE** gerar exclusivamente registros sintéticos sem nomes de pessoas reais, sem CPFs reais e sem e-mails corporativos reais, em conformidade com ADR-008 §7.
- **REQ-EXEC-12**: **Quando** o utilitário de seed for executado repetidas vezes consecutivas (`make dev-seed && make dev-seed`), o sistema **DEVE** manter o estado consistente e finalizar com código de saída 0, sem duplicar registros e sem violar restrições de unicidade.

### 5.4 Servidor HTTP REST e Verificação de Prontidão (`cmd/api`)
- **REQ-EXEC-13**: **Quando** o binário `cmd/api` for iniciado, o sistema **DEVE** vincular o servidor HTTP à porta configurada e atender requisições nos endpoints `/healthz` e `/readyz`.
- **REQ-EXEC-14**: **Quando** uma requisição `GET /healthz` for recebida, o sistema **DEVE** retornar imediatamente `HTTP 200 OK` com payload `{"status":"UP"}`, atestando a vivacidade (liveness) do processo da API independente do estado das dependências externas.
- **REQ-EXEC-15**: **Quando** uma requisição `GET /readyz` for recebida e tanto o PostgreSQL quanto o Redis responderem com sucesso aos pings de conexão em menos de 2 segundos, o sistema **DEVE** retornar `HTTP 200 OK` com payload indicando status `"UP"` para ambos os componentes.
- **REQ-EXEC-16**: **Quando** uma requisição `GET /readyz` for recebida e o PostgreSQL ou o Redis estiver inalcançável ou responder com erro, o sistema **DEVE** retornar `HTTP 503 Service Unavailable`, com cabeçalho `Content-Type: application/problem+json` e payload RFC 7807 contendo o detalhamento de cada dependência degradada.
- **REQ-EXEC-17**: **Quando** o processo `cmd/api` receber um sinal de terminação do sistema operacional (`SIGINT` ou `SIGTERM`), o sistema **DEVE** iniciar o encerramento gracioso, parando de aceitar novas conexões HTTP e concedendo até 15 segundos para finalização das requisições em trânsito antes de liberar os recursos do pool e terminar o processo.

### 5.5 Daemon Assíncrono de Mensageria (`cmd/worker`)
- **REQ-EXEC-18**: **Quando** o binário `cmd/worker` for iniciado, o sistema **DEVE** conectar-se ao Redis configurado e iniciar o servidor `asynq.Server` consumindo as filas `critical`, `default` e `low`.
- **REQ-EXEC-19**: **Quando** o worker for iniciado, o sistema **DEVE** registrar roteadores para as tarefas `task:ingest_statement`, `task:categorize_transaction` e `task:sync_open_finance` com handlers idempotentes que registram log estruturado e não geram pânico.
- **REQ-EXEC-20**: **Quando** o processo `cmd/worker` receber um sinal `SIGINT` ou `SIGTERM`, o sistema **DEVE** invocar o shutdown gracioso do servidor Asynq, aguardando as tarefas em execução finalizarem sem abortá-las abruptamente.

### 5.6 Orquestração Local com Makefile
- **REQ-EXEC-21**: **Quando** o comando `make dev-up` for executado em um ambiente com Docker e Go instalados, o sistema **DEVE** subir a infraestrutura Docker, aplicar as migrações, executar o seed e iniciar os processos `cmd/api` e `cmd/worker` em segundo plano, gravando seus identificadores de processo nos arquivos `.pids/api.pid` e `.pids/worker.pid`.
- **REQ-EXEC-22**: **Quando** o comando `make dev-down` for executado, o sistema **DEVE** enviar sinal `SIGTERM` aos processos registrados em `.pids/`, aguardar seu desligamento gracioso, remover os arquivos de PID e parar os contêineres Docker, **preservando integralmente os volumes de dados**.
- **REQ-EXEC-23**: **Quando** o comando `make dev-reset` for executado, o sistema **DEVE** parar os processos da aplicação, encerrar os contêineres Docker e **destruir os volumes nomeados** `intellifinance-postgres-data` e `intellifinance-redis-data`.
- **REQ-EXEC-24**: **Quando** o comando `make dev-status` for executado, o sistema **DEVE** inspecionar o status do contêiner Docker via `docker compose ps` e a vivacidade dos processos registrados nos arquivos de PID via verificação de sinal `kill -0`.
- **REQ-EXEC-25**: **Quando** `make dev-up` for executado sucessivamente duas vezes em seguida, o sistema **DEVE** completar a segunda execução com código de saída 0 sem corromper o estado do banco, sem gerar migrações duplicadas e sem criar novos processos duplicados concorrentes.

---

## 6. Matriz de Requisitos vs. Caminho de Verificação

| Requisito | Descrição Resumida | Comando Executável de Verificação |
|---|---|---|
| `REQ-EXEC-01` | Carga de `.env` | `go test -v ./internal/config -run TestLoad_FromEnvFile` |
| `REQ-EXEC-02` | Fail-fast em variável faltante | `DATABASE_URL="" go run cmd/api/main.go 2>&1 \| grep -q "DATABASE_URL"` |
| `REQ-EXEC-03` | Default de PORT=8080 | `go test -v ./internal/config -run TestLoad_DefaultPort` |
| `REQ-EXEC-04` | Default de ENVIRONMENT=development | `go test -v ./internal/config -run TestLoad_DefaultEnvironment` |
| `REQ-EXEC-05` | Carregamento sem arquivo `.env` | `go test -v ./internal/config -run TestLoad_ProcessEnv` |
| `REQ-EXEC-06` | Execução de migrações pendentes | `go run -tags 'postgres' ./internal/bootstrap/cmd/migrate-test` ou via teste de integração |
| `REQ-EXEC-07` | Idempotência de migrações | `go test -v ./internal/bootstrap -run TestRunMigrations_Idempotent` |
| `REQ-EXEC-08` | Tratamento de erro em DDL inválida | `go test -v ./internal/bootstrap -run TestRunMigrations_InvalidDDL` |
| `REQ-EXEC-09` | Criação de Tenant sintético no seed | `go run cmd/seed/main.go && psql $DATABASE_URL -c "SELECT slug FROM tenants WHERE slug='familia-marquitti';"` |
| `REQ-EXEC-10` | Respeito ao RLS com `WithTenantTx` | `go test -v ./cmd/seed -run TestSeed_RLSCompliance` |
| `REQ-EXEC-11` | Zero PII em usuários sintéticos | `psql $DATABASE_URL -c "SELECT email, tax_id FROM users;" \| grep -v "000\."` (deve retornar vazio de CPFs reais) |
| `REQ-EXEC-12` | Idempotência estrita do seed | `go run cmd/seed/main.go && go run cmd/seed/main.go` (ambos com exit code 0) |
| `REQ-EXEC-13` | API escutando na porta configurada | `curl -fsS http://localhost:8080/healthz` |
| `REQ-EXEC-14` | Endpoint `/healthz` liveness 200 | `curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/healthz \| grep "200"` |
| `REQ-EXEC-15` | Endpoint `/readyz` 200 com dependências ativas | `curl -fsS http://localhost:8080/readyz \| grep -q '"status":"UP"'` |
| `REQ-EXEC-16` | Endpoint `/readyz` 503 RFC 7807 se Postgres cair | `docker compose stop postgres && curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/readyz \| grep "503"` |
| `REQ-EXEC-17` | Graceful shutdown da API | `kill -TERM $(cat .pids/api.pid) && wait $(cat .pids/api.pid)` (encerra sem pânico) |
| `REQ-EXEC-18` | Worker conectado ao Redis | `asynqmon --redis-url="redis://localhost:6379"` ou logs do worker |
| `REQ-EXEC-19` | Registro dos 3 handlers de tarefas | `go test -v ./cmd/worker -run TestWorker_HandlersRegistered` |
| `REQ-EXEC-20` | Graceful shutdown do Worker | `kill -TERM $(cat .pids/worker.pid) && wait $(cat .pids/worker.pid)` |
| `REQ-EXEC-21` | Execução completa com `make dev-up` | `make dev-up && test -f .pids/api.pid && test -f .pids/worker.pid` |
| `REQ-EXEC-22` | Parada com preservação de volumes | `make dev-down && docker volume inspect intellifinance-postgres-data` |
| `REQ-EXEC-23` | Reset com destruição de volumes | `make dev-reset && ! docker volume inspect intellifinance-postgres-data 2>/dev/null` |
| `REQ-EXEC-24` | Status consolidado dos serviços | `make dev-status` |
| `REQ-EXEC-25` | Idempotência do `make dev-up` duplo | `make dev-up && make dev-up` (exit code 0 em ambas) |

---

## 7. Tratamento de Erros, Resiliência e Degradação

### 7.1 PostgreSQL Indisponível na Inicialização (Boot)
- **Comportamento**: A inicialização de `cmd/api`, `cmd/worker` e `cmd/seed` não deve entrar em loop infinito de reconexão bloqueante. O pool deve utilizar timeout configurado de conexão (5 segundos). Se o banco estiver inacessível, o processo emite log com nível `ERROR`, indicando causa raiz (`connection refused` ou `timeout`) e finaliza imediatamente com código de saída não-zero (`exit 1`).

### 7.2 Redis Indisponível em Runtime
- **Comportamento na API**: O servidor HTTP permanece ativo e responde requisições REST normais que não dependem do Redis. Requisições em `GET /healthz` continuam retornando `HTTP 200 OK`. Requisições em `GET /readyz` retornam `HTTP 503 Service Unavailable`, com `RFC 7807 Problem Details` reportando `checks.redis: "DOWN"`, permitindo que probes de orquestração identifiquem a degradação sem derrubar a API desnecessariamente.
- **Comportamento no Worker**: O daemon Asynq entra em modo defensivo com retentativas e backoff exponencial automático gerenciado pelo cliente Redis. Se a conexão for restabelecida, o consumo das filas é retomado sem necessidade de reinicialização do processo.

### 7.3 Falha Durante Aplicação de Migrações
- **Comportamento**: Se uma instrução SQL na migração falhar (ex: violação de constraint ou erro de sintaxe), a biblioteca `golang-migrate` aborta a transação DDL. A tabela de versão `schema_migrations` sinaliza o estado sujo (`dirty`). O runner do IntelliFinance detecta o estado e emite um alerta explícito indicando a versão problemática e o comando manual para resolução antes de prosseguir.

### 7.4 Colisão de Portas no Host Local (5432, 6379, 8080)
- **Comportamento**: Em máquinas compartilhadas ou servidores bare-metal (como o nó homelab `lm-claw`) que já possuam serviços PostgreSQL ou Redis ativos no host, o desenvolvedor pode sobrescrever as portas no arquivo `.env` através de `POSTGRES_PORT=5433`, `REDIS_PORT=6380` e `PORT=8081`. O `docker-compose.yml` e o `internal/config` leem essas variáveis transparentemente, evitando conflitos de bind (`EADDRINUSE`).

---

## 8. Definição de Pronto (Definition of Done) Verificável por Comando

Para que a implementação da Issue #35 seja declarada concluída em ciclos posteriores, os seguintes comandos devem ser executados com sucesso absoluto:

```bash
# 1. Compilação e tipagem estrita de todos os pacotes e binários
go build ./...

# 2. Execução da suíte completa de testes unitários com detector de corrida
go test -v -race ./...

# 3. Linter oficial com zero avisos ou supressões não autorizadas
golangci-lint run

# 4. Verificação de vazamento de credenciais e PII via Gitleaks
gitleaks detect --source . --verbose

# 5. Teste de ciclo de vida completo de execução local
make dev-up
curl -fsS http://localhost:8080/healthz | grep -q '{"status":"UP"}'
curl -fsS http://localhost:8080/readyz | grep -q '"status":"UP"'
make dev-down
```
