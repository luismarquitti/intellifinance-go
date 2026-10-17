# Plano de Implementação — [Execução Local] Bootstrap de Execução Local Ponta a Ponta

- **Task ID**: `gh-35` (Issue #35)
- **Implementador Responsável**: `02-code-implementer`
- **Artefato de Especificação**: [SPEC.md](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/.flow/tasks/gh-35/SPEC.md)
- **Status**: Planejamento Finalizado para Execução

---

## 1. Ordem Topológica de Implementação e Grafo de Dependências

A implementação deve seguir rigorosamente uma ordenação topológica para evitar quebras de compilação ou ciclos de dependência. Cada passo baseia-se em entregáveis consolidados no passo anterior:

```
[Passo 1: Dependências Externas (go.mod)]
               │
               ▼
[Passo 2: Configuração Tipada (internal/config)]
               │
               ├──────────────────────────────────────────┐
               ▼                                          ▼
[Passo 3: Migrations (internal/bootstrap/migrate)]  [Passo 4: Bootstrap & Readiness (internal/bootstrap)]
               │                                          │
               ├──────────────────────────────────────────┼─────────────────────────┐
               ▼                                          ▼                         ▼
[Passo 5: Seeder Sintético com RLS]         [Passo 6: Servidor API REST]   [Passo 7: Worker Asynq Daemon]
      (cmd/seed/main.go)                         (cmd/api/main.go)               (cmd/worker/main.go)
               │                                          │                         │
               └──────────────────────────────────────────┴─────────────────────────┘
                                           │
                                           ▼
                            [Passo 8: Orquestração Makefile]
                                           │
                                           ▼
                      [Passo 9: Documentação e Guia de Execução]
                           (docs/LOCAL_DEVELOPMENT.md, README.md)
```

---

## 2. Inventário de Arquivos: Ações e Conteúdo Esperado

| Passo | Caminho do Arquivo | Ação | Conteúdo Esperado |
|---|---|---|---|
| **Passo 1** | [`go.mod`](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/go.mod) / `go.sum` | **Modificar** | Adição das dependências externas: `github.com/hibiken/asynq`, `github.com/golang-migrate/migrate/v4` (com driver postgres/pgx), `github.com/redis/go-redis/v9`, `github.com/joho/godotenv`. |
| **Passo 2** | `internal/config/config.go` | **Criar** | Struct `Config`, `DatabaseConfig`, `RedisConfig`, `AppConfig`. Função `Load()` que lê variáveis de ambiente e `.env` opcional com fail-fast para `DATABASE_URL` e `REDIS_ADDR`. Mascaramento de segredos em logs/prints. |
| **Passo 2** | `internal/config/config_test.go` | **Criar** | Testes unitários cobrindo: carregamento com `.env`, defaults (`PORT=8080`, `ENVIRONMENT=development`), erro em variáveis obrigatórias faltantes. |
| **Passo 3** | `internal/bootstrap/migrate.go` | **Criar** | Função `RunMigrations(databaseURL, migrationsPath string) error` usando `golang-migrate` como biblioteca Go programática, com tratamento de `migrate.ErrNoChange` como sucesso idempotente. |
| **Passo 3** | `internal/bootstrap/migrate_test.go` | **Criar** | Testes de integração/unitários mockando ou executando migrações em banco de teste, validando idempotência de execução consecutiva. |
| **Passo 4** | `internal/bootstrap/readiness.go` | **Criar** | Interface `ReadinessChecker` e struct `ReadinessReport`. Implementação que executa `pool.Ping(ctx)` e `rdb.Ping(ctx)` com timeout de 2 segundos. Retorna `UP` ou `DOWN` por componente. |
| **Passo 4** | `internal/bootstrap/readiness_test.go` | **Criar** | Testes unitários com mocks de ping e teste de timeout do checker sob concorrência (`-race`). |
| **Passo 4** | `internal/bootstrap/bootstrap.go` | **Criar** | Helper consolidado que inicializa `pgxpool`, `redis.Client`, `ReadinessChecker` e `slog.Logger`, compartilhável por API, Worker e Seeder. |
| **Passo 5** | [`internal/adapters/inbound/http/router.go`](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/internal/adapters/inbound/http/router.go) | **Modificar** | Adicionar `ReadinessChecker bootstrap.ReadinessChecker` na struct `RouterConfig`. Modificar o endpoint `/readyz` para chamar o checker e retornar HTTP 200 quando saudável ou HTTP 503 com RFC 7807 Problem Details quando degradado. |
| **Passo 5** | [`internal/adapters/inbound/http/router_test.go`](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/internal/adapters/inbound/http/router_test.go) | **Modificar** | Adicionar testes de `/readyz` cobrindo cenário saudável (HTTP 200) e dependência degradada (HTTP 503 RFC 7807). |
| **Passo 6** | [`cmd/api/main.go`](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/cmd/api/main.go) | **Modificar** | Ponto de entrada real da API: carrega config, inicializa pool pgx e Redis via bootstrap, monta o router Chi, instancia `http.Server`, captura `SIGINT`/`SIGTERM` e executa encerramento gracioso com dreno de conexões (timeout 15s). |
| **Passo 6** | [`cmd/api/main_test.go`](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/cmd/api/main_test.go) | **Modificar** | Teste de fumaça inicializando o servidor em porta efêmera (`:0`), verificando que `/healthz` e `/readyz` respondem. |
| **Passo 7** | `cmd/worker/main.go` | **Criar** | Ponto de entrada do daemon Asynq: carrega config, inicializa `asynq.Server` com filas `critical` (6), `default` (3), `low` (1), registra handlers no-op seguros para `task:ingest_statement`, `task:categorize_transaction` e `task:sync_open_finance`, trata `SIGINT`/`SIGTERM` para shutdown gracioso. |
| **Passo 7** | `cmd/worker/main_test.go` | **Criar** | Teste unitário verificando o registro dos 3 handlers e configuração de filas. |
| **Passo 8** | `cmd/seed/main.go` | **Criar** | Utilitário executável de seed sintético determinístico: conecta ao banco via `pgxpool`, executa operações no envelope `postgres.WithTenantTx` (para conformidade com `FORCE ROW LEVEL SECURITY`), cria tenant "Família Marquitti", usuários Luis e Eluma, contas compartilhada e privada, categorias padrão e 20 transações sintéticas via `cmd/fixturegen`. |
| **Passo 8** | `cmd/seed/main_test.go` | **Criar** | Teste de conformidade verificando idempotência de execução consecutiva e ausência de CPFs/nomes reais. |
| **Passo 9** | `Makefile` | **Criar** | Targets automatizados: `dev-up`, `dev-down`, `dev-reset`, `dev-seed`, `dev-status`, `dev-logs`, `test-local`. Gerenciamento de PIDs em `.pids/api.pid` e `.pids/worker.pid`. |
| **Passo 10** | `docs/LOCAL_DEVELOPMENT.md` | **Criar** | Guia completo de desenvolvimento local: pré-requisitos, passo a passo do `make dev-up`, mapa de portas, troubleshooting de conflitos de porta, manual de healthchecks e seed. |
| **Passo 10** | [`README.md`](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/README.md) | **Modificar** | Atualizar a seção Quickstart com o fluxo de execução local de um comando só (`make dev-up`), links úteis e endpoints de health. |

---

## 3. Dependências Externas (Bibliotecas Go com Versões Exatas e Justificativas)

Para manter a integridade arquitetural do repositório, as seguintes bibliotecas devem ser adicionadas ao `go.mod`:

1. **`github.com/hibiken/asynq v0.25.1`**:
   - *Justificativa*: Biblioteca oficial para processamento distribuído de tarefas em segundo plano sobre Redis, requerida para o Homelab runtime (`cmd/worker`) conforme ADR-005 e ROADMAP §4.
2. **`github.com/redis/go-redis/v9 v9.7.1`**:
   - *Justificativa*: Cliente oficial do Redis utilizado internamente pelo Asynq e necessário para o ping de readiness independente no `internal/bootstrap/readiness.go`.
3. **`github.com/golang-migrate/migrate/v4 v4.18.2`**:
   - *Justificativa*: Framework canônico de migrações DDL em Go, executado como biblioteca programática em Go (sem exigir CLI binário externo pré-instalado na máquina do desenvolvedor).
   - Componentes adicionais: `github.com/golang-migrate/migrate/v4/database/postgres` e `github.com/golang-migrate/migrate/v4/source/file`.
4. **`github.com/joho/godotenv v1.5.1`**:
   - *Justificativa*: Biblioteca leve para parse idempotente do arquivo `.env` local, respeitando variáveis já exportadas no ambiente do processo sem sobrescrevê-las.

---

## 4. Estratégia de Teste por Camada

### Camada 1: Testes Unitários Herméticos (Executáveis 100% Offline e sem Docker)
- **Módulos**: `internal/config`, `internal/bootstrap/readiness_test.go`, `cmd/worker/main_test.go`.
- **Objetivo**: Validar validação de campos, atribuição de valores default, formatação de RFC 7807 Problem Details, registro de roteamento Asynq e fail-fast em variáveis ausentes.
- **Comando**: `go test -v -race ./internal/config/... ./cmd/worker/...`

### Camada 2: Testes de Integração com Banco de Dados e Redis
- **Módulos**: `internal/bootstrap/migrate_test.go`, `cmd/seed/main_test.go`, `internal/adapters/inbound/http/router_test.go`.
- **Objetivo**: Validar execução de migrações reais em contêiner PostgreSQL, conformidade com as regras de `FORCE ROW LEVEL SECURITY` no seed via `WithTenantTx`, idempotência em execuções repetidas e resposta de `/readyz` com pool pgx e Redis reais.
- **Comando**: `go test -v -race ./internal/bootstrap/... ./cmd/seed/...`

### Camada 3: Teste de Fumaça e Verificação de Ciclo de Vida Ponta a Ponta
- **Objetivo**: Validar que um usuário clonando o repositório do zero consegue subir toda a aplicação e operá-la com um único comando.
- **Sequência de Verificação**:
  1. `make dev-up`:
     - Contêineres `intellifinance-postgres` e `intellifinance-redis` sobem e atingem estado `healthy`.
     - Migrações são aplicadas até a versão mais recente.
     - Seeder popula 1 tenant, 2 usuários, 2 contas, categorias e transações sintéticas.
     - Processos `cmd/api` e `cmd/worker` iniciam e registram PIDs em `.pids/`.
  2. Testes de conectividade HTTP:
     - `curl -fsS http://localhost:8080/healthz` retorna `{"status":"UP"}` (HTTP 200).
     - `curl -fsS http://localhost:8080/readyz` retorna status `UP` para Postgres e Redis (HTTP 200).
  3. Teste de degradação:
     - Parar o contêiner do Postgres (`docker compose stop postgres`).
     - `curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/readyz` retorna HTTP 503.
     - `curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/healthz` continua retornando HTTP 200.
     - Reativar o Postgres (`docker compose start postgres`).
  4. Teste de desligamento gracioso:
     - `make dev-down`: Processos recebem `SIGTERM`, encerram sem abortar conexões e contêineres Docker param.
     - Os volumes `intellifinance-postgres-data` e `intellifinance-redis-data` permanecem preservados intactos.

---

## 5. Estimativa de Riscos por Passo e Estratégias de Mitigação

| Passo | Risco Identificado | Nível de Risco | Causa Potencial | Estratégia de Mitigação |
|---|---|---|---|---|
| **Passo 1** | Incompatibilidade de toolchain no `go get` | **Baixo** | Módulo compilado com Go 1.24 e toolchain 1.24.4 | Fixar versões canônicas e compatíveis no `go.mod`. |
| **Passo 3** | Falha de driver no `golang-migrate` | **Médio** | `golang-migrate` requer driver registrado via blank import (`_ "github.com/golang-migrate/migrate/v4/database/postgres"`) | Importar explicitamente os drivers de database e source; testar com URL formatada `postgres://`. |
| **Passo 5** | Serviços de aplicação não implementados quebram `router.go` | **Alto** | `RouterConfig` exige `AccountService`, `CategoryService`, `TransactionService` que ainda não existem como implementações completas no repositório | Criar implementações mínimas/stubs em `internal/app` que delegam para os repositórios existentes ou retornam erro RFC 7807 limpo, permitindo que o router suba e atenda `/healthz` e `/readyz` sem falhas. |
| **Passo 7** | Worker falha ao tentar executar tarefas sem RLS | **Médio** | Asynq executa goroutines isoladas sem contexto de tenant | Handlers iniciais validam `TenantID` no payload da tarefa e documentam a obrigação de usar `WithTenantTx`. |
| **Passo 8** | Violação de `FORCE ROW LEVEL SECURITY` no seeder | **Alto** | Tabelas possuem RLS forçado mesmo para o usuário owner da tabela | O seeder DEVE utilizar obrigatoriamente `postgres.WithTenantTx`, definindo `app.current_tenant_id` e `app.current_user_id` em toda mutação. |
| **Passo 9** | Processos zumbis após `make dev-down` | **Médio** | Execução em background (`&`) sem gravação de PID confiável | Armazenar PIDs em `.pids/api.pid` e `.pids/worker.pid` e enviar sinal gracioso `kill -TERM` com dreno e verificação de vivacidade. |
