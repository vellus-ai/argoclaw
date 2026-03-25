<!-- TODO: Replace _statics/ images with ArgoClaw branding -->

<p align="center">
  <img src="../_statics/argoclaw.png" alt="ArgoClaw" />
</p>

<h1 align="center">ArgoClaw</h1>

<p align="center"><strong>Plataforma Empresarial de Agentes IA por Vellus AI</strong></p>

<p align="center">
Gateway multi-tenant de agentes IA construido en Go. Fork de ArgoClaw con autenticacion PCI DSS, multi-tenancy empresarial, white-label y presets ARGO.<br/>
20+ proveedores LLM. 7 canales de mensajeria. Un solo binario. Listo para produccion.
</p>

<p align="center">
  <a href="https://github.com/vellus-ai/argoclaw">GitHub</a> &bull;
  <a href="https://docs.argoclaw.vellus.tech">Documentacion</a> &bull;
  <a href="https://docs.argoclaw.vellus.tech/#quick-start">Inicio Rapido</a>
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

Un fork en Go de [ArgoClaw](https://github.com/vellus-ai/arargoclaw) por [Vellus AI](https://github.com/vellus-ai), que agrega autenticacion PCI DSS, multi-tenancy empresarial, soporte completo de white-label y 6 presets de agentes ARGO.

**Idiomas:**
[Portugues (pt-BR)](../README.md) &middot;
[English](README.en.md) &middot;
[Espanol](README.es.md)

## Que Lo Hace Diferente

- **Autenticacion PCI DSS** -- Hash de contrasenas con Argon2id, rotacion de refresh tokens, bloqueo automatico de cuenta tras intentos fallidos
- **Multi-Tenancy Empresarial** -- Aislamiento de datos por tenant en PostgreSQL, claves API con alcance por tenant, sesiones y espacios de trabajo aislados
- **White-Label Completo** -- Logo, colores, dominio y plantillas de email personalizables por tenant
- **6 Presets de Agentes ARGO** -- Capitan, Timonel, Vigia, Artillero, Navegante, Herrero -- arquetipos de agentes empresariales listos para usar
- **Equipos de Agentes y Orquestacion** -- Equipos con tableros de tareas compartidos, delegacion entre agentes (sincrona/asincrona) y descubrimiento hibrido de agentes
- **20+ Proveedores LLM** -- Anthropic (HTTP+SSE nativo con cache de prompts), OpenAI, OpenRouter, Groq, DeepSeek, Gemini, Mistral, xAI, MiniMax, Cohere, Perplexity, DashScope, Bailian, Zai, Ollama, Ollama Cloud, Claude CLI, Codex, ACP y cualquier endpoint compatible con OpenAI
- **7 Canales de Mensajeria** -- Telegram, Discord, Slack, Zalo OA, Zalo Personal, Feishu/Lark, WhatsApp
- **i18n en 8 Idiomas** -- Soporte completo de internacionalizacion listo para usar
- **Un Solo Binario** -- Binario estatico de Go de ~25 MB, sin runtime de Node.js, inicio en <1s, funciona en un VPS de $5
- **Seguridad en 5 Capas** -- Autenticacion del gateway, politica global de herramientas, por agente, por canal, permisos solo del propietario, ademas de rate limiting, deteccion de inyeccion de prompts, proteccion SSRF, patrones de denegacion de shell y cifrado AES-256-GCM
- **Trazado LLM Integrado** -- Trazado a nivel de span con metricas de cache de prompts y exportacion opcional via OpenTelemetry OTLP
- **Pensamiento Extendido** -- Modo de pensamiento por proveedor (tokens de presupuesto Anthropic, esfuerzo de razonamiento OpenAI, presupuesto de pensamiento DashScope) con soporte de streaming
- **Sistema de Heartbeat** -- Verificaciones periodicas del agente via checklists HEARTBEAT.md con supresion en OK, horas activas, logica de reintento y entrega por canal
- **Programacion y Cron** -- Expresiones `at`, `every` y cron para tareas automatizadas de agentes con concurrencia basada en carriles

## Ecosistema Claw

|                 | OpenClaw        | ZeroClaw | PicoClaw | ArgoClaw                                  | **ArgoClaw**                            |
| --------------- | --------------- | -------- | -------- | --------------------------------------- | --------------------------------------- |
| Lenguaje        | TypeScript      | Rust     | Go       | Go                                      | **Go**                                  |
| Tamano binario  | 28 MB + Node.js | 3.4 MB   | ~8 MB    | ~25 MB (base) / ~36 MB (+ OTel)        | **~25 MB** (base) / **~36 MB** (+ OTel) |
| Imagen Docker   | --              | --       | --       | ~50 MB (Alpine)                         | **~50 MB** (Alpine)                     |
| RAM (inactivo)  | > 1 GB          | < 5 MB   | < 10 MB  | ~35 MB                                  | **~40 MB**                              |
| Inicio          | > 5 s           | < 10 ms  | < 1 s    | < 1 s                                   | **< 1 s**                               |
| Hardware objeto | $599+ Mac Mini  | $10 edge | $10 edge | $5 VPS+                                 | **$5 VPS+**                             |

| Funcionalidad              | OpenClaw                             | ZeroClaw                                     | PicoClaw                              | ArgoClaw                         | **ArgoClaw**                         |
| -------------------------- | ------------------------------------ | -------------------------------------------- | ------------------------------------- | ------------------------------ | ------------------------------------ |
| Multi-tenant (PostgreSQL)  | --                                   | --                                           | --                                    | Si                             | **Si (grado empresarial)**           |
| Autenticacion PCI DSS      | --                                   | --                                           | --                                    | --                             | **Si (Argon2id + rotacion de tokens)**|
| White-label                | --                                   | --                                           | --                                    | --                             | **Si (logo, colores, dominio, email)**|
| Presets ARGO               | --                                   | --                                           | --                                    | --                             | **6 presets**                        |
| Integracion MCP            | -- (usa ACP)                         | --                                           | --                                    | Si (stdio/SSE/streamable-http) | **Si (stdio/SSE/streamable-http)**   |
| Equipos de agentes         | --                                   | --                                           | --                                    | Si (tablero + buzon)           | **Si (tablero + buzon)**             |
| Seguridad reforzada        | Si (SSRF, path traversal, inyeccion) | Si (sandbox, rate limit, inyeccion, pairing) | Basica (restrict workspace, exec deny)| Defensa en 5 capas             | **5 capas + PCI DSS**                |
| Observabilidad OTel        | Si (extension opt-in)                | Si (Prometheus + OTLP)                       | --                                    | Si (OTLP, build tag opt-in)   | **Si (OTLP, build tag opt-in)**      |
| Cache de prompts           | --                                   | --                                           | --                                    | Si (Anthropic + OpenAI-compat) | **Si (Anthropic + OpenAI-compat)**   |
| Grafo de conocimiento      | --                                   | --                                           | --                                    | Si (extraccion LLM + recorrido)| **Si (extraccion LLM + recorrido)** |
| i18n                       | --                                   | --                                           | --                                    | 3 idiomas                      | **8 idiomas**                        |
| Proveedores LLM            | 10+                                  | 8 nativos + 29 compat                        | 13+                                   | 20+                            | **20+**                              |
| Secretos cifrados          | -- (solo env vars)                   | Si (ChaCha20-Poly1305)                       | -- (JSON plano)                       | Si (AES-256-GCM en DB)        | **Si (AES-256-GCM en DB)**           |

## Arquitectura

<p align="center">
  <img src="../_statics/architecture.jpg" alt="Arquitectura de ArgoClaw" width="800" />
</p>

## Inicio Rapido

**Prerrequisitos:** Go 1.26+, PostgreSQL 18 con pgvector, Docker (opcional)

### Desde el Codigo Fuente

```bash
git clone https://github.com/vellus-ai/argoclaw.git && cd argoclaw
make build
./argoclaw onboard        # Asistente de configuracion interactivo
source .env.local && ./argoclaw
```

### Con Docker

```bash
# Descargar la imagen mas reciente
docker pull ghcr.io/vellus-ai/argoclaw:latest

# Generar .env con secretos auto-generados
chmod +x prepare-env.sh && ./prepare-env.sh

# Agregar al menos una GOCLAW_*_API_KEY en .env, luego:
docker compose -f docker-compose.yml -f docker-compose.postgres.yml \
  -f docker-compose.selfservice.yml up -d

# Dashboard Web en http://localhost:3000
# Verificacion de salud: curl http://localhost:18790/health
```

Cuando las variables de entorno `GOCLAW_*_API_KEY` estan configuradas, el gateway se auto-configura sin prompts interactivos -- detecta el proveedor, ejecuta migraciones y carga datos por defecto.

> Para variantes de compilacion (OTel, Tailscale, Redis), tags de imagen Docker y overlays de compose, consulte la [Guia de Despliegue](https://docs.argoclaw.vellus.tech/#deploy-docker-compose).

## Presets de Agentes ARGO

ArgoClaw incluye 6 arquetipos de agentes preconfigurados disenados para flujos de trabajo empresariales:

| Preset | Rol | Descripcion |
|--------|-----|-------------|
| **Capitan** | Liderazgo y coordinacion | Orquesta equipos, delega tareas, toma decisiones estrategicas |
| **Timonel** | Ejecucion y operaciones | Maneja la ejecucion diaria de tareas y gestion de procesos |
| **Vigia** | Monitoreo e inteligencia | Vigila senales, recopila datos, proporciona conciencia situacional |
| **Artillero** | Accion y resolucion | Toma acciones decisivas sobre problemas, maneja incidentes y escalaciones |
| **Navegante** | Planificacion y estrategia | Traza rutas, analiza opciones, proporciona recomendaciones |
| **Herrero** | Construccion y herramientas | Crea herramientas, plantillas y automatizaciones para el equipo |

## Orquestacion Multi-Agente

ArgoClaw soporta equipos de agentes y delegacion entre agentes -- cada agente se ejecuta con su propia identidad, herramientas, proveedor LLM y archivos de contexto.

### Delegacion de Agentes

<p align="center">
  <img src="../_statics/agent-delegation.jpg" alt="Delegacion de Agentes" width="700" />
</p>

| Modo | Como funciona | Mejor para |
|------|---------------|------------|
| **Sincrono** | El Agente A le pregunta al Agente B y **espera** la respuesta | Consultas rapidas, verificacion de datos |
| **Asincrono** | El Agente A le pregunta al Agente B y **continua**. B anuncia despues | Tareas largas, reportes, analisis profundo |

Los agentes se comunican a traves de **enlaces de permisos** explicitos con control de direccion (`outbound`, `inbound`, `bidirectional`) y limites de concurrencia tanto a nivel de enlace como de agente.

### Equipos de Agentes

<p align="center">
  <img src="../_statics/agent-teams.jpg" alt="Flujo de Trabajo de Equipos de Agentes" width="800" />
</p>

- **Tablero de tareas compartido** -- Crear, reclamar, completar, buscar tareas con dependencias `blocked_by`
- **Buzon del equipo** -- Mensajeria directa entre pares y difusiones
- **Herramientas**: `team_tasks` para gestion de tareas, `team_message` para el buzon

> Para detalles de delegacion, enlaces de permisos y control de concurrencia, consulte la [documentacion de Equipos de Agentes](https://docs.argoclaw.vellus.tech/#teams-what-are-teams).

## Herramientas Integradas

| Herramienta          | Grupo         | Descripcion                                                  |
| -------------------- | ------------- | ------------------------------------------------------------ |
| `read_file`          | fs            | Leer contenido de archivos (con enrutamiento de FS virtual)  |
| `write_file`         | fs            | Escribir/crear archivos                                      |
| `edit_file`          | fs            | Aplicar ediciones dirigidas a archivos existentes            |
| `list_files`         | fs            | Listar contenido del directorio                              |
| `search`             | fs            | Buscar contenido de archivos por patron                      |
| `glob`               | fs            | Encontrar archivos por patron glob                           |
| `exec`               | runtime       | Ejecutar comandos de shell (con flujo de aprobacion)         |
| `web_search`         | web           | Buscar en la web (Brave, DuckDuckGo)                         |
| `web_fetch`          | web           | Obtener y parsear contenido web                              |
| `memory_search`      | memory        | Buscar en memoria a largo plazo (FTS + vector)               |
| `memory_get`         | memory        | Recuperar entradas de memoria                                |
| `skill_search`       | --            | Buscar habilidades (BM25 + embedding hibrido)                |
| `knowledge_graph_search` | memory    | Buscar entidades y recorrer relaciones del grafo de conocimiento |
| `create_image`       | media         | Generacion de imagenes (DashScope, MiniMax)                  |
| `create_audio`       | media         | Generacion de audio (OpenAI, ElevenLabs, MiniMax, Suno)      |
| `create_video`       | media         | Generacion de video (MiniMax, Veo)                           |
| `read_document`      | media         | Lectura de documentos (Gemini File API, cadena de proveedores)|
| `read_image`         | media         | Analisis de imagenes                                         |
| `read_audio`         | media         | Transcripcion y analisis de audio                            |
| `read_video`         | media         | Analisis de video                                            |
| `message`            | messaging     | Enviar mensajes a canales                                    |
| `tts`                | --            | Sintesis de texto a voz                                      |
| `spawn`              | --            | Crear un subagente                                           |
| `subagents`          | sessions      | Controlar subagentes en ejecucion                            |
| `team_tasks`         | teams         | Tablero compartido (listar, crear, reclamar, completar, buscar)|
| `team_message`       | teams         | Buzon del equipo (enviar, difundir, leer)                    |
| `sessions_list`      | sessions      | Listar sesiones activas                                      |
| `sessions_history`   | sessions      | Ver historial de sesiones                                    |
| `sessions_send`      | sessions      | Enviar mensaje a una sesion                                  |
| `sessions_spawn`     | sessions      | Crear una nueva sesion                                       |
| `session_status`     | sessions      | Verificar estado de sesion                                   |
| `cron`               | automation    | Programar y gestionar trabajos cron                          |
| `gateway`            | automation    | Administracion del gateway                                   |
| `browser`            | ui            | Automatizacion de navegador (navegar, clic, escribir, captura)|
| `announce_queue`     | automation    | Anuncio de resultados asincronos (para delegaciones asincronas)|

## Documentacion

Documentacion completa en **[docs.argoclaw.vellus.tech](https://docs.argoclaw.vellus.tech)** -- o explore el codigo fuente en [`argoclaw-docs/`](https://github.com/vellus-ai/argoclaw-docs)

| Seccion | Temas |
|---------|-------|
| [Primeros Pasos](https://docs.argoclaw.vellus.tech/#what-is-argoclaw) | Instalacion, Inicio Rapido, Configuracion, Tour del Dashboard Web |
| [Conceptos Basicos](https://docs.argoclaw.vellus.tech/#how-argoclaw-works) | Bucle del Agente, Sesiones, Herramientas, Memoria, Multi-Tenancy |
| [Agentes](https://docs.argoclaw.vellus.tech/#creating-agents) | Creacion de Agentes, Archivos de Contexto, Personalidad, Compartir y Acceso |
| [Proveedores](https://docs.argoclaw.vellus.tech/#providers-overview) | Anthropic, OpenAI, OpenRouter, Gemini, DeepSeek, +15 mas |
| [Canales](https://docs.argoclaw.vellus.tech/#channels-overview) | Telegram, Discord, Slack, Feishu, Zalo, WhatsApp, WebSocket |
| [Equipos de Agentes](https://docs.argoclaw.vellus.tech/#teams-what-are-teams) | Equipos, Tablero de Tareas, Mensajeria, Delegacion y Traspaso |
| [Avanzado](https://docs.argoclaw.vellus.tech/#custom-tools) | Herramientas Personalizadas, MCP, Habilidades, Cron, Sandbox, Hooks, RBAC |
| [Despliegue](https://docs.argoclaw.vellus.tech/#deploy-docker-compose) | Docker Compose, Base de Datos, Seguridad, Observabilidad, Tailscale |
| [Referencia](https://docs.argoclaw.vellus.tech/#cli-commands) | Comandos CLI, API REST, Protocolo WebSocket, Variables de Entorno |

## Pruebas

```bash
go test ./...                                    # Pruebas unitarias
go test -v ./tests/integration/ -timeout 120s    # Pruebas de integracion (requiere gateway en ejecucion)
```

## Agradecimientos

ArgoClaw es un fork de [ArgoClaw](https://github.com/vellus-ai/arargoclaw), que a su vez es un port en Go de [OpenClaw](https://github.com/openclaw/openclaw). Agradecemos la arquitectura y vision que inspiro ambos proyectos.

Construido y mantenido por [Vellus AI](https://github.com/vellus-ai).

## Licencia

[CC BY-NC 4.0](https://creativecommons.org/licenses/by-nc/4.0/) -- Creative Commons Atribucion-NoComercial 4.0 Internacional
