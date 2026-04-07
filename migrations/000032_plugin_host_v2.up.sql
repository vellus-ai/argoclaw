-- Migration 032: Plugin Host v2 — Fill gaps from audit-1.1
--
-- Adds missing pieces to the plugin_host tables created in migration 030:
--   1. plugin_catalog.icon_url          — icon URL for catalog display
--   2. tenant_plugins.version           — optimistic locking counter
--   3. plugin_data value size CHECK     — enforce max 1 MB per value
--   4. reject_audit_mutation() function — enforce audit log immutability at DB level
--   5. trg_audit_immutable trigger      — wires the function to plugin_audit_log
--
-- All statements are idempotent (IF NOT EXISTS / DO $$ guards).

-- ─────────────────────────────────────────────────────────────────────────────
-- 1. plugin_catalog: add icon_url column
-- ─────────────────────────────────────────────────────────────────────────────
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'plugin_catalog' AND column_name = 'icon_url'
    ) THEN
        ALTER TABLE plugin_catalog ADD COLUMN icon_url TEXT;
    END IF;
END
$$;

-- ─────────────────────────────────────────────────────────────────────────────
-- 2. tenant_plugins: add version column for optimistic locking
-- ─────────────────────────────────────────────────────────────────────────────
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'tenant_plugins' AND column_name = 'version'
    ) THEN
        ALTER TABLE tenant_plugins ADD COLUMN version INTEGER NOT NULL DEFAULT 1;
    END IF;
END
$$;

-- ─────────────────────────────────────────────────────────────────────────────
-- 3. plugin_data: enforce max 1 MB value size
-- ─────────────────────────────────────────────────────────────────────────────
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.constraint_column_usage
        WHERE table_name = 'plugin_data' AND constraint_name = 'chk_plugin_data_value_size'
    ) THEN
        ALTER TABLE plugin_data
            ADD CONSTRAINT chk_plugin_data_value_size
            CHECK (octet_length(value::text) <= 1048576);
    END IF;
END
$$;

-- ─────────────────────────────────────────────────────────────────────────────
-- 4. Audit log immutability: function + trigger
--    Prevents any UPDATE or DELETE on plugin_audit_log at the database level.
-- ─────────────────────────────────────────────────────────────────────────────
CREATE OR REPLACE FUNCTION reject_audit_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'audit log is immutable';
END;
$$ LANGUAGE plpgsql;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.triggers
        WHERE trigger_name = 'trg_audit_immutable'
          AND event_object_table = 'plugin_audit_log'
    ) THEN
        CREATE TRIGGER trg_audit_immutable
            BEFORE UPDATE OR DELETE ON plugin_audit_log
            FOR EACH ROW
            EXECUTE FUNCTION reject_audit_mutation();
    END IF;
END
$$;
