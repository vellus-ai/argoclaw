<!-- TODO: Substituir imagens _statics/ por branding ArgoClaw -->

<p align="center">
  <img src="_statics/goclaw.png" alt="ArgoClaw" />
</p>

<h1 align="center">ArgoClaw</h1>

<p align="center"><strong>Plataforma de Agentes IA Empresarial — por Vellus AI</strong></p>

<p align="center">
Gateway de agentes IA multi-tenant construido em Go. Fork do ArgoClaw com autenticacao PCI DSS, multi-tenancy empresarial, white-label e presets ARGO.<br/>
20+ provedores LLM. 7 canais de mensageria. PostgreSQL multi-tenant.<br/>
Binario unico. Testado em producao. Agentes que orquestram por voce.
</p>

<p align="center">
  <a href="https://github.com/vellus-ai/argoclaw">GitHub</a> &bull;
  <a href="#inicio-rapido">Inicio Rapido</a> &bull;
  <a href="https://vellus.tech">Vellus AI</a>
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

Fork Go do [OpenClaw](https://github.com/openclaw/openclaw) via [ArgoClaw](https://github.com/vellus-ai/argoclaw), com seguranca aprimorada, multi-tenancy empresarial, white-label e presets de agentes ARGO — mantido pela [Vellus AI](https://github.com/vellus-ai).

**Idiomas:**
[English](_readmes/README.en.md) &middot;
[Espanol](_readmes/README.es.md)

---

## O Que Torna Diferente

### Exclusivo ArgoClaw

- **Autenticacao PCI DSS** — Argon2id para hashing de senhas, rotacao automatica de refresh tokens, bloqueio de conta apos tentativas falhas, auditoria de acessos
- **Multi-tenancy empresarial** — Isolamento completo de dados por tenant, workspaces independentes, administracao centralizada com RBAC granular
- **White-label completo** — Personalizacao de logo, paleta de cores, dominio customizado, e-mail com remetente proprio, branding totalmente configuravel por tenant
- **6 presets de agente ARGO** — Agentes pre-configurados para diferentes funcoes empresariais:
  - **Capitao** — Orquestrador principal, coordena equipes e toma decisoes estrategicas
  - **Timoneiro** — Gerenciamento de fluxos e processos operacionais
  - **Vigia** — Monitoramento, alertas e observabilidade
  - **Artilheiro** — Execucao de tarefas intensivas e processamento em lote
  - **Navegador** — Pesquisa, busca de informacoes e navegacao web
  - **Ferreiro** — Criacao e manutencao de ferramentas, integracao e automacao
- **i18n em 8 idiomas** — Interface e mensagens em portugues, ingles, espanhol, chines, vietnamita, japones, coreano e arabe

### Herdado do ArgoClaw

- **Equipes de Agentes e Orquestracao** — Equipes com quadro de tarefas compartilhado, delegacao entre agentes (sincrona/assincrona) e descoberta hibrida de agentes
- **Multi-Tenant PostgreSQL** — Workspaces por usuario, arquivos de contexto por usuario, chaves de API criptografadas (AES-256-GCM), sessoes isoladas
- **Binario Unico** — ~25 MB binario estatico Go, sem runtime Node.js, startup <1s, roda em VPS de $5
- **Seguranca de Producao** — Sistema de permissoes em 5 camadas (auth do gateway -> politica global de tools -> por agente -> por canal -> apenas proprietario) + rate limiting, deteccao de prompt injection, protecao SSRF, padroes de bloqueio de shell e criptografia AES-256-GCM
- **20+ Provedores LLM** — Anthropic (HTTP+SSE nativo com cache de prompt), OpenAI, OpenRouter, Groq, DeepSeek, Gemini, Mistral, xAI, MiniMax, Cohere, Perplexity, DashScope, Bailian, Zai, Ollama, Ollama Cloud, Claude CLI, Codex, ACP e qualquer endpoint compativel com OpenAI
- **7 Canais de Mensageria** — Telegram, Discord, Slack, Zalo OA, Zalo Personal, Feishu/Lark, WhatsApp
- **Extended Thinking** — Modo de raciocinio por provedor (Anthropic budget tokens, OpenAI reasoning effort, DashScope thinking budget) com suporte a streaming
- **Sistema de Heartbeat** — Check-ins periodicos do agente via checklists HEARTBEAT.md com suppress-on-OK, horarios ativos, logica de retry e entrega por canal
- **Agendamento e Cron** — Expressoes `at`, `every` e cron para tarefas automatizadas de agentes com concorrencia baseada em lanes
- **Observabilidade** — Tracing integrado de chamadas LLM com spans e metricas de cache de prompt, exportacao OTLP OpenTelemetry opcional

---

## Ecossistema Claw

|                  | OpenClaw        | ZeroClaw | PicoClaw | ArgoClaw                                | **ArgoClaw**                            |
| ---------------- | --------------- | -------- | -------- | --------------------------------------- | --------------------------------------- |
| Linguagem        | TypeScript      | Rust     | Go       | Go                                      | **Go**                                  |
| Tamanho binario  | 28 MB + Node.js | 3.4 MB   | ~8 MB    | ~25 MB                                  | **~25 MB** (base) / **~36 MB** (+ OTel) |
| Imagem Docker    | —               | —        | —        | ~50 MB (Alpine)                         | **~50 MB** (Alpine)                     |
| RAM (ocioso)     | > 1 GB          | < 5 MB   | < 10 MB  | ~35 MB                                  | **~40 MB**                              |
| Startup          | > 5 s           | < 10 ms  | < 1 s    | < 1 s                                   | **< 1 s**                               |
| Hardware alvo    | $599+ Mac Mini  | $10 edge | $10 edge | $5 VPS+                                 | **$5 VPS+**                             |

| Recurso                       | OpenClaw                             | ZeroClaw                                     | PicoClaw                              | ArgoClaw                       | **ArgoClaw**                        |
| ----------------------------- | ------------------------------------ | -------------------------------------------- | ------------------------------------- | ------------------------------ | ----------------------------------- |
| Multi-tenant (PostgreSQL)     | —                                    | —                                            | —                                     | Sim                            | **Sim + isolamento por tenant**     |
| Integracao MCP                | — (usa ACP)                          | —                                            | —                                     | Sim (stdio/SSE/streamable-http)| **Sim (stdio/SSE/streamable-http)** |
| Equipes de agentes            | —                                    | —                                            | —                                     | Sim (Task board + mailbox)     | **Sim + presets ARGO**              |
| Seguranca                     | Sim (SSRF, path traversal, injection)| Sim (sandbox, rate limit, injection, pairing) | Basica                               | 5 camadas                      | **5 camadas + PCI DSS**            |
| Observabilidade OTel          | Sim (opt-in)                         | Sim (Prometheus + OTLP)                      | —                                     | Sim (OTLP opt-in)             | **Sim (OTLP opt-in)**              |
| Cache de prompt               | —                                    | —                                            | —                                     | Sim (Anthropic + OpenAI)       | **Sim (Anthropic + OpenAI)**        |
| Grafo de conhecimento         | —                                    | —                                            | —                                     | Sim (LLM + traversal)          | **Sim (LLM + traversal)**           |
| Sistema de skills             | Embeddings/semantico                 | SKILL.md + TOML                              | Basico                                | BM25 + pgvector hibrido        | **BM25 + pgvector hibrido**         |
| Agendador por lanes           | Sim                                  | Concorrencia limitada                        | —                                     | Sim (main/subagent/team/cron)  | **Sim (main/subagent/team/cron)**   |
| Canais de mensageria          | 37+                                  | 15+                                          | 10+                                   | 7+                             | **7+**                              |
| Apps complementares           | macOS, iOS, Android                  | Python SDK                                   | —                                     | Web dashboard                  | **Web dashboard + white-label**     |
| Live Canvas / Voz             | Sim (A2UI + TTS/STT)                | —                                            | Transcricao de voz                    | TTS (4 provedores)             | **TTS (4 provedores)**              |
| Provedores LLM                | 10+                                  | 8 nativos + 29 compat                        | 13+                                   | 20+                            | **20+**                             |
| Workspaces por usuario        | Sim (baseado em arquivos)            | —                                            | —                                     | Sim (PostgreSQL)               | **Sim (PostgreSQL + tenant)**       |
| Segredos criptografados       | — (env vars apenas)                  | ChaCha20-Poly1305                            | — (JSON plaintext)                    | AES-256-GCM no DB              | **AES-256-GCM no DB**              |
| White-label                   | —                                    | —                                            | —                                     | —                              | **Sim (logo, cores, dominio)**      |
| Presets de agentes            | —                                    | —                                            | —                                     | —                              | **6 presets ARGO**                  |
| i18n                          | —                                    | —                                            | —                                     | 3 idiomas                      | **8 idiomas**                       |

---

## Arquitetura

<p align="center">
  <img src="_statics/architecture.jpg" alt="Arquitetura ArgoClaw" width="800" />
</p>

---

## Inicio Rapido

**Pre-requisitos:** Go 1.26+, PostgreSQL 18 com pgvector, Docker (opcional)

### A Partir do Codigo Fonte

```bash
git clone https://github.com/vellus-ai/argoclaw.git && cd argoclaw
make build
./argoclaw onboard        # Assistente de configuracao interativo
source .env.local && ./argoclaw
```

### Com Docker

```bash
# Gerar .env com segredos auto-gerados
chmod +x prepare-env.sh && ./prepare-env.sh

# Adicione pelo menos uma ARGOCLAW_*_API_KEY ao .env, depois:
docker compose -f docker-compose.yml -f docker-compose.postgres.yml \
  -f docker-compose.selfservice.yml up -d

# Web Dashboard em http://localhost:3000
# Health check: curl http://localhost:18790/health
```

Quando variaveis de ambiente `ARGOCLAW_*_API_KEY` estao definidas, o gateway faz onboarding automatico sem prompts interativos — detecta o provedor, executa migrations e popula dados iniciais.

> Para variantes de build (OTel, Tailscale, Redis), tags de imagem Docker e overlays de compose, consulte o [Guia de Deploy](https://docs.argoclaw.vellus.tech/#deploy-docker-compose).

### Imagem Docker

```bash
docker pull ghcr.io/vellus-ai/argoclaw:latest
```

---

## Orquestracao Multi-Agente

ArgoClaw suporta equipes de agentes e delegacao entre agentes — cada agente opera com sua propria identidade, ferramentas, provedor LLM e arquivos de contexto.

### Delegacao de Agentes

<p align="center">
  <img src="_statics/agent-delegation.jpg" alt="Delegacao de Agentes" width="700" />
</p>

| Modo | Como funciona | Melhor para |
|------|--------------|-------------|
| **Sincrono** | Agente A pergunta ao Agente B e **aguarda** a resposta | Consultas rapidas, verificacao de fatos |
| **Assincrono** | Agente A pergunta ao Agente B e **segue em frente**. B anuncia depois | Tarefas longas, relatorios, analises profundas |

Agentes se comunicam atraves de **links de permissao** explicitos com controle de direcao (`outbound`, `inbound`, `bidirectional`) e limites de concorrencia nos niveis por link e por agente.

### Equipes de Agentes

<p align="center">
  <img src="_statics/agent-teams.jpg" alt="Fluxo de Equipes de Agentes" width="800" />
</p>

- **Quadro de tarefas compartilhado** — Criar, reivindicar, concluir, buscar tarefas com dependencias `blocked_by`
- **Caixa de mensagens da equipe** — Mensagens diretas ponto a ponto e broadcasts
- **Ferramentas**: `team_tasks` para gerenciamento de tarefas, `team_message` para caixa de mensagens

> Para detalhes sobre delegacao, links de permissao e controle de concorrencia, consulte a [documentacao de Equipes de Agentes](https://docs.argoclaw.vellus.tech/#teams-what-are-teams).

---

## Ferramentas Integradas

| Ferramenta          | Grupo        | Descricao                                                     |
| ------------------- | ------------ | ------------------------------------------------------------- |
| `read_file`         | fs           | Ler conteudo de arquivos (com roteamento FS virtual)          |
| `write_file`        | fs           | Escrever/criar arquivos                                       |
| `edit_file`         | fs           | Aplicar edicoes direcionadas em arquivos existentes           |
| `list_files`        | fs           | Listar conteudo de diretorios                                 |
| `search`            | fs           | Buscar conteudo de arquivos por padrao                        |
| `glob`              | fs           | Encontrar arquivos por padrao glob                            |
| `exec`              | runtime      | Executar comandos shell (com fluxo de aprovacao)              |
| `web_search`        | web          | Buscar na web (Brave, DuckDuckGo)                             |
| `web_fetch`         | web          | Buscar e processar conteudo web                               |
| `memory_search`     | memory       | Buscar memoria de longo prazo (FTS + vetor)                   |
| `memory_get`        | memory       | Recuperar entradas de memoria                                 |
| `skill_search`      | —            | Buscar skills (hibrido BM25 + embedding)                      |
| `knowledge_graph_search` | memory  | Buscar entidades e percorrer relacionamentos do grafo         |
| `create_image`      | media        | Geracao de imagens (DashScope, MiniMax)                       |
| `create_audio`      | media        | Geracao de audio (OpenAI, ElevenLabs, MiniMax, Suno)          |
| `create_video`      | media        | Geracao de video (MiniMax, Veo)                               |
| `read_document`     | media        | Leitura de documentos (Gemini File API, cadeia de provedores) |
| `read_image`        | media        | Analise de imagens                                            |
| `read_audio`        | media        | Transcricao e analise de audio                                |
| `read_video`        | media        | Analise de video                                              |
| `message`           | messaging    | Enviar mensagens para canais                                  |
| `tts`               | —            | Sintese de texto para fala                                    |
| `spawn`             | —            | Iniciar um subagente                                          |
| `subagents`         | sessions     | Controlar subagentes em execucao                              |
| `team_tasks`        | teams        | Quadro de tarefas (listar, criar, reivindicar, concluir, buscar) |
| `team_message`      | teams        | Caixa de mensagens da equipe (enviar, broadcast, ler)         |
| `sessions_list`     | sessions     | Listar sessoes ativas                                         |
| `sessions_history`  | sessions     | Visualizar historico de sessoes                               |
| `sessions_send`     | sessions     | Enviar mensagem para uma sessao                               |
| `sessions_spawn`    | sessions     | Iniciar nova sessao                                           |
| `session_status`    | sessions     | Verificar status da sessao                                    |
| `cron`              | automation   | Agendar e gerenciar jobs cron                                 |
| `gateway`           | automation   | Administracao do gateway                                      |
| `browser`           | ui           | Automacao de navegador (navegar, clicar, digitar, screenshot) |
| `announce_queue`    | automation   | Fila de anuncios assincronos (para delegacoes assincronas)    |

---

## Documentacao

Documentacao completa em **[docs.argoclaw.vellus.tech](https://docs.argoclaw.vellus.tech)** — ou navegue pelo codigo fonte em [`argoclaw-docs/`](https://github.com/vellus-ai/argoclaw-docs)

| Secao | Topicos |
|-------|---------|
| [Primeiros Passos](https://docs.argoclaw.vellus.tech/#what-is-argoclaw) | Instalacao, Inicio Rapido, Configuracao, Tour do Web Dashboard |
| [Conceitos Principais](https://docs.argoclaw.vellus.tech/#how-argoclaw-works) | Loop do Agente, Sessoes, Ferramentas, Memoria, Multi-Tenancy |
| [Agentes](https://docs.argoclaw.vellus.tech/#creating-agents) | Criando Agentes, Arquivos de Contexto, Personalidade, Compartilhamento e Acesso |
| [Provedores](https://docs.argoclaw.vellus.tech/#providers-overview) | Anthropic, OpenAI, OpenRouter, Gemini, DeepSeek e +15 |
| [Canais](https://docs.argoclaw.vellus.tech/#channels-overview) | Telegram, Discord, Slack, Feishu, Zalo, WhatsApp, WebSocket |
| [Equipes de Agentes](https://docs.argoclaw.vellus.tech/#teams-what-are-teams) | Equipes, Quadro de Tarefas, Mensageria, Delegacao e Handoff |
| [Avancado](https://docs.argoclaw.vellus.tech/#custom-tools) | Ferramentas Customizadas, MCP, Skills, Cron, Sandbox, Hooks, RBAC |
| [Deploy](https://docs.argoclaw.vellus.tech/#deploy-docker-compose) | Docker Compose, Banco de Dados, Seguranca, Observabilidade, Tailscale |
| [Referencia](https://docs.argoclaw.vellus.tech/#cli-commands) | Comandos CLI, API REST, Protocolo WebSocket, Variaveis de Ambiente |

---

## Testes

```bash
go test ./...                                    # Testes unitarios
go test -v ./tests/integration/ -timeout 120s    # Testes de integracao (requer gateway em execucao)
```

---

## Status do Projeto

Consulte o [CHANGELOG.md](CHANGELOG.md) para o status detalhado de funcionalidades, incluindo o que foi testado em producao e o que ainda esta em desenvolvimento.

---

## Agradecimentos

ArgoClaw e construido sobre o projeto original [OpenClaw](https://github.com/openclaw/openclaw) e seu port em Go, [ArgoClaw](https://github.com/vellus-ai/argoclaw). Somos gratos pela arquitetura e visao que inspiraram este fork empresarial.

---

## Licenca

[CC BY-NC 4.0](LICENSE) — Creative Commons Attribution-NonCommercial 4.0 International
