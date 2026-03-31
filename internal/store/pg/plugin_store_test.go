//go:build integration

package pg_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/vellus-ai/argoclaw/internal/store"
	"github.com/vellus-ai/argoclaw/internal/store/pg"
	"github.com/vellus-ai/argoclaw/internal/testutil"
)

// ─────────────────────────────────────────────────────────────────────────────
// Test helpers
// ─────────────────────────────────────────────────────────────────────────────

// pluginStore returns a fresh PGPluginStore connected to the test DB.
func pluginStore(t *testing.T) (store.PluginStore, *sql.DB) {
	t.Helper()
	db := testutil.SetupDB(t)
	return pg.NewPGPluginStore(db), db
}

// tenantCtx creates a test tenant and returns its context.
func tenantCtx(t *testing.T, db *sql.DB) (uuid.UUID, context.Context) {
	t.Helper()
	slug := "test-" + uuid.Must(uuid.NewV7()).String()[:8]
	tid := testutil.CreateTestTenant(t, db, slug, "Test Tenant "+slug)
	t.Cleanup(func() { testutil.CleanupPluginData(t, db, tid) })
	return tid, testutil.TenantCtx(tid)
}

// installPlugin is a test helper that installs a plugin for the given context.
func installPlugin(t *testing.T, s store.PluginStore, ctx context.Context, name string) {
	t.Helper()
	tp := &store.TenantPlugin{
		PluginName:    name,
		PluginVersion: "0.1.0",
		Config:        json.RawMessage(`{}`),
		Permissions:   json.RawMessage(`{}`),
	}
	if err := s.InstallPlugin(ctx, tp); err != nil {
		t.Fatalf("install plugin %q: %v", name, err)
	}
}

// uniquePlugin returns a unique plugin name for tests.
func uniquePlugin(prefix string) string {
	return prefix + "-" + uuid.Must(uuid.NewV7()).String()[:8]
}

// ─────────────────────────────────────────────────────────────────────────────
// Catalog CRUD
// ─────────────────────────────────────────────────────────────────────────────

func TestPGPluginStore_Catalog_UpsertAndGet(t *testing.T) {
	s, _ := pluginStore(t)
	ctx := context.Background() // built-in catalog entry (no tenant scope)

	name := uniquePlugin("catalog")
	entry := &store.PluginCatalogEntry{
		Name:        name,
		Version:     "1.0.0",
		DisplayName: "Test Catalog Plugin",
		Description: "Test description",
		Author:      "test-author",
		Manifest:    json.RawMessage(`{"key":"value"}`),
		Source:      "builtin",
		MinPlan:     "starter",
		Tags:        []string{"tag1", "tag2"},
	}

	if err := s.UpsertCatalogEntry(ctx, entry); err != nil {
		t.Fatalf("UpsertCatalogEntry: %v", err)
	}

	got, err := s.GetCatalogEntry(ctx, entry.ID)
	if err != nil {
		t.Fatalf("GetCatalogEntry: %v", err)
	}
	if got.Name != name {
		t.Errorf("name: got %q want %q", got.Name, name)
	}
	if len(got.Tags) != 2 {
		t.Errorf("tags: got %d want 2", len(got.Tags))
	}
}

func TestPGPluginStore_Catalog_UpsertUpdatesExisting(t *testing.T) {
	s, _ := pluginStore(t)
	ctx := context.Background()

	name := uniquePlugin("upsert")
	entry := &store.PluginCatalogEntry{
		Name:        name,
		Version:     "1.0.0",
		DisplayName: "Original",
		Manifest:    json.RawMessage(`{}`),
		Source:      "builtin",
		MinPlan:     "starter",
	}
	if err := s.UpsertCatalogEntry(ctx, entry); err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	entry.DisplayName = "Updated"
	if err := s.UpsertCatalogEntry(ctx, entry); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	got, err := s.GetCatalogEntryByName(ctx, name)
	if err != nil {
		t.Fatalf("GetCatalogEntryByName: %v", err)
	}
	if got.DisplayName != "Updated" {
		t.Errorf("display_name: got %q want %q", got.DisplayName, "Updated")
	}
}

func TestPGPluginStore_Catalog_GetNotFound(t *testing.T) {
	s, _ := pluginStore(t)
	_, err := s.GetCatalogEntry(context.Background(), uuid.New())
	if err != store.ErrPluginNotFound {
		t.Errorf("expected ErrPluginNotFound, got %v", err)
	}
}

func TestPGPluginStore_Catalog_List(t *testing.T) {
	s, _ := pluginStore(t)
	ctx := context.Background()

	name := uniquePlugin("list-catalog")
	_ = s.UpsertCatalogEntry(ctx, &store.PluginCatalogEntry{
		Name:    name,
		Version: "1.0.0",
		Manifest: json.RawMessage(`{}`),
		Source:  "builtin",
		MinPlan: "starter",
	})

	entries, err := s.ListCatalog(ctx)
	if err != nil {
		t.Fatalf("ListCatalog: %v", err)
	}
	found := false
	for _, e := range entries {
		if e.Name == name {
			found = true
		}
	}
	if !found {
		t.Errorf("expected to find %q in catalog list", name)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Tenant plugin lifecycle
// ─────────────────────────────────────────────────────────────────────────────

func TestPGPluginStore_TenantPlugin_InstallGetList(t *testing.T) {
	s, db := pluginStore(t)
	_, ctx := tenantCtx(t, db)

	name := uniquePlugin("vault")
	installPlugin(t, s, ctx, name)

	got, err := s.GetTenantPlugin(ctx, name)
	if err != nil {
		t.Fatalf("GetTenantPlugin: %v", err)
	}
	if got.PluginName != name {
		t.Errorf("plugin_name: got %q want %q", got.PluginName, name)
	}
	if got.State != store.PluginStateInstalled {
		t.Errorf("state: got %q want %q", got.State, store.PluginStateInstalled)
	}

	list, err := s.ListTenantPlugins(ctx)
	if err != nil {
		t.Fatalf("ListTenantPlugins: %v", err)
	}
	if len(list) == 0 {
		t.Error("expected at least 1 plugin in list")
	}
}

func TestPGPluginStore_TenantPlugin_InstallDuplicate_Rejected(t *testing.T) {
	s, db := pluginStore(t)
	_, ctx := tenantCtx(t, db)

	name := uniquePlugin("dup")
	installPlugin(t, s, ctx, name)

	err := s.InstallPlugin(ctx, &store.TenantPlugin{
		PluginName:    name,
		PluginVersion: "0.1.0",
		Config:        json.RawMessage(`{}`),
		Permissions:   json.RawMessage(`{}`),
	})
	if err != store.ErrPluginAlreadyInstalled {
		t.Errorf("expected ErrPluginAlreadyInstalled, got %v", err)
	}
}

func TestPGPluginStore_TenantPlugin_EnableDisable(t *testing.T) {
	s, db := pluginStore(t)
	_, ctx := tenantCtx(t, db)

	name := uniquePlugin("enable")
	installPlugin(t, s, ctx, name)

	if err := s.EnablePlugin(ctx, name, nil); err != nil {
		t.Fatalf("EnablePlugin: %v", err)
	}
	tp, _ := s.GetTenantPlugin(ctx, name)
	if tp.State != store.PluginStateEnabled {
		t.Errorf("state after enable: got %q", tp.State)
	}

	if err := s.DisablePlugin(ctx, name, nil); err != nil {
		t.Fatalf("DisablePlugin: %v", err)
	}
	tp, _ = s.GetTenantPlugin(ctx, name)
	if tp.State != store.PluginStateDisabled {
		t.Errorf("state after disable: got %q", tp.State)
	}
}

func TestPGPluginStore_TenantPlugin_EnableNotFound(t *testing.T) {
	s, db := pluginStore(t)
	_, ctx := tenantCtx(t, db)

	err := s.EnablePlugin(ctx, "nonexistent-plugin", nil)
	if err != store.ErrPluginNotFound {
		t.Errorf("expected ErrPluginNotFound, got %v", err)
	}
}

// TestPGPluginStore_TenantPlugin_Uninstall_CleansAllData tests that
// uninstalling a plugin removes all associated KV data (Gap G1).
func TestPGPluginStore_TenantPlugin_Uninstall_CleansAllData(t *testing.T) {
	s, db := pluginStore(t)
	_, ctx := tenantCtx(t, db)

	name := uniquePlugin("uninstall")
	installPlugin(t, s, ctx, name)

	// Add KV data.
	if err := s.PutData(ctx, name, "col", "key1", json.RawMessage(`"value"`), nil); err != nil {
		t.Fatalf("PutData: %v", err)
	}

	if err := s.UninstallPlugin(ctx, name, nil); err != nil {
		t.Fatalf("UninstallPlugin: %v", err)
	}

	// Plugin gone.
	_, err := s.GetTenantPlugin(ctx, name)
	if err != store.ErrPluginNotFound {
		t.Errorf("expected ErrPluginNotFound after uninstall, got %v", err)
	}

	// KV data cleaned up (G1).
	_, err = s.GetData(ctx, name, "col", "key1")
	if err != store.ErrPluginNotFound {
		t.Errorf("G1: expected KV data gone after uninstall, got %v", err)
	}
}

func TestPGPluginStore_TenantPlugin_SetError(t *testing.T) {
	s, db := pluginStore(t)
	_, ctx := tenantCtx(t, db)

	name := uniquePlugin("err-plugin")
	installPlugin(t, s, ctx, name)

	if err := s.SetPluginError(ctx, name, "crashed"); err != nil {
		t.Fatalf("SetPluginError: %v", err)
	}
	tp, _ := s.GetTenantPlugin(ctx, name)
	if tp.State != store.PluginStateError {
		t.Errorf("state: got %q want error", tp.State)
	}
	if tp.ErrorMessage != "crashed" {
		t.Errorf("error_message: got %q", tp.ErrorMessage)
	}
}

func TestPGPluginStore_TenantPlugin_UpdateConfig(t *testing.T) {
	s, db := pluginStore(t)
	_, ctx := tenantCtx(t, db)

	name := uniquePlugin("config")
	installPlugin(t, s, ctx, name)

	newConfig := json.RawMessage(`{"maxItems":100}`)
	if err := s.UpdatePluginConfig(ctx, name, newConfig, nil); err != nil {
		t.Fatalf("UpdatePluginConfig: %v", err)
	}

	tp, _ := s.GetTenantPlugin(ctx, name)
	if string(tp.Config) != string(newConfig) {
		t.Errorf("config: got %s want %s", tp.Config, newConfig)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Plugin data (KV store)
// ─────────────────────────────────────────────────────────────────────────────

func TestPGPluginStore_Data_PutGetDelete(t *testing.T) {
	s, db := pluginStore(t)
	_, ctx := tenantCtx(t, db)

	if err := s.PutData(ctx, "my-plugin", "prompts", "key-1", json.RawMessage(`{"v":1}`), nil); err != nil {
		t.Fatalf("PutData: %v", err)
	}

	entry, err := s.GetData(ctx, "my-plugin", "prompts", "key-1")
	if err != nil {
		t.Fatalf("GetData: %v", err)
	}
	if string(entry.Value) != `{"v":1}` {
		t.Errorf("value: got %s", entry.Value)
	}

	if err := s.DeleteData(ctx, "my-plugin", "prompts", "key-1"); err != nil {
		t.Fatalf("DeleteData: %v", err)
	}
	_, err = s.GetData(ctx, "my-plugin", "prompts", "key-1")
	if err != store.ErrPluginNotFound {
		t.Errorf("expected ErrPluginNotFound after delete, got %v", err)
	}
}

func TestPGPluginStore_Data_Upsert_UpdatesValue(t *testing.T) {
	s, db := pluginStore(t)
	_, ctx := tenantCtx(t, db)

	plugin, col, key := "upsert-plugin", "col", "key"
	_ = s.PutData(ctx, plugin, col, key, json.RawMessage(`{"v":1}`), nil)
	_ = s.PutData(ctx, plugin, col, key, json.RawMessage(`{"v":2}`), nil)

	entry, err := s.GetData(ctx, plugin, col, key)
	if err != nil {
		t.Fatalf("GetData: %v", err)
	}
	if string(entry.Value) != `{"v":2}` {
		t.Errorf("expected updated value, got %s", entry.Value)
	}
}

func TestPGPluginStore_Data_ListKeys(t *testing.T) {
	s, db := pluginStore(t)
	_, ctx := tenantCtx(t, db)

	plugin := uniquePlugin("list-plugin")
	for i := 0; i < 3; i++ {
		key := "item-" + uuid.Must(uuid.NewV7()).String()[:4]
		if err := s.PutData(ctx, plugin, "col", key, json.RawMessage(`{}`), nil); err != nil {
			t.Fatalf("PutData: %v", err)
		}
	}

	keys, err := s.ListDataKeys(ctx, plugin, "col", "", 10, 0)
	if err != nil {
		t.Fatalf("ListDataKeys: %v", err)
	}
	if len(keys) < 3 {
		t.Errorf("expected >=3 keys, got %d", len(keys))
	}
}

func TestPGPluginStore_Data_TTL_ExpiredKeyInvisible(t *testing.T) {
	s, db := pluginStore(t)
	_, ctx := tenantCtx(t, db)

	past := time.Now().Add(-1 * time.Hour)
	if err := s.PutData(ctx, "ttl-plugin", "col", "expired-key", json.RawMessage(`{}`), &past); err != nil {
		t.Fatalf("PutData: %v", err)
	}

	_, err := s.GetData(ctx, "ttl-plugin", "col", "expired-key")
	if err != store.ErrPluginNotFound {
		t.Errorf("expected ErrPluginNotFound for expired key, got %v", err)
	}
}

func TestPGPluginStore_Data_NotFoundReturnsError(t *testing.T) {
	s, db := pluginStore(t)
	_, ctx := tenantCtx(t, db)

	_, err := s.GetData(ctx, "no-plugin", "col", "no-key")
	if err != store.ErrPluginNotFound {
		t.Errorf("expected ErrPluginNotFound, got %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Agent plugin overrides
// ─────────────────────────────────────────────────────────────────────────────

func TestPGPluginStore_AgentPlugin_SetGetList(t *testing.T) {
	s, db := pluginStore(t)
	tid, ctx := tenantCtx(t, db)
	agentID := testutil.CreateAgent(t, db, tid, uniquePlugin("agent"), "Test Agent")

	ap := &store.AgentPlugin{
		AgentID:        agentID,
		PluginName:     "vault",
		Enabled:        false,
		ConfigOverride: json.RawMessage(`{"key":"val"}`),
	}
	if err := s.SetAgentPlugin(ctx, ap); err != nil {
		t.Fatalf("SetAgentPlugin: %v", err)
	}

	got, err := s.GetAgentPlugin(ctx, agentID, "vault")
	if err != nil {
		t.Fatalf("GetAgentPlugin: %v", err)
	}
	if got.Enabled {
		t.Error("expected enabled=false")
	}

	list, err := s.ListAgentPlugins(ctx, agentID)
	if err != nil {
		t.Fatalf("ListAgentPlugins: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 agent plugin, got %d", len(list))
	}
}

func TestPGPluginStore_AgentPlugin_NotFound(t *testing.T) {
	s, db := pluginStore(t)
	_, ctx := tenantCtx(t, db)

	_, err := s.GetAgentPlugin(ctx, uuid.New(), "nonexistent")
	if err != store.ErrPluginNotFound {
		t.Errorf("expected ErrPluginNotFound, got %v", err)
	}
}

func TestPGPluginStore_IsPluginEnabledForAgent_InheritsFromTenant(t *testing.T) {
	s, db := pluginStore(t)
	tid, ctx := tenantCtx(t, db)
	agentID := testutil.CreateAgent(t, db, tid, uniquePlugin("agent"), "Test Agent")

	name := uniquePlugin("inherit")
	installPlugin(t, s, ctx, name)
	if err := s.EnablePlugin(ctx, name, nil); err != nil {
		t.Fatalf("EnablePlugin: %v", err)
	}

	// No agent override → inherit tenant-level (enabled).
	enabled, err := s.IsPluginEnabledForAgent(ctx, agentID, name)
	if err != nil {
		t.Fatalf("IsPluginEnabledForAgent: %v", err)
	}
	if !enabled {
		t.Error("expected plugin enabled via tenant inheritance")
	}
}

func TestPGPluginStore_IsPluginEnabledForAgent_AgentOverrideDisables(t *testing.T) {
	s, db := pluginStore(t)
	tid, ctx := tenantCtx(t, db)
	agentID := testutil.CreateAgent(t, db, tid, uniquePlugin("agent"), "Test Agent")

	name := uniquePlugin("override")
	installPlugin(t, s, ctx, name)
	_ = s.EnablePlugin(ctx, name, nil)

	// Agent override disables.
	_ = s.SetAgentPlugin(ctx, &store.AgentPlugin{
		AgentID:        agentID,
		PluginName:     name,
		Enabled:        false,
		ConfigOverride: json.RawMessage(`{}`),
	})

	enabled, err := s.IsPluginEnabledForAgent(ctx, agentID, name)
	if err != nil {
		t.Fatalf("IsPluginEnabledForAgent: %v", err)
	}
	if enabled {
		t.Error("expected agent override to disable the plugin")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Audit log
// ─────────────────────────────────────────────────────────────────────────────

func TestPGPluginStore_Audit_LogAndList(t *testing.T) {
	s, db := pluginStore(t)
	tid, ctx := tenantCtx(t, db)

	entry := &store.PluginAuditEntry{
		TenantID:   tid,
		PluginName: "audit-plugin",
		Action:     store.AuditInstall,
		ActorType:  "user",
		Details:    json.RawMessage(`{"note":"test"}`),
	}
	if err := s.LogAudit(ctx, entry); err != nil {
		t.Fatalf("LogAudit: %v", err)
	}

	entries, err := s.ListAuditLog(ctx, "audit-plugin", 10)
	if err != nil {
		t.Fatalf("ListAuditLog: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected at least 1 audit entry")
	}
	if entries[0].Action != store.AuditInstall {
		t.Errorf("action: got %q want %q", entries[0].Action, store.AuditInstall)
	}
}

func TestPGPluginStore_Audit_InstallWritesEntry(t *testing.T) {
	s, db := pluginStore(t)
	_, ctx := tenantCtx(t, db)

	name := uniquePlugin("audit-install")
	installPlugin(t, s, ctx, name)

	entries, err := s.ListAuditLog(ctx, name, 10)
	if err != nil {
		t.Fatalf("ListAuditLog: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected audit entry after install")
	}
	if entries[0].Action != store.AuditInstall {
		t.Errorf("expected install action, got %q", entries[0].Action)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Cross-tenant isolation — Blocker G2
// ─────────────────────────────────────────────────────────────────────────────

// TestPGPluginStore_G2_DataNotCrossable is the primary G2 isolation test.
// Plugin data written for Tenant A must be invisible when queried with Tenant B context.
func TestPGPluginStore_G2_DataNotCrossable(t *testing.T) {
	s, db := pluginStore(t)
	_, ctxA := tenantCtx(t, db)
	_, ctxB := tenantCtx(t, db)

	// Write data as Tenant A.
	if err := s.PutData(ctxA, "shared-plugin", "col", "secret-key", json.RawMessage(`{"secret":"A"}`), nil); err != nil {
		t.Fatalf("PutData as A: %v", err)
	}

	// Tenant B must NOT see Tenant A data — primary G2 assertion.
	_, err := s.GetData(ctxB, "shared-plugin", "col", "secret-key")
	if err != store.ErrPluginNotFound {
		t.Errorf("G2 VIOLATION: Tenant B can read Tenant A's data! got err=%v", err)
	}
}

// TestPGPluginStore_G2_PluginInstallNotCrossable verifies plugin install isolation.
func TestPGPluginStore_G2_PluginInstallNotCrossable(t *testing.T) {
	s, db := pluginStore(t)
	_, ctxA := tenantCtx(t, db)
	_, ctxB := tenantCtx(t, db)

	name := uniquePlugin("isolated")
	installPlugin(t, s, ctxA, name)

	// Tenant B must NOT see Tenant A's installation.
	_, err := s.GetTenantPlugin(ctxB, name)
	if err != store.ErrPluginNotFound {
		t.Errorf("G2 VIOLATION: Tenant B can see Tenant A plugin install! err=%v", err)
	}

	listB, err := s.ListTenantPlugins(ctxB)
	if err != nil {
		t.Fatalf("ListTenantPlugins B: %v", err)
	}
	for _, p := range listB {
		if p.PluginName == name {
			t.Errorf("G2 VIOLATION: plugin %q from Tenant A appears in Tenant B list", name)
		}
	}
}

// TestPGPluginStore_G2_AuditLogNotCrossable verifies audit log isolation.
func TestPGPluginStore_G2_AuditLogNotCrossable(t *testing.T) {
	s, db := pluginStore(t)
	tidA, ctxA := tenantCtx(t, db)
	_, ctxB := tenantCtx(t, db)

	_ = s.LogAudit(ctxA, &store.PluginAuditEntry{
		TenantID:   tidA,
		PluginName: "secret-plugin",
		Action:     store.AuditInstall,
		ActorType:  "user",
		Details:    json.RawMessage(`{}`),
	})

	entries, err := s.ListAuditLog(ctxB, "secret-plugin", 10)
	if err != nil {
		t.Fatalf("ListAuditLog B: %v", err)
	}
	if len(entries) > 0 {
		t.Errorf("G2 VIOLATION: Tenant B sees %d audit entries from Tenant A", len(entries))
	}
}

// TestPGPluginStore_G2_ForgeTenantID verifies that even if a TenantPlugin struct
// has a different tenant_id, the context tenant_id wins (G2 forged-tenant attack).
func TestPGPluginStore_G2_ForgeTenantID(t *testing.T) {
	s, db := pluginStore(t)
	_, ctxA := tenantCtx(t, db)
	tidB, ctxB := tenantCtx(t, db)

	name := uniquePlugin("forge")

	// Attacker supplies TenantID=B in the struct, but context is Tenant A.
	tp := &store.TenantPlugin{
		TenantID:      tidB, // forged
		PluginName:    name,
		PluginVersion: "0.1.0",
		Config:        json.RawMessage(`{}`),
		Permissions:   json.RawMessage(`{}`),
	}
	if err := s.InstallPlugin(ctxA, tp); err != nil {
		t.Fatalf("InstallPlugin: %v", err)
	}

	// Tenant A (the actual context) sees the plugin.
	_, err := s.GetTenantPlugin(ctxA, name)
	if err != nil {
		t.Errorf("Tenant A should see its own plugin, got %v", err)
	}

	// Tenant B must NOT see it (forged tenant_id was ignored).
	_, err = s.GetTenantPlugin(ctxB, name)
	if err != store.ErrPluginNotFound {
		t.Errorf("G2 VIOLATION: forged tenant_id worked — Tenant B sees it. err=%v", err)
	}
}
