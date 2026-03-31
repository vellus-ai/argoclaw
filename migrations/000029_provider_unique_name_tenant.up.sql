-- Migration 029: Replace single-column UNIQUE(name) on llm_providers with
-- composite UNIQUE(name, tenant_id) to support multi-tenant provider seeding.
--
-- Background: Migration 027 added tenant_id to llm_providers but did not update
-- the UNIQUE constraint. The onboard seeding uses ON CONFLICT (name, tenant_id)
-- DO NOTHING, which requires a matching unique index on those exact columns.
--
-- Strategy:
--   1. Drop the old column-level UNIQUE on name alone.
--   2. Create a regular UNIQUE index on (name, tenant_id) for arbiter inference.
--      - When tenant_id IS NOT NULL: prevents duplicate (name, tenant) pairs.
--      - When tenant_id IS NULL: NULLs are distinct in UNIQUE, so duplicates allowed.
--   3. Create a partial UNIQUE index on (name) WHERE tenant_id IS NULL for legacy
--      rows that predate multi-tenancy (prevents duplicate names in single-tenant mode).

-- Step 1: Drop old constraint. Column-level UNIQUE constraints are named
-- "{table}_{column}_key" by PostgreSQL convention.
ALTER TABLE llm_providers DROP CONSTRAINT IF EXISTS llm_providers_name_key;

-- Step 2: Regular composite unique — required for ON CONFLICT (name, tenant_id) arbiter.
CREATE UNIQUE INDEX IF NOT EXISTS uq_providers_name_tenant
    ON llm_providers (name, tenant_id);

-- Step 3: Partial unique for legacy rows without tenant (backward compat).
CREATE UNIQUE INDEX IF NOT EXISTS uq_providers_name_no_tenant
    ON llm_providers (name)
    WHERE tenant_id IS NULL;
