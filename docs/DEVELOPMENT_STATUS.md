
## Verificação de 2026-11-11 — Consolidação de Open Finance, Faturas, Migrações e MCP HITL

**Marco atingido: Integração Open Finance (Pluggy), Modelo de Faturas de Cartão de Crédito, Migrador DDL com locks atômicos e Servidor MCP Go com governança HITL entregues e 100% testados.**

### Entregas Realizadas

1. **Migrador DDL Atômico e CLI (`cmd/migrate`) (Issue #46)**:
   - Motor sequencial atômico em `internal/bootstrap/migrate.go` com tabela de controle `schema_migrations`, detecção de dirty state e lock transacional serializado via advisory lock `pg_advisory_lock(847291048291)`.
   - Migrações versionadas:
     - `000001_init_schema.up.sql`: baseline do schema inicial.
     - `000002_open_finance_and_invoices.up.sql`: tabelas `provider_balance_snapshots`, `credit_card_invoices` e coluna `transactions.bill_id` com políticas RLS.
     - `000003_hitl_pending_actions.up.sql`: tabela `pending_actions` com políticas RLS.
   - CLI de migração dedicado `cmd/migrate/main.go` suportando flags `-status`, `-up` e `-version`.
2. **Modelagem de Domínio e Persistência de Faturas e Snapshots (Issue #47)**:
   - Entidades de domínio: `CreditCardInvoice`, `ProviderBalanceSnapshot` e `PendingAction`.
   - Repositórios PostgreSQL com injeção automática de contexto de isolamento RLS (`WithTenantTx`): `InvoiceRepository`, `ProviderBalanceSnapshotRepository` e `PendingActionRepository`.
   - Invariante inegociável da ADR-012: O saldo informado por Open Finance é gravado em snapshots históricos e **nunca** muta diretamente `accounts.balance`, preservando a consistência do ledger contábil.
3. **Conector Open Finance Pluggy em Go (Issue #13)**:
   - Pacote `internal/adapters/outbound/openfinance/pluggy/client.go` implementando `ports.OpenFinanceConnector`.
   - Autenticação com cache de `apiKey` temporária via `POST /auth`, mitigando overhead de rede.
   - Suporte a `CreateConnectToken`, `GetItem`, `FetchAccounts`, `FetchTransactions`, `FetchBills`, `TriggerSync` e `DeleteConnection`.
4. **Servidor MCP Go (`cmd/mcp`) e Endpoints HITL (Issue #48 / ADR-012)**:
   - Servidor MCP nativo sobre stdio (JSON-RPC 2.0).
   - Ferramentas de leitura: `list_accounts`, `get_balances`, `list_transactions`, `list_invoices`, `list_pending_actions`.
   - Ferramentas de proposta com governança HITL: `propose_categorization` gravando em `pending_actions` em estado `PENDING`.
   - Endpoints HTTP no Chi v5 para decisão humana:
     - `GET /api/v1/pending-actions`: listagem de propostas pendentes.
     - `POST /api/v1/pending-actions/{id}/approve`: aprovação e execução atômica da mutação.
     - `POST /api/v1/pending-actions/{id}/reject`: rejeição com motivo auditável.
5. **Qualidade e Confiabilidade**:
   - 445 testes unitários e de integração passando com `-race` detector (zero race conditions, zero flakes).
   - 0 issues no linter oficial (`scripts/local-dev.sh lint`).
   - Binário `bin/api` recompilado e ativo no systemd do `lm-claw` na porta `:8090`.

# Estado de desenvolvimento e ponto de retomada

## Verificação de 2026-10-31 — lm-main / Hermes

**Marco atingido: MVP executável no lm-claw para desenvolvimento e testes locais, com dados sintéticos.** Não é uma entrega de produção nem conclusão de todo o roadmap.

### Base e fontes

- Checkout: `~/workspace/intellifinance-go`.
- Branch de trabalho local: `work/local-development`, baseada em `origin/dev` no commit `d57f944`.
- `main` estava no scaffold `7ccd903`; começar por ela teria descartado o MVP já desenvolvido.
- Foram consultados `AGENTS.md`, `docs/ROADMAP.md`, specs de schema/API/frontend, os planos de #35/#36 e as issues #35/#42/#44 com seus comentários.
- O comentário de planejamento em #35 prioriza o MVP de #42–#44 **sem Redis**. O worker continua na etapa assíncrona posterior.
- O relato histórico de deploy em #44 não foi tratado como estado atual: o host atual é Ubuntu 26.10, sem a infraestrutura Docker descrita naquele relato.
- A PR #45 (`feat/gh-14`) estava aberta na inspeção e **não foi incorporada**. Branches `backup/...` foram inventariadas, não executadas nem importadas.

### Análise contra o roadmap

- **Fase 0:** scaffold, gerador sintético e configuração de CI existem. Havia drift entre Go 1.25 no módulo e Go 1.24/linter v1 na CI; o patch local alinha a configuração ao módulo e ao linter v2.
- **Fase 1 / MVP Dashboard:** domínio, portas, persistência, RLS, parsers, handlers e serviços existem em `dev`. O dashboard React está embutido no binário. Este checkpoint verifica o caminho CSV síncrono completo, não todas as capacidades anunciadas no README original.
- **Fase 2:** worker/Redis e conector Open Finance ainda não estão entregues no caminho validado. Não iniciar essas frentes antes de consolidar o bootstrap local.
- **Fases 3–5:** GCP/serverless, AI/OCR externo, fallback de LLM, Swagger/showcase e demais gates completos permanecem posteriores. A suíte RLS existente passou, mas isso não encerra automaticamente toda a issue #26.

## Correções locais desta retomada

1. **Saldo aplicado duas vezes:** serviços ajustavam o saldo depois de o trigger PostgreSQL já fazê-lo. Regressões reais reproduziram `-100.00 → -200.00` na criação e `-10.50 → -21.00` na importação. A persistência passou a ser o único responsável pelo efeito atômico no saldo. O contrato da porta foi explicitado, os mocks foram alinhados sem remover as asserções e foram acrescentados testes contra PostgreSQL.
2. **Readiness fictícia:** `/readyz` sempre retornava sucesso. Agora usa uma porta de ping com deadline de 2 s, retorna 503 RFC 7807 quando o banco está indisponível e não divulga mensagens privadas do driver. Liveness permanece independente.
3. **Configuração insegura:** removido o fallback de URL com credencial embutida; `DATABASE_URL` é obrigatório nos entrypoints. Bind e CORS padrão são locais e a identidade dev é sintética. Removida a impressão explícita da URL em logs de conexão/seed.
4. **Contagem vazia na UI:** frontend esperava `total_items`; o backend publica `total`. A UI agora usa o contrato real. A falha foi observada e depois aprovada no navegador.
5. **Upload sem limite efetivo:** o buffer multipart não limitava o tamanho da requisição. Adicionado limite total de 15 MiB, cleanup dos temporários e teste que exige HTTP 413. A exceção pontual G120 documenta um falso positivo do scanner e está coberta pelo teste de excesso de tamanho.
6. **Dependências/frontend:** o audit do lockfile apontou dois avisos moderados. Atualização compatível de Vite e postcss-selector-parser, sem trocar a arquitetura; audit final sem vulnerabilidades reportadas.
7. **Operação reproduzível:** Makefile, launcher de serviço de usuário, gate que reprova testes pulados, smoke HTTP e smoke de navegador. Quickstart corrigido, sem referência a `docker-compose.dev.yml` inexistente.
8. **Achados da revisão independente:** a unidade é recriada para usar o `ENV_FILE` atual; conflito de porta não pode ser confundido com readiness e não encerra processo alheio. O listener precisa pertencer ao `MainPID`. A recuperação dos smokes registra escopo antes do upload, descobre IDs após perda de resposta e confirma a limpeza por leitura. Falha persistente em limpar é reportada como `RECOVERY`, nunca como sucesso. O teste real revelou também um falso conflito causado por TCP `TIME_WAIT`, corrigido e coberto por regressão.

Pequenos ajustes mecânicos de lint em fixturegen e mocks de teste foram aplicados e exercitados. O nome português `Dividendos` foi preservado por configuração específica do spellchecker.

## Evidências executadas

- Build TypeScript/Vite e `go build` da API e seeder: aprovados.
- `go mod verify`: todos os módulos verificados.
- `make test`: **198 eventos de teste/casos Go aprovados (incluindo subtestes), 0 falhas e 0 testes pulados**, em 18 pacotes; `-race -count=1` e PostgreSQL real. Inclui troca de conta, transições de status e rollback de batch/update após falha. Pacotes sem arquivos de teste não contam como testes pulados.
- `make test-scripts`: **27 testes aprovados**, também executados pelo alvo `make test`; incluem gate de resultados, operação do serviço, propriedade de porta e falhas injetadas de rede/limpeza.
- `make lint`: **0 issues**; `go vet ./...` e `git diff --check`: aprovados.
- Smoke HTTP: 26 verificações aprovadas. Importados 100 registros de cada fixture em contas distintas; cada reimportação detectou 100 duplicados e inseriu zero. Saldos e KPIs conferidos com precisão decimal. Filtro, ordenação, categoria em lote, edição, exclusão individual/em lote e retorno dos saldos a zero verificados por leitura posterior.
- Smoke Chromium/Playwright: upload/reimportação, paginação, busca/reset, ordenação, categoria inline, edição, exclusão individual/em lote; mutações conferidas pela API. Viewport de 390 px sem overflow do documento e nenhum erro JavaScript de runtime observado.
- Perda real do banco: `/healthz` continuou 200; `/readyz` passou a 503 e retornou a 200 após a recuperação, sem reiniciar a API.
- Seeder executado duas vezes e API reiniciada: continuaram **3 contas, 10 categorias, 0 transações**, sem duplicação dos dados-base no cenário testado.
- Sem `DATABASE_URL`, API encerra com código 1 e mensagem de configuração obrigatória.
- Operação real: processo HTTP alheio respondendo 200 em 8091 foi recusado sem alterar o PID da API corrente; novo arquivo de configuração moveu a unidade para 8091, e a configuração padrão foi restaurada em 8090. Nenhum processo alheio foi encerrado.
- Falhas injetadas sobre a API/PostgreSQL reais: resposta perdida após commit e consulta transitória após upload. Em ambos os cenários o smoke encerrou com erro, recuperou e removeu os registros criados; leitura final confirmou zero transações e saldos zerados.

Artefatos locais ignorados pelo Git: `bin/test-results.jsonl`, `bin/qa/http-smoke.json`, `bin/qa/browser-smoke.json`, `bin/qa/launcher-proof.txt`, `bin/qa/http-fault-proof.json` e `bin/qa/dashboard.png`. A captura mostra dados sintéticos durante o teste; depois deles o banco dev ficou vazio de transações.

O serviço de usuário `intellifinance-dev.service` atende **127.0.0.1:8090**, com PostgreSQL em **127.0.0.1:55432**. Credenciais e cluster ficam fora do repositório. Não houve publicação, push, merge ou encerramento de issues nesta retomada.

## Fechamento da revisão

A revisão estática independente de fechamento foi aprovada, sem novos achados bloqueantes de lógica ou segurança no escopo local/sintético. A revisão não executou o código: os resultados de runtime e testes acima foram executados e conferidos pelo lm-main. Permanecem sugestões não bloqueantes de ampliar os testes de troca de PID durante o readiness, paginação da recuperação e perda da resposta de exclusão após commit.

## Ponto de parada e próxima entrega recomendada

O ambiente já permite desenvolver e testar. Operação: [LOCAL_DEVELOPMENT.md](LOCAL_DEVELOPMENT.md).

**Próxima entrega recomendada:** substituir a detecção de uma única tabela por migrações versionadas, com conta de migração separada da conta de execução, locking e testes de banco vazio/reexecução/falha parcial. Isso fecha uma lacuna explícita do planejamento #35/#42 antes de acrescentar worker ou novas integrações.

Pendências conhecidas, não ativadas automaticamente:

- Revisar o bootstrap/seed para cenários além do caso feliz: o seed atual pode remover usuário conflitante por e-mail e sua idempotência parcial de contas não está totalmente coberta.
- Autenticação/autorização reais antes de qualquer acesso de rede não confiável. O bypass dev e headers de tenant são conveniência local, não uma barreira de segurança de produção.
- Validar edição de categoria nula, visibilidade privada e contratos frontend/backend além dos fluxos exercitados.
- Eliminar aritmética/parsing monetário por `parseFloat` no frontend, conforme os gates completos do plano de frontend.
- A CI remota não foi executada nesta sessão. O alinhamento do workflow foi validado localmente, sem alegar execução verde no GitHub; o job remoto ainda não provisiona o banco para a suíte de integração.
- Unificar as divergências normativas dos documentos (Go mínimo, precisão NUMERIC e escopo dos marcos). Não alterar especificações históricas apenas para fazê-las parecer concluídas.
- Revisar a PR #45 e retomar a fase assíncrona apenas após consolidar a base local.

A implementação atual não equivale a uma auditoria integral de segurança, desempenho, acessibilidade ou produção. Dados reais continuam fora do escopo deste ambiente.

## Piloto pessoal — verificação de 06/11/2026 UTC

**Infraestrutura entregue em pré-carga; uso financeiro pessoal ainda bloqueado, não concluído.** O ambiente de desenvolvimento acima continua exclusivamente sintético. A autorização de dados reais aplica-se somente às instâncias isoladas do piloto, nunca ao bypass dev.

- Pessoal: HTTP `127.0.0.1:8092`, PostgreSQL `127.0.0.1:55433`. QA autenticada separada: HTTP `127.0.0.1:8093`, PostgreSQL `127.0.0.1:55434`. Unidades de usuário habilitadas, quatro serviços ativos e binds loopback conferidos. Nada publicado na LAN/internet.
- Basic/bcrypt global na SPA/API/aliases, identidade fixa, rejeição de origin/Host e spoofing; configuração inconsistente falha fechada. Runtime pessoal não executa seed nem migração automática. Papel sem superuser/BYPASSRLS e sem remoção de identidade/transações pessoais.
- Launchers e seed testados contra a configuração pessoal real: recusa explícita antes de mutação, seguida de leitura confirmando preservação. Suíte PostgreSQL agora também recusa alvo não descartável antes de conectar.
- Importador genérico canônico `cmd/import-ledger`: preview padrão; arquivo original integral em arquivo privado por conteúdo; proveniência por registro; importação SQL atômica e serializada; fingerprints canônicos, repetições legítimas e classificação manual preservadas. Conflito de ID original aborta lote. Testado com dados sintéticos na QA autenticada, inclusive oito replays concorrentes no PostgreSQL. Não resolve cartão, liquidação, transferências ou obrigações e não foi usado para carga pessoal.
- Gates finais: `make test` com 264 eventos Go/casos incluindo subtestes, 20 pacotes, zero falhas/skips; 28 testes operacionais Python. Lint zero issues; build, vet, verificação de módulos e diff check aprovados.
- Chromium real aprovou login válido/inválido/anônimo, navegação, spoofing, filtros e largura móvel de 390 px. Sem capturas pessoais. Logout de aplicação, importação/CRUD financeiro pessoal no browser continuam pendentes.
- Backup pré-carga restaurado em banco novo e conferido por hashes/contagens, com valores NUMERIC tratados como texto decimal exato. A prova cobre identidade/schema: não havia movimentos pessoais para restaurar.
- Inventário e conferência Decimal independentes guardados em área privada. Diferenças de representação, ausência de uma âncora e lacunas de domínio registradas sem PII nos documentos públicos. Não tratar cronogramas antigos ou holerites como novos movimentos de caixa.

O bloqueio inicial de reinício foi resolvido pelo processo interativo às 00:26 UTC de 06/11/2026. O worker validou a prova: dois PIDs PostgreSQL alterados, contagens/hashes iguais, serviços ativos; não repetiu o reinício. A persistência da estrutura/identidade está comprovada, mas dados financeiros ainda não foram carregados. Parada atual: quota Codex atingiu o limiar de 20% antes da próxima etapa extensa. O código final de guardas de proveniência está testado no checkout, mas ainda não substituiu os binários estáveis. Retomar pelo checkpoint privado, sem reexecutar setup de identidade/cluster e sem seed/cleanup pessoal.

Operação, caminhos de credencial (sem conteúdo), recuperação e limites: [PERSONAL_PILOT_OPERATIONS.md](PERSONAL_PILOT_OPERATIONS.md). Lista integral de aceite: [PERSONAL_PILOT_PLAN.md](PERSONAL_PILOT_PLAN.md). Nenhum commit, push, PR ou issue foi criado nesta execução.

### Rodada de 06/11/2026: Transferências Internas Confirmadas (Fatia Vertical de Backend)

Foi implementado um **candidato** de backend para transferências internas. **A revisão Codex de 06/11/2026 reprovou o candidato e bloqueou a implantação**, apesar dos gates iniciais verdes: identificou violações de integridade entre writers genéricos e o par, criação/replay com IDs não persistidos, riscos de concorrência e lacunas de proveniência/arquivo. A lista abaixo registra a implementação inicial, não uma certificação de integridade nem um deploy. As correções estão em andamento e exigem nova revisão e provas reais antes de substituir qualquer binário QA; a instância pessoal permanece inalterada.

**Retomada em 06/11/2026, 14:44 UTC:** os bloqueadores originais receberam correções com regressões reais; o pai reexecutou `make test` (392 eventos incluindo subtestes, sem falhas/skips), lint zero issues, builds API/CLI, módulos e diff check. A revisão independente seguinte confirmou o fechamento dos achados anteriores, mas **reprovou o candidato por dois novos bloqueadores estáticos**, ainda sem reprodução executada: (1) transferência aceita frações de centavo, arredondadas por NUMERIC, quebrando valor não zero e replay; (2) locks `FOR SHARE` de validação de moeda podem entrar em deadlock ao promover locks pelos triggers de saldo em confirmações concorrentes de pares distintos. Não houve deploy QA nem carga pessoal. A próxima correção precisa de RED/GREEN para ambos e revisão da ordem/modalidade de locks em todos os writers envolvidos; não basta trocar `FOR SHARE` isoladamente. Quota Codex consultada às 14:44 UTC: 15% curta/63% semanal; Gemini 99% curta/92% semanal. Execução extensa interrompida para decisão do usuário sobre recorte ou executor, sem fallback automático.

**Verificação de 06/11/2026, 16:56 UTC:** Gemini implementou a correção de centavos e reproduziu o bug por HTTP/PostgreSQL antes da mudança. O pai reexecutou a suíte: **395 eventos Go incluindo subtestes, sem falhas/skips**, e lint zero issues. Valores com fração de centavo são recusados antes da escrita; zeros decimais excedentes sem perda são aceitos. A investigação do deadlock ficou pendente após nova recusa de busca no executor. Não há worker ativo desta rodada nem deploy QA. Após três interrupções por regras de comandos, novas tentativas/ampliações de permissão estão pausadas para discutir uma interface de execução delimitada ou troca de executor autorizada. Base e binário pessoais foram novamente comparados com o snapshot e permanecem iguais. Checkpoint atualizado em `bin/agy-progress/checkpoint.md` (ignorado pelo Git); o checkpoint antigo de 01:54 UTC não representa esta rodada.

- **Domínio Puro (`internal/domain/transfer.go`)**:
  - `ValidateTransferPair`: valida que as transações pertencem ao mesmo tenant, a contas distintas, possuem valores opostos (`legA.Amount = -legB.Amount`) e tipo não `VOID`. Rejeita pares já vinculados a outras transações e proíbe emparelhamento automático.
  - `ConfirmTransferPair`: estabelece o vínculo bidirecional mútuo (`TransferPeerTransactionID`) e marca o tipo de ambas as pernas como `TRANSFER`.
  - `UnlinkTransferPair`: desassociação reversível restaurando os tipos originais (`INCOME`/`EXPENSE`) baseando-se no sinal do montante Decimal, limpando `TransferPeerTransactionID` sem remover registros ou proveniência.
  - `NewInternalTransfer`: construtor de transferência dual-leg nova direta entre contas próprias.
- **Portas e Aplicação (`internal/ports/transfer.go`, `internal/app/transfer_service.go`)**:
  - Contrato de porta `TransferRepository` e serviço de aplicação `TransferService` com métodos `ConfirmTransfer`, `CreateTransfer` e `UnlinkTransfer`.
- **Persistência PostgreSQL com RLS (`internal/adapters/outbound/persistence/postgres/transfer_repository.go`)**:
  - Execução atômica sob `WithTenantTx` com `SET LOCAL app.current_tenant_id`.
  - Ordenação determinística de IDs (`firstID, secondID`) e `SELECT ... FOR UPDATE` para evitar deadlocks de concorrência.
  - Resolução de chave estrangeira de autocitação mútua em duas etapas controladas.
  - Testes reais de integração validando que o fluxo de caixa consolidado (`SummarizeTransactions`) ignora movimentações `TRANSFER`, mantendo receitas e despesas inalteradas enquanto os saldos individuais das contas refletem a transferência via triggers.
  - Preservação estrita de proveniência (`metadata["provenance"]`), IDs literais e carimbos de auditoria.
- **Camada HTTP Chi e RFC 7807 (`internal/adapters/inbound/http/handlers/transfer_handler.go`)**:
  - Endpoints: `POST /transfers/confirm`, `POST /transfers` e `POST /transfers/unlink`.
  - Mapeamento de violações de regra de negócio para RFC 7807 Problem Details (403 Forbidden para violação cross-tenant, 409 Conflict para pernas já vinculadas ou estados conflitantes, 422 Unprocessable Entity para contas idênticas ou divergência de montante).
- **Gates Aprovados**:
  - `make test`: 292 eventos Go aprovados (20 pacotes, zero falhas, zero skips) + 28 testes operacionais Python aprovados.
  - `make lint`: 0 issues.
  - `make build`: binários compilados e bundle Vite SPA gerado com sucesso.
  - `go vet ./...`: 0 avisos.
  - `go mod verify`: todos os módulos verificados.
  - `git diff --check`: 0 issues.
- **Limites e Próximas Etapas**:
  - Cartões de crédito, faturas/obrigações, reconciliação de extratos externos contra fontes bancárias e carga financeira real permanecem como etapas posteriores do plano de piloto.

## Atualização de 11/11/2026: Resolução Definitiva dos Bloqueadores e Suíte Completa Green

Em 11 de Novembro de 2026, os dois bloqueadores estáticos identificados na revisão foram formalmente sanados com TDD (RED/GREEN) e validação mecânica estrita:

1. **Bloqueador 1 (Precisão Decimal de Centavos)**:
   - Rejeição pura no domínio (`IsRepresentableInCents`) e validação 422 no DTO para frações de centavos inferiores a R$ 0,01.
   - Proteção contra overflow numérico além dos limites de `NUMERIC(15,2)` com `MaxMonetaryAmount`.
   - Zeros irrelevantes sem perda de precisão (ex.: `1.000`) continuam aceitos e normalizados.
2. **Bloqueador 2 (Risco de Deadlock em Transferências Concorrentes)**:
   - Em `validateTransferCurrencies`, a consulta de validação de moedas foi corrigida de `ORDER BY id FOR SHARE` para `ORDER BY id ASC FOR NO KEY UPDATE`.
   - Essa ordem ascendente determinística e a modalidade exclusiva eliminam a necessidade de promoção de shared lock durante a execução do trigger de saldo `trg_sync_balance`.
   - Criado teste de estresse concorrente `TestReviewTransferConcurrentDeadlockAvoidance` em `internal/adapters/outbound/persistence/postgres/transfer_deadlock_test.go`, validando 8 goroutines simultâneas em direções opostas (A -> B e B -> A) para `CreateTransfer` e 4 pares concorrentes para `ConfirmTransfer`. Ambas as rotinas serializam perfeitamente com zero deadlocks e conservação estrita de saldo.
3. **Veredito Mecânico Final**:
   - `bash scripts/local-dev.sh test`: **396 eventos Go aprovados** (20 pacotes, zero falhas, zero skips) com `-race` ativado e PostgreSQL isolado.
   - `bash scripts/local-dev.sh lint`: **0 issues**.
   - Builds `bin/api`, `bin/seed`, `bin/import-ledger` e build web React 19 SPA aprovados.
