-- Migration 036: Seed bridge-project-management into plugin_catalog
--
-- Registers the Bridge PM plugin as a global built-in plugin (tenant_id NULL)
-- and retroactively installs it for all existing active tenants and their
-- default agents.
--
-- Idempotent: ON CONFLICT DO NOTHING / NOT EXISTS guards on all INSERTs.

-- ─────────────────────────────────────────────────────────────────────────────
-- 1. Register bridge-project-management in the global plugin catalog
-- ─────────────────────────────────────────────────────────────────────────────
INSERT INTO plugin_catalog (
    tenant_id, name, version, display_name, description, author,
    manifest, source, min_plan, tags
) VALUES (
    NULL,  -- global visibility (built-in)
    'bridge-project-management',
    '1.0.0',
    'Ponte by ARGO',
    'Gestão de projetos visual e agêntica com Kanban, workflows e notificações multi-canal',
    'Vellus',
    '{
      "metadata": {
        "name": "bridge-project-management",
        "display_name": "Ponte by ARGO",
        "description": "Gestão de projetos visual e agêntica com Kanban, workflows e notificações multi-canal",
        "version": "1.0.0",
        "author": "Vellus",
        "category": "bridge",
        "min_plan": "starter",
        "icon": "⚓"
      },
      "spec": {
        "type": "full"
      },
      "runtime": {
        "transport": "stdio",
        "command": "./bridge-project-management",
        "args": ["serve"]
      },
      "permissions": {
        "tools.provide": [
          "create_task", "update_task", "list_tasks", "move_task",
          "create_project", "list_projects", "get_project_summary",
          "add_member", "get_dashboard"
        ],
        "data.read": ["agents", "sessions", "users", "tenants", "projects", "team_tasks"],
        "data.write": ["plugin:bridge-pm-*", "projects", "team_tasks"],
        "events.subscribe": ["task_created", "task_status_changed", "task_assigned", "task_commented"]
      }
    }'::jsonb,
    'builtin',
    'starter',
    ARRAY['project-management', 'kanban', 'tasks', 'bridge']
) ON CONFLICT (tenant_id, name, version) DO NOTHING;

-- ─────────────────────────────────────────────────────────────────────────────
-- 2. Retroactive seed: install for existing active tenants
-- ─────────────────────────────────────────────────────────────────────────────
INSERT INTO tenant_plugins (tenant_id, plugin_name, plugin_version, state, enabled_at)
SELECT t.id, 'bridge-project-management', '1.0.0', 'enabled', NOW()
FROM tenants t
WHERE t.status = 'active'
  AND NOT EXISTS (
    SELECT 1 FROM tenant_plugins tp
    WHERE tp.tenant_id = t.id AND tp.plugin_name = 'bridge-project-management'
  );

-- ─────────────────────────────────────────────────────────────────────────────
-- 3. Retroactive seed: grant to default agents of tenants that have the plugin
-- ─────────────────────────────────────────────────────────────────────────────
INSERT INTO agent_plugins (tenant_id, agent_id, plugin_name, enabled)
SELECT a.tenant_id, a.id, 'bridge-project-management', true
FROM agents a
WHERE a.is_default = true
  AND a.deleted_at IS NULL
  AND a.tenant_id IN (
    SELECT tp.tenant_id FROM tenant_plugins tp
    WHERE tp.plugin_name = 'bridge-project-management' AND tp.state = 'enabled'
  )
  AND NOT EXISTS (
    SELECT 1 FROM agent_plugins ap
    WHERE ap.agent_id = a.id AND ap.plugin_name = 'bridge-project-management'
  );
