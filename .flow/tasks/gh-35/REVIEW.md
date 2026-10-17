# Relatório de Revisão e Consolidação do Comitê — [05-review-committee]

- **Task ID**: `gh-35` (Issue #35)
- **Instância Revisora**: `05-review-committee` (Comitê de Revisão Técnica)
- **Crítico Avaliado**: [DEVILS_ADVOCATE.md](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/.flow/tasks/gh-35/DEVILS_ADVOCATE.md)
- **Artefatos Auditados**: [SPEC.md](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/.flow/tasks/gh-35/SPEC.md), [PLAN.md](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/.flow/tasks/gh-35/PLAN.md)
- **Data da Deliberação**: 2026-10-17

---

## 1. Veredito Consolidado das Críticas Adversariais

O Comitê de Revisão analisou formalmente cada um dos 7 pontos de ataque levantados pelo Advogado do Diabo (`04-devils-advocate`), deliberando conforme a tabela a seguir:

| # | Crítica Adversarial | Veredito | Justificativa Sintética (1 linha) | Ação Incorporada nos Artefatos |
|---|---|---|---|---|
| **1** | Conflito `NUMERIC(14,2)` em `AGENTS.md` vs `NUMERIC(15,2)` no PRD e na DDL | **ACEITAR** | A DDL commitada e o PRD §4.1/§7.2 adotam `NUMERIC(15,2)`, que prevalece como verdade de schema sobre o `AGENTS.md`. | Adotar formalmente `NUMERIC(15, 2)` na especificação e registrar tarefa de correção documental em `AGENTS.md` §2.1. |
| **2** | Armadilha de `FORCE ROW LEVEL SECURITY` quebrando o Seeder | **ACEITAR** | No PostgreSQL, RLS forçado bloqueia até mesmo o owner da tabela se as variáveis de sessão não forem ativadas na transação. | Tornar obrigatório o uso do wrapper `postgres.WithTenantTx` em todas as mutações e consultas do `cmd/seed/main.go`. |
| **3** | Ilusão de prontidão: serviços inexistentes quebram o `router.go` | **ACEITAR** | A API não pode exigir serviços completos de Fase 1 sem incorrer em explosão indevida de escopo na Issue #35. | Definir stubs de serviço seguros em `internal/app` para alimentar `RouterConfig`, garantindo que `/healthz` e `/readyz` funcionem sem pânico. |
| **4** | Processos zumbis gerados por `go run` em background no Makefile | **ACEITAR** | O comando `go run` cria um processo pai intermediário cujo PID não corresponde ao do binário final escutando na porta. | Exigir compilação explícita prévia (`go build -o bin/api`) no Makefile antes do disparo em segundo plano, gravando o PID real do executável. |
| **5** | Conflito de portas em máquinas de desenvolvimento/homelab (`lm-claw`) | **ACEITAR** | O host `lm-claw` já opera Postgres e Redis nativos, necessitando de parametrização clara e pre-flight check. | Incluir checagem preventiva de portas no Makefile e documentar o override de `POSTGRES_PORT` e `REDIS_PORT` no `LOCAL_DEVELOPMENT.md`. |
| **6** | Incompatibilidade de drivers no runner do `golang-migrate` | **ACEITAR** | `golang-migrate` requer drivers explicitamente registrados via blank imports e connection string no formato padrão `postgres://`. | Formalizar os blank imports obrigatórios (`database/postgres` e `source/file`) no contrato de `internal/bootstrap/migrate.go`. |
| **7** | Falta de propagação de contexto de tenant nos payloads das tarefas do worker | **ACEITAR** | Tarefas consumidas pelo Asynq operam sob RLS e exigem `TenantID` e `UserID` no payload para permitir abertura de transação válida. | Declarar struct de envelope de payload contendo `TenantID` e `UserID` para todas as tarefas assíncronas do Asynq. |

---

## 2. Detalhamento das Deliberações e Decisões Arquiteturais

### 2.1 Resolução do Conflito de Precisão Numérica (`NUMERIC(14,2)` vs `NUMERIC(15,2)`)
- **Diagnóstico**: O `AGENTS.md` §2.1 prescreve `NUMERIC(14, 2)`. Por outro lado, o documento autoritativo de requisitos de produto (`docs/PRD.md` §4.1 FR-1.3 e §7.2.3) e a DDL física já executada e commitada em [`migrations/000001_init_schema.up.sql`](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/migrations/000001_init_schema.up.sql#L139) fixaram `NUMERIC(15, 2)`.
- **Decisão**: A realidade do banco de dados e do PRD prevalece. O padrão do repositório é e continuará sendo `NUMERIC(15, 2)` (13 dígitos inteiros e 2 decimais), perfeitamente suportado pelo tipo `shopspring/decimal.Decimal` em Go. O Comitê agenda a atualização do `AGENTS.md` §2.1 em PR separado de documentação para alinhar a regra com o código de produção.

### 2.2 Estratégia de Isolamento de Escopo para o Servidor API (`cmd/api`)
- **Diagnóstico**: O roteador `router.go` depende de três interfaces de serviços (`ports.AccountService`, `ports.CategoryService`, `ports.TransactionService`). A implementação dessas interfaces com regras completas de negócio é objeto de outras issues da Fase 1 (`gh-10`, `gh-8`, etc.).
- **Decisão**: Para viabilizar a execução local ponta a ponta sem violação de escopo, a Issue #35 implementará adaptadores mínimos de serviço no pacote de aplicação (`internal/app/stubs.go`), ou instanciará stubs funcionais que delegam consultas básicas ao repositório ou retornam RFC 7807 problem details para rotas não finalizadas. A prioridade máxima do `cmd/api` na Issue #35 é a prontidão dos endpoints `/healthz` e `/readyz` e o graceful shutdown.

### 2.3 Gestão Segura de Processos e PIDs no `Makefile`
- **Diagnóstico**: O disparo de `go run &` gera processos filhos desvinculados, fazendo com que o `make dev-down` mate o processo do compilador mas deixe o binário final rodando em background (processo órfão/zumbi).
- **Decisão**: O `Makefile` adotará o seguinte fluxo determinístico:
  1. Criação do diretório local `.pids/` (ignorado pelo git).
  2. Compilação explícita dos binários em `bin/api` e `bin/worker`.
  3. Inicialização direta dos binários compilados: `./bin/api >> logs/api.log 2>&1 & echo $$! > .pids/api.pid`.
  4. No comando `make dev-down`, o Makefile lê o PID gravado, envia sinal `SIGTERM`, aguarda até 10 segundos para término gracioso e confirma a liberação da porta via `lsof`/`fuser` ou `kill -0`.

### 2.4 Respeito Irrestrito ao RLS nas Operações do Seeder
- **Diagnóstico**: Como as tabelas possuem `ALTER TABLE ... FORCE ROW LEVEL SECURITY;`, qualquer instrução `INSERT` realizada sem as variáveis de sessão ativadas será rejeitada pelo PostgreSQL.
- **Decisão**: O seeder (`cmd/seed/main.go`) utilizará obrigatoriamente a função [`postgres.WithTenantTx`](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/internal/adapters/outbound/persistence/postgres/tx_manager.go#L44-L85), passando o UUID do Tenant de demonstração gerado ou recuperado. Isso garante que as variáveis `app.current_tenant_id` e `app.current_user_id` sejam aplicadas deterministicamente em cada comando.

---

## 3. Aprovação Final para Atualização da Issue e Execução

O Comitê de Revisão declara a especificação técnica [SPEC.md](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/.flow/tasks/gh-35/SPEC.md) e o plano de implementação [PLAN.md](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/.flow/tasks/gh-35/PLAN.md) **formalmente homologados**, estando autorizada a emissão do corpo revisado da issue em [ISSUE_UPDATE.md](file:///home/luismarquitti/.openclaw/worktrees/cd8fbc203fe2d9bc/gh-35-refine-local-exec/.flow/tasks/gh-35/ISSUE_UPDATE.md).
