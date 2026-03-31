-- Rollback: Restore original UNIQUE(name) constraint, drop composite indexes.
-- WARNING: This will fail if there are duplicate names across tenants.

DROP INDEX IF EXISTS uq_providers_name_tenant;
DROP INDEX IF EXISTS uq_providers_name_no_tenant;

ALTER TABLE llm_providers ADD CONSTRAINT llm_providers_name_key UNIQUE (name);
