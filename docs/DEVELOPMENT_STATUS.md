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
