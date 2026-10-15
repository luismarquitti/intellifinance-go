## 🎯 Objetivo da Mudança

Closes #

Descreva de forma concisa o que este PR introduz, corrige ou aprimora no sistema.

---

## 📦 O que foi Entregue

- [ ] Item 1...
- [ ] Item 2...

---

## 🛡️ Checklist de Engenharia & Governança (AGENTS.md & PRFlow)

### 1. Invariantes Financeiras & Domínio
- [ ] **Precisão Monetária**: Valores monetários utilizam exclusivamente `shopspring/decimal` ou `NUMERIC(15, 2)` (zero `float32`/`float64`).
- [ ] **Multi-Tenancy**: Toda consulta ou mutação financeira é escopada por `tenant_id` e respeita as regras de Row-Level Security (RLS).
- [ ] **Deduplicação**: Ingestões de extratos utilizam a fórmula canônica de fingerprint SHA-256.

### 2. Segurança & Zero-Leak (LGPD)
- [ ] **Zero PII Real**: Nenhum extrato, documento bancário, holerite, CPF ou dado pessoal real foi adicionado (uso exclusivo de `cmd/fixturegen` ou mocks sintéticos).
- [ ] **Segredos & Credenciais**: Nenhuma chave de API, segredo ou token hardcoded.

### 3. Validação Mecânica & Qualidade
- [ ] Compilação limpa sem warnings.
- [ ] Testes unitários adicionados/atualizados e passando: `go test -v -race ./...`
- [ ] Linter aprovado sem violações: `golangci-lint run`
- [ ] Commits seguem o padrão **Conventional Commits** (`feat:`, `fix:`, `refactor:`, `docs:`, `test:`).

### 4. Documentação Sincronizada
- [ ] As especificações em `docs/specs/` ou `docs/PRD.md` foram atualizadas caso tenha havido alteração de contrato ou schema.

---

## ⚠️ Avaliação de Risco (Preenchido pelo Autor/Agente)

- [ ] **Baixo Risco (Low)**: Fixtures, documentação, tipografia, refatorações sem alteração de comportamento.
- [ ] **Médio Risco (Medium)**: Novos parsers, endpoints REST auxiliares, testes de integração.
- [ ] **Alto Risco (High / Requer Aprovação Humana)**: Alterações de DDL (`migrations/`), políticas de RLS, autenticação, segredos ou conector bancário real.
