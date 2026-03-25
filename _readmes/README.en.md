<!-- TODO: Replace _statics/ images with ArgoClaw branding -->

<p align="center">
  <img src="../_statics/argoclaw.png" alt="ArgoClaw" />
</p>

<h1 align="center">ArgoClaw</h1>

<p align="center"><strong>Enterprise AI Agent Platform by Vellus AI</strong></p>

<p align="center">
Multi-tenant AI agent gateway built in Go. Fork of ArgoClaw with PCI DSS authentication, enterprise multi-tenancy, white-label, and ARGO presets.<br/>
20+ LLM providers. 7 messaging channels. Single binary. Production-ready.
</p>

<p align="center">
  <a href="https://github.com/vellus-ai/argoclaw">GitHub</a> &bull;
  <a href="https://docs.argoclaw.vellus.tech">Documentation</a> &bull;
  <a href="https://docs.argoclaw.vellus.tech/#quick-start">Quick Start</a>
</p>

<p align="center">
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go_1.26-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go" /></a>
  <a href="https://www.postgresql.org/"><img src="https://img.shields.io/badge/PostgreSQL_18-316192?style=flat-square&logo=postgresql&logoColor=white" alt="PostgreSQL" /></a>
  <a href="https://www.docker.com/"><img src="https://img.shields.io/badge/Docker-2496ED?style=flat-square&logo=docker&logoColor=white" alt="Docker" /></a>
  <a href="https://developer.mozilla.org/en-US/docs/Web/API/WebSocket"><img src="https://img.shields.io/badge/WebSocket-010101?style=flat-square&logo=socket.io&logoColor=white" alt="WebSocket" /></a>
  <a href="https://opentelemetry.io/"><img src="https://img.shields.io/badge/OpenTelemetry-000000?style=flat-square&logo=opentelemetry&logoColor=white" alt="OpenTelemetry" /></a>
  <a href="https://www.anthropic.com/"><img src="https://img.shields.io/badge/Anthropic-191919?style=flat-square&logo=anthropic&logoColor=white" alt="Anthropic" /></a>
  <a href="https://openai.com/"><img src="https://img.shields.io/badge/OpenAI_Compatible-412991?style=flat-square&logo=openai&logoColor=white" alt="OpenAI" /></a>
  <img src="https://img.shields.io/badge/License-CC_BY--NC_4.0-lightgrey?style=flat-square" alt="License: CC BY-NC 4.0" />
</p>

A Go fork of [ArgoClaw](https://github.com/vellus-ai/argoclaw) by [Vellus AI](https://github.com/vellus-ai), adding PCI DSS authentication, enterprise multi-tenancy, full white-label support, and 6 ARGO agent presets.

**Languages:**
[Portugues (pt-BR)](../README.md) &middot;
[English](README.en.md) &middot;
[Espanol](README.es.md)

## What Makes It Different

- **PCI DSS Authentication** -- Argon2id password hashing, refresh token rotation, automatic account lockout after failed attempts
- **Enterprise Multi-Tenancy** -- Per-tenant data isolation in PostgreSQL, tenant-scoped API keys, isolated sessions and workspaces
- **Full White-Label** -- Customizable logo, colors, domain, and email templates per tenant
- **6 ARGO Agent Presets** -- Captain, Helmsman, Lookout, Gunner, Navigator, Blacksmith -- ready-to-use enterprise agent archetypes
- **Agent Teams & Orchestration** -- Teams with shared task boards, inter-agent delegation (sync/async), and hybrid agent discovery
- **20+ LLM Providers** -- Anthropic (native HTTP+SSE with prompt caching), OpenAI, OpenRouter, Groq, DeepSeek, Gemini, Mistral, xAI, MiniMax, Cohere, Perplexity, DashScope, Bailian, Zai, Ollama, Ollama Cloud, Claude CLI, Codex, ACP, and any OpenAI-compatible endpoint
- **7 Messaging Channels** -- Telegram, Discord, Slack, Zalo OA, Zalo Personal, Feishu/Lark, WhatsApp
- **i18n in 8 Languages** -- Full internationalization support out of the box
- **Single Binary** -- ~25 MB static Go binary, no Node.js runtime, <1s startup, runs on a $5 VPS
- **5-Layer Security** -- Gateway auth, global tool policy, per-agent, per-channel, owner-only permissions plus rate limiting, prompt injection detection, SSRF protection, shell deny patterns, and AES-256-GCM encryption
- **Built-in LLM Tracing** -- Span-level tracing with prompt cache metrics and optional OpenTelemetry OTLP export
- **Extended Thinking** -- Per-provider thinking mode (Anthropic budget tokens, OpenAI reasoning effort, DashScope thinking budget) with streaming support
- **Heartbeat System** -- Periodic agent check-ins via HEARTBEAT.md checklists with suppress-on-OK, active hours, retry logic, and channel delivery
- **Scheduling & Cron** -- `at`, `every`, and cron expressions for automated agent tasks with lane-based concurrency

## Claw Ecosystem

|                 | OpenClaw        | ZeroClaw | PicoClaw | ArgoClaw                                  | **ArgoClaw**                            |
| --------------- | --------------- | -------- | -------- | --------------------------------------- | --------------------------------------- |
| Language        | TypeScript      | Rust     | Go       | Go                                      | **Go**                                  |
| Binary size     | 28 MB + Node.js | 3.4 MB   | ~8 MB    | ~25 MB (base) / ~36 MB (+ OTel)        | **~25 MB** (base) / **~36 MB** (+ OTel) |
| Docker image    | --              | --       | --       | ~50 MB (Alpine)                         | **~50 MB** (Alpine)                     |
| RAM (idle)      | > 1 GB          | < 5 MB   | < 10 MB  | ~35 MB                                  | **~40 MB**                              |
| Startup         | > 5 s           | < 10 ms  | < 1 s    | < 1 s                                   | **< 1 s**                               |
| Target hardware | $599+ Mac Mini  | $10 edge | $10 edge | $5 VPS+                                 | **$5 VPS+**                             |

| Feature                    | OpenClaw                             | ZeroClaw                                     | PicoClaw                              | ArgoClaw                         | **ArgoClaw**                         |
| -------------------------- | ------------------------------------ | -------------------------------------------- | ------------------------------------- | ------------------------------ | ------------------------------------ |
| Multi-tenant (PostgreSQL)  | --                                   | --                                           | --                                    | Yes                            | **Yes (enterprise-grade)**           |
| PCI DSS auth               | --                                   | --                                           | --                                    | --                             | **Yes (Argon2id + token rotation)**  |
| White-label                | --                                   | --                                           | --                                    | --                             | **Yes (logo, colors, domain, email)**|
| ARGO presets               | --                                   | --                                           | --                                    | --                             | **6 presets**                        |
| MCP integration            | -- (uses ACP)                        | --                                           | --                                    | Yes (stdio/SSE/streamable-http)| **Yes (stdio/SSE/streamable-http)**  |
| Agent teams                | --                                   | --                                           | --                                    | Yes (task board + mailbox)     | **Yes (task board + mailbox)**       |
| Security hardening         | Yes (SSRF, path traversal, injection)| Yes (sandbox, rate limit, injection, pairing)| Basic (workspace restrict, exec deny) | 5-layer defense                | **5-layer + PCI DSS**                |
| OTel observability         | Yes (opt-in extension)               | Yes (Prometheus + OTLP)                      | --                                    | Yes (OTLP, opt-in build tag)  | **Yes (OTLP, opt-in build tag)**     |
| Prompt caching             | --                                   | --                                           | --                                    | Yes (Anthropic + OpenAI-compat)| **Yes (Anthropic + OpenAI-compat)**  |
| Knowledge graph            | --                                   | --                                           | --                                    | Yes (LLM extraction + traversal)| **Yes (LLM extraction + traversal)**|
| i18n                       | --                                   | --                                           | --                                    | 3 languages                    | **8 languages**                      |
| LLM providers              | 10+                                  | 8 native + 29 compat                         | 13+                                   | 20+                            | **20+**                              |
| Encrypted secrets          | -- (env vars only)                   | Yes (ChaCha20-Poly1305)                      | -- (plaintext JSON)                   | Yes (AES-256-GCM in DB)       | **Yes (AES-256-GCM in DB)**          |

## Architecture

<p align="center">
  <img src="../_statics/architecture.jpg" alt="ArgoClaw Architecture" width="800" />
</p>

## Quick Start

**Prerequisites:** Go 1.26+, PostgreSQL 18 with pgvector, Docker (optional)

### From Source

```bash
git clone https://github.com/vellus-ai/argoclaw.git && cd argoclaw
make build
./argoclaw onboard        # Interactive setup wizard
source .env.local && ./argoclaw
```

### With Docker

```bash
# Pull the latest image
docker pull ghcr.io/vellus-ai/argoclaw:latest

# Generate .env with auto-generated secrets
chmod +x prepare-env.sh && ./prepare-env.sh

# Add at least one GOCLAW_*_API_KEY to .env, then:
docker compose -f docker-compose.yml -f docker-compose.postgres.yml \
  -f docker-compose.selfservice.yml up -d

# Web Dashboard at http://localhost:3000
# Health check: curl http://localhost:18790/health
```

When `GOCLAW_*_API_KEY` environment variables are set, the gateway auto-onboards without interactive prompts -- detects provider, runs migrations, and seeds default data.

> For build variants (OTel, Tailscale, Redis), Docker image tags, and compose overlays, see the [Deployment Guide](https://docs.argoclaw.vellus.tech/#deploy-docker-compose).

## ARGO Agent Presets

ArgoClaw includes 6 pre-configured agent archetypes designed for enterprise workflows:

| Preset | Role | Description |
|--------|------|-------------|
| **Captain** | Leadership & coordination | Orchestrates teams, delegates tasks, makes strategic decisions |
| **Helmsman** | Execution & operations | Handles day-to-day task execution and process management |
| **Lookout** | Monitoring & intelligence | Watches for signals, gathers data, provides situational awareness |
| **Gunner** | Action & resolution | Takes decisive action on issues, handles incidents and escalations |
| **Navigator** | Planning & strategy | Charts courses, analyzes options, provides recommendations |
| **Blacksmith** | Building & tooling | Creates tools, templates, and automations for the team |

## Multi-Agent Orchestration

ArgoClaw supports agent teams and inter-agent delegation -- each agent runs with its own identity, tools, LLM provider, and context files.

### Agent Delegation

<p align="center">
  <img src="../_statics/agent-delegation.jpg" alt="Agent Delegation" width="700" />
</p>

| Mode | How it works | Best for |
|------|-------------|----------|
| **Sync** | Agent A asks Agent B and **waits** for the answer | Quick lookups, fact checks |
| **Async** | Agent A asks Agent B and **moves on**. B announces later | Long tasks, reports, deep analysis |

Agents communicate through explicit **permission links** with direction control (`outbound`, `inbound`, `bidirectional`) and concurrency limits at both per-link and per-agent levels.

### Agent Teams

<p align="center">
  <img src="../_statics/agent-teams.jpg" alt="Agent Teams Workflow" width="800" />
</p>

- **Shared task board** -- Create, claim, complete, search tasks with `blocked_by` dependencies
- **Team mailbox** -- Direct peer-to-peer messaging and broadcasts
- **Tools**: `team_tasks` for task management, `team_message` for mailbox

> For delegation details, permission links, and concurrency control, see the [Agent Teams docs](https://docs.argoclaw.vellus.tech/#teams-what-are-teams).

## Built-in Tools

| Tool               | Group         | Description                                                  |
| ------------------ | ------------- | ------------------------------------------------------------ |
| `read_file`        | fs            | Read file contents (with virtual FS routing)                 |
| `write_file`       | fs            | Write/create files                                           |
| `edit_file`        | fs            | Apply targeted edits to existing files                       |
| `list_files`       | fs            | List directory contents                                      |
| `search`           | fs            | Search file contents by pattern                              |
| `glob`             | fs            | Find files by glob pattern                                   |
| `exec`             | runtime       | Execute shell commands (with approval workflow)              |
| `web_search`       | web           | Search the web (Brave, DuckDuckGo)                           |
| `web_fetch`        | web           | Fetch and parse web content                                  |
| `memory_search`    | memory        | Search long-term memory (FTS + vector)                       |
| `memory_get`       | memory        | Retrieve memory entries                                      |
| `skill_search`     | --            | Search skills (BM25 + embedding hybrid)                      |
| `knowledge_graph_search` | memory  | Search entities and traverse knowledge graph relationships   |
| `create_image`     | media         | Image generation (DashScope, MiniMax)                        |
| `create_audio`     | media         | Audio generation (OpenAI, ElevenLabs, MiniMax, Suno)         |
| `create_video`     | media         | Video generation (MiniMax, Veo)                              |
| `read_document`    | media         | Document reading (Gemini File API, provider chain)           |
| `read_image`       | media         | Image analysis                                               |
| `read_audio`       | media         | Audio transcription and analysis                             |
| `read_video`       | media         | Video analysis                                               |
| `message`          | messaging     | Send messages to channels                                    |
| `tts`              | --            | Text-to-Speech synthesis                                     |
| `spawn`            | --            | Spawn a subagent                                             |
| `subagents`        | sessions      | Control running subagents                                    |
| `team_tasks`       | teams         | Shared task board (list, create, claim, complete, search)    |
| `team_message`     | teams         | Team mailbox (send, broadcast, read)                         |
| `sessions_list`    | sessions      | List active sessions                                         |
| `sessions_history` | sessions      | View session history                                         |
| `sessions_send`    | sessions      | Send message to a session                                    |
| `sessions_spawn`   | sessions      | Spawn a new session                                          |
| `session_status`   | sessions      | Check session status                                         |
| `cron`             | automation    | Schedule and manage cron jobs                                |
| `gateway`          | automation    | Gateway administration                                       |
| `browser`          | ui            | Browser automation (navigate, click, type, screenshot)       |
| `announce_queue`   | automation    | Async result announcement (for async delegations)            |

## Documentation

Full documentation at **[docs.argoclaw.vellus.tech](https://docs.argoclaw.vellus.tech)** -- or browse the source in [`argoclaw-docs/`](https://github.com/vellus-ai/argoclaw-docs)

| Section | Topics |
|---------|--------|
| [Getting Started](https://docs.argoclaw.vellus.tech/#what-is-argoclaw) | Installation, Quick Start, Configuration, Web Dashboard Tour |
| [Core Concepts](https://docs.argoclaw.vellus.tech/#how-argoclaw-works) | Agent Loop, Sessions, Tools, Memory, Multi-Tenancy |
| [Agents](https://docs.argoclaw.vellus.tech/#creating-agents) | Creating Agents, Context Files, Personality, Sharing & Access |
| [Providers](https://docs.argoclaw.vellus.tech/#providers-overview) | Anthropic, OpenAI, OpenRouter, Gemini, DeepSeek, +15 more |
| [Channels](https://docs.argoclaw.vellus.tech/#channels-overview) | Telegram, Discord, Slack, Feishu, Zalo, WhatsApp, WebSocket |
| [Agent Teams](https://docs.argoclaw.vellus.tech/#teams-what-are-teams) | Teams, Task Board, Messaging, Delegation & Handoff |
| [Advanced](https://docs.argoclaw.vellus.tech/#custom-tools) | Custom Tools, MCP, Skills, Cron, Sandbox, Hooks, RBAC |
| [Deployment](https://docs.argoclaw.vellus.tech/#deploy-docker-compose) | Docker Compose, Database, Security, Observability, Tailscale |
| [Reference](https://docs.argoclaw.vellus.tech/#cli-commands) | CLI Commands, REST API, WebSocket Protocol, Environment Variables |

## Testing

```bash
go test ./...                                    # Unit tests
go test -v ./tests/integration/ -timeout 120s    # Integration tests (requires running gateway)
```

## Acknowledgments

ArgoClaw is a fork of [ArgoClaw](https://github.com/vellus-ai/argoclaw), which is itself a Go port of [OpenClaw](https://github.com/openclaw/openclaw). We are grateful for the architecture and vision that inspired both projects.

Built and maintained by [Vellus AI](https://github.com/vellus-ai).

## License

[CC BY-NC 4.0](https://creativecommons.org/licenses/by-nc/4.0/) -- Creative Commons Attribution-NonCommercial 4.0 International
