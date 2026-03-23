# syntax=docker/dockerfile:1

# ── Stage 1: Build ──
FROM golang:1.26-bookworm AS builder

WORKDIR /src

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build args
ARG ENABLE_OTEL=false
ARG ENABLE_TSNET=false
ARG ENABLE_REDIS=false
ARG VERSION=dev

# Build static binary (CGO disabled for scratch/alpine compatibility)
RUN set -eux; \
    TAGS=""; \
    if [ "$ENABLE_OTEL" = "true" ]; then TAGS="otel"; fi; \
    if [ "$ENABLE_TSNET" = "true" ]; then \
        if [ -n "$TAGS" ]; then TAGS="$TAGS,tsnet"; else TAGS="tsnet"; fi; \
    fi; \
    if [ "$ENABLE_REDIS" = "true" ]; then \
        if [ -n "$TAGS" ]; then TAGS="$TAGS,redis"; else TAGS="redis"; fi; \
    fi; \
    if [ -n "$TAGS" ]; then TAGS="-tags $TAGS"; fi; \
    CGO_ENABLED=0 GOOS=linux \
    go build -ldflags="-s -w -X github.com/nextlevelbuilder/argoclaw/cmd.Version=${VERSION}" \
    ${TAGS} -o /out/argoclaw . && \
    CGO_ENABLED=0 GOOS=linux \
    go build -ldflags="-s -w" -o /out/pkg-helper ./cmd/pkg-helper

# ── Stage 2: Runtime ──
FROM alpine:3.22

ARG ENABLE_SANDBOX=false
ARG ENABLE_PYTHON=false
ARG ENABLE_NODE=false
ARG ENABLE_FULL_SKILLS=false

# Install ca-certificates + wget (healthcheck) + optional runtimes.
# ENABLE_FULL_SKILLS=true pre-installs all skill deps (larger image, no on-demand install needed).
# Otherwise, skill packages are installed on-demand via the admin UI.
RUN set -eux; \
    apk add --no-cache ca-certificates wget su-exec; \
    if [ "$ENABLE_SANDBOX" = "true" ]; then \
        apk add --no-cache docker-cli; \
    fi; \
    if [ "$ENABLE_FULL_SKILLS" = "true" ]; then \
        apk add --no-cache python3 py3-pip nodejs npm pandoc github-cli poppler-utils bash curl; \
        pip3 install --no-cache-dir --break-system-packages \
            pypdf==5.4.0 openpyxl==3.1.5 pandas==2.2.3 python-pptx==1.0.2 \
            markitdown==0.1.1 defusedxml==0.7.1 lxml==5.3.1 \
            pdfplumber==0.11.6 pdf2image==1.17.0 anthropic==0.52.0; \
        npm install -g --cache /tmp/npm-cache docx pptxgenjs; \
        rm -rf /tmp/npm-cache /root/.cache /var/cache/apk/*; \
    else \
        if [ "$ENABLE_PYTHON" = "true" ]; then \
            apk add --no-cache python3 py3-pip; \
            pip3 install --no-cache-dir --break-system-packages edge-tts==7.0.2; \
        fi; \
        if [ "$ENABLE_NODE" = "true" ]; then \
            apk add --no-cache nodejs npm; \
        fi; \
    fi

# Non-root user
RUN adduser -D -u 1000 -h /app argoclaw
WORKDIR /app

# Copy binary, migrations, and bundled skills
COPY --from=builder /out/argoclaw /app/argoclaw
COPY --from=builder /out/pkg-helper /app/pkg-helper
COPY --from=builder /src/migrations/ /app/migrations/
COPY --from=builder /src/skills/ /app/bundled-skills/
COPY docker-entrypoint.sh /app/docker-entrypoint.sh

# Fix Windows git clone issues:
# 1. CRLF line endings in shell scripts (Windows git adds \r)
# 2. Broken symlinks: On Windows (core.symlinks=false), git creates text files
#    or skips symlinks entirely. Skills like docx/pptx/xlsx need _shared/office
#    module in their scripts/ dir (originally symlinked as scripts/office -> ../../_shared/office).
RUN set -eux; \
    sed -i 's/\r$//' /app/docker-entrypoint.sh; \
    cd /app/bundled-skills; \
    for skill in docx pptx xlsx; do \
        if [ -d "${skill}/scripts" ] && [ ! -d "${skill}/scripts/office" ]; then \
            rm -f "${skill}/scripts/office"; \
            cp -r _shared/office "${skill}/scripts/office"; \
        fi; \
    done

RUN chmod +x /app/docker-entrypoint.sh && \
    chmod 755 /app/pkg-helper && chown root:root /app/pkg-helper

# Create data directories.
# .runtime has split ownership: root owns the dir (so pkg-helper can write apk-packages),
# while pip/npm subdirs are argoclaw-owned (runtime installs by the app process).
RUN mkdir -p /app/workspace /app/data/.runtime/pip /app/data/.runtime/npm-global/lib \
        /app/data/.runtime/pip-cache /app/skills /app/tsnet-state /app/.argoclaw \
    && touch /app/data/.runtime/apk-packages \
    && chown -R argoclaw:argoclaw /app/workspace /app/skills /app/tsnet-state /app/.argoclaw \
    && chown argoclaw:argoclaw /app/bundled-skills /app/data \
    && chown root:argoclaw /app/data/.runtime /app/data/.runtime/apk-packages \
    && chmod 0750 /app/data/.runtime \
    && chmod 0640 /app/data/.runtime/apk-packages \
    && chown -R argoclaw:argoclaw /app/data/.runtime/pip /app/data/.runtime/npm-global /app/data/.runtime/pip-cache

# Default environment
ENV ARGOCLAW_CONFIG=/app/config.json \
    ARGOCLAW_WORKSPACE=/app/workspace \
    ARGOCLAW_DATA_DIR=/app/data \
    ARGOCLAW_SKILLS_DIR=/app/skills \
    ARGOCLAW_MIGRATIONS_DIR=/app/migrations \
    ARGOCLAW_HOST=0.0.0.0 \
    ARGOCLAW_PORT=18790

# Entrypoint runs as root to install persisted packages and start pkg-helper,
# then drops to argoclaw user via su-exec before starting the app.

EXPOSE 18790

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:18790/health || exit 1

ENTRYPOINT ["/app/docker-entrypoint.sh"]
CMD ["serve"]
