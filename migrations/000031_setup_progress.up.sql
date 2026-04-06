-- Migration 031: Onboarding Phase 2 — setup progress tracking + forced password change.

-- Track tenant onboarding progress (one row per tenant, created on-demand via UPSERT).
CREATE TABLE IF NOT EXISTS setup_progress (
    tenant_id           UUID PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
    account_type        VARCHAR(20),
    industry            VARCHAR(100),
    team_size           VARCHAR(20),
    onboarding_complete BOOLEAN NOT NULL DEFAULT false,
    completed_at        TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Forced password change flag for provisioned users with temporary passwords.
ALTER TABLE users ADD COLUMN IF NOT EXISTS must_change_password BOOLEAN NOT NULL DEFAULT false;
