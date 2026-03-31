-- Migration 030: Plugin Host Infrastructure
--
-- Creates the 5 tables that power the ArgoClaw Plugin Host:
--   1. plugin_catalog      — global registry of available plugins
--   2. tenant_plugins      — per-tenant installation + lifecycle state
--   3. agent_plugins       — per-agent plugin override (enable/disable)
--   4. plugin_data         — sandboxed KV store per plugin per tenant
--   5. plugin_audit_log    — append-only audit trail of all plugin actions
--
-- Design principles:
--   - All tables scoped by tenant_id for strict multi-tenant isolation
--   - FK CASCADE: deleting a tenant cascades to all plugin records
--   - plugin_data and plugin_audit_log also cascade from tenant_plugins
--   - plugin_audit_log is append-only (no UPDATE/DELETE at app layer)
--   - IF NOT EXISTS on all objects for idempotency

-- ─────────────────────────────────────────────────────────────────────────────
-- 1. plugin_catalog
--    Global registry of plugin definitions. Entries are scoped to a tenant
--    (for custom/private plugins) or unscoped (tenant_id IS NULL) for
--    built-in/marketplace plugins visible to all tenants.
-- ─────────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS plugin_catalog (
    id           UUID        NOT NULL DEFAULT gen_random_uuid(),
    -- NULL tenant_id = built-in/marketplace plugin (visible to all)
    -- non-NULL tenant_id = private plugin (visible only to that tenant)
    tenant_id    UUID        REFERENCES tenants(id) ON DELETE CASCADE,
    name         VARCHAR(100) NOT NULL,
    version      VARCHAR(50)  NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    description  TEXT         NOT NULL DEFAULT '',
    author       VARCHAR(255) NOT NULL DEFAULT '',
    -- Full parsed plugin.yaml stored as JSONB for fast access
    manifest     JSONB        NOT NULL DEFAULT '{}',
    -- Source: builtin, marketplace, custom
    source       VARCHAR(50)  NOT NULL DEFAULT 'builtin',
    -- Minimum plan required: starter, pro, enterprise
    min_plan     VARCHAR(50)  NOT NULL DEFAULT 'starter',
    -- SHA-256 of the plugin binary/bundle for integrity verification
    checksum     VARCHAR(128),
    tags         TEXT[]       NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT plugin_catalog_pkey PRIMARY KEY (id),
    -- A given (name, version) pair is unique per tenant scope.
    -- NULL tenant_id (built-in) enforced separately by partial index below.
    CONSTRAINT uq_plugin_catalog_name_version_tenant UNIQUE NULLS NOT DISTINCT (tenant_id, name, version)
);

CREATE INDEX IF NOT EXISTS idx_plugin_catalog_tenant
    ON plugin_catalog (tenant_id)
    WHERE tenant_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_plugin_catalog_name
    ON plugin_catalog (name);

CREATE INDEX IF NOT EXISTS idx_plugin_catalog_tags
    ON plugin_catalog USING GIN (tags);

-- ─────────────────────────────────────────────────────────────────────────────
-- 2. tenant_plugins
--    Tracks which plugins are installed for a tenant and their current state.
--    Lifecycle: installed → enabled ⇄ disabled → (uninstalled = row deleted)
-- ─────────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS tenant_plugins (
    id             UUID         NOT NULL DEFAULT gen_random_uuid(),
    tenant_id      UUID         NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    plugin_name    VARCHAR(100) NOT NULL,
    plugin_version VARCHAR(50)  NOT NULL,
    -- Lifecycle state
    state          VARCHAR(20)  NOT NULL DEFAULT 'installed'
                   CHECK (state IN ('installed', 'enabled', 'disabled', 'error')),
    -- Per-tenant config JSON, validated against plugin's configSchema
    config         JSONB        NOT NULL DEFAULT '{}',
    -- Snapshot of approved permissions at install time
    permissions    JSONB        NOT NULL DEFAULT '{}',
    -- Error message when state = 'error'
    error_message  TEXT,
    -- Who installed it (user ID)
    installed_by   UUID,
    enabled_at     TIMESTAMPTZ,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT tenant_plugins_pkey PRIMARY KEY (id),
    -- A tenant can only have one installation of a given plugin name
    CONSTRAINT uq_tenant_plugins_tenant_name UNIQUE (tenant_id, plugin_name)
);

CREATE INDEX IF NOT EXISTS idx_tenant_plugins_tenant
    ON tenant_plugins (tenant_id);

CREATE INDEX IF NOT EXISTS idx_tenant_plugins_state
    ON tenant_plugins (tenant_id, state);

-- ─────────────────────────────────────────────────────────────────────────────
-- 3. agent_plugins
--    Per-agent override of plugin enablement.
--    When a plugin is enabled for a tenant, all agents inherit it by default.
--    This table allows disabling/enabling a specific plugin for a specific agent,
--    or providing per-agent config overrides.
-- ─────────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS agent_plugins (
    id              UUID         NOT NULL DEFAULT gen_random_uuid(),
    tenant_id       UUID         NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    agent_id        UUID         NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    plugin_name     VARCHAR(100) NOT NULL,
    enabled         BOOLEAN      NOT NULL DEFAULT TRUE,
    -- Per-agent config JSON, merged on top of tenant config
    config_override JSONB        NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT agent_plugins_pkey PRIMARY KEY (id),
    -- One override row per (agent, plugin) pair
    CONSTRAINT uq_agent_plugins_agent_name UNIQUE (agent_id, plugin_name)
);

CREATE INDEX IF NOT EXISTS idx_agent_plugins_tenant
    ON agent_plugins (tenant_id);

CREATE INDEX IF NOT EXISTS idx_agent_plugins_agent
    ON agent_plugins (agent_id);

-- ─────────────────────────────────────────────────────────────────────────────
-- 4. plugin_data
--    Sandboxed key-value store. Each plugin gets its own namespace per tenant,
--    further scoped by collection (logical grouping).
--    Plugins access this through the Data Proxy API, never directly.
-- ─────────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS plugin_data (
    id          UUID         NOT NULL DEFAULT gen_random_uuid(),
    tenant_id   UUID         NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    plugin_name VARCHAR(100) NOT NULL,
    -- Logical collection namespace within the plugin (e.g. "prompts", "cache")
    collection  VARCHAR(100) NOT NULL DEFAULT 'default',
    key         VARCHAR(500) NOT NULL,
    value       JSONB        NOT NULL DEFAULT '{}',
    -- Optional TTL: row can be purged after this timestamp
    expires_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT plugin_data_pkey PRIMARY KEY (id),
    CONSTRAINT uq_plugin_data_key UNIQUE (tenant_id, plugin_name, collection, key)
);

-- Primary lookup index: tenant + plugin + collection
CREATE INDEX IF NOT EXISTS idx_plugin_data_lookup
    ON plugin_data (tenant_id, plugin_name, collection);

-- TTL-based expiry: partial index only on rows that have an expiry
CREATE INDEX IF NOT EXISTS idx_plugin_data_expires
    ON plugin_data (expires_at)
    WHERE expires_at IS NOT NULL;

-- ─────────────────────────────────────────────────────────────────────────────
-- 5. plugin_audit_log
--    Immutable append-only record of all plugin lifecycle actions.
--    At the application layer: INSERT only, never UPDATE or DELETE.
-- ─────────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS plugin_audit_log (
    id          UUID         NOT NULL DEFAULT gen_random_uuid(),
    tenant_id   UUID         NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    plugin_name VARCHAR(100) NOT NULL,
    -- Action type: install, enable, disable, uninstall, config_change,
    --              tool_call, data_access, error, permission_denied
    action      VARCHAR(50)  NOT NULL,
    -- Actor: user UUID or system identifier
    actor_id    UUID,
    actor_type  VARCHAR(20)  NOT NULL DEFAULT 'user'
                CHECK (actor_type IN ('user', 'system', 'agent')),
    -- Contextual details (JSON): tool_name, error_message, config_diff, etc.
    details     JSONB        NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT plugin_audit_log_pkey PRIMARY KEY (id)
);

-- Primary query pattern: tenant + plugin, ordered by time descending
CREATE INDEX IF NOT EXISTS idx_plugin_audit_tenant_plugin
    ON plugin_audit_log (tenant_id, plugin_name, created_at DESC);

-- Temporal queries across all plugins for a tenant
CREATE INDEX IF NOT EXISTS idx_plugin_audit_tenant_time
    ON plugin_audit_log (tenant_id, created_at DESC);
