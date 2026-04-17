-- Migration 036 DOWN: Remove bridge-project-management seed data
--
-- Reverse order: agent_plugins → tenant_plugins → plugin_catalog

DELETE FROM agent_plugins WHERE plugin_name = 'bridge-project-management';
DELETE FROM tenant_plugins WHERE plugin_name = 'bridge-project-management';
DELETE FROM plugin_catalog WHERE name = 'bridge-project-management' AND tenant_id IS NULL;
