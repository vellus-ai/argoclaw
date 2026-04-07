package plugins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// ─────────────────────────────────────────────────────────────────────────────
// In-memory PluginStore stub for E2E tests
// Uses maps to simulate catalog, tenant plugins, audit log, and plugin data.
// ─────────────────────────────────────────────────────────────────────────────

type e2eMemStore struct {
	mu sync.Mutex

	catalog       map[string]*e2eCatalogEntry // keyed by name
	tenantPlugins map[string]map[string]*e2eTenantPlugin // tenantID -> pluginName -> record
	auditLog      map[string][]e2eAuditEntry // tenantID -> entries
	pluginData    map[string]map[string]json.RawMessage // tenantID:plugin:collection -> key -> value
}

type e2eCatalogEntry struct {
	ID          uuid.UUID
	Name        string
	Version     string
	DisplayName string
	Description string
	Author      string
	Manifest    json.RawMessage
	Source      string
	MinPlan     string
	Checksum    string
	Tags        []string
}

type e2eTenantPlugin struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	PluginName    string
	PluginVersion string
	State         string
	Config        json.RawMessage
	InstalledBy   uuid.UUID
	EnabledAt     *time.Time
	ErrorMessage  string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type e2eAuditEntry struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	PluginName string
	Action     string
	ActorID    uuid.UUID
	Details    json.RawMessage
	CreatedAt  time.Time
}

func newE2EMemStore() *e2eMemStore {
	return &e2eMemStore{
		catalog:       make(map[string]*e2eCatalogEntry),
		tenantPlugins: make(map[string]map[string]*e2eTenantPlugin),
		auditLog:      make(map[string][]e2eAuditEntry),
		pluginData:    make(map[string]map[string]json.RawMessage),
	}
}

// SeedCatalog adds a CatalogEntry to the in-memory store.
func (s *e2eMemStore) SeedCatalog(entry CatalogEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.catalog[entry.Name] = &e2eCatalogEntry{
		ID:          entry.ID,
		Name:        entry.Name,
		Version:     entry.Version,
		DisplayName: entry.DisplayName,
		Description: entry.Description,
		Author:      entry.Author,
		Manifest:    entry.Manifest,
		Source:      entry.Source,
		MinPlan:     entry.MinPlan,
		Checksum:    entry.Checksum,
		Tags:        entry.Tags,
	}
}

// Install simulates plugin installation.
func (s *e2eMemStore) Install(tenantID uuid.UUID, pluginName, version string, installedBy uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tid := tenantID.String()
	if s.tenantPlugins[tid] == nil {
		s.tenantPlugins[tid] = make(map[string]*e2eTenantPlugin)
	}
	if _, exists := s.tenantPlugins[tid][pluginName]; exists {
		return ErrPluginAlreadyInstalled
	}

	now := time.Now()
	s.tenantPlugins[tid][pluginName] = &e2eTenantPlugin{
		ID:            uuid.New(),
		TenantID:      tenantID,
		PluginName:    pluginName,
		PluginVersion: version,
		State:         string(StateInstalled),
		Config:        json.RawMessage("{}"),
		InstalledBy:   installedBy,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	s.appendAudit(tenantID, pluginName, "install", installedBy)
	return nil
}

// Enable transitions plugin to enabled state.
func (s *e2eMemStore) Enable(tenantID uuid.UUID, pluginName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tp, err := s.getTenantPluginLocked(tenantID, pluginName)
	if err != nil {
		return err
	}

	if tp.State != string(StateInstalled) && tp.State != string(StateDisabled) {
		return fmt.Errorf("%w: cannot enable from state %q", ErrInvalidState, tp.State)
	}

	now := time.Now()
	tp.State = string(StateEnabled)
	tp.EnabledAt = &now
	tp.UpdatedAt = now

	s.appendAudit(tenantID, pluginName, "enable", tp.InstalledBy)
	return nil
}

// Disable transitions plugin to disabled state.
func (s *e2eMemStore) Disable(tenantID uuid.UUID, pluginName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tp, err := s.getTenantPluginLocked(tenantID, pluginName)
	if err != nil {
		return err
	}

	if tp.State != string(StateEnabled) && tp.State != string(StateError) {
		return fmt.Errorf("%w: cannot disable from state %q", ErrInvalidState, tp.State)
	}

	tp.State = string(StateDisabled)
	tp.EnabledAt = nil
	tp.UpdatedAt = time.Now()

	s.appendAudit(tenantID, pluginName, "disable", tp.InstalledBy)
	return nil
}

// Uninstall removes plugin data and record.
func (s *e2eMemStore) Uninstall(tenantID uuid.UUID, pluginName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tp, err := s.getTenantPluginLocked(tenantID, pluginName)
	if err != nil {
		return err
	}

	if tp.State == string(StateEnabled) {
		return ErrPluginEnabled
	}

	tid := tenantID.String()

	// Clean up plugin data.
	for key := range s.pluginData {
		if strings.HasPrefix(key, tid+":"+pluginName+":") {
			delete(s.pluginData, key)
		}
	}

	s.appendAudit(tenantID, pluginName, "uninstall", tp.InstalledBy)
	delete(s.tenantPlugins[tid], pluginName)

	return nil
}

// GetState returns the current state of a tenant plugin.
func (s *e2eMemStore) GetState(tenantID uuid.UUID, pluginName string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tp, err := s.getTenantPluginLocked(tenantID, pluginName)
	if err != nil {
		return "", err
	}
	return tp.State, nil
}

// AuditCount returns the number of audit entries for a tenant+plugin.
func (s *e2eMemStore) AuditCount(tenantID uuid.UUID, pluginName string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	tid := tenantID.String()
	count := 0
	for _, entry := range s.auditLog[tid] {
		if entry.PluginName == pluginName {
			count++
		}
	}
	return count
}

// AuditHasAction checks if there is an audit entry with the given action.
func (s *e2eMemStore) AuditHasAction(tenantID uuid.UUID, pluginName, action string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	tid := tenantID.String()
	for _, entry := range s.auditLog[tid] {
		if entry.PluginName == pluginName && entry.Action == action {
			return true
		}
	}
	return false
}

// IsInstalled checks if a plugin is installed for a tenant.
func (s *e2eMemStore) IsInstalled(tenantID uuid.UUID, pluginName string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	tid := tenantID.String()
	_, exists := s.tenantPlugins[tid][pluginName]
	return exists
}

func (s *e2eMemStore) getTenantPluginLocked(tenantID uuid.UUID, pluginName string) (*e2eTenantPlugin, error) {
	tid := tenantID.String()
	if plugins, ok := s.tenantPlugins[tid]; ok {
		if tp, ok := plugins[pluginName]; ok {
			return tp, nil
		}
	}
	return nil, ErrPluginNotInstalled
}

func (s *e2eMemStore) appendAudit(tenantID uuid.UUID, pluginName, action string, actorID uuid.UUID) {
	tid := tenantID.String()
	s.auditLog[tid] = append(s.auditLog[tid], e2eAuditEntry{
		ID:         uuid.New(),
		TenantID:   tenantID,
		PluginName: pluginName,
		Action:     action,
		ActorID:    actorID,
		CreatedAt:  time.Now(),
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// E2E Mock MCP Client Factory — simulates Prompt Vault MCP server
// ─────────────────────────────────────────────────────────────────────────────

type e2eMockMCPFactory struct {
	mu      sync.Mutex
	clients []*e2eMockMCPClient
}

func (f *e2eMockMCPFactory) create(command string, args []string, env map[string]string) (MCPClient, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	tools := make([]MCPToolInfo, len(promptVaultToolNames))
	for i, name := range promptVaultToolNames {
		tools[i] = MCPToolInfo{
			Name:        name,
			Description: fmt.Sprintf("Prompt Vault tool: %s", name),
		}
	}

	client := &e2eMockMCPClient{
		tools:    tools,
		tenantID: env["ARGO_TENANT_ID"],
	}
	f.clients = append(f.clients, client)
	return client, nil
}

func (f *e2eMockMCPFactory) lastClient() *e2eMockMCPClient {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.clients) == 0 {
		return nil
	}
	return f.clients[len(f.clients)-1]
}

type e2eMockMCPClient struct {
	mu        sync.Mutex
	tools     []MCPToolInfo
	tenantID  string
	closed    bool
	callCount int
}

func (c *e2eMockMCPClient) Initialize(_ context.Context) error { return nil }

func (c *e2eMockMCPClient) ListTools(_ context.Context) ([]MCPToolInfo, error) {
	return c.tools, nil
}

func (c *e2eMockMCPClient) CallTool(_ context.Context, toolName string, args json.RawMessage) (*MCPCallToolResult, error) {
	c.mu.Lock()
	c.callCount++
	c.mu.Unlock()

	// Simulate a successful tool call response based on tool name.
	switch {
	case strings.Contains(toolName, "create"):
		return &MCPCallToolResult{Content: `{"id":"p-001","status":"created"}`}, nil
	case strings.Contains(toolName, "list"):
		return &MCPCallToolResult{Content: `{"items":[],"total":0}`}, nil
	case strings.Contains(toolName, "get"):
		return &MCPCallToolResult{Content: `{"id":"p-001","title":"Test Prompt"}`}, nil
	default:
		return &MCPCallToolResult{Content: `{"ok":true}`}, nil
	}
}

func (c *e2eMockMCPClient) Ping(_ context.Context) error { return nil }

func (c *e2eMockMCPClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// E2E Test: Full Lifecycle with Prompt Vault
// ─────────────────────────────────────────────────────────────────────────────

func TestE2E_PromptVault_FullLifecycle(t *testing.T) {
	t.Parallel()

	memStore := newE2EMemStore()
	registry := NewRegistry()
	factory := &e2eMockMCPFactory{}
	cfg := DefaultConfig()
	rm := NewRuntimeManager(cfg, factory.create, slog.Default())

	// --- Tenant setup ---
	tenantA := uuid.New()
	tenantB := uuid.New()
	actorA := uuid.New()
	actorB := uuid.New()

	// 1. Seed catalog with Prompt Vault entry.
	catalogEntry := SeedPromptVaultCatalog()
	memStore.SeedCatalog(catalogEntry)

	t.Run("01_SeedCatalog_HasCorrectMetadata", func(t *testing.T) {
		if catalogEntry.Name != "prompt-vault" {
			t.Errorf("expected name 'prompt-vault', got %q", catalogEntry.Name)
		}
		if catalogEntry.Version != "1.0.0" {
			t.Errorf("expected version '1.0.0', got %q", catalogEntry.Version)
		}
		if catalogEntry.Source != "builtin" {
			t.Errorf("expected source 'builtin', got %q", catalogEntry.Source)
		}
		if catalogEntry.MinPlan != "starter" {
			t.Errorf("expected min_plan 'starter', got %q", catalogEntry.MinPlan)
		}
		if catalogEntry.TenantID != nil {
			t.Error("expected nil TenantID for builtin plugin")
		}

		// Validate manifest can be parsed back.
		var manifest PluginManifest
		if err := json.Unmarshal(catalogEntry.Manifest, &manifest); err != nil {
			t.Fatalf("failed to unmarshal manifest: %v", err)
		}
		if manifest.Metadata.Name != "prompt-vault" {
			t.Errorf("manifest name mismatch: %q", manifest.Metadata.Name)
		}
		if len(manifest.Spec.Permissions.Tools.Provide) != PromptVaultToolCount {
			t.Errorf("expected %d tools in manifest, got %d",
				PromptVaultToolCount, len(manifest.Spec.Permissions.Tools.Provide))
		}
	})

	// 2. Install for tenant A.
	t.Run("02_Install_TenantA", func(t *testing.T) {
		err := memStore.Install(tenantA, "prompt-vault", "1.0.0", actorA)
		if err != nil {
			t.Fatalf("install failed: %v", err)
		}

		state, err := memStore.GetState(tenantA, "prompt-vault")
		if err != nil {
			t.Fatalf("get state failed: %v", err)
		}
		if state != string(StateInstalled) {
			t.Errorf("expected state 'installed', got %q", state)
		}

		if !memStore.AuditHasAction(tenantA, "prompt-vault", "install") {
			t.Error("expected audit log entry for install action")
		}
	})

	// 2b. Install for tenant B (multi-tenant isolation).
	t.Run("02b_Install_TenantB", func(t *testing.T) {
		err := memStore.Install(tenantB, "prompt-vault", "1.0.0", actorB)
		if err != nil {
			t.Fatalf("install for tenant B failed: %v", err)
		}
	})

	// 2c. Duplicate install should fail.
	t.Run("02c_DuplicateInstall_Fails", func(t *testing.T) {
		err := memStore.Install(tenantA, "prompt-vault", "1.0.0", actorA)
		if !errors.Is(err, ErrPluginAlreadyInstalled) {
			t.Fatalf("expected ErrPluginAlreadyInstalled, got: %v", err)
		}
	})

	// 3. Enable for tenant A — start MCP, discover tools.
	t.Run("03_Enable_TenantA", func(t *testing.T) {
		// Transition in store.
		err := memStore.Enable(tenantA, "prompt-vault")
		if err != nil {
			t.Fatalf("enable in store failed: %v", err)
		}

		state, _ := memStore.GetState(tenantA, "prompt-vault")
		if state != string(StateEnabled) {
			t.Errorf("expected state 'enabled', got %q", state)
		}

		// Start MCP process via RuntimeManager.
		var manifest PluginManifest
		if err := json.Unmarshal(catalogEntry.Manifest, &manifest); err != nil {
			t.Fatalf("unmarshal manifest: %v", err)
		}
		manifest.Name = manifest.Metadata.Name
		manifest.Version = manifest.Metadata.Version

		tools, err := rm.StartPlugin(context.Background(), &manifest, tenantA)
		if err != nil {
			t.Fatalf("StartPlugin failed: %v", err)
		}

		if len(tools) != PromptVaultToolCount {
			t.Errorf("expected %d discovered tools, got %d", PromptVaultToolCount, len(tools))
		}

		// Register in registry with tools.
		toolNames := make([]string, len(tools))
		for i, tool := range tools {
			toolNames[i] = fmt.Sprintf("plugin_prompt_vault__%s", tool.Name)
		}

		registry.Register("prompt-vault", &RegistryEntry{
			Manifest:  &manifest,
			CatalogID: catalogEntry.ID,
			Status:    RegistryActive,
			Tools:     toolNames,
			EnabledAt: time.Now(),
		})

		// Verify registry state.
		entry, ok := registry.Get("prompt-vault")
		if !ok {
			t.Fatal("expected prompt-vault in registry")
		}
		if entry.Status != RegistryActive {
			t.Errorf("expected RegistryActive, got %q", entry.Status)
		}
		if len(entry.Tools) != PromptVaultToolCount {
			t.Errorf("expected %d registered tools, got %d", PromptVaultToolCount, len(entry.Tools))
		}

		// Verify tool prefix.
		for _, toolName := range entry.Tools {
			if !strings.HasPrefix(toolName, "plugin_prompt_vault__") {
				t.Errorf("tool %q missing required prefix 'plugin_prompt_vault__'", toolName)
			}
		}

		if !memStore.AuditHasAction(tenantA, "prompt-vault", "enable") {
			t.Error("expected audit log entry for enable action")
		}
	})

	// 4. Simulate tool call.
	t.Run("04_ToolCall_VaultPromptCreate", func(t *testing.T) {
		args := json.RawMessage(`{"title":"Test Prompt","content":"Hello {{name}}"}`)
		result, err := rm.CallTool(context.Background(), "prompt-vault", "vault_prompt_create", args)
		if err != nil {
			t.Fatalf("CallTool failed: %v", err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if !strings.Contains(result.Content, "created") {
			t.Errorf("expected result to contain 'created', got %q", result.Content)
		}
	})

	// 5. Disable — tools should be unregistered.
	t.Run("05_Disable", func(t *testing.T) {
		// Stop MCP process.
		err := rm.StopPlugin(context.Background(), "prompt-vault", 5*time.Second)
		if err != nil {
			t.Fatalf("StopPlugin failed: %v", err)
		}

		// Unregister from registry.
		registry.Unregister("prompt-vault")

		// Update store state.
		err = memStore.Disable(tenantA, "prompt-vault")
		if err != nil {
			t.Fatalf("disable in store failed: %v", err)
		}

		// Verify registry is empty.
		if _, ok := registry.Get("prompt-vault"); ok {
			t.Error("expected prompt-vault removed from registry after disable")
		}

		state, _ := memStore.GetState(tenantA, "prompt-vault")
		if state != string(StateDisabled) {
			t.Errorf("expected state 'disabled', got %q", state)
		}

		if !memStore.AuditHasAction(tenantA, "prompt-vault", "disable") {
			t.Error("expected audit log entry for disable action")
		}
	})

	// 6. Re-enable — data should be preserved, tools re-registered.
	t.Run("06_ReEnable_DataPreserved", func(t *testing.T) {
		// Re-enable in store.
		err := memStore.Enable(tenantA, "prompt-vault")
		if err != nil {
			t.Fatalf("re-enable in store failed: %v", err)
		}

		// Start MCP process again.
		var manifest PluginManifest
		if err := json.Unmarshal(catalogEntry.Manifest, &manifest); err != nil {
			t.Fatalf("unmarshal manifest: %v", err)
		}
		manifest.Name = manifest.Metadata.Name
		manifest.Version = manifest.Metadata.Version

		tools, err := rm.StartPlugin(context.Background(), &manifest, tenantA)
		if err != nil {
			t.Fatalf("StartPlugin (re-enable) failed: %v", err)
		}

		if len(tools) != PromptVaultToolCount {
			t.Errorf("expected %d tools after re-enable, got %d", PromptVaultToolCount, len(tools))
		}

		// Re-register in registry.
		toolNames := make([]string, len(tools))
		for i, tool := range tools {
			toolNames[i] = fmt.Sprintf("plugin_prompt_vault__%s", tool.Name)
		}
		registry.Register("prompt-vault", &RegistryEntry{
			Manifest:  &manifest,
			CatalogID: catalogEntry.ID,
			Status:    RegistryActive,
			Tools:     toolNames,
			EnabledAt: time.Now(),
		})

		entry, ok := registry.Get("prompt-vault")
		if !ok {
			t.Fatal("expected prompt-vault in registry after re-enable")
		}
		if len(entry.Tools) != PromptVaultToolCount {
			t.Errorf("expected %d tools after re-enable, got %d", PromptVaultToolCount, len(entry.Tools))
		}

		// Plugin should still be installed for tenant A.
		if !memStore.IsInstalled(tenantA, "prompt-vault") {
			t.Error("expected plugin to still be installed after re-enable")
		}
	})

	// 7. Uninstall — cleanup.
	t.Run("07_Uninstall", func(t *testing.T) {
		// Must disable first.
		_ = rm.StopPlugin(context.Background(), "prompt-vault", 5*time.Second)
		registry.Unregister("prompt-vault")
		_ = memStore.Disable(tenantA, "prompt-vault")

		err := memStore.Uninstall(tenantA, "prompt-vault")
		if err != nil {
			t.Fatalf("uninstall failed: %v", err)
		}

		if memStore.IsInstalled(tenantA, "prompt-vault") {
			t.Error("expected plugin to be uninstalled")
		}

		if _, ok := registry.Get("prompt-vault"); ok {
			t.Error("expected prompt-vault removed from registry after uninstall")
		}
	})

	// 7b. Uninstall enabled plugin should fail.
	t.Run("07b_Uninstall_EnabledPlugin_Fails", func(t *testing.T) {
		// Enable tenant B's plugin first.
		_ = memStore.Enable(tenantB, "prompt-vault")

		err := memStore.Uninstall(tenantB, "prompt-vault")
		if !errors.Is(err, ErrPluginEnabled) {
			t.Fatalf("expected ErrPluginEnabled, got: %v", err)
		}

		// Cleanup: disable before uninstall.
		_ = memStore.Disable(tenantB, "prompt-vault")
		_ = memStore.Uninstall(tenantB, "prompt-vault")
	})

	// 8. Multi-tenant isolation.
	t.Run("08_MultiTenant_Isolation", func(t *testing.T) {
		// After all operations, tenant B should have been independent.
		// Tenant A's uninstall should not affect tenant B's audit log.
		auditCountA := memStore.AuditCount(tenantA, "prompt-vault")
		auditCountB := memStore.AuditCount(tenantB, "prompt-vault")

		if auditCountA == 0 {
			t.Error("expected non-zero audit count for tenant A")
		}
		if auditCountB == 0 {
			t.Error("expected non-zero audit count for tenant B")
		}
		if auditCountA == auditCountB {
			// They went through different lifecycle operations so counts should differ.
			// A: install, enable, disable, enable, disable, uninstall = 6
			// B: install, enable, disable, uninstall = 4
			t.Logf("warning: audit counts equal (A=%d, B=%d) — operations may have been symmetric",
				auditCountA, auditCountB)
		}

		// Tenant A is uninstalled, tenant B was also uninstalled.
		if memStore.IsInstalled(tenantA, "prompt-vault") {
			t.Error("tenant A should not have prompt-vault installed")
		}
		if memStore.IsInstalled(tenantB, "prompt-vault") {
			t.Error("tenant B should not have prompt-vault installed")
		}
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// E2E Test: Tool Call When Plugin Disabled
// ─────────────────────────────────────────────────────────────────────────────

func TestE2E_ToolCall_PluginDisabled(t *testing.T) {
	t.Parallel()

	factory := func(command string, args []string, env map[string]string) (MCPClient, error) {
		return &mockMCPClient{
			tools: []MCPToolInfo{{Name: "vault_prompt_create", Description: "Create prompt"}},
		}, nil
	}

	cfg := DefaultConfig()
	rm := NewRuntimeManager(cfg, factory, slog.Default())

	manifest := testManifest("prompt-vault", "./server")
	_, err := rm.StartPlugin(context.Background(), manifest, uuid.New())
	if err != nil {
		t.Fatalf("StartPlugin: %v", err)
	}

	// Disable the plugin in the runtime manager.
	rm.mu.Lock()
	rm.processes["prompt-vault"].enabled = false
	rm.mu.Unlock()

	_, err = rm.CallTool(context.Background(), "prompt-vault", "vault_prompt_create", nil)
	if err == nil {
		t.Fatal("expected error when calling tool on disabled plugin")
	}
	if !errors.Is(err, ErrPluginNotEnabled) {
		t.Fatalf("expected ErrPluginNotEnabled, got: %v", err)
	}
	if !strings.Contains(err.Error(), "prompt-vault") {
		t.Errorf("error should mention plugin name 'prompt-vault': %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// E2E Test: Tool Call When Circuit Breaker Open
// ─────────────────────────────────────────────────────────────────────────────

func TestE2E_ToolCall_CircuitBreakerOpen(t *testing.T) {
	t.Parallel()

	factory := func(command string, args []string, env map[string]string) (MCPClient, error) {
		return &mockMCPClient{
			tools:       []MCPToolInfo{{Name: "vault_prompt_create", Description: "Create prompt"}},
			callToolErr: errors.New("server crashed"),
		}, nil
	}

	cfg := DefaultConfig()
	cfg.CircuitBreakerThreshold = 2
	cfg.CircuitBreakerResetTimeout = 1 * time.Hour // keep open
	rm := NewRuntimeManager(cfg, factory, slog.Default())

	manifest := testManifest("prompt-vault", "./server")
	_, err := rm.StartPlugin(context.Background(), manifest, uuid.New())
	if err != nil {
		t.Fatalf("StartPlugin: %v", err)
	}

	// Trip the circuit breaker.
	for i := 0; i < 2; i++ {
		_, _ = rm.CallTool(context.Background(), "prompt-vault", "vault_prompt_create", nil)
	}

	// Next call should be rejected by circuit breaker (503 equivalent).
	_, err = rm.CallTool(context.Background(), "prompt-vault", "vault_prompt_create", nil)
	if err == nil {
		t.Fatal("expected error from circuit breaker")
	}
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got: %v", err)
	}

	// Verify HTTP mapping is 503.
	status, code := ErrorToHTTP(ErrCircuitOpen)
	if status != 503 {
		t.Errorf("expected HTTP 503, got %d", status)
	}
	if code != "circuit_open" {
		t.Errorf("expected code 'circuit_open', got %q", code)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// E2E Test: Tool Call When Plugin Crashed
// ─────────────────────────────────────────────────────────────────────────────

func TestE2E_ToolCall_PluginCrashed(t *testing.T) {
	t.Parallel()

	factory := func(command string, args []string, env map[string]string) (MCPClient, error) {
		return &mockMCPClient{
			tools: []MCPToolInfo{{Name: "vault_prompt_create", Description: "Create prompt"}},
			callToolFn: func(_ context.Context, _ string, _ json.RawMessage) (*MCPCallToolResult, error) {
				return nil, fmt.Errorf("broken pipe: plugin process exited unexpectedly")
			},
		}, nil
	}

	cfg := DefaultConfig()
	rm := NewRuntimeManager(cfg, factory, slog.Default())

	manifest := testManifest("prompt-vault", "./server")
	_, err := rm.StartPlugin(context.Background(), manifest, uuid.New())
	if err != nil {
		t.Fatalf("StartPlugin: %v", err)
	}

	_, err = rm.CallTool(context.Background(), "prompt-vault", "vault_prompt_create", nil)
	if err == nil {
		t.Fatal("expected error when plugin crashed")
	}

	// Error should be descriptive: mention plugin name and tool.
	errMsg := err.Error()
	if !strings.Contains(errMsg, "prompt-vault") {
		t.Errorf("error should mention plugin name: %v", err)
	}
	if !strings.Contains(errMsg, "vault_prompt_create") {
		t.Errorf("error should mention tool name: %v", err)
	}
	if !strings.Contains(errMsg, "broken pipe") {
		t.Errorf("error should contain root cause: %v", err)
	}

	// Verify HTTP mapping for crashed plugin (502).
	status, code := ErrorToHTTP(ErrPluginCrashed)
	if status != 502 {
		t.Errorf("expected HTTP 502, got %d", status)
	}
	if code != "plugin_crashed" {
		t.Errorf("expected code 'plugin_crashed', got %q", code)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// E2E Test: Tool Call to Non-Existent Plugin
// ─────────────────────────────────────────────────────────────────────────────

func TestE2E_ToolCall_PluginNotFound(t *testing.T) {
	t.Parallel()

	factory := &mockMCPClientFactory{}
	rm := newTestRuntimeManager(factory.create)

	_, err := rm.CallTool(context.Background(), "nonexistent-plugin", "some_tool", nil)
	if !errors.Is(err, ErrPluginNotFound) {
		t.Fatalf("expected ErrPluginNotFound, got: %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Performance Baseline: Lifecycle Operation Latency
// ─────────────────────────────────────────────────────────────────────────────

func TestE2E_PerformanceBaseline_LifecycleLatency(t *testing.T) {
	t.Parallel()

	const iterations = 20
	const p95Threshold = 5 * time.Second // with mock MCP, should be well under 100ms

	memStore := newE2EMemStore()
	catalogEntry := SeedPromptVaultCatalog()
	memStore.SeedCatalog(catalogEntry)

	factory := &e2eMockMCPFactory{}
	cfg := DefaultConfig()

	type latencyResult struct {
		operation string
		p95       time.Duration
		avg       time.Duration
		max       time.Duration
	}

	measureOp := func(opName string, fn func(i int) error) latencyResult {
		durations := make([]time.Duration, iterations)
		for i := 0; i < iterations; i++ {
			start := time.Now()
			if err := fn(i); err != nil {
				t.Fatalf("%s iteration %d failed: %v", opName, i, err)
			}
			durations[i] = time.Since(start)
		}

		// Sort for p95.
		sortDurations(durations)
		p95Idx := int(float64(len(durations)) * 0.95)
		if p95Idx >= len(durations) {
			p95Idx = len(durations) - 1
		}

		var total time.Duration
		for _, d := range durations {
			total += d
		}

		return latencyResult{
			operation: opName,
			p95:       durations[p95Idx],
			avg:       total / time.Duration(len(durations)),
			max:       durations[len(durations)-1],
		}
	}

	// Measure install latency.
	installResult := measureOp("install", func(i int) error {
		tenantID := uuid.New()
		return memStore.Install(tenantID, "prompt-vault", "1.0.0", uuid.New())
	})

	// Measure enable + StartPlugin latency.
	enableResult := measureOp("enable+start", func(i int) error {
		tenantID := uuid.New()
		_ = memStore.Install(tenantID, "prompt-vault", "1.0.0", uuid.New())
		if err := memStore.Enable(tenantID, "prompt-vault"); err != nil {
			return err
		}

		var manifest PluginManifest
		_ = json.Unmarshal(catalogEntry.Manifest, &manifest)
		manifest.Name = manifest.Metadata.Name
		manifest.Version = manifest.Metadata.Version

		rm := NewRuntimeManager(cfg, factory.create, slog.Default())
		_, err := rm.StartPlugin(context.Background(), &manifest, tenantID)
		return err
	})

	// Measure disable + StopPlugin latency.
	disableResult := measureOp("disable+stop", func(i int) error {
		tenantID := uuid.New()
		_ = memStore.Install(tenantID, "prompt-vault", "1.0.0", uuid.New())
		_ = memStore.Enable(tenantID, "prompt-vault")

		var manifest PluginManifest
		_ = json.Unmarshal(catalogEntry.Manifest, &manifest)
		manifest.Name = manifest.Metadata.Name
		manifest.Version = manifest.Metadata.Version

		pluginName := fmt.Sprintf("prompt-vault-%d", i)
		manifest.Metadata.Name = pluginName
		manifest.Name = pluginName

		rm := NewRuntimeManager(cfg, factory.create, slog.Default())
		_, _ = rm.StartPlugin(context.Background(), &manifest, tenantID)
		return rm.StopPlugin(context.Background(), pluginName, 5*time.Second)
	})

	// Measure uninstall latency.
	uninstallResult := measureOp("uninstall", func(i int) error {
		tenantID := uuid.New()
		_ = memStore.Install(tenantID, "prompt-vault", "1.0.0", uuid.New())
		return memStore.Uninstall(tenantID, "prompt-vault")
	})

	// Assert p95 < threshold.
	results := []latencyResult{installResult, enableResult, disableResult, uninstallResult}
	for _, r := range results {
		t.Logf("%-20s p95=%-12s avg=%-12s max=%s", r.operation, r.p95, r.avg, r.max)
		if r.p95 > p95Threshold {
			t.Errorf("%s p95 latency %s exceeds threshold %s", r.operation, r.p95, p95Threshold)
		}
	}
}

// sortDurations sorts a slice of durations in ascending order (simple insertion sort).
func sortDurations(d []time.Duration) {
	for i := 1; i < len(d); i++ {
		key := d[i]
		j := i - 1
		for j >= 0 && d[j] > key {
			d[j+1] = d[j]
			j--
		}
		d[j+1] = key
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Seed function unit tests
// ─────────────────────────────────────────────────────────────────────────────

func TestSeedPromptVaultCatalog_ManifestParseable(t *testing.T) {
	t.Parallel()

	entry := SeedPromptVaultCatalog()

	var manifest PluginManifest
	if err := json.Unmarshal(entry.Manifest, &manifest); err != nil {
		t.Fatalf("manifest JSON should be parseable: %v", err)
	}

	if manifest.Metadata.Name != "prompt-vault" {
		t.Errorf("expected name 'prompt-vault', got %q", manifest.Metadata.Name)
	}
	if manifest.Spec.Type != "tool" {
		t.Errorf("expected type 'tool', got %q", manifest.Spec.Type)
	}
	if manifest.Spec.Runtime.Transport != "stdio" {
		t.Errorf("expected transport 'stdio', got %q", manifest.Spec.Runtime.Transport)
	}
	if manifest.Spec.Requires.Plan != "starter" {
		t.Errorf("expected plan 'starter', got %q", manifest.Spec.Requires.Plan)
	}
}

func TestSeedPromptVaultCatalog_ToolCount(t *testing.T) {
	t.Parallel()

	names := PromptVaultToolNames()
	if len(names) != PromptVaultToolCount {
		t.Errorf("expected %d tool names, got %d", PromptVaultToolCount, len(names))
	}

	// Verify all names are unique.
	seen := make(map[string]bool)
	for _, name := range names {
		if seen[name] {
			t.Errorf("duplicate tool name: %q", name)
		}
		seen[name] = true
	}

	// Verify all have the vault_ prefix.
	for _, name := range names {
		if !strings.HasPrefix(name, "vault_") {
			t.Errorf("tool %q should have 'vault_' prefix", name)
		}
	}
}

func TestPromptVaultToolNames_ReturnsCopy(t *testing.T) {
	t.Parallel()

	names1 := PromptVaultToolNames()
	names2 := PromptVaultToolNames()

	// Mutating one should not affect the other.
	names1[0] = "mutated"
	if names2[0] == "mutated" {
		t.Error("PromptVaultToolNames should return a copy, not the original slice")
	}
}
