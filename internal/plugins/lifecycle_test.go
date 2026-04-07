package plugins_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/vellus-ai/argoclaw/internal/plugins"
	"github.com/vellus-ai/argoclaw/internal/store"
)

// ─────────────────────────────────────────────────────────────────────────────
// Stub for PluginStore lifecycle tests
// (only ListTenantPlugins and GetTenantPlugin are needed)
// ─────────────────────────────────────────────────────────────────────────────

type stubLifecycleStore struct {
	listTenantPlugins func(ctx context.Context) ([]store.TenantPlugin, error)
	getTenantPlugin   func(ctx context.Context, name string) (*store.TenantPlugin, error)
	enablePlugin      func(ctx context.Context, name string, actor *uuid.UUID) error
	disablePlugin     func(ctx context.Context, name string, actor *uuid.UUID) error
}

func (s *stubLifecycleStore) ListTenantPlugins(ctx context.Context) ([]store.TenantPlugin, error) {
	if s.listTenantPlugins != nil {
		return s.listTenantPlugins(ctx)
	}
	return nil, nil
}
func (s *stubLifecycleStore) GetTenantPlugin(ctx context.Context, name string) (*store.TenantPlugin, error) {
	if s.getTenantPlugin != nil {
		return s.getTenantPlugin(ctx, name)
	}
	return nil, store.ErrPluginNotFound
}
func (s *stubLifecycleStore) EnablePlugin(ctx context.Context, name string, actor *uuid.UUID) error {
	if s.enablePlugin != nil {
		return s.enablePlugin(ctx, name, actor)
	}
	return nil
}
func (s *stubLifecycleStore) DisablePlugin(ctx context.Context, name string, actor *uuid.UUID) error {
	if s.disablePlugin != nil {
		return s.disablePlugin(ctx, name, actor)
	}
	return nil
}

// Unimplemented methods — panic if called.
func (s *stubLifecycleStore) UpsertCatalogEntry(_ context.Context, _ *store.PluginCatalogEntry) error {
	panic("not implemented")
}
func (s *stubLifecycleStore) GetCatalogEntry(_ context.Context, _ uuid.UUID) (*store.PluginCatalogEntry, error) {
	panic("not implemented")
}
func (s *stubLifecycleStore) GetCatalogEntryByName(_ context.Context, _ string) (*store.PluginCatalogEntry, error) {
	panic("not implemented")
}
func (s *stubLifecycleStore) ListCatalog(_ context.Context) ([]store.PluginCatalogEntry, error) {
	panic("not implemented")
}
func (s *stubLifecycleStore) InstallPlugin(_ context.Context, _ *store.TenantPlugin) error {
	panic("not implemented")
}
func (s *stubLifecycleStore) UninstallPlugin(_ context.Context, _ string, _ *uuid.UUID) error {
	panic("not implemented")
}
func (s *stubLifecycleStore) UpdatePluginConfig(_ context.Context, _ string, _ json.RawMessage, _ *uuid.UUID) error {
	panic("not implemented")
}
func (s *stubLifecycleStore) SetPluginError(_ context.Context, _, _ string) error {
	panic("not implemented")
}
func (s *stubLifecycleStore) SetAgentPlugin(_ context.Context, _ *store.AgentPlugin) error {
	panic("not implemented")
}
func (s *stubLifecycleStore) GetAgentPlugin(_ context.Context, _ uuid.UUID, _ string) (*store.AgentPlugin, error) {
	panic("not implemented")
}
func (s *stubLifecycleStore) ListAgentPlugins(_ context.Context, _ uuid.UUID) ([]store.AgentPlugin, error) {
	panic("not implemented")
}
func (s *stubLifecycleStore) IsPluginEnabledForAgent(_ context.Context, _ uuid.UUID, _ string) (bool, error) {
	panic("not implemented")
}
func (s *stubLifecycleStore) PutData(_ context.Context, _, _, _ string, _ json.RawMessage, _ *time.Time) error {
	panic("not implemented")
}
func (s *stubLifecycleStore) GetData(_ context.Context, _, _, _ string) (*store.PluginDataEntry, error) {
	panic("not implemented")
}
func (s *stubLifecycleStore) ListDataKeys(_ context.Context, _, _, _ string, _, _ int) ([]string, error) {
	panic("not implemented")
}
func (s *stubLifecycleStore) DeleteData(_ context.Context, _, _, _ string) error { panic("not implemented") }
func (s *stubLifecycleStore) DeleteCollectionData(_ context.Context, _, _ string) error {
	panic("not implemented")
}
func (s *stubLifecycleStore) LogAudit(_ context.Context, _ *store.PluginAuditEntry) error {
	panic("not implemented")
}
func (s *stubLifecycleStore) ListAuditLog(_ context.Context, _ string, _ int) ([]store.PluginAuditEntry, error) {
	panic("not implemented")
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests — TDD Red phase (written before implementation)
// ─────────────────────────────────────────────────────────────────────────────

func TestLifecycle_LoadAll_NoPlugins(t *testing.T) {
	stub := &stubLifecycleStore{
		listTenantPlugins: func(_ context.Context) ([]store.TenantPlugin, error) {
			return nil, nil
		},
	}
	reg := plugins.NewRegistry()
	lc := plugins.NewLifecycle(stub, reg)

	if err := lc.LoadAll(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reg.Count() != 0 {
		t.Errorf("expected empty registry, got %d plugins", reg.Count())
	}
}

func TestLifecycle_LoadAll_SkipsDisabledPlugins(t *testing.T) {
	stub := &stubLifecycleStore{
		listTenantPlugins: func(_ context.Context) ([]store.TenantPlugin, error) {
			return []store.TenantPlugin{
				{PluginName: "vault", State: store.PluginStateDisabled},
				{PluginName: "memory", State: store.PluginStateInstalled},
			}, nil
		},
	}
	reg := plugins.NewRegistry()
	lc := plugins.NewLifecycle(stub, reg)

	if err := lc.LoadAll(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reg.Count() != 0 {
		t.Errorf("expected 0 active plugins (disabled + installed skipped), got %d", reg.Count())
	}
}

func TestLifecycle_LoadAll_RegistersEnabledPlugins(t *testing.T) {
	catalogID := uuid.New()
	stub := &stubLifecycleStore{
		listTenantPlugins: func(_ context.Context) ([]store.TenantPlugin, error) {
			return []store.TenantPlugin{
				{PluginName: "vault", PluginVersion: "1.0.0", State: store.PluginStateEnabled},
				{PluginName: "memory", PluginVersion: "0.5.0", State: store.PluginStateEnabled},
				{PluginName: "bridge", PluginVersion: "0.1.0", State: store.PluginStateDisabled},
			}, nil
		},
	}
	reg := plugins.NewRegistry()
	lc := plugins.NewLifecycle(stub, reg)
	_ = catalogID

	if err := lc.LoadAll(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reg.Count() != 2 {
		t.Errorf("expected 2 enabled plugins in registry, got %d", reg.Count())
	}
	if _, ok := reg.Get("vault"); !ok {
		t.Error("expected 'vault' in registry")
	}
	if _, ok := reg.Get("memory"); !ok {
		t.Error("expected 'memory' in registry")
	}
	if _, ok := reg.Get("bridge"); ok {
		t.Error("expected 'bridge' NOT in registry (disabled)")
	}
}

func TestLifecycle_LoadAll_StoreError(t *testing.T) {
	stub := &stubLifecycleStore{
		listTenantPlugins: func(_ context.Context) ([]store.TenantPlugin, error) {
			return nil, errors.New("db unavailable")
		},
	}
	reg := plugins.NewRegistry()
	lc := plugins.NewLifecycle(stub, reg)

	err := lc.LoadAll(context.Background())
	if err == nil {
		t.Fatal("expected error from store, got nil")
	}
}

func TestLifecycle_RegisterPlugin_AddsToRegistry(t *testing.T) {
	reg := plugins.NewRegistry()
	lc := plugins.NewLifecycle(&stubLifecycleStore{}, reg)

	state := &plugins.RegistryEntry{
		Manifest:  &plugins.PluginManifest{Name: "vault", Version: "1.0.0"},
		CatalogID: uuid.New(),
		Status:    plugins.RegistryActive,
	}
	lc.RegisterPlugin("vault", state)

	got, ok := reg.Get("vault")
	if !ok {
		t.Fatal("expected 'vault' in registry after RegisterPlugin")
	}
	if got.Status != plugins.RegistryActive {
		t.Errorf("expected RegistryActive, got %q", got.Status)
	}
}

func TestLifecycle_UnregisterPlugin_RemovesFromRegistry(t *testing.T) {
	reg := plugins.NewRegistry()
	reg.Register("vault", &plugins.RegistryEntry{
		Manifest: &plugins.PluginManifest{Name: "vault"},
		Status:   plugins.RegistryActive,
	})
	lc := plugins.NewLifecycle(&stubLifecycleStore{}, reg)

	lc.UnregisterPlugin("vault")

	if _, ok := reg.Get("vault"); ok {
		t.Error("expected 'vault' to be removed from registry")
	}
}

func TestLifecycle_UnregisterPlugin_NonExistent_IsNoOp(t *testing.T) {
	reg := plugins.NewRegistry()
	lc := plugins.NewLifecycle(&stubLifecycleStore{}, reg)
	// Should not panic.
	lc.UnregisterPlugin("nonexistent")
}

func TestLifecycle_ActivePluginNames_ReturnsOnlyActive(t *testing.T) {
	reg := plugins.NewRegistry()
	reg.Register("vault", &plugins.RegistryEntry{Status: plugins.RegistryActive})
	reg.Register("memory", &plugins.RegistryEntry{Status: plugins.RegistryError})
	reg.Register("bridge", &plugins.RegistryEntry{Status: plugins.RegistryActive})

	lc := plugins.NewLifecycle(&stubLifecycleStore{}, reg)
	names := lc.ActivePluginNames()

	if len(names) != 2 {
		t.Errorf("expected 2 active plugins, got %d: %v", len(names), names)
	}
}

func TestLifecycle_Stop_ClearsRegistry(t *testing.T) {
	reg := plugins.NewRegistry()
	reg.Register("vault", &plugins.RegistryEntry{Status: plugins.RegistryActive})
	reg.Register("memory", &plugins.RegistryEntry{Status: plugins.RegistryActive})

	lc := plugins.NewLifecycle(&stubLifecycleStore{}, reg)
	lc.Stop()

	if reg.Count() != 0 {
		t.Errorf("expected empty registry after Stop, got %d plugins", reg.Count())
	}
}
