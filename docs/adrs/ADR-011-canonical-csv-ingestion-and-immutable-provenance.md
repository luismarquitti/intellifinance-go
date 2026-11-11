# ADR-011: Ingestão Canônica CSV com Proveniência Imutável e Proteção de Fatos Históricos

- **Status**: Accepted
- **Deciders**: Luis Eduardo Angelini Marquitti (Product Owner & Tech Lead)
- **Data**: 2026-11-11
- **Contexto Técnico**: Ingestão atômica de arquivos CSV reais e preservação de trilha de auditoria
- **Supersedes**: none
- **Related**: ADR-004 (Persistence and Database), ADR-008 (Public Showcase & Zero-Leak Security)

---

## 1. Contexto & Problema

A carga de históricos financeiros pessoais (135 arquivos CSV consolidados no piloto) requer um importador que garanta:
1. **Idempotência Absoluta**: Reexecuções do importador não podem duplicar transações nem alterar saldos de contas.
2. **Imutabilidade dos Fatos Originais**: Transações geradas a partir de extratos bancários não devem ter seus campos originais (data, descrição original, valor, conta de origem) adulterados por operações genéricas de edição (`Update` genérico).
3. **Preservação de Repetições Legítimas**: Duas compras no mesmo dia, no mesmo estabelecimento e pelo mesmo valor (ex.: dois cafezinhos de R$ 5,00) são transações distintas e válidas, não duplicatas artificiais.

---

## 2. Decisão Arquitetural

1. **Importador Canônico Dedicado (`cmd/import-ledger`)**:
   - Desenvolvido como entrypoint CLI autônomo com modo `--preview` (somente leitura) e `--apply` (gravação atômica).
   - O arquivo CSV original é armazenado integralmente em repositório seguro com hash SHA-256 de conteúdo.
2. **Fingerprint Criptográfico com Índice Sequencial**:
   - Cada linha física do extrato recebe um índice sequencial determinístico para distinguir repetições idênticas na mesma data.
   - O fingerprint é computado sobre: `TenantID | AccountID | Date | Amount | CleanDescription | SequenceIndex`.
3. **Guardas de Proveniência na Persistência**:
   - O repositório PostgreSQL valida que apenas o importador canônico pode atribuir o objeto `metadata.provenance`.
   - Mutadores genéricos são bloqueados de forjar proveniência ou alterar dados essenciais de lançamentos bancários importados. Edições de usuário devem atuar estritamente sobre categorias, tags e notas analíticas.

---

## 3. Consequências & Validação

- **Positivas**:
  - Rastreabilidade ponta a ponta desde a linha física do CSV até o saldo consolidado no ledger.
  - Idempotência testada contra 8 replays simultâneos com zero duplicatas e zero desvios de saldo.
