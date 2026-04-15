-- Rollback migration 000035
-- Remove the Vellus operator seed tenant and the operator_level column.

DELETE FROM tenants WHERE slug = 'vellus';

ALTER TABLE tenants DROP COLUMN IF EXISTS operator_level;
