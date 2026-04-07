package plugins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/vellus-ai/argoclaw/internal/store"
)

// ─────────────────────────────────────────────────────────────────────────────
// Mock implementations for PluginHost tests
// ─────────────────────────────────────────────────────────────────────────────

// mockPluginStoreHost implements store.PluginStore for PluginHost tests.
type mockPluginStoreHost struct {
	mu sync.Mutex

	catalogEntries     map[string]*store.PluginCatalogEntry
	tenantPlugins      map[string]map[string]*store.TenantPlugin // tenantID -> pluginName -> tp
	auditEntries       []store.PluginAuditEntry
	installErr         error
	enableErr          error
	disableErr         error
	uninstallErr       error
	updateConfigErr    error
	setPluginErrorErr  error
	getPluginOverride  func(ctx context.Context, name string) (*store.TenantPlugin, error)
	installPluginHook  func(ctx context.Context, tp *store.TenantPlugin) error
}

func newMockPluginStoreHost() *mockPluginStoreHost {
	return &mockPluginStoreHost{
		catalogEntries: make(map[string]*store.PluginCatalogEntry),
		tenantPlugins:  make(map[string]map[string]*store.TenantPlugin),
	}
}

func (m *mockPluginStoreHost) addCatalogEntry(e *store.PluginCatalogEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.catalogEntries[e.Name] = e
}

func (m *mockPluginStoreHost) addTenantPlugin(tenantID uuid.UUID, tp *store.TenantPlugin) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tid := tenantID.String()
	if m.tenantPlugins[tid] == nil {
		m.tenantPlugins[tid] = make(map[string]*store.TenantPlugin)
	}
	m.tenantPlugins[tid][tp.PluginName] = tp
}

func (m *mockPluginStoreHost) UpsertCatalogEntry(_ context.Context, e *store.PluginCatalogEntry) error {
	m.addCatalogEntry(e)
	return nil
}

func (m *mockPluginStoreHost) GetCatalogEntry(_ context.Context, id uuid.UUID) (*store.PluginCatalogEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.catalogEntries {
		if e.ID == id {
			return e, nil
		}
	}
	return nil, store.ErrPluginNotFound
}

func (m *mockPluginStoreHost) GetCatalogEntryByName(_ context.Context, name string) (*store.PluginCatalogEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.catalogEntries[name]
	if !ok {
		return nil, store.ErrPluginNotFound
	}
	return e, nil
}

func (m *mockPluginStoreHost) ListCatalog(_ context.Context) ([]store.PluginCatalogEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []store.PluginCatalogEntry
	for _, e := range m.catalogEntries {
		result = append(result, *e)
	}
	return result, nil
}

func (m *mockPluginStoreHost) InstallPlugin(ctx context.Context, tp *store.TenantPlugin) error {
	if m.installPluginHook != nil {
		return m.installPluginHook(ctx, tp)
	}
	if m.installErr != nil {
		return m.installErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	tid := store.TenantIDFromContext(ctx).String()
	if m.tenantPlugins[tid] == nil {
		m.tenantPlugins[tid] = make(map[string]*store.TenantPlugin)
	}
	if _, exists := m.tenantPlugins[tid][tp.PluginName]; exists {
		return store.ErrPluginAlreadyInstalled
	}
	m.tenantPlugins[tid][tp.PluginName] = tp
	return nil
}

func (m *mockPluginStoreHost) EnablePlugin(ctx context.Context, pluginName string, _ *uuid.UUID) error {
	if m.enableErr != nil {
		return m.enableErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	tid := store.TenantIDFromContext(ctx).String()
	if tp, ok := m.tenantPlugins[tid][pluginName]; ok {
		tp.State = store.PluginStateEnabled
	}
	return nil
}

func (m *mockPluginStoreHost) DisablePlugin(ctx context.Context, pluginName string, _ *uuid.UUID) error {
	if m.disableErr != nil {
		return m.disableErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	tid := store.TenantIDFromContext(ctx).String()
	if tp, ok := m.tenantPlugins[tid][pluginName]; ok {
		tp.State = store.PluginStateDisabled
	}
	return nil
}

func (m *mockPluginStoreHost) UninstallPlugin(ctx context.Context, pluginName string, _ *uuid.UUID) error {
	if m.uninstallErr != nil {
		return m.uninstallErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	tid := store.TenantIDFromContext(ctx).String()
	if _, ok := m.tenantPlugins[tid][pluginName]; !ok {
		return store.ErrPluginNotFound
	}
	delete(m.tenantPlugins[tid], pluginName)
	return nil
}

func (m *mockPluginStoreHost) GetTenantPlugin(ctx context.Context, pluginName string) (*store.TenantPlugin, error) {
	if m.getPluginOverride != nil {
		return m.getPluginOverride(ctx, pluginName)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	tid := store.TenantIDFromContext(ctx).String()
	if tp, ok := m.tenantPlugins[tid][pluginName]; ok {
		return tp, nil
	}
	return nil, store.ErrPluginNotFound
}

func (m *mockPluginStoreHost) ListTenantPlugins(ctx context.Context) ([]store.TenantPlugin, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tid := store.TenantIDFromContext(ctx).String()
	var result []store.TenantPlugin
	for _, tp := range m.tenantPlugins[tid] {
		result = append(result, *tp)
	}
	return result, nil
}

func (m *mockPluginStoreHost) UpdatePluginConfig(ctx context.Context, pluginName string, config json.RawMessage, _ *uuid.UUID) error {
	if m.updateConfigErr != nil {
		return m.updateConfigErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	tid := store.TenantIDFromContext(ctx).String()
	if tp, ok := m.tenantPlugins[tid][pluginName]; ok {
		tp.Config = config
		return nil
	}
	return store.ErrPluginNotFound
}

func (m *mockPluginStoreHost) SetPluginError(ctx context.Context, pluginName, errMsg string) error {
	if m.setPluginErrorErr != nil {
		return m.setPluginErrorErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	tid := store.TenantIDFromContext(ctx).String()
	if tp, ok := m.tenantPlugins[tid][pluginName]; ok {
		tp.State = store.PluginStateError
		tp.ErrorMessage = errMsg
	}
	return nil
}

// Unused but required by interface.
func (m *mockPluginStoreHost) SetAgentPlugin(_ context.Context, _ *store.AgentPlugin) error {
	return nil
}
func (m *mockPluginStoreHost) GetAgentPlugin(_ context.Context, _ uuid.UUID, _ string) (*store.AgentPlugin, error) {
	return nil, store.ErrPluginNotFound
}
func (m *mockPluginStoreHost) ListAgentPlugins(_ context.Context, _ uuid.UUID) ([]store.AgentPlugin, error) {
	return nil, nil
}
func (m *mockPluginStoreHost) IsPluginEnabledForAgent(_ context.Context, _ uuid.UUID, _ string) (bool, error) {
	return true, nil
}
func (m *mockPluginStoreHost) PutData(_ context.Context, _, _, _ string, _ json.RawMessage, _ *time.Time) error {
	return nil
}
func (m *mockPluginStoreHost) GetData(_ context.Context, _, _, _ string) (*store.PluginDataEntry, error) {
	return nil, store.ErrPluginNotFound
}
func (m *mockPluginStoreHost) ListDataKeys(_ context.Context, _, _, _ string, _, _ int) ([]string, error) {
	return nil, nil
}
func (m *mockPluginStoreHost) DeleteData(_ context.Context, _, _, _ string) error {
	return nil
}
func (m *mockPluginStoreHost) DeleteCollectionData(_ context.Context, _, _ string) error {
	return nil
}
func (m *mockPluginStoreHost) LogAudit(_ context.Context, _ *store.PluginAuditEntry) error {
	return nil
}
func (m *mockPluginStoreHost) ListAuditLog(_ context.Context, _ string, _ int) ([]store.PluginAuditEntry, error) {
	return nil, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Mock ToolRegistry
// ─────────────────────────────────────────────────────────────────────────────

type mockToolRegistry struct {
	mu    sync.Mutex
	tools map[string]interface{}
}

func newMockToolRegistry() *mockToolRegistry {
	return &mockToolRegistry{tools: make(map[string]interface{})}
}

func (r *mockToolRegistry) Register(name string, tool interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[name] = tool
	return nil
}

func (r *mockToolRegistry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tools, name)
	return nil
}

func (r *mockToolRegistry) registered() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var names []string
	for n := range r.tools {
		names = append(names, n)
	}
	return names
}

// ─────────────────────────────────────────────────────────────────────────────
// Mock PolicyEngine
// ─────────────────────────────────────────────────────────────────────────────

type mockPolicyEngine struct {
	mu     sync.Mutex
	groups map[string][]string
}

func newMockPolicyEngine() *mockPolicyEngine {
	return &mockPolicyEngine{groups: make(map[string][]string)}
}

func (p *mockPolicyEngine) RegisterToolGroup(name string, members []string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.groups[name] = members
	return nil
}

func (p *mockPolicyEngine) UnregisterToolGroup(name string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.groups, name)
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Mock CronStore
// ─────────────────────────────────────────────────────────────────────────────

type mockCronStore struct {
	mu   sync.Mutex
	jobs map[string]bool // name -> enabled
}

func newMockCronStore() *mockCronStore {
	return &mockCronStore{jobs: make(map[string]bool)}
}

func (c *mockCronStore) RegisterJob(name, _ string, _ func(ctx context.Context) error) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.jobs[name] = true
	return nil
}

func (c *mockCronStore) DisableJob(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.jobs[name] = false
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Test helpers
// ─────────────────────────────────────────────────────────────────────────────

// validManifestYAML returns a valid manifest YAML for testing.
func validManifestYAML(name, version string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`metadata:
  name: %s
  version: %s
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
        - create
`, name, version))
}

// validManifestYAMLWithPlan returns a manifest YAML with a plan requirement.
func validManifestYAMLWithPlan(name, version, plan string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`metadata:
  name: %s
  version: %s
  manifestVersion: "1.0"
spec:
  type: tool
  requires:
    plan: %s
  runtime:
    transport: stdio
    command: ./server
  permissions:
    tools:
      provide:
        - search
`, name, version, plan))
}

// validManifestYAMLWithPlatform returns a manifest with a platform requirement.
func validManifestYAMLWithPlatform(name, version, platform string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`metadata:
  name: %s
  version: %s
  manifestVersion: "1.0"
spec:
  type: tool
  requires:
    platform: "%s"
  runtime:
    transport: stdio
    command: ./server
  permissions:
    tools:
      provide:
        - search
`, name, version, platform))
}

// validManifestYAMLWithDeps returns a manifest with plugin dependencies.
func validManifestYAMLWithDeps(name, version string, deps []string) json.RawMessage {
	depsYAML := ""
	for _, d := range deps {
		depsYAML += fmt.Sprintf("\n        - %s", d)
	}
	return json.RawMessage(fmt.Sprintf(`metadata:
  name: %s
  version: %s
  manifestVersion: "1.0"
spec:
  type: tool
  requires:
    plugins: %s
  runtime:
    transport: stdio
    command: ./server
  permissions:
    tools:
      provide:
        - search
`, name, version, depsYAML))
}

// validManifestYAMLWithFeatures returns a manifest with feature requirements.
func validManifestYAMLWithFeatures(name, version string, features []string) json.RawMessage {
	featYAML := ""
	for _, f := range features {
		featYAML += fmt.Sprintf("\n        - %s", f)
	}
	return json.RawMessage(fmt.Sprintf(`metadata:
  name: %s
  version: %s
  manifestVersion: "1.0"
spec:
  type: tool
  requires:
    features: %s
  runtime:
    transport: stdio
    command: ./server
  permissions:
    tools:
      provide:
        - search
`, name, version, featYAML))
}

func newCatalogEntry(name, version string, manifest json.RawMessage) *store.PluginCatalogEntry {
	return &store.PluginCatalogEntry{
		BaseModel: store.BaseModel{ID: uuid.New()},
		Name:      name,
		Version:   version,
		Manifest:  manifest,
		MinPlan:   "starter",
		Source:    "builtin",
	}
}

func tenantCtx(tenantID uuid.UUID) context.Context {
	return store.WithTenantID(context.Background(), tenantID)
}

func tenantCtxWithPlan(tenantID uuid.UUID, plan string) context.Context {
	ctx := store.WithTenantID(context.Background(), tenantID)
	return WithTenantPlan(ctx, plan)
}

func newTestPluginHost(s store.PluginStore) (*PluginHost, *mockToolRegistry, *mockPolicyEngine, *mockCronStore) {
	cfg := DefaultConfig()
	registry := NewRegistry()
	toolReg := newMockToolRegistry()
	policy := newMockPolicyEngine()
	cronSt := newMockCronStore()

	factory := &mockMCPClientFactory{
		clients: []*mockMCPClient{
			{tools: []MCPToolInfo{
				{Name: "search", Description: "Search tool"},
				{Name: "create", Description: "Create tool"},
			}},
		},
	}

	runtime := NewRuntimeManager(cfg, factory.create, nil)
	events := NewEventBridge(newMockMessageBus(), nil)

	host := NewPluginHost(
		cfg, s, registry, runtime, nil, NewMigrationRunner(),
		nil, events, toolReg, policy, cronSt, nil,
	)
	return host, toolReg, policy, cronSt
}

// mockMessageBus is a simple mock for PluginMessageBus.
type mockMessageBus struct{}

func newMockMessageBus() *mockMessageBus { return &mockMessageBus{} }

func (b *mockMessageBus) Subscribe(_ string, _ func(event interface{})) (func(), error) {
	return func() {}, nil
}

func (b *mockMessageBus) Publish(_ string, _ interface{}) error {
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Install tests
// ─────────────────────────────────────────────────────────────────────────────

func TestInstall_Success(t *testing.T) {
	t.Parallel()
	s := newMockPluginStoreHost()
	s.addCatalogEntry(newCatalogEntry("test-plugin", "1.0.0", validManifestYAML("test-plugin", "1.0.0")))

	host, _, _, _ := newTestPluginHost(s)
	tenantID := uuid.New()
	ctx := tenantCtx(tenantID)

	tp, err := host.Install(ctx, "test-plugin")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if tp == nil {
		t.Fatal("expected tenant plugin, got nil")
	}
	if tp.PluginName != "test-plugin" {
		t.Errorf("expected plugin name test-plugin, got %s", tp.PluginName)
	}
	if tp.State != store.PluginStateInstalled {
		t.Errorf("expected state installed, got %s", tp.State)
	}
}

func TestInstall_PluginNotFoundInCatalog(t *testing.T) {
	t.Parallel()
	s := newMockPluginStoreHost()
	host, _, _, _ := newTestPluginHost(s)
	ctx := tenantCtx(uuid.New())

	_, err := host.Install(ctx, "nonexistent")
	if !errors.Is(err, ErrPluginNotFound) {
		t.Errorf("expected ErrPluginNotFound, got %v", err)
	}
}

func TestInstall_PlanInsufficient(t *testing.T) {
	t.Parallel()
	s := newMockPluginStoreHost()
	e := newCatalogEntry("pro-plugin", "1.0.0", validManifestYAMLWithPlan("pro-plugin", "1.0.0", "enterprise"))
	e.MinPlan = "enterprise"
	s.addCatalogEntry(e)

	host, _, _, _ := newTestPluginHost(s)
	ctx := tenantCtxWithPlan(uuid.New(), "starter")

	_, err := host.Install(ctx, "pro-plugin")
	if !errors.Is(err, ErrPlanInsufficient) {
		t.Errorf("expected ErrPlanInsufficient, got %v", err)
	}
}

func TestInstall_PlatformVersionIncompatible(t *testing.T) {
	t.Parallel()
	s := newMockPluginStoreHost()
	s.addCatalogEntry(newCatalogEntry("future-plugin", "1.0.0", validManifestYAMLWithPlatform("future-plugin", "1.0.0", "99.0.0")))

	host, _, _, _ := newTestPluginHost(s)
	// Save and restore PlatformVersion.
	old := PlatformVersion
	PlatformVersion = "1.0.0"
	defer func() { PlatformVersion = old }()

	ctx := tenantCtx(uuid.New())
	_, err := host.Install(ctx, "future-plugin")
	if !errors.Is(err, ErrPlatformVersion) {
		t.Errorf("expected ErrPlatformVersion, got %v", err)
	}
}

func TestInstall_DependencyMissing(t *testing.T) {
	t.Parallel()
	s := newMockPluginStoreHost()
	s.addCatalogEntry(newCatalogEntry("dep-plugin", "1.0.0", validManifestYAMLWithDeps("dep-plugin", "1.0.0", []string{"missing-dep"})))

	host, _, _, _ := newTestPluginHost(s)
	ctx := tenantCtx(uuid.New())

	_, err := host.Install(ctx, "dep-plugin")
	if !errors.Is(err, ErrDependencyMissing) {
		t.Errorf("expected ErrDependencyMissing, got %v", err)
	}
}

func TestInstall_FeatureMissing(t *testing.T) {
	t.Parallel()
	s := newMockPluginStoreHost()
	s.addCatalogEntry(newCatalogEntry("feat-plugin", "1.0.0", validManifestYAMLWithFeatures("feat-plugin", "1.0.0", []string{"quantum-compute"})))

	host, _, _, _ := newTestPluginHost(s)
	ctx := tenantCtx(uuid.New())

	_, err := host.Install(ctx, "feat-plugin")
	if !errors.Is(err, ErrDependencyMissing) {
		t.Errorf("expected ErrDependencyMissing, got %v", err)
	}
}

func TestInstall_DuplicateInstall(t *testing.T) {
	t.Parallel()
	s := newMockPluginStoreHost()
	s.addCatalogEntry(newCatalogEntry("dup-plugin", "1.0.0", validManifestYAML("dup-plugin", "1.0.0")))

	host, _, _, _ := newTestPluginHost(s)
	tenantID := uuid.New()
	ctx := tenantCtx(tenantID)

	// First install succeeds.
	_, err := host.Install(ctx, "dup-plugin")
	if err != nil {
		t.Fatalf("first install failed: %v", err)
	}

	// Second install should return already installed.
	_, err = host.Install(ctx, "dup-plugin")
	if !errors.Is(err, ErrPluginAlreadyInstalled) {
		t.Errorf("expected ErrPluginAlreadyInstalled, got %v", err)
	}
}

func TestInstall_PlanHierarchy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		pluginSlug string
		reqPlan    string
		tenantPlan string
		wantErr    bool
	}{
		{"starter_can_install_starter", "plan-s-install-s", "starter", "starter", false},
		{"pro_can_install_starter", "plan-p-install-s", "starter", "pro", false},
		{"pro_can_install_pro", "plan-p-install-p", "pro", "pro", false},
		{"enterprise_can_install_all", "plan-e-install-all", "enterprise", "enterprise", false},
		{"starter_cannot_install_pro", "plan-s-no-p", "pro", "starter", true},
		{"starter_cannot_install_enterprise", "plan-s-no-e", "enterprise", "starter", true},
		{"pro_cannot_install_enterprise", "plan-p-no-e", "enterprise", "pro", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := newMockPluginStoreHost()
			pluginName := tt.pluginSlug
			ce := newCatalogEntry(pluginName, "1.0.0", validManifestYAMLWithPlan(pluginName, "1.0.0", tt.reqPlan))
			ce.MinPlan = tt.reqPlan
			s.addCatalogEntry(ce)

			host, _, _, _ := newTestPluginHost(s)
			ctx := tenantCtxWithPlan(uuid.New(), tt.tenantPlan)

			_, err := host.Install(ctx, pluginName)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Uninstall tests
// ─────────────────────────────────────────────────────────────────────────────

func TestUninstall_Success(t *testing.T) {
	t.Parallel()
	s := newMockPluginStoreHost()
	tenantID := uuid.New()

	s.addCatalogEntry(newCatalogEntry("rm-plugin", "1.0.0", validManifestYAML("rm-plugin", "1.0.0")))

	host, _, _, _ := newTestPluginHost(s)
	ctx := tenantCtx(tenantID)

	// Install first.
	_, err := host.Install(ctx, "rm-plugin")
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}

	// Uninstall.
	err = host.Uninstall(ctx, "rm-plugin")
	if err != nil {
		t.Fatalf("uninstall failed: %v", err)
	}

	// Verify it's gone.
	_, err = s.GetTenantPlugin(ctx, "rm-plugin")
	if !errors.Is(err, store.ErrPluginNotFound) {
		t.Errorf("expected ErrPluginNotFound after uninstall, got %v", err)
	}
}

func TestUninstall_NotInstalled(t *testing.T) {
	t.Parallel()
	s := newMockPluginStoreHost()
	host, _, _, _ := newTestPluginHost(s)
	ctx := tenantCtx(uuid.New())

	err := host.Uninstall(ctx, "nonexistent")
	if !errors.Is(err, ErrPluginNotInstalled) {
		t.Errorf("expected ErrPluginNotInstalled, got %v", err)
	}
}

func TestUninstall_DependentsExist(t *testing.T) {
	t.Parallel()
	s := newMockPluginStoreHost()
	tenantID := uuid.New()

	// Plugin A (base).
	s.addCatalogEntry(newCatalogEntry("base-plugin", "1.0.0", validManifestYAML("base-plugin", "1.0.0")))

	// Plugin B depends on A.
	s.addCatalogEntry(newCatalogEntry("dep-child", "1.0.0", validManifestYAMLWithDeps("dep-child", "1.0.0", []string{"base-plugin"})))

	host, _, _, _ := newTestPluginHost(s)
	ctx := tenantCtx(tenantID)

	// Install both.
	_, err := host.Install(ctx, "base-plugin")
	if err != nil {
		t.Fatalf("install base failed: %v", err)
	}
	_, err = host.Install(ctx, "dep-child")
	if err != nil {
		t.Fatalf("install child failed: %v", err)
	}

	// Try to uninstall base — should fail.
	err = host.Uninstall(ctx, "base-plugin")
	if !errors.Is(err, ErrDependentExists) {
		t.Errorf("expected ErrDependentExists, got %v", err)
	}
}

func TestUninstall_TenantIsolation(t *testing.T) {
	t.Parallel()
	s := newMockPluginStoreHost()
	tenantA := uuid.New()
	tenantB := uuid.New()

	s.addCatalogEntry(newCatalogEntry("iso-plugin", "1.0.0", validManifestYAML("iso-plugin", "1.0.0")))

	host, _, _, _ := newTestPluginHost(s)

	// Install for both tenants.
	ctxA := tenantCtx(tenantA)
	ctxB := tenantCtx(tenantB)

	_, err := host.Install(ctxA, "iso-plugin")
	if err != nil {
		t.Fatalf("install A failed: %v", err)
	}
	_, err = host.Install(ctxB, "iso-plugin")
	if err != nil {
		t.Fatalf("install B failed: %v", err)
	}

	// Uninstall for tenant A.
	err = host.Uninstall(ctxA, "iso-plugin")
	if err != nil {
		t.Fatalf("uninstall A failed: %v", err)
	}

	// Tenant B should still have it.
	tp, err := s.GetTenantPlugin(ctxB, "iso-plugin")
	if err != nil {
		t.Fatalf("expected tenant B to still have plugin, got %v", err)
	}
	if tp.PluginName != "iso-plugin" {
		t.Errorf("expected iso-plugin, got %s", tp.PluginName)
	}
}

func TestUninstall_DisablesEnabledPlugin(t *testing.T) {
	t.Parallel()
	s := newMockPluginStoreHost()
	tenantID := uuid.New()

	s.addCatalogEntry(newCatalogEntry("enabled-plugin", "1.0.0", validManifestYAML("enabled-plugin", "1.0.0")))

	host, _, _, _ := newTestPluginHost(s)
	ctx := tenantCtx(tenantID)

	// Install.
	_, err := host.Install(ctx, "enabled-plugin")
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}

	// Enable.
	_, err = host.Enable(ctx, "enabled-plugin")
	if err != nil {
		t.Fatalf("enable failed: %v", err)
	}

	// Uninstall should disable then remove.
	err = host.Uninstall(ctx, "enabled-plugin")
	if err != nil {
		t.Fatalf("uninstall failed: %v", err)
	}

	// Verify gone.
	_, err = s.GetTenantPlugin(ctx, "enabled-plugin")
	if !errors.Is(err, store.ErrPluginNotFound) {
		t.Errorf("expected plugin to be removed, got %v", err)
	}
}
