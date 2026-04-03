package plugins_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/vellus-ai/argoclaw/internal/plugins"
	"github.com/vellus-ai/argoclaw/internal/store"
)

// ─────────────────────────────────────────────────────────────────────────────
// Minimal stub implementing store.PluginStore (only data-proxy-relevant methods)
// ─────────────────────────────────────────────────────────────────────────────

type stubPluginStoreDP struct {
	getTenantPlugin func(ctx context.Context, name string) (*store.TenantPlugin, error)
	putData         func(ctx context.Context, plugin, col, key string, val json.RawMessage, exp *time.Time) error
	getData         func(ctx context.Context, plugin, col, key string) (*store.PluginDataEntry, error)
	listDataKeys    func(ctx context.Context, plugin, col, prefix string, limit, offset int) ([]string, error)
	deleteData      func(ctx context.Context, plugin, col, key string) error
}

// installedStub returns a stub that reports a plugin as installed.
func installedStub(name string) *stubPluginStoreDP {
	return &stubPluginStoreDP{
		getTenantPlugin: func(ctx context.Context, n string) (*store.TenantPlugin, error) {
			if n == name {
				return &store.TenantPlugin{PluginName: n, State: store.PluginStateEnabled}, nil
			}
			return nil, store.ErrPluginNotFound
		},
	}
}

// notInstalledStub returns a stub that reports all plugins as not installed.
func notInstalledStub() *stubPluginStoreDP {
	return &stubPluginStoreDP{
		getTenantPlugin: func(_ context.Context, _ string) (*store.TenantPlugin, error) {
			return nil, store.ErrPluginNotFound
		},
	}
}

// Implement the full store.PluginStore interface (unused methods panic).
func (s *stubPluginStoreDP) GetTenantPlugin(ctx context.Context, n string) (*store.TenantPlugin, error) {
	return s.getTenantPlugin(ctx, n)
}
func (s *stubPluginStoreDP) PutData(ctx context.Context, plugin, col, key string, val json.RawMessage, exp *time.Time) error {
	if s.putData != nil {
		return s.putData(ctx, plugin, col, key, val, exp)
	}
	return nil
}
func (s *stubPluginStoreDP) GetData(ctx context.Context, plugin, col, key string) (*store.PluginDataEntry, error) {
	if s.getData != nil {
		return s.getData(ctx, plugin, col, key)
	}
	return &store.PluginDataEntry{}, nil
}
func (s *stubPluginStoreDP) ListDataKeys(ctx context.Context, plugin, col, prefix string, limit, offset int) ([]string, error) {
	if s.listDataKeys != nil {
		return s.listDataKeys(ctx, plugin, col, prefix, limit, offset)
	}
	return nil, nil
}
func (s *stubPluginStoreDP) DeleteData(ctx context.Context, plugin, col, key string) error {
	if s.deleteData != nil {
		return s.deleteData(ctx, plugin, col, key)
	}
	return nil
}

// Unimplemented interface methods — panic if called unexpectedly.
func (s *stubPluginStoreDP) UpsertCatalogEntry(_ context.Context, _ *store.PluginCatalogEntry) error {
	panic("not implemented")
}
func (s *stubPluginStoreDP) GetCatalogEntry(_ context.Context, _ uuid.UUID) (*store.PluginCatalogEntry, error) {
	panic("not implemented")
}
func (s *stubPluginStoreDP) GetCatalogEntryByName(_ context.Context, _ string) (*store.PluginCatalogEntry, error) {
	panic("not implemented")
}
func (s *stubPluginStoreDP) ListCatalog(_ context.Context) ([]store.PluginCatalogEntry, error) {
	panic("not implemented")
}
func (s *stubPluginStoreDP) InstallPlugin(_ context.Context, _ *store.TenantPlugin) error {
	panic("not implemented")
}
func (s *stubPluginStoreDP) EnablePlugin(_ context.Context, _ string, _ *uuid.UUID) error {
	panic("not implemented")
}
func (s *stubPluginStoreDP) DisablePlugin(_ context.Context, _ string, _ *uuid.UUID) error {
	panic("not implemented")
}
func (s *stubPluginStoreDP) UninstallPlugin(_ context.Context, _ string, _ *uuid.UUID) error {
	panic("not implemented")
}
func (s *stubPluginStoreDP) ListTenantPlugins(_ context.Context) ([]store.TenantPlugin, error) {
	panic("not implemented")
}
func (s *stubPluginStoreDP) UpdatePluginConfig(_ context.Context, _ string, _ json.RawMessage, _ *uuid.UUID) error {
	panic("not implemented")
}
func (s *stubPluginStoreDP) SetPluginError(_ context.Context, _, _ string) error {
	panic("not implemented")
}
func (s *stubPluginStoreDP) SetAgentPlugin(_ context.Context, _ *store.AgentPlugin) error {
	panic("not implemented")
}
func (s *stubPluginStoreDP) GetAgentPlugin(_ context.Context, _ uuid.UUID, _ string) (*store.AgentPlugin, error) {
	panic("not implemented")
}
func (s *stubPluginStoreDP) ListAgentPlugins(_ context.Context, _ uuid.UUID) ([]store.AgentPlugin, error) {
	panic("not implemented")
}
func (s *stubPluginStoreDP) IsPluginEnabledForAgent(_ context.Context, _ uuid.UUID, _ string) (bool, error) {
	panic("not implemented")
}
func (s *stubPluginStoreDP) DeleteCollectionData(_ context.Context, _, _ string) error {
	panic("not implemented")
}
func (s *stubPluginStoreDP) LogAudit(_ context.Context, _ *store.PluginAuditEntry) error {
	panic("not implemented")
}
func (s *stubPluginStoreDP) ListAuditLog(_ context.Context, _ string, _ int) ([]store.PluginAuditEntry, error) {
	panic("not implemented")
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func tenantCtxDP(tenantID uuid.UUID) context.Context {
	return store.WithTenantID(context.Background(), tenantID)
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests — written BEFORE implementation (TDD Red phase)
// ─────────────────────────────────────────────────────────────────────────────

func TestDataProxy_Put_NoTenantInContext(t *testing.T) {
	proxy := plugins.NewDataProxy(installedStub("vault"))
	err := proxy.Put(context.Background(), "vault", "prompts", "key1", json.RawMessage(`"val"`), nil)
	if !errors.Is(err, plugins.ErrMissingTenantContext) {
		t.Fatalf("expected ErrMissingTenantContext, got %v", err)
	}
}

func TestDataProxy_Put_KeyTooLong(t *testing.T) {
	ctx := tenantCtxDP(uuid.New())
	proxy := plugins.NewDataProxy(installedStub("vault"))
	longKey := strings.Repeat("k", 501)
	err := proxy.Put(ctx, "vault", "prompts", longKey, json.RawMessage(`"val"`), nil)
	if !errors.Is(err, plugins.ErrKeyTooLong) {
		t.Fatalf("expected ErrKeyTooLong, got %v", err)
	}
}

func TestDataProxy_Put_KeyAtMaxLength(t *testing.T) {
	ctx := tenantCtxDP(uuid.New())
	stub := installedStub("vault")
	stub.putData = func(_ context.Context, _, _, _ string, _ json.RawMessage, _ *time.Time) error { return nil }
	proxy := plugins.NewDataProxy(stub)
	maxKey := strings.Repeat("k", 500)
	err := proxy.Put(ctx, "vault", "prompts", maxKey, json.RawMessage(`"val"`), nil)
	if err != nil {
		t.Fatalf("unexpected error for key at max length: %v", err)
	}
}

func TestDataProxy_Put_CollectionTooLong(t *testing.T) {
	ctx := tenantCtxDP(uuid.New())
	proxy := plugins.NewDataProxy(installedStub("vault"))
	longCol := strings.Repeat("c", 101)
	err := proxy.Put(ctx, "vault", longCol, "key1", json.RawMessage(`"val"`), nil)
	if !errors.Is(err, plugins.ErrCollectionTooLong) {
		t.Fatalf("expected ErrCollectionTooLong, got %v", err)
	}
}

func TestDataProxy_Put_CollectionAtMaxLength(t *testing.T) {
	ctx := tenantCtxDP(uuid.New())
	stub := installedStub("vault")
	stub.putData = func(_ context.Context, _, _, _ string, _ json.RawMessage, _ *time.Time) error { return nil }
	proxy := plugins.NewDataProxy(stub)
	maxCol := strings.Repeat("c", 100)
	err := proxy.Put(ctx, "vault", maxCol, "key1", json.RawMessage(`"val"`), nil)
	if err != nil {
		t.Fatalf("unexpected error for collection at max length: %v", err)
	}
}

func TestDataProxy_Put_PluginNotInstalled(t *testing.T) {
	ctx := tenantCtxDP(uuid.New())
	proxy := plugins.NewDataProxy(notInstalledStub())
	err := proxy.Put(ctx, "vault", "prompts", "key1", json.RawMessage(`"val"`), nil)
	if !errors.Is(err, plugins.ErrPluginNotInstalled) {
		t.Fatalf("expected ErrPluginNotInstalled, got %v", err)
	}
}

func TestDataProxy_Put_HappyPath(t *testing.T) {
	ctx := tenantCtxDP(uuid.New())
	stub := installedStub("vault")
	var captured struct {
		plugin, col, key string
		val               json.RawMessage
	}
	stub.putData = func(_ context.Context, plugin, col, key string, val json.RawMessage, _ *time.Time) error {
		captured.plugin = plugin
		captured.col = col
		captured.key = key
		captured.val = val
		return nil
	}
	proxy := plugins.NewDataProxy(stub)
	err := proxy.Put(ctx, "vault", "prompts", "my-key", json.RawMessage(`"hello"`), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured.plugin != "vault" || captured.col != "prompts" || captured.key != "my-key" {
		t.Errorf("unexpected capture: %+v", captured)
	}
}

func TestDataProxy_Get_NoTenantInContext(t *testing.T) {
	proxy := plugins.NewDataProxy(installedStub("vault"))
	_, err := proxy.Get(context.Background(), "vault", "prompts", "key1")
	if !errors.Is(err, plugins.ErrMissingTenantContext) {
		t.Fatalf("expected ErrMissingTenantContext, got %v", err)
	}
}

func TestDataProxy_Get_PluginNotInstalled(t *testing.T) {
	ctx := tenantCtxDP(uuid.New())
	proxy := plugins.NewDataProxy(notInstalledStub())
	_, err := proxy.Get(ctx, "vault", "prompts", "key1")
	if !errors.Is(err, plugins.ErrPluginNotInstalled) {
		t.Fatalf("expected ErrPluginNotInstalled, got %v", err)
	}
}

func TestDataProxy_Get_HappyPath(t *testing.T) {
	ctx := tenantCtxDP(uuid.New())
	wantEntry := &store.PluginDataEntry{Key: "key1", Value: json.RawMessage(`"hello"`)}
	stub := installedStub("vault")
	stub.getData = func(_ context.Context, _, _, _ string) (*store.PluginDataEntry, error) {
		return wantEntry, nil
	}
	proxy := plugins.NewDataProxy(stub)
	got, err := proxy.Get(ctx, "vault", "prompts", "key1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != wantEntry {
		t.Errorf("expected same entry pointer")
	}
}

func TestDataProxy_Delete_NoTenantInContext(t *testing.T) {
	proxy := plugins.NewDataProxy(installedStub("vault"))
	err := proxy.Delete(context.Background(), "vault", "prompts", "key1")
	if !errors.Is(err, plugins.ErrMissingTenantContext) {
		t.Fatalf("expected ErrMissingTenantContext, got %v", err)
	}
}

func TestDataProxy_Delete_PluginNotInstalled(t *testing.T) {
	ctx := tenantCtxDP(uuid.New())
	proxy := plugins.NewDataProxy(notInstalledStub())
	err := proxy.Delete(ctx, "vault", "prompts", "key1")
	if !errors.Is(err, plugins.ErrPluginNotInstalled) {
		t.Fatalf("expected ErrPluginNotInstalled, got %v", err)
	}
}

func TestDataProxy_Delete_HappyPath(t *testing.T) {
	ctx := tenantCtxDP(uuid.New())
	called := false
	stub := installedStub("vault")
	stub.deleteData = func(_ context.Context, _, _, _ string) error {
		called = true
		return nil
	}
	proxy := plugins.NewDataProxy(stub)
	err := proxy.Delete(ctx, "vault", "prompts", "key1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected store.DeleteData to be called")
	}
}

func TestDataProxy_ListKeys_NoTenantInContext(t *testing.T) {
	proxy := plugins.NewDataProxy(installedStub("vault"))
	_, err := proxy.ListKeys(context.Background(), "vault", "prompts", "", 10, 0)
	if !errors.Is(err, plugins.ErrMissingTenantContext) {
		t.Fatalf("expected ErrMissingTenantContext, got %v", err)
	}
}

func TestDataProxy_ListKeys_CollectionTooLong(t *testing.T) {
	ctx := tenantCtxDP(uuid.New())
	proxy := plugins.NewDataProxy(installedStub("vault"))
	longCol := strings.Repeat("c", 101)
	_, err := proxy.ListKeys(ctx, "vault", longCol, "", 10, 0)
	if !errors.Is(err, plugins.ErrCollectionTooLong) {
		t.Fatalf("expected ErrCollectionTooLong, got %v", err)
	}
}

func TestDataProxy_ListKeys_HappyPath(t *testing.T) {
	ctx := tenantCtxDP(uuid.New())
	stub := installedStub("vault")
	stub.listDataKeys = func(_ context.Context, _, _, _ string, _, _ int) ([]string, error) {
		return []string{"key1", "key2"}, nil
	}
	proxy := plugins.NewDataProxy(stub)
	keys, err := proxy.ListKeys(ctx, "vault", "prompts", "", 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

// TestDataProxy_CrossTenant verifies that tenantID is always taken from context.
// Even if the same DataProxy is used with different tenant contexts,
// the store is always called with the correct context tenant.
func TestDataProxy_CrossTenant(t *testing.T) {
	tenantA := uuid.New()
	tenantB := uuid.New()

	// Both tenants have the plugin installed.
	stub := &stubPluginStoreDP{
		getTenantPlugin: func(ctx context.Context, name string) (*store.TenantPlugin, error) {
			return &store.TenantPlugin{PluginName: name, State: store.PluginStateEnabled}, nil
		},
	}

	var capturedCtxTenantID uuid.UUID
	stub.putData = func(ctx context.Context, _, _, _ string, _ json.RawMessage, _ *time.Time) error {
		capturedCtxTenantID = store.TenantIDFromContext(ctx)
		return nil
	}

	proxy := plugins.NewDataProxy(stub)

	// Use with tenant A context
	ctxA := tenantCtxDP(tenantA)
	if err := proxy.Put(ctxA, "vault", "prompts", "key", json.RawMessage(`1`), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedCtxTenantID != tenantA {
		t.Errorf("store called with wrong tenant: got %v, want %v", capturedCtxTenantID, tenantA)
	}

	// Use with tenant B context
	ctxB := tenantCtxDP(tenantB)
	if err := proxy.Put(ctxB, "vault", "prompts", "key", json.RawMessage(`2`), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedCtxTenantID != tenantB {
		t.Errorf("store called with wrong tenant: got %v, want %v", capturedCtxTenantID, tenantB)
	}
}
