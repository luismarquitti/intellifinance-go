# Operação do piloto pessoal — pré-carga

## Estado verificado em 06/11/2026 UTC

A infraestrutura autenticada está disponível, mas **o piloto financeiro não está concluído**. A base pessoal tem apenas a identidade inicial, sem contas nem movimentos financeiros. Não utilizar os KPIs vazios como diagnóstico das finanças. O catálogo privado permanece somente leitura.

Instâncias persistentes, sem root:

| Uso | HTTP loopback | PostgreSQL loopback | Unidades de usuário |
| --- | --- | --- | --- |
| Pessoal | 127.0.0.1:8092 | 127.0.0.1:55433 | intellifinance-personal.service / intellifinance-personal-postgres.service |
| QA autenticada | 127.0.0.1:8093 | 127.0.0.1:55434 | intellifinance-personal-qa.service / intellifinance-personal-qa-postgres.service |

As quatro unidades estão habilitadas no `default.target`; `Linger=yes` foi verificado. O reinício inicialmente bloqueado no worker foi executado pelo processo interativo em 06/11/2026 às 00:26 UTC: os dois PIDs PostgreSQL mudaram e contagens/hashes de todas as tabelas permaneceram iguais. A prova foi validada pelo worker sem repetir o reinício. Isso cobre schema/identidade, não dados financeiros ainda ausentes nem um teste de reboot do host. Nenhum firewall, proxy, Tailscale Serve/Funnel ou serviço Hermes foi alterado.

Dados, configuração e binários estáveis ficam em diretórios privados fora do Git:

- Raiz: `$HOME/.local/state/intellifinance/personal-rollout/` (0700).
- Instância pessoal: `personal/`; QA: `qa/`.
- Configuração de cada instância: `app.env` (0600), com hash bcrypt e identidade fixa.
- Credencial para consulta exclusivamente local pelo usuário: `personal/login.txt` (0600). Não colar o conteúdo em chat, relatório, log ou Git.
- Banco: `personal/pgdata/`; backups: `personal/backups/`.
- Serviços usam `personal/bin/api` e `qa/bin/api`, não o binário dev do checkout. Rebuild de desenvolvimento não substitui esses arquivos.

O papel SQL de execução é NOSUPERUSER/NOBYPASSRLS/NOCREATEDB/NOCREATEROLE. Não pode excluir usuários, truncar tabelas nem excluir transações pessoais. Identidade e marcador são provisionados uma única vez. Startup pessoal verifica essas proteções e não executa seed nem auto-migration.

## Acesso pelo navegador

Usar somente o túnel SSH existente, inicialmente com a mesma porta local:

```bash
ssh -N -L 8092:127.0.0.1:8092 luismarquitti@lm-claw
```

Abrir `http://127.0.0.1:8092`, não `localhost`: a validação de Host/origin é exata. O navegador solicita HTTP Basic; a credencial está no arquivo privado indicado acima. Basic não deve trafegar fora do loopback/túnel criptografado. Não é autenticação apropriada para publicação pública.

Não há logout de aplicação nesta versão. Fechar um contexto de navegador descartável foi testado; isso não comprova limpeza do cache de credenciais de todo navegador comum. Para uso compartilhado, empregar uma sessão privada dedicada e encerrá-la completamente. Um logout próprio continua pendente.

Celular sem túnel não acessa esse endereço. Acesso privado além de loopback requer autorização específica e outro gate; não foi habilitado.

## Guardas e importação

- `make test`, seed e launcher dev recusam configuração/namespace pessoal antes de conectar ou operar serviços. Foram apontados de verdade para a configuração pessoal e recusaram, com identidade/contagens relidas sem alteração.
- A suíte Go também valida o destino antes de abrir o banco, exige banco local descartável `*_test` e não pula integração indisponível.
- Rotas pessoais exigem autenticação global, inclusive SPA/assets e aliases na raiz. Headers de tenant/user não escolhem a identidade. Origin/Host incorretos e mutações sem origin são recusados.
- Exclusão pessoal automática retorna 403. Não há reset/truncate/delete-all para a base pessoal.
- O upload CSV legado retorna 409 no modo pessoal: ele não fornece a proveniência exigida. A UI ainda oferece esse fluxo; isso não significa suporte de importação pessoal pelo navegador.
- O novo `cmd/import-ledger` faz preview por padrão. Aceita CSV canônico estrito `ID,Data,Descrição,Valor,Tipo`, com magnitudes positivas e Tipo explícito Receita/Despesa. Rejeita arquivo inválido inteiro; não pula linhas silenciosamente.
- `--apply` exige arquivo de origem retido em arquivo privado fora de Git, conta de caixa existente e configuração pessoal/QA autenticada. Não usar esse comando com dados reais antes dos gates financeiros e de deploy. A existência de uma opção não é aprovação de carga.
- Proveniência: hash do arquivo, nome de origem, ID original, linha física, campos originais e hash da linha. Arquivo integral é retido por conteúdo sem overwrite. A aplicação impede alterações da proveniência importada no caminho de update.
- Importação serializada por conta, transação SQL atômica, sequência canônica e fingerprint de seis campos. Conflito de ID original aborta o lote; reimportação não sobrescreve classificação manual. Repetições legítimas com IDs diferentes não desaparecem.

O CLI foi exercitado em QA autenticada com fixture sintética: importação, duas reimportações, classificação manual, saldo Decimal, arquivo original arquivado e limpeza exclusiva de registros sintéticos confirmada por leitura. O novo guard de proveniência está no código/candidato; os binários estáveis não foram substituídos nesta etapa final. A execução parou no limiar de quota estabelecido. Não alegar que toda a revisão final já está em execução nos serviços.

## Backup e recuperação

Existem backups privados do banco vazio antes do schema e da identidade antes de finanças. Um backup foi restaurado de verdade em `intellifinance_restore_proof`, banco separado, sem permissão pública de conexão. Contagens e hashes de todas as tabelas, mais a representação decimal textual de saldos, foram comparados e a base original relida sem alterações.

Essa prova cobre schema/identidade, **não dados financeiros ainda não carregados**. Antes da primeira carga real e de cada migração pessoal: novo backup, restauração em banco novo e conferência; nunca restaurar por cima da base pessoal como teste. Nenhum sistema local garante perda impossível; backup externo criptografado não foi autorizado nem realizado.

## Retomada segura

Checkpoint privado: `$HOME/.local/state/intellifinance/personal-rollout/checkpoint.md`. Mapeamento e lacunas: `mapping-review.md` no mesmo diretório.

O gate de reinício inicial está concluído, com prova privada `parent-restart-proof.json`; não é necessário repeti-lo para resolver a antiga barreira. Um próximo deploy deve preservar backup/identidade e aplicar o binário final com verificação posterior, sem seed nem recriação. Após carga financeira, ainda será necessário outro ensaio específico de persistência e recuperação com os dados presentes. Não tocar no cluster dev, no Hermes ou na política global de aprovações.

Continuam pendentes: domínio correto de faturas/parcelas/liquidação, transferências próprias confirmadas, obrigações/dívidas fora do caixa, cobertura/abertura por conta, QA com dados reais, importação pessoal, backup pós-carga restaurado e persistência de dados financeiros após reinício. Holerites e cronogramas históricos não foram convertidos em caixa fictício.
