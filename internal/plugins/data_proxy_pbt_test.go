package plugins_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"pgregory.net/rapid"

	"github.com/vellus-ai/argoclaw/internal/plugins"
	"github.com/vellus-ai/argoclaw/internal/store"
)

// ─────────────────────────────────────────────────────────────────────────────
// In-memory KV store for PBT — simulates real DB isolation by plugin+tenant
// ─────────────────────────────────────────────────────────────────────────────

type inMemoryPluginStore struct {
	mu   sync.Mutex
	data map[string]json.RawMessage // key: "tenant:plugin:collection:key"
}

func newInMemoryPluginStore() *inMemoryPluginStore {
	return &inMemoryPluginStore{
		data: make(map[string]json.RawMessage),
	}
}

func (s *inMemoryPluginStore) compositeKey(tenantID uuid.UUID, plugin, collection, key string) string {
	return fmt.Sprintf("%s:%s:%s:%s", tenantID, plugin, collection, key)
}

func (s *inMemoryPluginStore) GetTenantPlugin(_ context.Context, name string) (*store.TenantPlugin, error) {
	// All plugins are considered installed+enabled for PBT.
	return &store.TenantPlugin{PluginName: name, State: store.PluginStateEnabled}, nil
}

func (s *inMemoryPluginStore) PutData(ctx context.Context, plugin, col, key string, val json.RawMessage, _ *time.Time) error {
	tenantID := store.TenantIDFromContext(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[s.compositeKey(tenantID, plugin, col, key)] = val
	return nil
}

func (s *inMemoryPluginStore) GetData(ctx context.Context, plugin, col, key string) (*store.PluginDataEntry, error) {
	tenantID := store.TenantIDFromContext(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	ck := s.compositeKey(tenantID, plugin, col, key)
	val, ok := s.data[ck]
	if !ok {
		return nil, store.ErrPluginNotFound
	}
	return &store.PluginDataEntry{
		TenantID:   tenantID,
		PluginName: plugin,
		Collection: col,
		Key:        key,
		Value:      val,
	}, nil
}

func (s *inMemoryPluginStore) ListDataKeys(ctx context.Context, plugin, col, prefix string, limit, offset int) ([]string, error) {
	return nil, nil
}

func (s *inMemoryPluginStore) DeleteData(ctx context.Context, plugin, col, key string) error {
	tenantID := store.TenantIDFromContext(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, s.compositeKey(tenantID, plugin, col, key))
	return nil
}

// Unimplemented methods — panic if called unexpectedly.
func (s *inMemoryPluginStore) UpsertCatalogEntry(_ context.Context, _ *store.PluginCatalogEntry) error {
	panic("not implemented")
}
func (s *inMemoryPluginStore) GetCatalogEntry(_ context.Context, _ uuid.UUID) (*store.PluginCatalogEntry, error) {
	panic("not implemented")
}
func (s *inMemoryPluginStore) GetCatalogEntryByName(_ context.Context, _ string) (*store.PluginCatalogEntry, error) {
	panic("not implemented")
}
func (s *inMemoryPluginStore) ListCatalog(_ context.Context) ([]store.PluginCatalogEntry, error) {
	panic("not implemented")
}
func (s *inMemoryPluginStore) InstallPlugin(_ context.Context, _ *store.TenantPlugin) error {
	panic("not implemented")
}
func (s *inMemoryPluginStore) EnablePlugin(_ context.Context, _ string, _ *uuid.UUID) error {
	panic("not implemented")
}
func (s *inMemoryPluginStore) DisablePlugin(_ context.Context, _ string, _ *uuid.UUID) error {
	panic("not implemented")
}
func (s *inMemoryPluginStore) UninstallPlugin(_ context.Context, _ string, _ *uuid.UUID) error {
	panic("not implemented")
}
func (s *inMemoryPluginStore) ListTenantPlugins(_ context.Context) ([]store.TenantPlugin, error) {
	panic("not implemented")
}
func (s *inMemoryPluginStore) UpdatePluginConfig(_ context.Context, _ string, _ json.RawMessage, _ *uuid.UUID) error {
	panic("not implemented")
}
func (s *inMemoryPluginStore) SetPluginError(_ context.Context, _, _ string) error {
	panic("not implemented")
}
func (s *inMemoryPluginStore) SetAgentPlugin(_ context.Context, _ *store.AgentPlugin) error {
	panic("not implemented")
}
func (s *inMemoryPluginStore) GetAgentPlugin(_ context.Context, _ uuid.UUID, _ string) (*store.AgentPlugin, error) {
	panic("not implemented")
}
func (s *inMemoryPluginStore) ListAgentPlugins(_ context.Context, _ uuid.UUID) ([]store.AgentPlugin, error) {
	panic("not implemented")
}
func (s *inMemoryPluginStore) IsPluginEnabledForAgent(_ context.Context, _ uuid.UUID, _ string) (bool, error) {
	panic("not implemented")
}
func (s *inMemoryPluginStore) DeleteCollectionData(_ context.Context, _, _ string) error {
	panic("not implemented")
}
func (s *inMemoryPluginStore) LogAudit(_ context.Context, _ *store.PluginAuditEntry) error {
	panic("not implemented")
}
func (s *inMemoryPluginStore) ListAuditLog(_ context.Context, _ string, _ int) ([]store.PluginAuditEntry, error) {
	panic("not implemented")
}

// ─────────────────────────────────────────────────────────────────────────────
// Generators
// ─────────────────────────────────────────────────────────────────────────────

// genProxyPluginName generates valid kebab-case plugin names (3-20 chars).
func genProxyPluginName() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		// Generate a prefix letter a-z.
		prefix := string(rune('a' + rapid.IntRange(0, 25).Draw(t, "prefix")))
		// Generate suffix of lowercase alnum.
		suffixLen := rapid.IntRange(2, 10).Draw(t, "suffixLen")
		chars := make([]byte, suffixLen)
		for i := range chars {
			if rapid.Bool().Draw(t, "digit") {
				chars[i] = byte('0' + rapid.IntRange(0, 9).Draw(t, "d"))
			} else {
				chars[i] = byte('a' + rapid.IntRange(0, 25).Draw(t, "c"))
			}
		}
		return prefix + string(chars)
	})
}

// genKey generates collection/key names (1-50 chars, alphanumeric).
func genKey() *rapid.Generator[string] {
	lowercaseRunes := []rune("abcdefghijklmnopqrstuvwxyz")
	return rapid.StringOfN(rapid.RuneFrom(lowercaseRunes), 1, 50, -1)
}

// ─────────────────────────────────────────────────────────────────────────────
// P10: Data written by plugin A is NOT accessible by plugin B (same tenant)
// ─────────────────────────────────────────────────────────────────────────────

func TestDataProxy_PBT_P10_CrossPluginIsolation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		pluginA := genProxyPluginName().Draw(t, "pluginA")
		pluginB := genProxyPluginName().Draw(t, "pluginB")

		// Ensure plugin names are different.
		if pluginA == pluginB {
			pluginB = pluginB + "x"
		}

		collection := genKey().Draw(t, "collection")
		key := genKey().Draw(t, "key")
		lowercaseRunes := []rune("abcdefghijklmnopqrstuvwxyz")
		value := json.RawMessage(fmt.Sprintf(`"value-%s"`, rapid.StringOfN(rapid.RuneFrom(lowercaseRunes), 1, 20, -1).Draw(t, "value")))

		tenantID := uuid.New()
		ctx := store.WithTenantID(context.Background(), tenantID)

		memStore := newInMemoryPluginStore()
		proxy := plugins.NewDataProxy(memStore)

		// Plugin A writes data.
		err := proxy.Put(ctx, pluginA, collection, key, value, nil)
		if err != nil {
			t.Fatalf("plugin A put failed: %v", err)
		}

		// Plugin A can read its own data.
		entry, err := proxy.Get(ctx, pluginA, collection, key)
		if err != nil {
			t.Fatalf("plugin A get own data failed: %v", err)
		}
		if string(entry.Value) != string(value) {
			t.Fatalf("plugin A value mismatch: got %s, want %s", entry.Value, value)
		}

		// Plugin B CANNOT read plugin A's data.
		_, err = proxy.Get(ctx, pluginB, collection, key)
		if err == nil {
			t.Fatalf("ISOLATION VIOLATION: plugin %q read data written by plugin %q (collection=%q, key=%q)",
				pluginB, pluginA, collection, key)
		}
		if !errors.Is(err, store.ErrPluginNotFound) {
			t.Fatalf("expected ErrPluginNotFound for cross-plugin read, got: %v", err)
		}
	})
}
