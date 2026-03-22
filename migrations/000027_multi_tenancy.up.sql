-- Migration 027: Multi-tenancy at enterprise level.
-- Tenant = company/client (Vellus, Axis, Pitflow, etc.), NOT individual user.

-- Tenants table
CREATE TABLE IF NOT EXISTS tenants (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    slug            VARCHAR(63) NOT NULL,               -- URL-safe identifier (e.g. "vellus", "axis")
    name            VARCHAR(255) NOT NULL,              -- Display name (e.g. "Vellus Tecnologia")
    plan            VARCHAR(30) NOT NULL DEFAULT 'trial', -- trial, starter, pro, enterprise
    status          VARCHAR(20) NOT NULL DEFAULT 'active', -- active, suspended, cancelled
    trial_ends_at   TIMESTAMPTZ,                        -- NULL if not on trial
    settings        JSONB NOT NULL DEFAULT '{}',        -- Tenant-wide settings
    stripe_customer_id VARCHAR(255),                    -- Stripe customer ID for billing
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_tenants_slug UNIQUE (slug)
);

CREATE INDEX idx_tenants_status ON tenants(status);
CREATE INDEX idx_tenants_stripe ON tenants(stripe_customer_id) WHERE stripe_customer_id IS NOT NULL;

-- Link users to tenants with roles
CREATE TABLE IF NOT EXISTS tenant_users (
    tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role        VARCHAR(20) NOT NULL DEFAULT 'member', -- owner, admin, member
    joined_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (tenant_id, user_id)
);

CREATE INDEX idx_tenant_users_user ON tenant_users(user_id);

-- Add tenant_id to existing tables for isolation
ALTER TABLE agents ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE;
ALTER TABLE llm_providers ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE;
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE;
ALTER TABLE channel_instances ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE;
ALTER TABLE agent_teams ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE;
ALTER TABLE cron_jobs ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE;
ALTER TABLE custom_tools ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE;
ALTER TABLE mcp_servers ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE;
ALTER TABLE skills ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE;

-- Indexes for tenant isolation queries
CREATE INDEX IF NOT EXISTS idx_agents_tenant ON agents(tenant_id) WHERE tenant_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_providers_tenant ON llm_providers(tenant_id) WHERE tenant_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_sessions_tenant ON sessions(tenant_id) WHERE tenant_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_channel_instances_tenant ON channel_instances(tenant_id) WHERE tenant_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_teams_tenant ON agent_teams(tenant_id) WHERE tenant_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_cron_tenant ON cron_jobs(tenant_id) WHERE tenant_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_custom_tools_tenant ON custom_tools(tenant_id) WHERE tenant_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_mcp_servers_tenant ON mcp_servers(tenant_id) WHERE tenant_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_skills_tenant ON skills(tenant_id) WHERE tenant_id IS NOT NULL;

-- Tenant branding (white-label)
CREATE TABLE IF NOT EXISTS tenant_branding (
    tenant_id       UUID PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
    logo_url        TEXT,                               -- Logo image URL
    favicon_url     TEXT,                               -- Favicon URL
    primary_color   VARCHAR(7),                         -- Hex color (e.g. "#1E40AF")
    palette         JSONB NOT NULL DEFAULT '{}',        -- Generated WCAG AA palette
    custom_domain   VARCHAR(255),                       -- e.g. "app.empresa.com.br"
    sender_email    VARCHAR(320),                       -- e.g. "ia@empresa.com.br"
    product_name    VARCHAR(100) DEFAULT 'ARGO',        -- White-label product name
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_branding_domain ON tenant_branding(custom_domain) WHERE custom_domain IS NOT NULL;
