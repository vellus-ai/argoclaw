-- Migration 032 rollback: Plugin Host v2
--
-- Reverses all changes from the up migration in dependency order.

-- 1. Drop audit immutability trigger and function
DROP TRIGGER IF EXISTS trg_audit_immutable ON plugin_audit_log;
DROP FUNCTION IF EXISTS reject_audit_mutation();

-- 2. Drop value size constraint from plugin_data
ALTER TABLE plugin_data DROP CONSTRAINT IF EXISTS chk_plugin_data_value_size;

-- 3. Drop optimistic locking column from tenant_plugins
ALTER TABLE tenant_plugins DROP COLUMN IF EXISTS version;

-- 4. Drop icon_url column from plugin_catalog
ALTER TABLE plugin_catalog DROP COLUMN IF EXISTS icon_url;
