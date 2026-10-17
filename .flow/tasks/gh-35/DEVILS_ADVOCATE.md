# Ataque Técnico Adversarial — [04-devils-advocate / Shadow Judge]

- **Task ID**: `gh-35` (Issue #35)
- **Crítico Técnico**: `04-devils-advocate` (Shadow Judge)
- **Alvo**: [SPEC.md](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/.flow/tasks/gh-35/SPEC.md) e [PLAN.md](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/.flow/tasks/gh-35/PLAN.md)
- **Data da Avaliação**: 2026-10-17

---

## 1. Sumário Executivo do Julgamento

A especificação técnica e o plano de implementação para o bootstrap de execução local (`gh-35`) apresentam uma estrutura sólida e ambiciosa. No entanto, sob uma lente forense adversarial, identificam-se **sete vulnerabilidades críticas**, incluindo contradições explícitas entre documentos fundamentais do repositório, armadilhas silenciosas no motor de banco de dados que quebrarão o seeder em tempo de execução, ilusões de escopo que paralisarão agentes executores e vulnerabilidades de concorrência no ciclo de vida dos processos locais.

Abaixo estão expostos os pontos de ataque detalhados que exigem deliberação obrigatória do Comitê de Revisão antes que qualquer linha de código seja submetida.

---

## 2. Pontos de Ataque e Vulnerabilidades Identificadas

### Ataque 1: A Contradição Flagrante `NUMERIC(14,2)` vs `NUMERIC(15,2)` entre `AGENTS.md`, PRD e DDL Real
- **Localização do Conflito**:
  - [`AGENTS.md` §2.1](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/AGENTS.md): *"In PostgreSQL DDL, use NUMERIC(14, 2)."*
  - [`docs/PRD.md` §4.1 (FR-1.3)](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/docs/PRD.md#L193) e [§7.2 (3)](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/docs/PRD.md#L649): *"All monetary values are defined as PostgreSQL NUMERIC(15, 2) and mapped to shopspring/decimal.Decimal in Go."*
  - [`migrations/000001_init_schema.up.sql` L139, L179, L209](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/migrations/000001_init_schema.up.sql#L139):
    ```sql
    balance NUMERIC(15, 2) NOT NULL DEFAULT 0.00,
    amount NUMERIC(15, 2) NOT NULL,
    ```
- **Argumento Adversarial**:
  Existe uma divergência flagrante entre as regras inegociáveis para agentes (`AGENTS.md`) e a especificação de produto (`PRD.md`), e o código commitado na DDL inicial seguiu o PRD (`NUMERIC(15, 2)`). Se um agente ou validador automatizado conferir o código contra `AGENTS.md`, a DDL existente é considerada violadora! Se o seeder ou novos scripts utilizarem `NUMERIC(14, 2)`, haverá discrepância na precisão suportada (12 inteiros vs 13 inteiros). A especificação `gh-35` não pode se omitir perante esse conflito: ela deve formalizar qual é a autoridade canônica e exigir a correção documental correspondente.

---

### Ataque 2: A Armadilha Fatal do `FORCE ROW LEVEL SECURITY` no Seeder
- **Localização da Vulnerabilidade**:
  - [`migrations/000001_init_schema.up.sql` L450-L463](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/migrations/000001_init_schema.up.sql#L450-L463):
    ```sql
    ALTER TABLE accounts ENABLE ROW LEVEL SECURITY;
    ALTER TABLE accounts FORCE ROW LEVEL SECURITY;
    ALTER TABLE categories ENABLE ROW LEVEL SECURITY;
    ALTER TABLE categories FORCE ROW LEVEL SECURITY;
    ALTER TABLE transactions ENABLE ROW LEVEL SECURITY;
    ALTER TABLE transactions FORCE ROW LEVEL SECURITY;
    ```
  - [`migrations/000001_init_schema.up.sql` L482-L490](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/migrations/000001_init_schema.up.sql#L482-L490):
    ```sql
    CREATE POLICY rls_accounts_tenant_isolation ON accounts
        FOR ALL USING (
            tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::UUID
            AND (visibility = 'SHARED' OR owner_user_id = NULLIF(current_setting('app.current_user_id', true), '')::UUID)
        );
    ```
- **Argumento Adversarial**:
  No PostgreSQL, a diretiva `FORCE ROW LEVEL SECURITY` aplica as políticas de RLS **inclusive para o próprio dono da tabela** (o usuário da aplicação `intellifinance`). Quando o seeder tentar executar `INSERT INTO accounts (...)` ou `SELECT ... FROM accounts` usando uma conexão simples sem configurar os parâmetros de sessão, o comando falhará com erro de violação de política ou retornará 0 linhas!
  Se a especificação do seeder não declarar explicitamente que a conexão DEVE invocar [`postgres.WithTenantTx`](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/internal/adapters/outbound/persistence/postgres/tx_manager.go#L44-L85) passando o `tenantID` e o `userID` da transação, o `cmd/seed/main.go` falhará silenciosamente ou explodirá em tempo de execução.

---

### Ataque 3: A Ilusão de Prontidão da API e a Explosão de Escopo de Serviços Inexistentes
- **Localização da Vulnerabilidade**:
  - [`internal/adapters/inbound/http/router.go` L15-L21](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/internal/adapters/inbound/http/router.go#L15-L21):
    ```go
    type RouterConfig struct {
        AccountService     ports.AccountService
        CategoryService    ports.CategoryService
        TransactionService ports.TransactionService
        Logger             *slog.Logger
        CORSOptions        *cors.Options
    }
    ```
- **Argumento Adversarial**:
  O `router.go` atual exige três implementações completas de serviços (`ports.AccountService`, `ports.CategoryService`, `ports.TransactionService`). No entanto, **nenhum desses serviços existe implementado no repositório**, e nem mesmo `AccountRepository` ou `CategoryRepository` possuem implementações em PostgreSQL!
  Se o agente da Issue #35 tentar implementar todas as regras de negócio de contas, categorias e transações para conseguir instanciar o `RouterConfig`, a tarefa explodirá em escopo, duplicando o trabalho planejado para as issues da Fase 1 (`gh-10`, `gh-8`, etc.).
  Por outro lado, se passar `nil` para o `RouterConfig`, qualquer requisição nas rotas registradas em `router.go` resultará em pânico de desreferência de ponteiro nulo (`nil pointer dereference`). O plano precisa definir expressamente uma estratégia de stubs de serviço seguros ou adaptadores mínimos em `internal/app` para o bootstrap inicial, focando estritamente nos healthchecks.

---

### Ataque 4: Vulnerabilidade de Concorrência e Processos Zumbis na Orquestração com Makefile
- **Localização da Vulnerabilidade**:
  - Proposta de `make dev-up` e `make dev-down` via execução de processos em segundo plano (`&`) com PIDs gravados em `.pids/`.
- **Argumento Adversarial**:
  Executar comandos Go em segundo plano via Makefile através de `go run cmd/api/main.go & echo $$! > .pids/api.pid` possui uma armadilha fatal no Linux:
  O comando `go run` cria **dois processos**: o processo compilador/orquestrador `go` e o binário final resultante em `/tmp/go-build.../exe/main`. O `$$!` captura o PID do comando `go`, não do binário da API! Quando o `make dev-down` executar `kill -TERM $(cat .pids/api.pid)`, o compilador `go` é morto, mas o processo filho real da API **continua rodando como um processo zumbi** mantendo a porta 8080 aberta!
  O plano DEVE exigir a compilação explícita prévia (`go build -o bin/api cmd/api/main.go`) e a execução direta do binário compilado (`bin/api & echo $$! > .pids/api.pid`).

---

### Ataque 5: Conflito de Portas no Host Bare-Metal Homelab (`lm-claw`)
- **Localização da Vulnerabilidade**:
  - [`docker-compose.yml` L11 e L26](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/docker-compose.yml#L11):
    ```yaml
    ports:
      - "${POSTGRES_PORT:-5432}:5432"
      - "${REDIS_PORT:-6379}:6379"
    ```
- **Argumento Adversarial**:
  O ambiente alvo primário de desenvolvimento do IntelliFinance é o servidor físico `lm-claw` (`192.168.3.10`, Debian 13), que já opera como servidor central de banco de dados e mídia do homelab. Se o host já tiver instâncias ativas do PostgreSQL na 5432 e Redis na 6379, um simples `docker compose up -d` sem validação prévia falhará imediatamente com erro de bind: `bind: address already in use`.
  Embora o `docker-compose.yml` suporte overrides por variáveis, a documentação e os scripts devem alertar e fornecer validação de pré-voo (pre-flight checks) no Makefile antes de disparar o compose.

---

### Ataque 6: Incompatibilidade Oculta de Drivers no `golang-migrate`
- **Localização da Vulnerabilidade**:
  - Proposta do runner programático `internal/bootstrap/migrate.go`.
- **Argumento Adversarial**:
  A biblioteca `github.com/golang-migrate/migrate/v4` não registra drivers de banco de dados automaticamente. Ela exige a importação explícita de drivers de banco e fontes via *blank imports*:
  ```go
  import (
      _ "github.com/golang-migrate/migrate/v4/database/postgres"
      _ "github.com/golang-migrate/migrate/v4/source/file"
  )
  ```
  Além disso, o driver `postgres` da `golang-migrate` opera sobre o pacote `database/sql` e espera uma connection string iniciada por `postgres://` ou `postgresql://`. Se a string tiver parâmetros incompatíveis com o `lib/pq` (ou `pgx`), o runner falhará silenciosamente com erro de parse. O plano precisa especificar exatamente os pacotes importados e o formato canônico da connection string.

---

### Ataque 7: Falta de Contexto de Tenant nos Payloads das Tarefas do Worker
- **Localização da Vulnerabilidade**:
  - `cmd/worker/main.go` e [`internal/ports/task_queue.go` L20-L29](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/internal/ports/task_queue.go#L20-L29).
- **Argumento Adversarial**:
  O daemon Asynq consome mensagens serializadas da fila Redis. No entanto, as tarefas (`task:ingest_statement`, `task:categorize_transaction`, `task:sync_open_finance`) operam sobre dados que residem em tabelas com RLS forçado.
  Se os payloads de tarefa do Asynq não incluírem explicitamente os campos `TenantID` e `UserID`, ou se o handler do worker não os propagar para um envelope `WithTenantTx`, o processamento de background falhará ao tentar ler ou gravar no banco. A especificação do worker deve formalizar essa regra de contexto desde a sua concepção.

---

## 3. Conclusão do Julgamento

A especificação `gh-35` é de alta qualidade, mas contém fragilidades ocultas que impediriam a homologação em um pipeline rigoroso de produção. O Comitê de Revisão deve analisar e consolidar formalmente as correções para os sete pontos levantados.
