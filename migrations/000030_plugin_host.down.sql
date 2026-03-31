-- Migration 030 rollback: Plugin Host Infrastructure
--
-- Drop tables in reverse FK dependency order to avoid constraint violations.

DROP TABLE IF EXISTS plugin_audit_log;
DROP TABLE IF EXISTS plugin_data;
DROP TABLE IF EXISTS agent_plugins;
DROP TABLE IF EXISTS tenant_plugins;
DROP TABLE IF EXISTS plugin_catalog;
