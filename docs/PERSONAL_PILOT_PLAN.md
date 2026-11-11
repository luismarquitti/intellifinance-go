# Plano do piloto pessoal persistente

## Pedido e escopo — 2026-11-05

Autorizado pelo usuário: usar o catálogo financeiro privado local como fonte para testar a aplicação profundamente e iniciar o uso pessoal real; manter conta e dados persistentes; acesso por navegador com usuário/senha local; informar marcos e bloqueios pelo Telegram. Não publicar fontes, PII ou credenciais no Git, em serviços externos ou nas mensagens de progresso. Não habilitar sincronizações históricas do catálogo de dados.

**Entrega parcial verificada em 06/11/2026 UTC; piloto financeiro bloqueado, ainda em pré-carga. Os itens não marcados não estão entregues.** Evidências privadas e dados ficam fora do checkout. O catálogo de origem permanece somente leitura nesta atividade. A política de fixtures sintéticas do AGENTS.md continua valendo para testes versionados. Operação e limites: [PERSONAL_PILOT_OPERATIONS.md](PERSONAL_PILOT_OPERATIONS.md).

## Constatações iniciais verificadas

- Checkout em `work/local-development`, inicialmente limpo, um commit à frente de `origin/dev`.
- MVP possui CSV, contas, categorias, transações e dashboard; não possui autenticação local real no caminho validado.
- O ambiente de desenvolvimento tem bypass de administrador e não pode receber dados pessoais.
- O seed de desenvolvimento tem risco conhecido de remover usuário conflitante por e-mail. Nunca usar esse seed no piloto pessoal.
- O catálogo tem representações mensais e consolidadas do mesmo histórico. Importar ambas duplicaria os movimentos.
- Documentação do catálogo distingue contratos/documentos, revisões posteriores e cenários. Cronogramas antigos não são prova automática do passivo atual.

## Etapas e critérios de aceite

### 1. Inventário, proveniência e validação de origem

- [ ] Registrar commit de origem, hashes SHA-256 e esquema/quantidade/cobertura de cada fonte selecionada em manifesto privado.
- [ ] Selecionar uma representação canônica por conta/cartão, comparar multiconjuntos com mensais e preservar repetições legítimas.
- [ ] Executar o validador existente e uma conferência decimal independente dos dados importáveis; não confundir contagem de linhas mensais+consolidadas com transações únicas.
- [ ] Separar fatos, previsões, simulações, lacunas de cobertura e ambiguidades; não inventar saldo inicial nem criar ajuste para forçar conciliação.

### 2. Base pessoal protegida e operação

- [x] Instância, banco, papel SQL, configuração, diretório de dados e serviço separados de dev/test/QA; sem privilégios de superusuário/BYPASSRLS para a aplicação.
- [x] Autenticação local fail-closed: login obrigatório inclusive na API e rotas alternativas; impedir spoofing de tenant/user por headers e bypass dev no modo pessoal.
- [x] Armazenar somente hash de senha na configuração privada; entregar a credencial por mecanismo privado local, nunca em Git/log/mensagem Telegram.
- [ ] Testes/seed/smokes recusam a base pessoal; não iniciar migração destrutiva, reset, truncate, cascata ou limpeza sobre ela. Preservar os registros importados e seus originais; preferir reversão/auditoria a exclusão física.
- [ ] Persistência independente da sessão e de rebuild/restart; inicialização sem recriar usuários ou sobrescrever finanças.
- [x] Bind em loopback inicialmente; acesso por túnel SSH. Qualquer publicação além do loopback exige decisão específica. Não abrir firewall, instalar proxy público ou habilitar Tailscale Serve/Funnel implicitamente.

### 3. Importação rastreável e incremental

- [ ] Validar primeiro em base QA; depois importar no piloto mediante gates aprovados, sem executar rotinas de limpeza pessoal.
- [ ] Preservar ID original, origem, hash do arquivo e linha; ledger/importação idempotente e transacional. Reexecução deve inserir zero duplicados e não alterar classificação manual.
- [ ] Contas correntes: saldos iniciais fundamentados, cobertura e saldos finais conferidos em Decimal.
- [ ] Cartões: compras, estornos, parcelas, faturas e pagamento separados; evitar contar compra e liquidação da fatura como duas despesas.
- [ ] Transferências internas: vincular apenas pares comprovados; não confundir mesma quantia/dia com prova. Não alterar receita/despesa consolidada com movimentos internos; sinalizar candidatos ambíguos para revisão.
- [ ] Dívidas/empréstimos: obrigações e cronogramas apartados do caixa; desembolso, juros, principal e pagamento sem duplicação. Não tratar cronograma antigo ou simulação como dívida confirmada.
- [ ] Holerites são evidência de composição da renda, não segundo crédito sobre depósito bancário; respeitar escopo e não exportar dados do empregador.
- [ ] Lacunas do produto ficam explícitas; não representar dívida como conta corrente fictícia só para declarar cobertura.

### 4. QA com isolamento real

- [x] Suíte Go real com PostgreSQL e `-race -count=1`, gate sem testes obrigatórios pulados; testes versionados sintéticos.
- [x] Regressões antes de correções (RED/GREEN), lint e diff check.
- [ ] QA privada com dados reais: contagens, valores, deduplicação, saldos, filtros, paginação, classificação, transferências e obrigações realmente suportadas.
- [ ] Criar/editar/excluir, falhas/rollback e reimportar somente em QA. Nunca usar o usuário pessoal como fixture descartável.
- [ ] Navegador real: login/inválido/logout, acesso sem sessão, tentativas de tenant spoofing, importação e navegação, desktop e largura móvel.

### 5. Recuperação e entrega

- [ ] Backup privado antes da primeira carga pessoal e antes de cada mudança de schema; nenhuma cópia de backup dentro do checkout.
- [ ] Restaurar backup em banco novo e comparar contagens/hashes/saldos; ter um arquivo de dump sem restaurá-lo não comprova recuperabilidade.
- [ ] Reiniciar serviço e banco e confirmar que usuário/dados/identificadores continuam; não interromper dev ou serviços alheios desnecessariamente.
- [ ] Entregar endereço real, instrução de acesso privado, localização da credencial e roteiro de operação/recuperação.
- [ ] Atualizar DEVELOPMENT_STATUS com o que passou e o que permanece pendente, sem incluir dados reais.

## Progresso e interrupções

Durante a execução autorizada, avisar pelo Telegram em intervalos de cerca de 15 minutos **quando houver mudança significativa**, e imediatamente em conclusão ou bloqueio que necessite decisão. Mensagens operacionais sem saldos, nomes de terceiros, documentos ou senhas. Não criar cobrança de provedor, integração, bot ou perfil adicional. Ao atingir limite do provedor, salvar checkpoint e pedir decisão, sem fallback pago automático.

## Garantia de preservação: limites

A regra é não remover automaticamente o usuário nem dados pessoais em seed, teste, deploy ou cleanup. Backup e restauração são parte da entrega. Nenhum sistema local garante literalmente perda impossível: falhas de disco e ações administrativas continuam sendo riscos; uma cópia externa criptografada será decisão posterior, não upload implícito.

## Evidência desta execução — 06/11/2026 UTC

- Inventário privado: commit/hashes/esquema/cobertura de 135 CSVs, oito candidatos a mestres. Candidatos não são importação aprovada. Comparação de multiconjuntos encontrou diferenças de representação e registros inválidos a revisar; não houve descarte silencioso.
- Conferência decimal independente: 12 âncoras bancárias confrontadas com contagens, originais/hash, ledger final e disponível. Três diferenças diárias documentadas permanecem explicitamente distintas do ledger, sem ajuste artificial. Há um mestre bancário sem âncora nesse manifesto; abertura/fechamento não foram inventados.
- Serviços pessoal/QA autenticados e separados, somente loopback. Papel SQL e marcador de identidade verificados no startup. Não há seed nem auto-migration pessoal. Configuração pessoal real foi recusada por launcher test/seed/up e binário seed; leitura posterior confirmou identidade/contagens intactas.
- Novo importador canônico no produto, exercitado com fixture sintética na QA autenticada: origem integral privada, linha física/ID/hashes, sinal pelo Tipo, transação atômica, conflito de ID abortando lote, repetição legítima, reimportação preservando classificação manual. Oito replays concorrentes testados com PostgreSQL e race detector. Isso não implementa semântica de cartão/transferência/dívida.
- `make test`: 264 eventos Go incluindo subtestes, 20 pacotes, zero falhas/skips; 28 testes operacionais Python aprovados. `make lint`, `make build`, `go vet`, `go mod verify` e `git diff --check` aprovados.
- Chromium real: autenticação válida/inválida/anônima, SPA, spoofing, filtros, viewport 390 e ausência de erros JS. Importação pessoal via UI, logout próprio e CRUD financeiro real pelo navegador continuam pendentes.
- Backup de schema/identidade restaurado em banco separado, hashes/contagens idênticos. Ainda sem dados financeiros pessoais: não é prova de restauração pós-carga.
- Reinício: a recusa inicial no worker (`message_id=66`) foi resolvida pelo processo interativo em 06/11/2026 às 00:26 UTC, sem alteração global de aprovações. Prova validada: PIDs PostgreSQL alterados e contagens/hashes preservados. Isso cobre estrutura/identidade, não persistência pós-carga. Código final de proveniência ainda não aplicado aos binários estáveis. Parada atual: quota Codex no limiar de 20% antes da próxima etapa extensa; decisão de divisão/retomada necessária.

Estado de aceite geral: **NÃO CONCLUÍDO**. Nenhum CSV financeiro pessoal foi importado; conta pessoal e fontes permanecem preservadas. Faltam principalmente semântica financeira e revisão do mapeamento, QA real, deploy final verificado, carga pessoal e recuperação/persistência pós-carga.
