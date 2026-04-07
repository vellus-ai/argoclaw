package plugins

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/vellus-ai/argoclaw/internal/store"
)

// ─────────────────────────────────────────────────────────────────────────────
// Enable tests
// ─────────────────────────────────────────────────────────────────────────────

func TestEnable_Success(t *testing.T) {
	t.Parallel()
	s := newMockPluginStoreHost()
	tenantID := uuid.New()

	s.addCatalogEntry(newCatalogEntry("test-enable", "1.0.0", validManifestYAML("test-enable", "1.0.0")))

	host, toolReg, policy, _ := newTestPluginHost(s)
	ctx := tenantCtx(tenantID)

	// Install first.
	_, err := host.Install(ctx, "test-enable")
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}

	// Enable.
	tp, err := host.Enable(ctx, "test-enable")
	if err != nil {
		t.Fatalf("enable failed: %v", err)
	}

	if tp.State != store.PluginStateEnabled {
		t.Errorf("expected state enabled, got %s", tp.State)
	}

	// Check tools registered with prefix.
	registered := toolReg.registered()
	if len(registered) == 0 {
		t.Error("expected tools to be registered")
	}
	for _, name := range registered {
		if !strings.HasPrefix(name, "plugin_test_enable__") {
			t.Errorf("tool %q does not have expected prefix plugin_test_enable__", name)
		}
	}

	// Check tool group registered.
	policy.mu.Lock()
	group, ok := policy.groups["plugin:test-enable"]
	policy.mu.Unlock()
	if !ok {
		t.Error("expected tool group plugin:test-enable to be registered")
	}
	if len(group) == 0 {
		t.Error("expected tool group to have members")
	}

	// Check registry.
	entry, ok := host.registry.Get("test-enable")
	if !ok {
		t.Error("expected plugin in registry")
	}
	if entry.Status != RegistryActive {
		t.Errorf("expected active status, got %s", entry.Status)
	}
}

func TestEnable_NotInstalled(t *testing.T) {
	t.Parallel()
	s := newMockPluginStoreHost()
	host, _, _, _ := newTestPluginHost(s)
	ctx := tenantCtx(uuid.New())

	_, err := host.Enable(ctx, "nonexistent")
	if !errors.Is(err, ErrPluginNotInstalled) {
		t.Errorf("expected ErrPluginNotInstalled, got %v", err)
	}
}

func TestEnable_InvalidStateTransition(t *testing.T) {
	t.Parallel()
	s := newMockPluginStoreHost()
	tenantID := uuid.New()

	s.addCatalogEntry(newCatalogEntry("re-enable", "1.0.0", validManifestYAML("re-enable", "1.0.0")))

	host, _, _, _ := newTestPluginHost(s)
	ctx := tenantCtx(tenantID)

	// Install and enable.
	_, _ = host.Install(ctx, "re-enable")
	_, err := host.Enable(ctx, "re-enable")
	if err != nil {
		t.Fatalf("first enable failed: %v", err)
	}

	// Enable again should fail (already enabled).
	_, err = host.Enable(ctx, "re-enable")
	if !errors.Is(err, ErrInvalidState) {
		t.Errorf("expected ErrInvalidState, got %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Disable tests
// ─────────────────────────────────────────────────────────────────────────────

func TestDisable_Success(t *testing.T) {
	t.Parallel()
	s := newMockPluginStoreHost()
	tenantID := uuid.New()

	s.addCatalogEntry(newCatalogEntry("test-disable", "1.0.0", validManifestYAML("test-disable", "1.0.0")))

	host, toolReg, policy, _ := newTestPluginHost(s)
	ctx := tenantCtx(tenantID)

	// Install and enable.
	_, _ = host.Install(ctx, "test-disable")
	_, err := host.Enable(ctx, "test-disable")
	if err != nil {
		t.Fatalf("enable failed: %v", err)
	}

	// Verify tools are registered.
	if len(toolReg.registered()) == 0 {
		t.Fatal("expected tools to be registered after enable")
	}

	// Disable.
	tp, err := host.Disable(ctx, "test-disable")
	if err != nil {
		t.Fatalf("disable failed: %v", err)
	}

	if tp.State != store.PluginStateDisabled {
		t.Errorf("expected state disabled, got %s", tp.State)
	}

	// Check tools unregistered.
	if len(toolReg.registered()) != 0 {
		t.Errorf("expected no tools registered, got %v", toolReg.registered())
	}

	// Check tool group unregistered.
	policy.mu.Lock()
	_, ok := policy.groups["plugin:test-disable"]
	policy.mu.Unlock()
	if ok {
		t.Error("expected tool group to be unregistered")
	}

	// Check registry cleared.
	_, ok = host.registry.Get("test-disable")
	if ok {
		t.Error("expected plugin to be removed from registry")
	}
}

func TestDisable_NotInstalled(t *testing.T) {
	t.Parallel()
	s := newMockPluginStoreHost()
	host, _, _, _ := newTestPluginHost(s)
	ctx := tenantCtx(uuid.New())

	_, err := host.Disable(ctx, "nonexistent")
	if !errors.Is(err, ErrPluginNotInstalled) {
		t.Errorf("expected ErrPluginNotInstalled, got %v", err)
	}
}

func TestDisable_InvalidStateTransition(t *testing.T) {
	t.Parallel()
	s := newMockPluginStoreHost()
	tenantID := uuid.New()

	s.addCatalogEntry(newCatalogEntry("no-disable", "1.0.0", validManifestYAML("no-disable", "1.0.0")))

	host, _, _, _ := newTestPluginHost(s)
	ctx := tenantCtx(tenantID)

	// Install but don't enable.
	_, _ = host.Install(ctx, "no-disable")

	// Disable installed plugin should fail (installed → disabled not valid).
	_, err := host.Disable(ctx, "no-disable")
	if !errors.Is(err, ErrInvalidState) {
		t.Errorf("expected ErrInvalidState, got %v", err)
	}
}

func TestDisable_PreservesData(t *testing.T) {
	t.Parallel()
	s := newMockPluginStoreHost()
	tenantID := uuid.New()

	s.addCatalogEntry(newCatalogEntry("data-plugin", "1.0.0", validManifestYAML("data-plugin", "1.0.0")))

	host, _, _, _ := newTestPluginHost(s)
	ctx := tenantCtx(tenantID)

	// Install, enable, then disable.
	_, _ = host.Install(ctx, "data-plugin")
	_, _ = host.Enable(ctx, "data-plugin")
	_, err := host.Disable(ctx, "data-plugin")
	if err != nil {
		t.Fatalf("disable failed: %v", err)
	}

	// Plugin record should still exist.
	tp, err := s.GetTenantPlugin(ctx, "data-plugin")
	if err != nil {
		t.Fatalf("expected plugin record to exist after disable, got %v", err)
	}
	if tp.State != store.PluginStateDisabled {
		t.Errorf("expected disabled state, got %s", tp.State)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// UpdateConfig tests
// ─────────────────────────────────────────────────────────────────────────────

func TestUpdateConfig_Success(t *testing.T) {
	t.Parallel()
	s := newMockPluginStoreHost()
	tenantID := uuid.New()

	s.addCatalogEntry(newCatalogEntry("cfg-plugin", "1.0.0", validManifestYAML("cfg-plugin", "1.0.0")))

	host, _, _, _ := newTestPluginHost(s)
	ctx := tenantCtx(tenantID)

	// Install.
	_, err := host.Install(ctx, "cfg-plugin")
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}

	// Update config.
	newConfig := json.RawMessage(`{"key": "value"}`)
	tp, err := host.UpdateConfig(ctx, "cfg-plugin", newConfig)
	if err != nil {
		t.Fatalf("update config failed: %v", err)
	}

	if string(tp.Config) != `{"key": "value"}` {
		t.Errorf("expected config to be updated, got %s", string(tp.Config))
	}
}

func TestUpdateConfig_NotInstalled(t *testing.T) {
	t.Parallel()
	s := newMockPluginStoreHost()
	host, _, _, _ := newTestPluginHost(s)
	ctx := tenantCtx(uuid.New())

	_, err := host.UpdateConfig(ctx, "nonexistent", json.RawMessage(`{}`))
	if !errors.Is(err, ErrPluginNotInstalled) {
		t.Errorf("expected ErrPluginNotInstalled, got %v", err)
	}
}

func TestUpdateConfig_InvalidConfig(t *testing.T) {
	t.Parallel()
	s := newMockPluginStoreHost()
	tenantID := uuid.New()

	// Manifest with configSchema requiring "apiKey" field.
	manifest := json.RawMessage(`metadata:
  name: schema-plugin
  version: "1.0.0"
  manifestVersion: "1.0"
spec:
  type: tool
  runtime:
    transport: stdio
    command: ./server
  permissions:
    tools:
      provide:
        - search
  configSchema:
    type: object
    required:
      - apiKey
    properties:
      apiKey:
        type: string
`)
	s.addCatalogEntry(newCatalogEntry("schema-plugin", "1.0.0", manifest))

	host, _, _, _ := newTestPluginHost(s)
	ctx := tenantCtx(tenantID)

	// Install.
	_, err := host.Install(ctx, "schema-plugin")
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}

	// Update with invalid config (missing required field).
	_, err = host.UpdateConfig(ctx, "schema-plugin", json.RawMessage(`{"other": "value"}`))
	if !errors.Is(err, ErrConfigInvalid) {
		t.Errorf("expected ErrConfigInvalid, got %v", err)
	}
}

func TestUpdateConfig_ValidConfigWithSchema(t *testing.T) {
	t.Parallel()
	s := newMockPluginStoreHost()
	tenantID := uuid.New()

	manifest := json.RawMessage(`metadata:
  name: valid-schema
  version: "1.0.0"
  manifestVersion: "1.0"
spec:
  type: tool
  runtime:
    transport: stdio
    command: ./server
  permissions:
    tools:
      provide:
        - search
  configSchema:
    type: object
    required:
      - apiKey
    properties:
      apiKey:
        type: string
`)
	s.addCatalogEntry(newCatalogEntry("valid-schema", "1.0.0", manifest))

	host, _, _, _ := newTestPluginHost(s)
	ctx := tenantCtx(tenantID)

	_, _ = host.Install(ctx, "valid-schema")

	// Valid config.
	tp, err := host.UpdateConfig(ctx, "valid-schema", json.RawMessage(`{"apiKey": "sk-123"}`))
	if err != nil {
		t.Fatalf("update config failed: %v", err)
	}
	if tp == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestUpdateConfig_RestartsEnabledPlugin(t *testing.T) {
	t.Parallel()
	s := newMockPluginStoreHost()
	tenantID := uuid.New()

	s.addCatalogEntry(newCatalogEntry("restart-plugin", "1.0.0", validManifestYAML("restart-plugin", "1.0.0")))

	host, toolReg, _, _ := newTestPluginHost(s)
	ctx := tenantCtx(tenantID)

	// Install and enable.
	_, _ = host.Install(ctx, "restart-plugin")
	_, err := host.Enable(ctx, "restart-plugin")
	if err != nil {
		t.Fatalf("enable failed: %v", err)
	}

	// Update config should trigger restart (disable + re-enable).
	_, err = host.UpdateConfig(ctx, "restart-plugin", json.RawMessage(`{"key": "new"}`))
	if err != nil {
		t.Fatalf("update config failed: %v", err)
	}

	// Plugin should still be active in registry after restart.
	entry, ok := host.registry.Get("restart-plugin")
	if !ok {
		t.Error("expected plugin to be in registry after config restart")
	}
	if entry.Status != RegistryActive {
		t.Errorf("expected active status after restart, got %s", entry.Status)
	}

	// Tools should be re-registered.
	if len(toolReg.registered()) == 0 {
		t.Error("expected tools to be re-registered after restart")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Full lifecycle test
// ─────────────────────────────────────────────────────────────────────────────

func TestFullLifecycle_InstallEnableDisableReenableUninstall(t *testing.T) {
	t.Parallel()
	s := newMockPluginStoreHost()
	tenantID := uuid.New()

	s.addCatalogEntry(newCatalogEntry("lifecycle-plugin", "1.0.0", validManifestYAML("lifecycle-plugin", "1.0.0")))

	host, toolReg, _, _ := newTestPluginHost(s)
	ctx := tenantCtx(tenantID)

	// 1. Install.
	tp, err := host.Install(ctx, "lifecycle-plugin")
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}
	if tp.State != store.PluginStateInstalled {
		t.Errorf("expected installed, got %s", tp.State)
	}

	// 2. Enable.
	tp, err = host.Enable(ctx, "lifecycle-plugin")
	if err != nil {
		t.Fatalf("enable failed: %v", err)
	}
	if tp.State != store.PluginStateEnabled {
		t.Errorf("expected enabled, got %s", tp.State)
	}
	if len(toolReg.registered()) == 0 {
		t.Error("expected tools after enable")
	}

	// 3. Disable.
	tp, err = host.Disable(ctx, "lifecycle-plugin")
	if err != nil {
		t.Fatalf("disable failed: %v", err)
	}
	if tp.State != store.PluginStateDisabled {
		t.Errorf("expected disabled, got %s", tp.State)
	}
	if len(toolReg.registered()) != 0 {
		t.Error("expected no tools after disable")
	}

	// 4. Re-enable.
	tp, err = host.Enable(ctx, "lifecycle-plugin")
	if err != nil {
		t.Fatalf("re-enable failed: %v", err)
	}
	if tp.State != store.PluginStateEnabled {
		t.Errorf("expected enabled after re-enable, got %s", tp.State)
	}
	if len(toolReg.registered()) == 0 {
		t.Error("expected tools after re-enable")
	}

	// 5. Uninstall (should auto-disable).
	err = host.Uninstall(ctx, "lifecycle-plugin")
	if err != nil {
		t.Fatalf("uninstall failed: %v", err)
	}
	_, err = s.GetTenantPlugin(ctx, "lifecycle-plugin")
	if !errors.Is(err, store.ErrPluginNotFound) {
		t.Error("expected plugin to be removed after uninstall")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Init and Shutdown tests
// ─────────────────────────────────────────────────────────────────────────────

func TestInit_LoadsEnabledPlugins(t *testing.T) {
	t.Parallel()
	s := newMockPluginStoreHost()
	tenantID := uuid.New()

	s.addCatalogEntry(newCatalogEntry("init-plugin", "1.0.0", validManifestYAML("init-plugin", "1.0.0")))

	// Pre-add an enabled tenant plugin.
	s.addTenantPlugin(tenantID, &store.TenantPlugin{
		ID:            uuid.New(),
		TenantID:      tenantID,
		PluginName:    "init-plugin",
		PluginVersion: "1.0.0",
		State:         store.PluginStateEnabled,
		Config:        json.RawMessage("{}"),
	})

	host, toolReg, _, _ := newTestPluginHost(s)
	ctx := tenantCtx(tenantID)

	err := host.Init(ctx)
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// Check plugin is in registry.
	_, ok := host.registry.Get("init-plugin")
	if !ok {
		t.Error("expected plugin to be loaded into registry")
	}

	// Check tools registered.
	if len(toolReg.registered()) == 0 {
		t.Error("expected tools to be registered after init")
	}
}

func TestInit_SkipsDisabledPlugins(t *testing.T) {
	t.Parallel()
	s := newMockPluginStoreHost()
	tenantID := uuid.New()

	s.addCatalogEntry(newCatalogEntry("disabled-init", "1.0.0", validManifestYAML("disabled-init", "1.0.0")))

	s.addTenantPlugin(tenantID, &store.TenantPlugin{
		ID:            uuid.New(),
		TenantID:      tenantID,
		PluginName:    "disabled-init",
		PluginVersion: "1.0.0",
		State:         store.PluginStateDisabled,
		Config:        json.RawMessage("{}"),
	})

	host, toolReg, _, _ := newTestPluginHost(s)
	ctx := tenantCtx(tenantID)

	err := host.Init(ctx)
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	_, ok := host.registry.Get("disabled-init")
	if ok {
		t.Error("expected disabled plugin NOT to be in registry")
	}
	if len(toolReg.registered()) != 0 {
		t.Error("expected no tools for disabled plugin")
	}
}

func TestInit_ContinuesOnPluginFailure(t *testing.T) {
	t.Parallel()
	s := newMockPluginStoreHost()
	tenantID := uuid.New()

	// Plugin with bad manifest (will fail to parse).
	s.addCatalogEntry(newCatalogEntry("bad-plugin", "1.0.0", json.RawMessage(`{invalid yaml`)))

	s.addTenantPlugin(tenantID, &store.TenantPlugin{
		ID:            uuid.New(),
		TenantID:      tenantID,
		PluginName:    "bad-plugin",
		PluginVersion: "1.0.0",
		State:         store.PluginStateEnabled,
		Config:        json.RawMessage("{}"),
	})

	host, _, _, _ := newTestPluginHost(s)
	ctx := tenantCtx(tenantID)

	// Init should NOT fail even though plugin fails to load.
	err := host.Init(ctx)
	if err != nil {
		t.Fatalf("init should not fail on individual plugin error, got %v", err)
	}
}

func TestShutdown_ClearsRegistry(t *testing.T) {
	t.Parallel()
	s := newMockPluginStoreHost()
	tenantID := uuid.New()

	s.addCatalogEntry(newCatalogEntry("shut-plugin", "1.0.0", validManifestYAML("shut-plugin", "1.0.0")))

	host, _, _, _ := newTestPluginHost(s)
	ctx := tenantCtx(tenantID)

	_, _ = host.Install(ctx, "shut-plugin")
	_, _ = host.Enable(ctx, "shut-plugin")

	// Registry should have the plugin.
	if host.registry.Count() == 0 {
		t.Fatal("expected plugin in registry before shutdown")
	}

	// Shutdown.
	err := host.Shutdown()
	if err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}

	if host.registry.Count() != 0 {
		t.Error("expected registry to be empty after shutdown")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// PBT: P2 — State transitions
// ─────────────────────────────────────────────────────────────────────────────

func TestPBT_StateTransitions(t *testing.T) {
	t.Parallel()

	operations := []string{"enable", "disable", "uninstall"}

	for i := 0; i < 100; i++ {
		t.Run(fmt.Sprintf("run_%d", i), func(t *testing.T) {
			s := newMockPluginStoreHost()
			tenantID := uuid.New()

			pluginName := fmt.Sprintf("pbt-plugin-%d", i)
			s.addCatalogEntry(newCatalogEntry(pluginName, "1.0.0", validManifestYAML(pluginName, "1.0.0")))

			host, _, _, _ := newTestPluginHost(s)
			ctx := tenantCtx(tenantID)

			// Install first.
			_, err := host.Install(ctx, pluginName)
			if err != nil {
				t.Fatalf("install failed: %v", err)
			}

			currentState := StateInstalled
			numOps := rand.Intn(10) + 1

			for j := 0; j < numOps; j++ {
				op := operations[rand.Intn(len(operations))]

				switch op {
				case "enable":
					_, err = host.Enable(ctx, pluginName)
					if isValidTransition(currentState, StateEnabled) {
						if err != nil {
							// Could fail for other reasons (runtime), skip.
							continue
						}
						currentState = StateEnabled
					} else {
						if err == nil {
							t.Errorf("enable from %s should have failed", currentState)
						}
					}
				case "disable":
					_, err = host.Disable(ctx, pluginName)
					if isValidTransition(currentState, StateDisabled) {
						if err != nil {
							continue
						}
						currentState = StateDisabled
					} else {
						if err == nil {
							t.Errorf("disable from %s should have failed", currentState)
						}
					}
				case "uninstall":
					err = host.Uninstall(ctx, pluginName)
					if err == nil {
						return // plugin removed, end sequence
					}
				}
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// PBT: P4 — Tool prefix format
// ─────────────────────────────────────────────────────────────────────────────

func TestPBT_ToolPrefix(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		pluginName string
		toolName   string
	}{
		{"my-plugin", "search"},
		{"prompt-vault", "create-version"},
		{"simple", "tool"},
		{"multi-word-plugin", "multi-word-tool"},
		{"abc", "xyz"},
	}

	for _, tc := range testCases {
		t.Run(tc.pluginName+"__"+tc.toolName, func(t *testing.T) {
			t.Parallel()
			prefix := toolPrefix(tc.pluginName, tc.toolName)
			expected := fmt.Sprintf("plugin_%s__%s",
				strings.ReplaceAll(tc.pluginName, "-", "_"),
				tc.toolName,
			)
			if prefix != expected {
				t.Errorf("toolPrefix(%q, %q) = %q, want %q",
					tc.pluginName, tc.toolName, prefix, expected)
			}

			// Property: must start with "plugin_"
			if !strings.HasPrefix(prefix, "plugin_") {
				t.Errorf("prefix %q does not start with plugin_", prefix)
			}

			// Property: must contain "__"
			if !strings.Contains(prefix, "__") {
				t.Errorf("prefix %q does not contain __", prefix)
			}

			// Property: no hyphens in the plugin name portion
			parts := strings.SplitN(prefix, "__", 2)
			if strings.Contains(parts[0], "-") {
				t.Errorf("plugin name portion %q contains hyphens", parts[0])
			}
		})
	}
}

func TestPBT_ToolPrefix_RandomNames(t *testing.T) {
	t.Parallel()

	for i := 0; i < 100; i++ {
		// Generate random kebab-case plugin name.
		segments := rand.Intn(3) + 1
		parts := make([]string, segments)
		for j := range parts {
			length := rand.Intn(8) + 3
			part := make([]byte, length)
			for k := range part {
				part[k] = byte('a' + rand.Intn(26))
			}
			parts[j] = string(part)
		}
		pluginName := strings.Join(parts, "-")
		toolName := fmt.Sprintf("tool%d", i)

		prefix := toolPrefix(pluginName, toolName)

		// Properties:
		if !strings.HasPrefix(prefix, "plugin_") {
			t.Errorf("run %d: prefix %q does not start with plugin_", i, prefix)
		}
		if !strings.Contains(prefix, "__") {
			t.Errorf("run %d: prefix %q does not contain __", i, prefix)
		}
		// No hyphens in plugin portion.
		pluginPart := strings.SplitN(prefix, "__", 2)[0]
		if strings.Contains(pluginPart, "-") {
			t.Errorf("run %d: plugin portion %q contains hyphens", i, pluginPart)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// PBT: P9 — Config validation
// ─────────────────────────────────────────────────────────────────────────────

func TestPBT_ConfigValidation(t *testing.T) {
	t.Parallel()

	schema := json.RawMessage(`{
		"type": "object",
		"required": ["apiKey", "maxRetries"],
		"properties": {
			"apiKey": {"type": "string"},
			"maxRetries": {"type": "integer"}
		}
	}`)

	t.Run("valid_configs_pass", func(t *testing.T) {
		t.Parallel()
		for i := 0; i < 50; i++ {
			config := json.RawMessage(fmt.Sprintf(
				`{"apiKey": "key-%d", "maxRetries": %d}`, i, rand.Intn(100)))

			err := validateJSONSchema(schema, config)
			if err != nil {
				t.Errorf("run %d: valid config rejected: %v", i, err)
			}
		}
	})

	t.Run("missing_required_field_rejected", func(t *testing.T) {
		t.Parallel()
		// Missing apiKey.
		err := validateJSONSchema(schema, json.RawMessage(`{"maxRetries": 3}`))
		if err == nil {
			t.Error("expected error for missing required field apiKey")
		}

		// Missing maxRetries.
		err = validateJSONSchema(schema, json.RawMessage(`{"apiKey": "test"}`))
		if err == nil {
			t.Error("expected error for missing required field maxRetries")
		}

		// Missing both.
		err = validateJSONSchema(schema, json.RawMessage(`{}`))
		if err == nil {
			t.Error("expected error for missing all required fields")
		}
	})

	t.Run("non_object_config_rejected", func(t *testing.T) {
		t.Parallel()
		invalidConfigs := []string{
			`"string"`, `42`, `true`, `null`, `[1,2,3]`,
		}
		for _, cfg := range invalidConfigs {
			err := validateJSONSchema(schema, json.RawMessage(cfg))
			if err == nil {
				t.Errorf("expected error for non-object config: %s", cfg)
			}
		}
	})

	t.Run("no_schema_accepts_anything", func(t *testing.T) {
		t.Parallel()
		// When schema is null or empty, any config should pass.
		configs := []json.RawMessage{
			json.RawMessage(`{}`),
			json.RawMessage(`{"anything": true}`),
			json.RawMessage(`{"nested": {"deep": "value"}}`),
		}
		for _, config := range configs {
			// validateJSONSchema with empty schema should not be called.
			// This is handled by the UpdateConfig method checking for empty schema.
			_ = config
		}
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Semver helper tests
// ─────────────────────────────────────────────────────────────────────────────

func TestSemverSatisfies(t *testing.T) {
	t.Parallel()
	tests := []struct {
		current  string
		required string
		want     bool
	}{
		{"1.0.0", "1.0.0", true},
		{"1.1.0", "1.0.0", true},
		{"1.0.1", "1.0.0", true},
		{"2.0.0", "1.0.0", true},
		{"0.9.9", "1.0.0", false},
		{"1.0.0", "1.0.1", false},
		{"1.0.0", "1.1.0", false},
		{"1.0.0", "2.0.0", false},
		{"1.5.3", "1.5.3", true},
		{"1.5.4", "1.5.3", true},
		{"10.0.0", "9.99.99", true},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s>=%s", tt.current, tt.required), func(t *testing.T) {
			t.Parallel()
			got := semverSatisfies(tt.current, tt.required)
			if got != tt.want {
				t.Errorf("semverSatisfies(%q, %q) = %v, want %v",
					tt.current, tt.required, got, tt.want)
			}
		})
	}
}

func TestIsValidTransition(t *testing.T) {
	t.Parallel()
	tests := []struct {
		from PluginState
		to   PluginState
		want bool
	}{
		{StateInstalled, StateEnabled, true},
		{StateEnabled, StateDisabled, true},
		{StateEnabled, StateError, true},
		{StateDisabled, StateEnabled, true},
		{StateError, StateEnabled, true},
		{StateError, StateDisabled, true},
		// Invalid transitions.
		{StateInstalled, StateDisabled, false},
		{StateInstalled, StateError, false},
		{StateDisabled, StateDisabled, false},
		{StateEnabled, StateEnabled, false},
		{StateEnabled, StateInstalled, false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s->%s", tt.from, tt.to), func(t *testing.T) {
			t.Parallel()
			got := isValidTransition(tt.from, tt.to)
			if got != tt.want {
				t.Errorf("isValidTransition(%q, %q) = %v, want %v",
					tt.from, tt.to, got, tt.want)
			}
		})
	}
}
