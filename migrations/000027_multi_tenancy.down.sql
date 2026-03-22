DROP TABLE IF EXISTS tenant_branding;
DROP TABLE IF EXISTS tenant_users;

ALTER TABLE agents DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE llm_providers DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE sessions DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE channel_instances DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE agent_teams DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE cron_jobs DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE custom_tools DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE mcp_servers DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE skills DROP COLUMN IF EXISTS tenant_id;

DROP TABLE IF EXISTS tenants;
