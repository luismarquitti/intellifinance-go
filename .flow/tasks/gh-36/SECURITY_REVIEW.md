# Revisão de Segurança Aplicada & DevSecOps — Frontend IntelliFinance

- **Document ID**: SEC-001-FRONTEND-AUDIT
- **Auditor**: Security Reviewer (`security-reviewer`)
- **Data**: 2026-10-17
- **Classificação**: Confidencial / Engenharia Interna
- **Conformidade Normativa**: LGPD (Lei 13.709/2018), OWASP Top 10 (2021), OWASP ASVS v4.0, NFR-SEC-01 a 05

---

## 1. Escopo e Objetivos da Revisão

Esta auditoria avalia os vetores de ataque, riscos de vazamento de informações e integridade criptográfica da interface do usuário (SPA React + Vite) consumindo a API REST do IntelliFinance. 

O objetivo é garantir que nenhuma decisão na camada de frontend fragilize as garantias de **Isolamento Multi-Tenant via RLS**, a **Política Zero-Leak de PII (`ADR-008`)**, ou os requisitos de **Armazenamento de Tokens e Sessão (`NFR-SEC-05`)**.

---

## 2. Auditoria dos Vetores de Segurança

### 2.1 Gestão de Credenciais e Vetores de Armazenamento (`NFR-SEC-05`)

| Vetor Auditado | Risco Identificado | Diretriz Obrigatória no Frontend | Status |
|:---|:---|:---|:---:|
| **Persistência de Access Token** | Roubo de credenciais via XSS caso o JWT seja salvo em `localStorage` ou `sessionStorage`. | O token JWT DEVE ser armazenado **estritamente em variável de escopo de módulo (memória)**. Ao recarregar a página (F5), a sessão é reidratada exclusivamente via refresh silencioso. | **Aprovado** |
| **Refresh Token Cookie** | Interceptação em trânsito ou vazamento para scripts terceiros. | O backend DEVE emitir o refresh token com os atributos: `HttpOnly`, `Secure`, `SameSite=Strict` e `Path=/api/v1/auth/refresh`. O JavaScript do frontend não tem acesso de leitura ao cookie. | **Aprovado** |
| **Silent Refresh Race Condition** | Múltiplas requisições paralelas 401 gerando rotações simultâneas e quebra de sessão. | Implementação mandatória de **Mutex / Fila de Requisições** no interceptor HTTP. Requisições 401 subsequentes aguardam a resolução da primeira. | **Mitigado na Spec** |
| **Logout & Revogação** | Sessão órfã no cliente se a revogação no servidor falhar. | A ação de Logout no cliente DEVE purgar imediatamente o cache de memória do TanStack Query e o token volátil, independentemente da resposta HTTP do endpoint de revogação. | **Aprovado** |

### 2.2 Isolamento Multi-Tenant e Fronteiras de RLS (`NFR-TEN-01` a `03`)

- **Vulnerabilidade Teórica**: Risco de desenvolvedores utilizarem filtros no cliente como "camada de autorização" ou ocultação de registros privados/de terceiros.
- **Veredito de Segurança**:
  - O frontend **NUNCA DEVE** atuar como mecanismo de segurança ou controle de acesso.
  - A API REST e o PostgreSQL com `FORCE ROW LEVEL SECURITY` e `WithTenantTx` são as **únicas barreiras autoritativas**.
  - A visibilidade `PERSONAL_PRIVATE` deve ser compreendida na UI como uma marcação de intenção do usuário para o backend; se um registro privado retornar da API, o frontend deve exibi-lo, pois quem governa o que pode ser retornado é a query RLS do banco de dados baseada em `app.current_user_id` e `app.current_tenant_id`.
  - É proibido ocultar dados via CSS (`display: none`) ou filtragem local de arrays `items.filter(...)` por motivos de privacidade.

### 2.3 Política Zero PII e Sanitização de Dados Bancários (`ADR-008`, `NFR-PRV-04`)

- **Vulnerabilidade**: Vazamento de PII em logs de telemetria de erro (Sentry, Datadog ou console do navegador).
- **Diretrizes Estritas**:
  1. **Sanitização de Payloads de Erro**: O interceptor que processa RFC 7807 (`application/problem+json`) DEVE mascarar valores de campos sensíveis (senhas, CPFs, números de conta) antes de qualquer log no console.
  2. **Zero PII em Fixtures e Demos**: Todas as telas exibidas em capturas ou no showcase público DEVEM utilizar apenas pessoas fictícias geradas pelo `cmd/fixturegen` ("Carlos Drummond", "Clarice Lispector") e CPFs com prefixo de teste `000.xxx.xxx-xx`.
  3. **Campos Mascarados**: O backend envia `account_number_masked` (ex: `****1234-5`). O frontend nunca deve solicitar ou reconstruir o número completo da conta.

### 2.4 Prevenção de XSS e Injeção de Conteúdo em Extratos

- **Vetor de Risco**: Descrições de transações importadas de arquivos externos (`.ofx`, `.csv`, `.pdf`) podem conter caracteres de injeção HTML/JavaScript (ex: `<script>alert(1)</script>` ou payloads UTF-8 maliciosos).
- **Mitigação**:
  - O React 19 realiza escape automático de strings no JSX por padrão (`{tx.description}`).
  - Fica **terminantemente proibido** o uso de `dangerouslySetInnerHTML` em qualquer componente do frontend.
  - O backend já realiza limpeza e normalização em `clean_description`, mas o frontend deve tratar todo texto como dado não-confiável.

### 2.5 Integridade Contábil: Proibição de Ponto Flutuante (`PRD §7.2`, `AGENTS.md §2.1`)

- **Risco**: Erros de arredondamento em cálculos client-side (`0.1 + 0.2 = 0.30000000000000004`) distorcendo saldos, reconciliações ou orçamentos.
- **Regra de Blindagem**:
  - Proibição absoluta de `parseFloat`, `Number(amount)` ou operadores matemáticos (`+`, `-`, `*`, `/`) em valores monetários no cliente.
  - Toda conversão para exibição DEVE ocorrer via `Intl.NumberFormat` ou biblioteca decimal estrita.
  - A asserção será fiscalizada por regra estática de linter no CI (`! grep -rE "parseFloat|Number\(.*amount" web/src`).

### 2.6 Cabeçalhos de Segurança HTTP (Para o Servidor de Assets Go)

Como o binário Go (`cmd/api`) servirá os arquivos estáticos compilados da SPA, os seguintes cabeçalhos de segurança DEVEM ser injetados pelo middleware Chi:
1. `Content-Security-Policy`: `default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self' http://localhost:* https://*.intellifinance.app; frame-ancestors 'none';`
2. `X-Frame-Options`: `DENY` (prevenção contra Clickjacking)
3. `X-Content-Type-Options`: `nosniff`
4. `Referrer-Policy`: `strict-origin-when-cross-origin`
5. `Permissions-Policy`: `camera=(), microphone=(), geolocation=()`

---

## 3. Conclusão da Revisão de Segurança

O plano arquitetural refinado atende a todos os critérios de DevSecOps, não introduz novas vulnerabilidades de sessão e respeita categoricamente os perímetros de privacidade de tenant e integridade contábil.
