# ADR-010: Transferências Internas Confirmadas, Conservação de Saldo e Locks Anti-Deadlock

- **Status**: Accepted
- **Deciders**: Luis Eduardo Angelini Marquitti (Product Owner & Tech Lead)
- **Data**: 2026-11-11
- **Contexto Técnico**: Resolução de invariantes de transferências internas e concorrência no ledger
- **Supersedes**: none
- **Related**: ADR-002 (API Architecture), ADR-004 (Persistence and Database), `docs/CONSOLIDATED_MASTER_PLAN.md`

---

## 1. Contexto & Problema

Transferências entre contas próprias do mesmo titular/tenant (ex.: Conta Corrente -> Poupança ou Carteira) representam movimentações neutras para o fluxo de caixa consolidado: o patrimônio líquido total do tenant permanece inalterado.

A implementação dessa capacidade no modelo contábil de partidas dobradas impõe desafios críticos:
1. **Conservação de Saldo e Prevenção de Dupla Contagem**: Uma transferência consiste em duas pernas reais no ledger (`EXPENSE` e `INCOME`), com valores simétricos exatos (`legA.Amount + legB.Amount == 0`). O cálculo de receitas e despesas agregadas não pode inflar os relatórios com transferências internas.
2. **Precisão Numérica de Sub-Centavos**: O PostgreSQL utiliza a coluna `NUMERIC(15,2)`. Se a aplicação aceitar frações inferiores a R$ 0,01 (ex.: R$ 10,005), o banco arredonda silenciosamente para R$ 10,00, quebrando a simetria exata das duas pernas e gerando anomalias no fechamento de caixa.
3. **Prevenção de Deadlock Concorrente**: Na verificação cruzada de moedas entre duas contas distintas, transações simultâneas de direções opostas (A -> B e B -> A) executando `SELECT currency FROM accounts ... FOR SHARE` colidiam com o trigger de atualização de saldo `trg_sync_balance` (`UPDATE accounts SET balance = balance + NEW.amount`), que exige lock exclusivo. A tentativa mútua de promover o lock compartilhado para exclusivo gerava deadlocks cíclicos no PGX.

---

## 2. Decisão Arquitetural

1. **Modelo de Dados Simétrico em Duas Pernas**:
   - Cada perna de transferência é uma linha na tabela `transactions`, preservando data, conta, notas e proveniência originais.
   - O relacionamento é estabelecido via chave estrangeira mútua `transfer_peer_transaction_id` com tipo `TRANSFER`.
   - O endpoint de sumarização (`SummarizeTransactions`) filtra e desconsidera movimentações com `type =  TRANSFER`, assegurando que o fluxo de caixa reflita apenas receitas e despesas efetivas com terceiros.
2. **Rejeição Estrita de Frações de Centavo no Domínio**:
   - Adicionada a regra pura: `IsRepresentableInCents(d) = d.Truncate(2).Equal(d)`.
   - Valores com mais de 2 casas decimais não zero são terminantemente rejeitados no DTO (HTTP 422) e no domínio com `domain.ErrInvalidAmount`, prevenindo truncamento silencioso pelo PostgreSQL.
3. **Ordenação Determinística de Locks Row-Level Anti-Deadlock**:
   - Em `validateTransferCurrencies`, a consulta de contas foi padronizada para:
     ```sql
     SELECT currency FROM accounts
     WHERE tenant_id = $1 AND id IN ($2, $3) AND deleted_at IS NULL
     ORDER BY id ASC FOR NO KEY UPDATE;
     ```
   - Em `ConfirmTransfer`, as transações são travadas por `ORDER BY id ASC FOR UPDATE`.
   - Em `CreateTransfer`, advisory locks transacionais são adquiridos na mesma ordem (`ORDER BY id ASC`).
   - Essa ordem ascendente global e a modalidade exclusiva eliminam a necessidade de promoção de shared lock durante o disparo de `trg_sync_balance`, impedindo a formação de ciclos de deadlock.

---

## 3. Consequências & Validação

- **Positivas**:
  - Eliminação comprovada de deadlocks em transferências bidirecionais concorrentes (validado via `transfer_deadlock_test.go`).
  - Integridade contábil estrita sem descompasso de centavos.
  - Desvinculação reversível (`UnlinkTransfer`) que restaura os tipos originais sem apagar registros ou dados de auditoria.
- **Negativas**:
  - Transações simultâneas envolvendo as mesmas contas são serializadas pelo banco (comportamento desejável para consistência de saldo).
