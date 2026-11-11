# ADR-012: Conector Pluggy Open Finance e Servidor MCP Go com Governança HITL

- **Status**: Accepted
- **Deciders**: Luis Eduardo Angelini Marquitti (Product Owner & Tech Lead)
- **Data**: 2026-11-11
- **Contexto Técnico**: Integração bancária Open Finance e automação com agentes de IA (MCP)
- **Supersedes**: none
- **Related**: ADR-007 (Open Finance Strategy), `docs/CONSOLIDATED_MASTER_PLAN.md`

---

## 1. Contexto & Problema

A integração com o ecossistema brasileiro de Open Finance (via API Pluggy.ai) e a exposição de ferramentas financeiras para agentes de Inteligência Artificial via protocolo MCP (*Model Context Protocol*) exigem controles de governança estritos:
1. **Risco de Alucinação ou Mutação Indevida por IA**: Agentes LLM não devem possuir autoridade para realizar mutações diretas no saldo bancário, criar transferências automáticas sem supervisão ou aprovar regras de categorização de forma irreversível.
2. **Divergência entre Saldo Informado pelo Banco e Saldo Reconstruído**: O saldo retornado pela API bancária (snapshot do provedor) pode divergir do saldo somado dos movimentos contábeis do ledger (devido a cheques a compensar, lançamentos futuros ou tarifas pendentes).
3. **Segurança de Webhooks**: Notificações de eventos bancários exigem autenticação criptográfica rigorosa com verificação em tempo constante (HMAC SHA-256) e janelas estritas contra ataques de repetição (*replay attacks*).

---

## 2. Decisão Arquitetural

1. **Separação entre Snapshot de Provedor e Saldo de Ledger**:
   - O campo `accounts.balance` permanece sob controle exclusivo dos movimentos atômicos do ledger via trigger `trg_sync_balance`.
   - Saldos informados pelo Open Finance são gravados como snapshots de observação (`provider_balance_snapshot`), permitindo cálculo de reconciliação explícito sem inventar saldos de abertura artificiais.
2. **Servidor MCP Nativo Go (`cmd/mcp`)**:
   - Implementação nativa em Go das ferramentas MCP com segregação clara de privilégios:
     - **Ferramentas de Leitura**: `list_accounts`, `get_balances`, `list_transactions`, `summarize_transactions`.
     - **Ferramentas de Proposta (Assíncronas)**: `propose_categorization`, `request_sync`.
     - **Bloqueio de Mutação Direta**: Nenhuma ferramenta MCP de escrita direta ou aprovação automática é exposta.
3. **Engine HITL (*Human-In-The-Loop*)**:
   - Todas as sugestões geradas por IA ou automações bancárias são registradas na tabela `pending_actions` no estado `PENDING`.
   - Apenas uma decisão humana explícita (`APPROVED` via interface web ou CLI) autoriza a execução transacional pelo serviço. Ações recusadas transitam para `REJECTED` com auditoria imutável.
4. **Webhooks com HMAC Constant-Time**:
   - Assinatura validada via `subtle.ConstantTimeCompare` com tolerância máxima de timestamp de ±300s, com resolução segura de tenant por item bancário.

---

## 3. Consequências & Validação

- **Positivas**:
  - Impossibilidade de desvio contábil autônomo por modelos de linguagem.
  - Alinhamento total com as diretrizes do Banco Central do Brasil e LGPD.
