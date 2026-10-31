# Desenvolvimento local — MVP Dashboard

## Escopo e limites

O caminho validado é **API Go + SPA embutida + PostgreSQL 16**. Redis, worker, Pluggy, Vertex AI e Ollama não são necessários para este MVP. Consulte [o estado verificado e a retomada](DEVELOPMENT_STATUS.md) antes de escolher a próxima tarefa do roadmap.

O modo `development` usa um **Admin sintético automático, sem autenticação de produção**. Execute somente em loopback e com dados sintéticos. Não abra firewall, não configure proxy público e não use dados bancários reais neste ambiente. Headers de tenant não substituem autenticação.

## Ambiente preparado no lm-claw

Verificado em 2026-10-31: Ubuntu 26.10, Go 1.25.14, PostgreSQL 16.15, GCC 15.3 e golangci-lint 2.15.0. As ferramentas foram extraídas no diretório do usuário, sem instalar pacotes do sistema ou usar sudo.

- Ferramentas: `$HOME/.local/share/intellifinance/tools/`.
- Carregamento de PATH/compilador/bibliotecas: `tools/env.sh` nesse diretório.
- Cluster isolado: `$HOME/.local/share/intellifinance/pgdata/`.
- PostgreSQL: **127.0.0.1:55432**; banco `intellifinance_dev` e banco independente `intellifinance_test`.
- API/dashboard: **http://127.0.0.1:8090**.
- Configuração privada: `$HOME/.config/intellifinance/development.env` e `test.env`, modo 0600.
- Papel de execução: `intellifinance_app`, **NOSUPERUSER NOBYPASSRLS**. A credencial é aleatória, privada e não aparece neste documento.

A migração inicial foi aplicada separadamente aos dois bancos usando a conta administrativa do cluster. A API usa apenas a conta de aplicação, nunca o superusuário. O bootstrap atual reconhece a tabela `tenants`, mas **ainda não é um runner de migrações versionadas**; veja as pendências no estado de desenvolvimento.

Esses diretórios locais não acompanham um clone Git. Em outro host, provisione Go 1.25+, compilador C (para `-race`), Node/npm, make, PostgreSQL 16 e golangci-lint 2.15.0; crie bancos separados, aplique o schema como administrador e configure privadamente o papel da aplicação. A infraestrutura Docker existente é uma alternativa de provisionamento, não um pré-requisito deste host.

## Operação

A partir do checkout:

```bash
source "$HOME/.local/share/intellifinance/tools/env.sh"
make pg-up          # inicia apenas o cluster isolado já preparado
make mvp-up         # npm ci + build SPA + binários + serviço de usuário + readiness
make mvp-status
make mvp-logs
```

`mvp-up` usa `systemd-run --user`, sem sudo. O serviço `intellifinance-dev.service` tem reinício em falha e roda fora da sessão do terminal. **É transitório: não foi habilitado para boot nem tem promessa de sobreviver a reboot/logout do gerenciador de usuário.**

```bash
make mvp-down       # para somente a API, preservando banco e dados
make pg-down        # para somente o cluster isolado, preservando seus arquivos
make build          # recompila frontend, API e seeder sem iniciar serviços
make mvp-seed       # seed idempotente de desenvolvimento; sem transações por padrão
```

Não há alvo de reset destrutivo. `mvp-up` valida porta e configuração, recusa expor o Admin além de `127.0.0.1` e verifica a ocupação antes de parar a própria unidade. Não encerra processos alheios. Recria a unidade transitória para aplicar o `ENV_FILE` atual; readiness só passa se o listener loopback pertencer ao `MainPID` da unidade ativa antes e depois da resposta HTTP. A verificação de propriedade usa `/proc` no Linux.

Variáveis do launcher:

- `ENV_FILE`: caminho **absoluto** do arquivo de desenvolvimento, padrão acima.
- `TEST_ENV_FILE`: arquivo do banco de testes, padrão `test.env` acima.
- `TOOLS_ENV`: arquivo opcional de toolchain; ausente, as ferramentas devem estar no PATH.
- `GOLANGCI_LINT`: caminho opcional do linter.
- `PGDATA`/`PGLOG`: diretórios do cluster/log preparados, se distintos dos padrões.

O binário Go lê variáveis de ambiente; **não carrega `.env` sozinho**. O launcher carrega o arquivo privado explicitamente. `.env.example` documenta os campos, não fornece uma credencial funcional. Nunca use `set -x` ao carregar credenciais.

## Testes e gates

```bash
make test           # banco local *_test obrigatório, -race, -count=1 e falha em teste pulado
make lint           # zero issues no golangci-lint v2
python3 test/check_test_results_test.py
make smoke          # fluxo HTTP real; requer tenant sintético vazio
```

`make test` executa primeiro os testes do gate de resultados e as regressões operacionais com systemd/rede/Playwright falsos (também disponíveis via `make test-scripts`, sem banco ou serviço real). Depois grava os eventos Go reais em `bin/test-results.jsonl` (ignorado pelo Git). Pacotes sem arquivos de teste não são confundidos com testes pulados. O banco de desenvolvimento **não** é usado para testes de integração.

`make smoke` importa `extrato_itau.csv` e `fatura_nubank.csv` em contas diferentes, reimporta para confirmar deduplicação, confere saldos/KPIs com `Decimal`, filtra, ordena, classifica, edita e exclui. Recusa tenant não vazio, sessão não sintética e alvo remoto. Registra o escopo de cada conta antes do upload e, mesmo se a resposta se perder após commit, recupera e remove somente IDs novos desse escopo, confirmando a limpeza por leitura. Deve rodar sem edições concorrentes. Falhas persistentes de rede resultam em erro `RECOVERY` com escopo para recuperação manual; nunca em sucesso de limpeza. Os dois CSVs são fixtures do projeto, não extratos pessoais. A mesma regra de limpeza vale para o smoke de navegador, que fecha o navegador mesmo quando a limpeza falha.

### Navegador real (opcional)

O teste usa Playwright externo ao projeto para não acrescentar dependências de produção:

```bash
export PLAYWRIGHT_MODULE="$HOME/.local/share/intellifinance/tools/qa/node_modules/playwright"
export CHROMIUM_EXECUTABLE="$HOME/.hermes/tools/chromium-1208/chrome-linux64/chrome"
export LD_LIBRARY_PATH="$HOME/.local/share/intellifinance/tools/browser-libs/usr/lib/x86_64-linux-gnu${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
export FONTCONFIG_FILE="$HOME/.local/share/intellifinance/tools/browser-libs/fonts.conf"
export SMOKE_SCREENSHOT="$PWD/bin/qa/dashboard.png"
mkdir -p bin/qa
make smoke-browser
```

Esses caminhos foram verificados neste host e são configuráveis, não garantias de disponibilidade em outros hosts. Playwright 1.58.2 e Chromium 145.0.7632.6 foram usados. Bibliotecas/fontes do navegador foram extraídas no usuário, sem alterar o navegador do Hermes nem seus plugins/configuração.

O teste opera a UI real: upload, reimportação, paginação, filtro, ordenação, categoria inline, edição e exclusão individual/em lote. Confere as mutações lendo a API, testa viewport de 390 px e verifica erros JavaScript. Também requer tenant sintético vazio e remove os próprios registros. **Não equivale a uma auditoria completa de acessibilidade ou segurança do frontend.**

## Healthchecks e recuperação

```bash
curl -fsS http://127.0.0.1:8090/healthz
curl -fsS http://127.0.0.1:8090/readyz
```

- `/healthz`: liveness, não depende do banco.
- `/readyz`: consulta PostgreSQL com deadline de 2 segundos; HTTP 200 com `status=UP` e `checks.postgres=UP` ou HTTP 503 RFC 7807. Não retorna a mensagem privada do driver.

A perda e recuperação do banco foram testadas de verdade com `make pg-down`/`make pg-up`: liveness permaneceu 200, readiness mudou para 503 e voltou para 200 sem reiniciar a API. Os comandos interrompem somente o cluster isolado deste projeto.

## Acesso pelo laptop

O serviço não foi publicado na LAN. Use um túnel pela conexão SSH já utilizada para o host, por exemplo:

```bash
ssh -N -L 8090:127.0.0.1:8090 luismarquitti@lm-claw
```

Depois abra `http://127.0.0.1:8090` no laptop. A resolução de `lm-claw` e a conectividade do laptop não foram testadas a partir do servidor. Se 8090 estiver ocupada no laptop, mapeie outra porta local, sem mudar o bind do serviço no host.
