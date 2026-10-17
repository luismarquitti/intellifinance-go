## Contexto

O repositório `intellifinance-go` possui uma infraestrutura declarada em `docker-compose.yml` (PostgreSQL 16 + Redis 7) e scripts auxiliares (`scripts/infra-up.sh|ps1`), mas **não existe nenhum caminho de execução local de ponta a ponta**. Evidências no estado atual da branch `origin/dev`:

| Componente | Estado Atual | Evidência Forense no Código |
|---|---|---|
| `cmd/api/main.go` | Stub sem servidor HTTP | [`cmd/api/main.go:6`](cmd/api/main.go#L6) apenas imprime mensagem no console; não escuta porta nem trata sinais. |
| `cmd/worker` | **Inexistente** | Diretório e ponto de entrada não existem; fila de background do `ROADMAP.md` §4 / ADR-005 sem consumidor. |
| `internal/adapters/inbound/http/router.go` | Handlers sem runtime | [`router.go:25`](internal/adapters/inbound/http/router.go#L25) define rotas, mas `/readyz` responde estaticamente sem checar PostgreSQL ou Redis. |
| Migrations | Sem runner automatizado | [`migrations/000001_init_schema.up.sql`](migrations/000001_init_schema.up.sql) existe com RLS forçado, mas nenhum fluxo automatiza sua aplicação local. |
| `.env.example` | Sem carregamento | [`/.env.example`](.env.example) existe, mas nenhum componente da aplicação faz parse de variáveis de ambiente com validação. |
| `Makefile` | **Inexistente** | Não há arquivo de orquestração de desenvolvimento local para unificar subida, migração, seed e encerramento. |
| `scripts/infra-up.sh` | Subida crua de contêineres | Apenas dispara `docker compose up -d`, sem verificação de prontidão, migração ou carga de dados. |

**Consequência**: Um engenheiro, avaliador técnico ou recrutador que clona o repositório não consegue subir a aplicação com facilidade. Não há API respondendo, não há worker escutando filas e não há dados sintéticos de demonstração para validar o funcionamento do sistema.

**Contexto Autoritativo**: `docs/ROADMAP.md` §2-§4 (Fases 0-2), `docs/PRD.md` §1.3 (Dual-Target Operating Paradigm), §3 (Product Architecture), §5 (NFRs), `docs/adrs/ADR-005-asynchronous-queueing.md` e `docs/adrs/ADR-008-public-showcase-security.md`.

---

## Objetivo

Entregar **execução local de um comando só (`make dev-up`)**, orquestrando Docker Compose (PostgreSQL 16 + Redis 7), aplicação idempotente de migrações DDL via biblioteca Go, carga inicial de dados sintéticos determinísticos (seed) em conformidade com PostgreSQL RLS, servidor HTTP REST (`cmd/api`) com graceful shutdown e verificador de prontidão dinâmico em `/readyz` (RFC 7807 em falhas), e daemon assíncrono Asynq (`cmd/worker`) consumindo filas com dreno gracioso de tarefas.

---

## Entregáveis

| # | Entregável | Caminho | Descrição Técnica |
|---|---|---|---|
| **1** | Bootstrap de Configuração Tipado | `internal/config/config.go` | Carrega variáveis de ambiente e `.env` opcional, com validação *fail-fast* (erro se `DATABASE_URL` ausente) e mascaramento de segredos em logs. |
| **2** | Runner de Migrações Idempotente | `internal/bootstrap/migrate.go` | Executa `golang-migrate` programaticamente em Go sobre `migrations/`, tratando `ErrNoChange` como sucesso e garantindo atomicidade. |
| **3** | Verificador de Prontidão (Readiness) | `internal/bootstrap/readiness.go` | Interface `ReadinessChecker` executando pings defensivos (timeout 2s) contra PostgreSQL (`pgxpool.Ping`) e Redis (`redis.Ping`). |
| **4** | Bootstrap Compartilhado de Infraestrutura | `internal/bootstrap/bootstrap.go` | Inicializador reutilizável de pool `*pgxpool.Pool`, conexão Redis, logger estruturado e health checker para API, Worker e Seeder. |
| **5** | Servidor HTTP REST Real com Graceful Shutdown | `cmd/api/main.go` | Monta `router.NewRouter`, atende `/healthz` (liveness HTTP 200) e `/readyz` dinâmico (HTTP 200 quando saudável, HTTP 503 RFC 7807 se dependência cair), escuta em `:8080` e trata `SIGINT`/`SIGTERM` (dreno 15s). |
| **6** | Daemon Assíncrono de Mensageria Asynq | `cmd/worker/main.go` | Inicia servidor `hibiken/asynq` nas filas `critical` (6), `default` (3) e `low` (1), registrando handlers no-op idempotentes para `task:ingest_statement`, `task:categorize_transaction` e `task:sync_open_finance` com graceful shutdown. |
| **7** | Seeder de Demonstração Sintético com RLS | `cmd/seed/main.go` | Popula deterministicamente tenant "Família Marquitti", usuários Luis e Eluma, contas e categorias, operando obrigatoriamente dentro de `postgres.WithTenantTx` para satisfazer `FORCE ROW LEVEL SECURITY`. 100% livre de PII real via `cmd/fixturegen`. |
| **8** | Orquestração Local Completa | `Makefile` | Targets `dev-up`, `dev-down`, `dev-reset`, `dev-seed`, `dev-status`, `dev-logs` e `test-local`. Compila binários previamente e gerencia PIDs em `.pids/` para evitar processos zumbis. |
| **9** | Guia de Desenvolvimento Local | `docs/LOCAL_DEVELOPMENT.md` | Documentação técnica completa: arquitetura local, mapa de portas, resolução de colisões de porta (homelab `lm-claw`), manual de healthchecks e seed. |
| **10** | Seção Quickstart Atualizada | `README.md` | Instruções diretas de início rápido com comando único (`make dev-up`), tabela de endpoints e comandos de parada. |

---

## Invariantes Obrigatórios (Red Lines)

1. **Zero PII Real**: O seeder não pode conter nomes, CPFs, contas bancárias ou valores de pessoas reais. Todos os dados devem ser sintéticos e determinísticos gerados via regras de `cmd/fixturegen` (ADR-008 §7).
2. **Sem Segredos Hardcoded**: Nenhuma credencial em código-fonte; tudo configurado via variáveis de ambiente com template em `.env.example` e `.env` gitignored (`AGENTS.md` §3).
3. **Domínio Hexagonal Puro**: Nenhuma importação de pacotes de adapters, drivers de banco (`pgx`), web frameworks (`chi`) ou clientes de mensageria (`asynq`) dentro de `internal/domain` ou `internal/ports` (`AGENTS.md` §1).
4. **Precisão Monetária Sem Ponto Flutuante (`Zero Floats`)**: Proibição estrita de `float32`/`float64` para valores monetários. Uso obrigatório de `shopspring/decimal.Decimal` no código Go e `NUMERIC(15, 2)` no schema PostgreSQL (conforme PRD §4.1/§7.2 e DDL real de migração).
5. **Conformidade Obrigatória com `FORCE ROW LEVEL SECURITY` no Seeder**: Como a DDL aplica RLS forçado em `accounts`, `categories` e `transactions`, todo insert e select do `cmd/seed/main.go` DEVE ser executado no contexto de `postgres.WithTenantTx`, sob pena de bloqueio pelo PostgreSQL.
6. **Preservação de Volumes em `make dev-down`**: O comando `make dev-down` deve suspender os processos locais e parar os contêineres Docker preservando os volumes (`intellifinance-postgres-data` e `intellifinance-redis-data`). A destruição de dados é restrita ao alvo explícito `make dev-reset`.
7. **Idempotência Estrita**: Executar `make dev-up` ou `make dev-seed` duas ou mais vezes consecutivas não pode falhar, duplicar dados nem violar constraints de banco.
8. **Gestão Limpa de Processos (Anti-Zumbis)**: O Makefile deve compilar os binários em `bin/` antes de dispará-los em background, gravando o PID real do executável em `.pids/` para permitir término gracioso com `kill -TERM`.

---

## Critérios de Aceitação (Gherkin BDD)

```gherkin
Cenário: Execução local do zero em máquina limpa
  Dado um clone limpo do repositório em uma máquina com Docker e Go 1.24+
  Quando eu executo "make dev-up"
  Então os contêineres PostgreSQL e Redis sobem e atingem estado "healthy"
  E as migrações SQL são aplicadas com sucesso até a versão mais recente
  E o seed de demonstração popula tenant "Família Marquitti", usuários, contas e categorias sintéticas
  E o binário "bin/api" inicia escutando em :8080 com PID gravado em ".pids/api.pid"
  E o binário "bin/worker" inicia conectado ao Redis consumindo as filas critical/default/low com PID em ".pids/worker.pid"
  E uma requisição "GET http://localhost:8080/healthz" retorna HTTP 200 com payload '{"status":"UP"}'
  E uma requisição "GET http://localhost:8080/readyz" retorna HTTP 200 com checks de Postgres e Redis "UP"

Cenário: Prontidão falha e reporta RFC 7807 quando o PostgreSQL cai
  Dado que a stack local está em execução saudável
  Quando eu paro o contêiner do PostgreSQL via "docker compose stop postgres"
  Então uma requisição "GET http://localhost:8080/readyz" retorna HTTP 503 Service Unavailable
  E o cabeçalho "Content-Type" é "application/problem+json"
  E o payload indica falha na verificação de "postgres"
  E uma requisição "GET http://localhost:8080/healthz" continua retornando HTTP 200 OK

Cenário: Shutdown gracioso e preservação de volumes
  Dado que api e worker estão rodando em background
  Quando eu executo "make dev-down"
  Então os processos recebem sinal SIGTERM e encerram sem abortar tarefas em voo
  E os arquivos em ".pids/" são removidos
  E os contêineres Docker são finalizados
  E os volumes "intellifinance-postgres-data" e "intellifinance-redis-data" permanecem íntegros no Docker

Cenário: Reset destrutivo explícito
  Dado que a stack local foi executada e possui dados
  Quando eu executo "make dev-reset"
  Então os processos e contêineres são encerrados
  E os volumes "intellifinance-postgres-data" e "intellifinance-redis-data" são completamente excluídos

Cenário: Idempotência de execução consecutiva
  Dado que "make dev-up" já foi executado com êxito
  Quando eu executo "make dev-up" novamente de forma imediata
  Então o comando finaliza com código de saída 0
  E nenhuma mensagem de erro de migração suja (dirty) é emitida
  E não há duplicação de tenants, usuários ou contas de demonstração no banco

Cenário: Variável obrigatória ausente falha rápido (fail-fast)
  Dado que a variável DATABASE_URL está vazia ou ausente
  Quando eu tento iniciar o binário da API
  Então o processo termina imediatamente com código de saída 1
  E a saída de erro especifica a ausência de "DATABASE_URL"
  E nenhuma credencial ou segredo aparece exposto no console
```

---

## Definição de Pronto (Definition of Done)

- [ ] Todos os pacotes compilam estritamente: `go build ./...`
- [ ] Suíte completa de testes passa com detector de corrida: `go test -v -race ./...`
- [ ] Linter oficial sem nenhum alerta: `golangci-lint run`
- [ ] Varredura do Gitleaks limpa: `gitleaks detect --source . --verbose`
- [ ] Validação do ciclo de vida completo executada com sucesso: `make dev-up && make dev-down`
- [ ] Diffs limpos de arquivos temporários, logs ou binários comitados (`AGENTS.md` §5).

---

## Fora de Escopo

- Implementação de adapters cloud para Google Cloud Tasks (`ports.TaskQueue` via Cloud Tasks) — *Phase 3*.
- Integração real com API externa da Pluggy.ai — *Phase 2*.
- Telas de frontend / UI — *Issue de frontend dedicada*.
- Scripts de automação de infraestrutura GCP (Terraform / Cloud Run) — *Phase 3*.

---

## Matriz de Riscos e Mitigações

| Risco | Probabilidade | Impacto | Estratégia de Mitigação Homologada |
|---|---|---|---|
| **Conflito de portas no host (`lm-claw`)** | Média | Alto | `docker-compose.yml` e `internal/config` suportam parametrização de portas via `POSTGRES_PORT` e `REDIS_PORT`; pre-flight check no Makefile avisa se a porta estiver ocupada. |
| **Violação de RLS no Seeder** | Alta | Crítico | O seeder opera obrigatoriamente envelopado por `postgres.WithTenantTx`, definindo `app.current_tenant_id` e `app.current_user_id` em toda mutação. |
| **Processos zumbis órfãos em background** | Alta | Médio | Makefile compila binários em `bin/` previamente e executa o binário final direto com gravação de PID em `.pids/`, assegurando envio correto de `SIGTERM`. |
| **Divergência de precisão decimal `NUMERIC(14,2)` vs `(15,2)`** | Baixa | Médio | Padrão formalizado como `NUMERIC(15, 2)` conforme DDL e PRD §4.1/§7.2, alinhado com `decimal.Decimal` em Go. |
