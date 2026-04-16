# CHANGELOG — ArgoClaw

> ArgoClaw is a fork of [ArgoClaw](https://github.com/vellus-ai/argoclaw) by Vellus.
> This changelog tracks all modifications: internal features, upstream merges, and community contributions.

---

## [Unreleased]

### Added
- feat(operator): Ponte de Comando Central — visibilidade cross-tenant para o operador Vellus via dual-check `operator_level + Role` (migration 000035, context helpers, middleware, handler, 4 endpoints `/v1/operator/*`)
- feat(operator): migration 000035 — coluna `operator_level` na tabela `tenants` com seed idempotente do tenant `vellus` (`operator_level=1`, `plan=internal`)
- feat(operator): context helpers `WithOperatorMode`, `OperatorModeFromContext`, `IsOperatorMode` para propagação de Operator Mode via contexto Go
- feat(operator): middleware `requireOperatorRole` com dual-check: `operator_level >= 1` AND `Role >= RoleOperator`
- feat(operator): endpoints `GET /v1/operator/tenants`, `/tenants/{id}/agents`, `/tenants/{id}/sessions`, `/tenants/{id}/usage` com paginação e validação de UUID
- feat(infra): split de deployment K8s em `argoclaw-central` (canary, node pool dedicado) e `argoclaw-customers` (stable)
- feat(infra): node pool Terraform `argoclaw-control-pool` com taint `vellus.ai/tier=control:NoSchedule`
- feat(ci): `cloudbuild.yaml` para deploy automático no `argoclaw-central` + `promote.sh` para promoção manual para `argoclaw-customers`
- test(operator): testes de integração da matriz de acesso operator com PBT bicondicional (banco real)
- test(operator): testes de integração de isolamento cross-tenant e idempotência da migration 000035
- test(operator): testes E2E do fluxo completo login → JWT → acesso operator → propagação WS/HTTP

### Security
- Proteção write do campo `operator_level` — rejeitado com 422 em todos os handlers de criação/atualização de tenant
- `operator_level` lido exclusivamente do banco de dados na autenticação — nunca derivado de JWT claims
- Audit trail completo: `slog.Info("operator.access")` em todos os acessos, `slog.Warn("security.operator_access_denied")` em todas as rejeições
- Endpoints operator são somente leitura — nenhuma operação write em tenants de clientes

### Fixed
- Corrigido loop infinito de login para usuarios autenticados por email e senha — conexao WebSocket agora reconhece JWT e atribui a role correta (#44)
- Login por email retornava 401 em endpoints autenticados (ex: /v1/providers) porque `resolveAuth()` nao reconhecia JWT como metodo de autenticacao
- Tela de login exibia opcoes de Token e Pareamento alem do email; agora exibe apenas login por email conforme definicao de projeto

### Upstream Community Merges (from ArgoClaw)

| PR | Title | Author | Status | Date |
|---|---|---|---|---|
| [#314](https://github.com/vellus-ai/argoclaw/pull/314) | fix(agent): sanitize runID for Anthropic compatibility | @duhd-vnpay | ✅ Merged | 2026-03-22 |
| [#226](https://github.com/vellus-ai/argoclaw/pull/226) | fix: tsnet Server Listen resource leak | @lsytj0413 | ✅ Merged | 2026-03-22 |
| [#356](https://github.com/vellus-ai/argoclaw/pull/356) | fix(summoning): prevent identity markdown corrupting display names | @kaitranntt | ✅ Merged | 2026-03-22 |
| [#352](https://github.com/vellus-ai/argoclaw/pull/352) | fix(ui): chat nav route + crypto.randomUUID in non-secure contexts | @maxfraieho | ✅ Merged | 2026-03-22 |
| [#339](https://github.com/vellus-ai/argoclaw/pull/339) | build(docker): add curl to runtime image | @tolkonepiu | ✅ Merged | 2026-03-22 |
| [#316](https://github.com/vellus-ai/argoclaw/pull/316) | feat: project-scoped MCP isolation + **env blocklist + tenant_id** | @duhd-vnpay | ✅ Merged (PR #8) + Security TDD/PBT | 2026-03-23 |
| [#343](https://github.com/vellus-ai/argoclaw/pull/343) | feat: Anthropic OAuth setup tokens + **configurable system prompt** | @anhle128 | ✅ Merged (PR #9) + PBT | 2026-03-23 |
| [#202](https://github.com/vellus-ai/argoclaw/pull/202) | fix: Telegram @mention preservation + bot-to-bot routing | @nvt-ak | ✅ Merged (PR #10) + PBT | 2026-03-23 |

| [#182](https://github.com/vellus-ai/argoclaw/pull/182) | fix: nil pointer SSE + extractDefaultModel (cherry-pick, no Party Mode) | @duhd-vnpay | ✅ Merged (PR #11) + PBT | 2026-03-23 |
| [#346](https://github.com/vellus-ai/argoclaw/pull/346) | fix(zalo): allow QR session restart | @ductrantrong | ✅ Merged (PR #11) | 2026-03-23 |

### Upstream PRs — Skipped (fix already applied)

| PR | Title | Reason |
|---|---|---|
| [#350](https://github.com/vellus-ai/argoclaw/pull/350) | fix: listing providers + session key | Core fix (generateId) already in PR #352. UX improvements deferred. |

### Upstream PRs — Under Review

| PR | Title | Author | Priority |
|---|---|---|---|
| [#343](https://github.com/vellus-ai/argoclaw/pull/343) | feat(providers): Anthropic OAuth setup tokens | @anhle128 | Medium |
| [#202](https://github.com/vellus-ai/argoclaw/pull/202) | fix(telegram): bot-to-bot mention routing | @nvt-ak | Medium |
| [#315](https://github.com/vellus-ai/argoclaw/pull/315) | feat: Party Mode — multi-persona discussions | @duhd-vnpay | Low |
| [#316](https://github.com/vellus-ai/argoclaw/pull/316) | feat: project-scoped MCP isolation | @duhd-vnpay | Low |
| [#196](https://github.com/vellus-ai/argoclaw/pull/196) | feat: Google Chat channel (Pub/Sub) | @duhd-vnpay | Low — comparing with #148 |
| [#148](https://github.com/vellus-ai/argoclaw/pull/148) | feat: Google Chat channel integration | @tuntran | Low — comparing with #196 |

### Upstream PRs — Rejected/Skipped

| PR | Title | Reason |
|---|---|---|
| [#132](https://github.com/vellus-ai/argoclaw/pull/132) | fix: Windows syscall build error | Rejected — removes Linux security checks. Needs build tags. |
| [#238](https://github.com/vellus-ai/argoclaw/pull/238) | feat(feishu): thread history | Skipped — Feishu not relevant for ARGO |

---

## [0.4.0] — 2026-03-22 — Sprint 0 Complete

### ArgoClaw Internal Features

- **Auth PCI DSS** — Email + password login with bcrypt, JWT, 12+ char policy, lockout
- **Multi-tenancy Enterprise** — Tenant isolation via `tenant_id` FK, middleware
- **White-label** — Per-tenant branding (logo, colors, favicon) via `tenant_settings` JSONB
- **i18n 8 locales** — pt-BR, en-US, es-ES, fr-FR, it-IT, de-DE, zh-CN, ja-JP
- **ARGO Presets** — 6 agent personalities: Capitao, Timoneiro, Vigia, Artilheiro, Navegador, Ferreiro
- **AppSec Audit** — Hardened SQL, input validation, password policy, rate limiting

### Internal PRs

| PR | Title |
|---|---|
| [#1](https://github.com/vellus-ai/argoclaw/pull/1) | fix: AppSec audit — SQL injection, input validation |
| [#2](https://github.com/vellus-ai/argoclaw/pull/2) | feat: Auth PCI DSS — users table, JWT, login flow |
| [#3](https://github.com/vellus-ai/argoclaw/pull/3) | feat: Multi-tenancy enterprise — tenant isolation |
| [#4](https://github.com/vellus-ai/argoclaw/pull/4) | feat: White-label + i18n 8 locales + ARGO presets |

---

## [0.1.0] — 2026-03-22 — Initial Fork

- Forked from [ArgoClaw v0.6.0](https://github.com/vellus-ai/argoclaw)
- Renamed to ArgoClaw
- Repository: https://github.com/vellus-ai/argoclaw
