# CHANGELOG — ArgoClaw

> ArgoClaw is a fork of [GoClaw](https://github.com/nextlevelbuilder/goclaw) by Vellus.
> This changelog tracks all modifications: internal features, upstream merges, and community contributions.

---

## [Unreleased]

### Upstream Community Merges (from GoClaw)

| PR | Title | Author | Status | Date |
|---|---|---|---|---|
| [#314](https://github.com/nextlevelbuilder/goclaw/pull/314) | fix(agent): sanitize runID for Anthropic compatibility | @duhd-vnpay | ✅ Merged | 2026-03-22 |
| [#226](https://github.com/nextlevelbuilder/goclaw/pull/226) | fix: tsnet Server Listen resource leak | @lsytj0413 | ✅ Merged | 2026-03-22 |
| [#356](https://github.com/nextlevelbuilder/goclaw/pull/356) | fix(summoning): prevent identity markdown corrupting display names | @kaitranntt | ✅ Merged | 2026-03-22 |
| [#352](https://github.com/nextlevelbuilder/goclaw/pull/352) | fix(ui): chat nav route + crypto.randomUUID in non-secure contexts | @maxfraieho | ✅ Merged | 2026-03-22 |
| [#339](https://github.com/nextlevelbuilder/goclaw/pull/339) | build(docker): add curl to runtime image | @tolkonepiu | ✅ Merged | 2026-03-22 |

### Upstream PRs — Approved, Pending Conflict Resolution

| PR | Title | Author | Issue |
|---|---|---|---|
| [#182](https://github.com/nextlevelbuilder/goclaw/pull/182) | fix: prevent nil pointer crash in OpenAI SSE | @duhd-vnpay | Conflicts in cmd/gateway.go, agent/loop.go |
| [#350](https://github.com/nextlevelbuilder/goclaw/pull/350) | fix: listing providers error + session key gen | @anhle128 | Conflicts in ui/web routes.tsx |
| [#346](https://github.com/nextlevelbuilder/goclaw/pull/346) | fix(zalo): allow QR session restart | @ductrantrong | Conflicts in zalo/qr.go |

### Upstream PRs — Under Review

| PR | Title | Author | Priority |
|---|---|---|---|
| [#343](https://github.com/nextlevelbuilder/goclaw/pull/343) | feat(providers): Anthropic OAuth setup tokens | @anhle128 | Medium |
| [#202](https://github.com/nextlevelbuilder/goclaw/pull/202) | fix(telegram): bot-to-bot mention routing | @nvt-ak | Medium |
| [#315](https://github.com/nextlevelbuilder/goclaw/pull/315) | feat: Party Mode — multi-persona discussions | @duhd-vnpay | Low |
| [#316](https://github.com/nextlevelbuilder/goclaw/pull/316) | feat: project-scoped MCP isolation | @duhd-vnpay | Low |
| [#196](https://github.com/nextlevelbuilder/goclaw/pull/196) | feat: Google Chat channel (Pub/Sub) | @duhd-vnpay | Low — comparing with #148 |
| [#148](https://github.com/nextlevelbuilder/goclaw/pull/148) | feat: Google Chat channel integration | @tuntran | Low — comparing with #196 |

### Upstream PRs — Rejected/Skipped

| PR | Title | Reason |
|---|---|---|
| [#132](https://github.com/nextlevelbuilder/goclaw/pull/132) | fix: Windows syscall build error | Rejected — removes Linux security checks. Needs build tags. |
| [#238](https://github.com/nextlevelbuilder/goclaw/pull/238) | feat(feishu): thread history | Skipped — Feishu not relevant for ARGO |

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

- Forked from [GoClaw v0.6.0](https://github.com/nextlevelbuilder/goclaw)
- Renamed to ArgoClaw
- Repository: https://github.com/vellus-ai/argoclaw
