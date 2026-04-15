-- Migration 000035: Add operator_level to tenants for Ponte de Comando Central
-- operator_level = 0 → regular tenant (default, current behaviour)
-- operator_level = 1 → Vellus operator (cross-tenant read access)
-- operator_level = 2 → super-admin (reserved — cross-tenant write, not yet implemented)

ALTER TABLE tenants ADD COLUMN IF NOT EXISTS operator_level INT NOT NULL DEFAULT 0;

-- Sparse index: only index rows where operator_level > 0 (very few rows)
-- Note: CONCURRENTLY omitted — cannot run inside a transaction block (golang-migrate wraps each migration)
CREATE INDEX IF NOT EXISTS idx_tenants_operator_level
    ON tenants (operator_level) WHERE operator_level > 0;

-- Seed the Vellus operator tenant (idempotent)
INSERT INTO tenants (id, slug, name, plan, status, operator_level)
    VALUES (gen_random_uuid(), 'vellus', 'Vellus AI', 'internal', 'active', 1)
    ON CONFLICT (slug) DO UPDATE
        SET operator_level = 1,
            plan = 'internal',
            status = 'active',
            updated_at = NOW();
