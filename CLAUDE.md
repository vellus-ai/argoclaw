# ArgoClaw Gateway

PostgreSQL multi-tenant AI agent gateway with WebSocket RPC + HTTP API.

## Tech Stack

**Backend:** Go 1.26, Cobra CLI, gorilla/websocket, pgx/v5 (database/sql, no ORM), golang-migrate, go-rod/rod, telego (Telegram)
**Web UI:** React 19, Vite 6, TypeScript, Tailwind CSS 4, Radix UI, Zustand, React Router 7. Located in `ui/web/`. **Use `pnpm` (not npm).**
**Database:** PostgreSQL 18 with pgvector. Raw SQL with `$1, $2` positional params. Nullable columns: `*string`, `*time.Time`, etc.

## Project Structure

```
cmd/                          CLI commands, gateway startup, onboard wizard, migrations
internal/
├── agent/                    Agent loop (think→act→observe), router, resolver, input guard
├── bootstrap/                System prompt files (SOUL.md, IDENTITY.md) + seeding + per-user seed
├── bus/                      Event bus system
├── cache/                    Caching layer
├── channels/                 Channel manager: Telegram, Feishu/Lark, Zalo, Discord, WhatsApp
├── config/                   Config loading (JSON5) + env var overlay
├── crypto/                   AES-256-GCM encryption for API keys
├── cron/                     Cron scheduling (at/every/cron expr)
├── gateway/                  WS + HTTP server, client, method router
│   └── methods/              RPC handlers (chat, agents, sessions, config, skills, cron, pairing)
├── hooks/                    Hook system for extensibility
├── http/                     HTTP API (/v1/chat/completions, /v1/agents, /v1/skills, etc.)
├── i18n/                     Message catalog: T(locale, key, args...) + per-locale catalogs (en/vi/zh)
├── knowledgegraph/           Knowledge graph storage and traversal
├── mcp/                      Model Context Protocol bridge/server
├── media/                    Media handling utilities
├── memory/                   Memory system (pgvector)
├── oauth/                    OAuth authentication
├── permissions/              RBAC (admin/operator/viewer)
├── providers/                LLM providers: Anthropic (native HTTP+SSE), OpenAI-compat (HTTP+SSE), DashScope (Alibaba Qwen), Claude CLI (stdio+MCP bridge), ACP (Anthropic Console Proxy), Codex (OpenAI)
├── sandbox/                  Docker-based code sandbox
├── scheduler/                Lane-based concurrency (main/subagent/cron)
├── sessions/                 Session management
├── skills/                   SKILL.md loader + BM25 search
├── store/                    Store interfaces + pg/ (PostgreSQL) implementations
├── tasks/                    Task management
├── tools/                    Tool registry, filesystem, exec, web, memory, subagent, MCP bridge
├── tracing/                  LLM call tracing + optional OTel export (build-tag gated)
├── tts/                      Text-to-Speech (OpenAI, ElevenLabs, Edge, MiniMax)
├── upgrade/                  Database schema version tracking
pkg/protocol/                 Wire types (frames, methods, errors, events)
pkg/browser/                  Browser automation (Rod + CDP)
migrations/                   PostgreSQL migration files
ui/web/                       React SPA (pnpm, Vite, Tailwind, Radix UI)
```

## Auth Context Injection

- `injectJWTContext(ctx, r)` — helper central para injetar tenant_id + user_id dos JWT claims no contexto
- Chamado por: `requireAuth`, `requireAuthBearer`, e 4 handlers diretos (chat_completions, responses, tools_invoke, wake)
- `requireAuthBearer` retorna `(*http.Request, bool)` — callers DEVEM usar o request retornado: `if r, ok = requireAuthBearer(...); !ok { return }`
- HTTP auth priority: gateway token → API key → browser pairing → JWT claims (4th fallback)
- WS auth priority (`handleConnect`): gateway token → API key → **JWT token** → no-token fallback → browser pairing → viewer fallback. JWT path added in PR #44 (`v1.90.0-ws-jwt-auth`). Fail-closed: invalid JWT → ErrUnauthorized (never falls to viewer).
- Role mapping: owner/admin → RoleAdmin, member/operator → RoleOperator, default → RoleViewer

## Key Patterns

- **Store layer:** Interface-based (`store.SessionStore`, `store.AgentStore`, etc.) with pg/ (PostgreSQL) implementations. Uses `database/sql` + `pgx/v5/stdlib`, raw SQL, `execMapUpdate()` helper in `pg/helpers.go`
- **Agent types:** `open` (per-user context, 7 files) vs `predefined` (shared context + USER.md per-user)
- **Context files:** `agent_context_files` (agent-level) + `user_context_files` (per-user), routed via `ContextFileInterceptor`
- **Providers:** Anthropic (native HTTP+SSE), OpenAI-compat (HTTP+SSE), DashScope (Alibaba Qwen), Claude CLI (stdio+MCP bridge), ACP (Anthropic Console Proxy), Codex (OpenAI). All use `RetryDo()` for retries. Loads from `llm_providers` table with encrypted API keys
- **Agent loop:** `RunRequest` → think→act→observe → `RunResult`. Events: `run.started`, `run.completed`, `chunk`, `tool.call`, `tool.result`. Auto-summarization at >75% context
- **Context propagation:** `store.WithAgentType(ctx)`, `store.WithUserID(ctx)`, `store.WithAgentID(ctx)`, `store.WithLocale(ctx)`, `store.WithTenantID(ctx)`
- **Multi-tenancy (fail-closed):** All PG stores use `requireTenantID(ctx)` — returns `ErrTenantRequired` if `tenant_id` absent. System operations without user context (onboard seeder, skill seeder, cron scheduler, backfill) MUST use `store.WithCrossTenant(ctx)` with comment `// appsec:cross-tenant-bypass — <justification>`. WS RPC: `MethodRouter.Handle()` injects tenant_id automatically; exempt methods: `connect`, `health`, `browser_pairing_status`
- **Skill store wrappers:** `UpdateSkill(id, updates)` and `DeleteSkill(id)` are legacy wrappers with `WithCrossTenant` — used ONLY by system skill seeder. WS/HTTP handlers MUST use `UpdateSkillWithCtx(ctx, id, updates)` / `DeleteSkillWithCtx(ctx, id)` for tenant isolation
- **WebSocket protocol (v3):** Frame types `req`/`res`/`event`. First request must be `connect`
- **Config:** JSON5 at `ARGOCLAW_CONFIG` env. Secrets in `.env.local` or env vars, never in config.json
- **Security:** Rate limiting, input guard (detection-only), CORS, shell deny patterns, SSRF protection, path traversal prevention, AES-256-GCM encryption. All security logs: `slog.Warn("security.*")`
- **Telegram formatting:** LLM output → `SanitizeAssistantContent()` → `markdownToTelegramHTML()` → `chunkHTML()` → `sendHTML()`. Tables rendered as ASCII in `<pre>` tags
- **i18n:** Web UI uses `i18next` with namespace-split locale files in `ui/web/src/i18n/locales/{lang}/`. Backend uses `internal/i18n` message catalog with `i18n.T(locale, key, args...)`. Locale propagated via `store.WithLocale(ctx)` — WS `connect` param `locale`, HTTP `Accept-Language` header. Supported: en (default), vi, zh, pt, es, fr, it, de. New user-facing strings: add key to `internal/i18n/keys.go`, add translations to all 3 backend catalog files. New UI strings: add key to all 8 locale dirs (`ui/web/src/i18n/locales/{en,vi,zh,pt,es,fr,it,de}/`). Bootstrap templates (SOUL.md, etc.) stay English-only (LLM consumption).

## Running

```bash
go build -o argoclaw . && ./argoclaw onboard && source .env.local && ./argoclaw  # entrypoint is root `.`, NOT ./cmd/argoclaw/
./argoclaw migrate up                 # DB migrations
go test -v ./tests/integration/     # Integration tests

cd ui/web && pnpm install && pnpm dev   # Web dashboard (dev)
```

- **Web UI Docker build**: Arquivos `.test.tsx` são incluídos no `tsc -b` quando `ENABLE_WEB_UI=true`. Imports não usados ou erros de tipo em testes quebram o Docker build. Sempre rodar `cd ui/web && pnpm exec tsc -b` antes de push.
- **Contexto em testes**: Quando um campo obrigatório é adicionado a uma interface de contexto (ex: `OnboardingContext`), TODOS os objetos `INITIAL_CONTEXT` em TODOS os arquivos de teste devem ser atualizados. Campo ausente em qualquer teste quebra o `tsc -b` no Cloud Build.

## GKE Deploy

```bash
# Docker build via Cloud Build — OBRIGATÓRIO usar cloudbuild.yaml para passar ENABLE_WEB_UI=true
# `--tag` puro não aceita --build-arg; sem ENABLE_WEB_UI=true a UI não é embutida (gateway retorna 404)
gcloud builds submit . --config=cloudbuild.yaml --substitutions="_IMAGE=us-central1-docker.pkg.dev/vellus-ai-agent-platform/argoclaw/argoclaw:<TAG>" --project=vellus-ai-agent-platform

# CRÍTICO: kubectl só funciona via endpoint privado. SEMPRE usar --internal-ip ao gerar kubeconfig:
gcloud container clusters get-credentials argoclaw-cluster --zone=us-central1-a --internal-ip
# Sem --internal-ip: timeout em 34.59.181.103:443 (privateEndpointEnforcementEnabled=true — público inacessível)

# IMPORTANTE: atualizar AMBOS os containers (main + init)
# Se apenas o container principal for atualizado, o init container `onboard`
# continua com a tag antiga e pode falhar (migration mismatch, ErrTenantRequired)
kubectl -n control-plane set image deployment/argoclaw-vellus argoclaw=<registry>:<TAG> onboard=<registry>:<TAG>

# SSH workaround para Windows (gcloud ssh -- "cmd" dá Bad port error):
# 1. SCP script para VM:
#    gcloud compute scp script.sh argoclaw-admin:/tmp/ --zone=us-central1-a --tunnel-through-iap
# 2. Tunnel + ssh direto:
#    gcloud compute start-iap-tunnel argoclaw-admin 22 --zone=us-central1-a --local-host-port=localhost:2299
#    ssh -o ProxyCommand=none -p 2299 -i ~/.ssh/google_compute_engine milton_vellus_tech@localhost "bash /tmp/script.sh"
```

**IMPORTANTE**: Sempre usar o Dockerfile do projeto com `--build-arg ENABLE_WEB_UI=true`. Dockerfile minimo (sem web UI) causa 404 em producao — `UIDistFS()` retorna nil.

```bash
# Build CORRETO para GKE (na VM argoclaw-admin):
git clone --depth 1 --branch main https://github.com/vellus-ai/argoclaw.git /tmp/argoclaw-build
cd /tmp/argoclaw-build
sudo docker build --no-cache --build-arg ENABLE_WEB_UI=true --build-arg VERSION=<tag> -t us-central1-docker.pkg.dev/vellus-ai-agent-platform/argoclaw/argoclaw:<tag> .
sudo docker push us-central1-docker.pkg.dev/vellus-ai-agent-platform/argoclaw/argoclaw:<tag>
kubectl -n control-plane set image deployment/argoclaw-vellus argoclaw=us-central1-docker.pkg.dev/vellus-ai-agent-platform/argoclaw/argoclaw:<tag> onboard=us-central1-docker.pkg.dev/vellus-ai-agent-platform/argoclaw/argoclaw:<tag>
kubectl -n control-plane rollout status deployment/argoclaw-vellus --timeout=5m
rm -rf /tmp/argoclaw-build
```

## Known Gotchas

### Go: embedded struct literals
- Structs que embebem `BaseModel` (ex: `LLMProviderData`, `AgentData`) **não aceitam campos promovidos diretamente** em composite literals
- ❌ `store.LLMProviderData{ID: uuid.NewSHA1(...)}` → `unknown field ID in struct literal`
- ✅ `store.LLMProviderData{BaseModel: store.BaseModel{ID: uuid.NewSHA1(...)}, ...}`

### Plugin manifest: dual-format validation
- A API de catálogo de plugins recebe manifestos em dois formatos: nested (`spec.permissions`) e legado flat (`permissions` no root)
- Sempre validar ambos — bridge struct com fallback: `perms := bridge.Spec.Permissions; if empty { perms = bridge.Permissions }`

### Cloud Build: sem triggers automáticos
- **Nenhum** Cloud Build trigger está configurado no projeto — tags Git **não disparam** build automaticamente
- O step `kubectl` do `cloudbuild.yaml` sempre falha (cluster privado, `privateEndpointEnforcementEnabled=true`)
- `gcloud builds submit` sem `.gcloudignore` envia ~365 MiB (inclui node_modules e worktrees)

### GKE Deploy: fluxo real via VM
O deploy de produção é sempre via script na VM `argoclaw-admin`:
```bash
# 1. Escrever script local com: git clone → docker build (ENABLE_WEB_UI=true) → docker push → kubectl set image (central + customers)
# 2. Enviar para VM:
gcloud compute scp deploy.sh argoclaw-admin:/tmp/ --zone=us-central1-a --tunnel-through-iap
# 3. Executar (tunnel como background + SSH direto):
gcloud compute start-iap-tunnel argoclaw-admin 22 --zone=us-central1-a --local-host-port=localhost:2299 &
ssh -p 2299 -i ~/.ssh/google_compute_engine milton_vellus_tech@localhost "bash /tmp/deploy.sh"
```
- Atualizar **sempre** os dois deployments no mesmo script: `argoclaw-central` e `argoclaw-customers`
- Tag `onboard` container DEVE ser atualizada junto com `argoclaw` (init container — mismatch causa crash)

## Post-Implementation Checklist

After implementing or modifying Go code, run these checks:

```bash
go fix ./...                        # Apply Go version upgrades (run before commit)
go build ./...                      # Compile check
go vet ./internal/... ./pkg/...     # Static analysis (skip cmd/pkg-helper — uses syscall.Umask, Linux-only)
go test ./internal/... ./pkg/...    # Unit tests (race detector requires CGO on Windows — use CI for -race)
```

Go conventions to follow:
- Use `errors.Is(err, sentinel)` instead of `err == sentinel`
- Use `switch/case` instead of `if/else if` chains on the same variable
- Use `append(dst, src...)` instead of loop-based append
- Always handle errors; don't ignore return values
- **Migrations:** When adding a new SQL migration file in `migrations/`, bump `RequiredSchemaVersion` in `internal/upgrade/version.go` to match the new migration number
- **i18n strings:** When adding user-facing error messages, add key to `internal/i18n/keys.go` and translations to `catalog_en.go`, `catalog_vi.go`, `catalog_zh.go`. For UI strings, add to all locale JSON files in `ui/web/src/i18n/locales/{en,vi,zh}/`
- **SQL safety:** When implementing or modifying SQL store code (`store/pg/*.go`), always verify: (1) All user inputs use parameterized queries (`$1, $2, ...`), never string concatenation — prevents SQL injection. (2) Queries are optimized — no N+1 queries, no unnecessary full table scans. (3) WHERE clauses, JOINs, and ORDER BY columns use existing indices — check migration files for available indexes
- **DB query reuse:** Before adding a new DB query for key entities (teams, agents, sessions, users), check if the same data is already fetched earlier in the current flow/pipeline. Prefer passing resolved data through context, event payloads, or function params rather than re-querying. Duplicate queries waste DB resources and add latency
- **Solution design:** When designing a fix or feature, identify the root cause first — don't just patch symptoms. Think through production scenarios (high concurrency, multi-tenant isolation, failure cascades, long-running sessions) to ensure the solution holds up. Prefer explicit configuration over runtime heuristics. Prefer the simplest solution that addresses the root cause directly

## Mobile UI/UX Rules

When implementing or modifying web UI components, follow these rules to ensure mobile compatibility:

- **Viewport height:** Use `h-dvh` (dynamic viewport height), never `h-screen`. `h-screen` causes content to hide behind mobile browser chrome and virtual keyboards
- **Input font-size:** All `<input>`, `<textarea>`, `<select>` must use `text-base md:text-sm` (16px on mobile). Font-size < 16px triggers iOS Safari auto-zoom on focus
- **Safe areas:** Root layout must use `viewport-fit=cover` meta tag. Apply `safe-top`, `safe-bottom`, `safe-left`, `safe-right` utility classes on edge-anchored elements (app shell, sidebar, toasts, chat input) for notched devices
- **Touch targets:** Icon buttons must have ≥44px hit area on touch devices. CSS in `index.css` uses `@media (pointer: coarse)` with `::after` pseudo-elements to expand targets
- **Tables:** Always wrap `<table>` in `<div className="overflow-x-auto">` and set `min-w-[600px]` on the table for horizontal scroll on narrow screens
- **Grid layouts:** Use mobile-first responsive grids: `grid-cols-1 sm:grid-cols-2 lg:grid-cols-N`. Never use fixed `grid-cols-N` without a mobile breakpoint
- **Dialogs:** Full-screen on mobile with slide-up animation (`max-sm:inset-0`), centered with zoom on desktop (`sm:max-w-lg`). Handled in `ui/dialog.tsx`
- **Virtual keyboard:** Chat input uses `useVirtualKeyboard()` hook + `var(--keyboard-height, 0px)` CSS var to stay above the keyboard
- **Scroll behavior:** Use `overscroll-contain` on scrollable areas to prevent background scroll. Auto-scroll: smooth for incoming messages, instant on user send
- **Landscape:** Use `landscape-compact` class on top bars to reduce padding in phone landscape orientation (`max-height: 500px`)
- **Portal dropdowns in dialogs:** Custom dropdown components using `createPortal(content, document.body)` MUST add `pointer-events-auto` class to the dropdown element. Radix Dialog sets `pointer-events: none` on `document.body` — without this class, dropdowns are unclickable. Radix-native portals (Select, Popover) handle this automatically
- **Timezone:** User timezone stored in Zustand (`useUiStore`). Charts use `formatBucketTz()` from `lib/format.ts` with native `Intl.DateTimeFormat` — no date-fns-tz dependency
- **ErrorBoundary key:** `AppLayout` uses `<ErrorBoundary key={stableErrorBoundaryKey(pathname)}>` which strips dynamic segments (`/chat/session-A` → `/chat`). NEVER use `key={location.pathname}` on ErrorBoundary/Suspense wrapping `<Outlet>` — it causes full page remount on param changes. Pages with sub-navigation (chat sessions, detail pages) must share a stable key
- **Route params as source of truth:** For pages with URL params (e.g. `/chat/:sessionKey`), derive state from `useParams()` — do NOT duplicate into `useState`. Dual state causes race conditions between `setState` and `navigate()` leading to UI flash (state bounces: B→A→B). Use optional params (`/chat/:sessionKey?`) instead of two separate routes

## Ponte de Comando Central — DESIGN APROVADO (2026-04-15)

Design completo: `argo/docs/architecture/central-command-bridge-design.md`

### Dual Deployment

O ArgoClaw opera com dois deployments K8s independentes:
- **`argoclaw-central`** → Vellus (`vellus-argo.consilium.tec.br`) — canary, node pool dedicado (tainted `vellus.ai/tier=control`)
- **`argoclaw-customers`** → Clientes (`{slug}-argo.consilium.tec.br`) — stable, node pool padrão

CI deploya automaticamente no Central. Promoção para Customers via `./promote.sh <tag>` (script em `argo/infra/terraform/k8s/scripts/`).

### Super Admin / Operator Mode

**Migration:** `operator_level INT NOT NULL DEFAULT 0` na tabela `tenants`

**Padrão no código:**
```go
// internal/gateway/router.go — handleConnect
if tenant.OperatorLevel >= 1 {
    ctx = store.WithCrossTenant(ctx)   // padrão existente (seeder/cron)
    ctx = store.WithOperatorMode(ctx, tenant.TenantID)
}
```

**Endpoints operator** (protegidos por `requireOperatorRole`):
- `GET /v1/operator/tenants` — lista todos os tenants
- `GET /v1/operator/tenants/{id}/sessions`
- `GET /v1/operator/tenants/{id}/agents`
- `GET /v1/operator/tenants/{id}/usage`

**Regra crítica:** `operator_level` é write-protected em todos os handlers públicos. Nunca expor no request body de criação/update de tenant.

### Implementação Concluída (2026-04-16)

Todos os componentes implementados e testados:
- Migration 000035 (`operator_level` + seed tenant vellus) + bump `RequiredSchemaVersion` → 35
- Context helpers: `WithOperatorMode`, `OperatorModeFromContext`, `IsOperatorMode` em `internal/store/context.go`
- `ListAllTenantsForOperator` no `TenantStore` com `WithCrossTenant` obrigatório
- Propagação de Operator Mode no WS handshake (`handleConnect`) e HTTP (`TenantMiddleware.Wrap`)
- Middleware `requireOperatorRole` em `internal/http/` — dual-check operator_level + Role
- 4 endpoints `/v1/operator/*` em `internal/http/operator.go`
- K8s manifests: `argoclaw-central/` + `argoclaw-customers/` (forks do `argoclaw-vellus/`)
- Terraform: node pool `argoclaw-control-pool` com taint `vellus.ai/tier=control`
- CI/CD: `cloudbuild.yaml` → Central + `promote.sh` → Customers
- Testes: integração (matriz de acesso PBT), isolamento cross-tenant, E2E (login → operator endpoints)

## Onboarding Conversacional — CONCLUIDO

PR #53 mergeado. Motor deterministico (12 estados, zero LLM). Arquivos-chave:
- Backend: `internal/http/onboarding.go` (GET /v1/onboarding/status, POST /v1/onboarding/action)
- Engine: `ui/web/src/pages/setup/hooks/use-onboarding-engine.ts` (state machine)
- Migration: 000033 (`last_completed_state` em `setup_progress`)
- i18n: secao `onboarding` em `setup.json` de cada locale (8 idiomas)
- Wizard legado removido: step-provider/model/agent/channel/stepper
