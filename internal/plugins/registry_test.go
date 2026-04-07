package plugins_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"pgregory.net/rapid"

	"github.com/vellus-ai/argoclaw/internal/plugins"
	"github.com/vellus-ai/argoclaw/internal/store"
)

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func makeEntry(name string, status plugins.RegistryStatus) *plugins.RegistryEntry {
	return &plugins.RegistryEntry{
		Manifest: &plugins.PluginManifest{
			Metadata: plugins.ManifestMetadata{
				Name:    name,
				Version: "1.0.0",
			},
			Name:    name,
			Version: "1.0.0",
		},
		CatalogID: uuid.New(),
		Status:    status,
		Tools:     []string{fmt.Sprintf("plugin_%s__tool1", name)},
		EnabledAt: time.Now(),
	}
}

// mockPluginStore implements the subset of store.PluginStore needed by LoadFromDB.
// Only ListTenantPlugins and GetCatalogEntryByName are exercised.
type mockPluginStore struct {
	tenantPlugins    []store.TenantPlugin
	catalogEntries   map[string]*store.PluginCatalogEntry
	listErr          error
	catalogErr       error
	listCallCount    int
	catalogCallCount int
	mu               sync.Mutex
}

func newMockPluginStore() *mockPluginStore {
	return &mockPluginStore{
		catalogEntries: make(map[string]*store.PluginCatalogEntry),
	}
}

func (m *mockPluginStore) UpsertCatalogEntry(_ context.Context, _ *store.PluginCatalogEntry) error {
	return nil
}
func (m *mockPluginStore) GetCatalogEntry(_ context.Context, _ uuid.UUID) (*store.PluginCatalogEntry, error) {
	return nil, nil
}

func (m *mockPluginStore) GetCatalogEntryByName(_ context.Context, name string) (*store.PluginCatalogEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.catalogCallCount++
	if m.catalogErr != nil {
		return nil, m.catalogErr
	}
	entry, ok := m.catalogEntries[name]
	if !ok {
		return nil, store.ErrPluginNotFound
	}
	return entry, nil
}

func (m *mockPluginStore) ListCatalog(_ context.Context) ([]store.PluginCatalogEntry, error) {
	return nil, nil
}
func (m *mockPluginStore) InstallPlugin(_ context.Context, _ *store.TenantPlugin) error {
	return nil
}
func (m *mockPluginStore) EnablePlugin(_ context.Context, _ string, _ *uuid.UUID) error {
	return nil
}
func (m *mockPluginStore) DisablePlugin(_ context.Context, _ string, _ *uuid.UUID) error {
	return nil
}
func (m *mockPluginStore) UninstallPlugin(_ context.Context, _ string, _ *uuid.UUID) error {
	return nil
}
func (m *mockPluginStore) GetTenantPlugin(_ context.Context, _ string) (*store.TenantPlugin, error) {
	return nil, nil
}

func (m *mockPluginStore) ListTenantPlugins(_ context.Context) ([]store.TenantPlugin, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listCallCount++
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.tenantPlugins, nil
}

func (m *mockPluginStore) UpdatePluginConfig(_ context.Context, _ string, _ json.RawMessage, _ *uuid.UUID) error {
	return nil
}
func (m *mockPluginStore) SetPluginError(_ context.Context, _, _ string) error {
	return nil
}
func (m *mockPluginStore) SetAgentPlugin(_ context.Context, _ *store.AgentPlugin) error {
	return nil
}
func (m *mockPluginStore) GetAgentPlugin(_ context.Context, _ uuid.UUID, _ string) (*store.AgentPlugin, error) {
	return nil, nil
}
func (m *mockPluginStore) ListAgentPlugins(_ context.Context, _ uuid.UUID) ([]store.AgentPlugin, error) {
	return nil, nil
}
func (m *mockPluginStore) IsPluginEnabledForAgent(_ context.Context, _ uuid.UUID, _ string) (bool, error) {
	return false, nil
}
func (m *mockPluginStore) PutData(_ context.Context, _, _, _ string, _ json.RawMessage, _ *time.Time) error {
	return nil
}
func (m *mockPluginStore) GetData(_ context.Context, _, _, _ string) (*store.PluginDataEntry, error) {
	return nil, nil
}
func (m *mockPluginStore) ListDataKeys(_ context.Context, _, _, _ string, _, _ int) ([]string, error) {
	return nil, nil
}
func (m *mockPluginStore) DeleteData(_ context.Context, _, _, _ string) error {
	return nil
}
func (m *mockPluginStore) DeleteCollectionData(_ context.Context, _, _ string) error {
	return nil
}
func (m *mockPluginStore) LogAudit(_ context.Context, _ *store.PluginAuditEntry) error {
	return nil
}
func (m *mockPluginStore) ListAuditLog(_ context.Context, _ string, _ int) ([]store.PluginAuditEntry, error) {
	return nil, nil
}

// makeCatalogEntry creates a store.PluginCatalogEntry with a valid manifest JSON.
func makeCatalogEntry(name, version string) *store.PluginCatalogEntry {
	manifest := fmt.Sprintf(`{"metadata":{"name":"%s","version":"%s","manifestVersion":"1.0","description":"test","author":"test"},"spec":{"type":"tool","runtime":{"transport":"stdio","command":"./server"},"permissions":{"tools":{"provide":["tool1"]}}}}`, name, version)
	return &store.PluginCatalogEntry{
		BaseModel: store.BaseModel{ID: uuid.New()},
		Name:      name,
		Version:   version,
		Manifest:  json.RawMessage(manifest),
		Source:    "builtin",
		MinPlan:   "starter",
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Table-driven tests: Register / Get
// Validates: Requirements 14.1, 14.2, 14.5
// ─────────────────────────────────────────────────────────────────────────────

func TestRegistry_Register_Get(t *testing.T) {
	tests := []struct {
		name       string
		register   map[string]plugins.RegistryStatus // name → status to register
		getKey     string
		wantFound  bool
		wantStatus plugins.RegistryStatus
	}{
		{
			name:       "get registered plugin returns entry",
			register:   map[string]plugins.RegistryStatus{"vault": plugins.RegistryActive},
			getKey:     "vault",
			wantFound:  true,
			wantStatus: plugins.RegistryActive,
		},
		{
			name:      "get nonexistent plugin returns false",
			register:  map[string]plugins.RegistryStatus{},
			getKey:    "nonexistent",
			wantFound: false,
		},
		{
			name: "get one of many registered plugins",
			register: map[string]plugins.RegistryStatus{
				"vault":  plugins.RegistryActive,
				"memory": plugins.RegistryDisabled,
				"bridge": plugins.RegistryError,
			},
			getKey:     "memory",
			wantFound:  true,
			wantStatus: plugins.RegistryDisabled,
		},
		{
			name:       "register overwrites existing entry",
			register:   map[string]plugins.RegistryStatus{"vault": plugins.RegistryError},
			getKey:     "vault",
			wantFound:  true,
			wantStatus: plugins.RegistryError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := plugins.NewRegistry()
			for name, status := range tt.register {
				r.Register(name, makeEntry(name, status))
			}
			// For the "overwrites" case, register again with the target status.
			if tt.name == "register overwrites existing entry" {
				first := makeEntry("vault", plugins.RegistryActive)
				r.Register("vault", first)
				second := makeEntry("vault", plugins.RegistryError)
				r.Register("vault", second)
			}

			got, ok := r.Get(tt.getKey)
			if ok != tt.wantFound {
				t.Fatalf("Get(%q): found=%v, want %v", tt.getKey, ok, tt.wantFound)
			}
			if tt.wantFound && got.Status != tt.wantStatus {
				t.Errorf("Get(%q).Status = %q, want %q", tt.getKey, got.Status, tt.wantStatus)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Table-driven tests: Unregister
// Validates: Requirements 14.2
// ─────────────────────────────────────────────────────────────────────────────

func TestRegistry_Unregister(t *testing.T) {
	tests := []struct {
		name         string
		register     []string
		unregister   string
		wantCount    int
		wantGone     string
		wantPresent  []string
	}{
		{
			name:        "unregister existing plugin removes it",
			register:    []string{"vault", "memory"},
			unregister:  "vault",
			wantCount:   1,
			wantGone:    "vault",
			wantPresent: []string{"memory"},
		},
		{
			name:        "unregister nonexistent is no-op",
			register:    []string{"vault"},
			unregister:  "nonexistent",
			wantCount:   1,
			wantPresent: []string{"vault"},
		},
		{
			name:       "unregister last plugin leaves empty registry",
			register:   []string{"vault"},
			unregister: "vault",
			wantCount:  0,
			wantGone:   "vault",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := plugins.NewRegistry()
			for _, name := range tt.register {
				r.Register(name, makeEntry(name, plugins.RegistryActive))
			}

			r.Unregister(tt.unregister)

			if r.Count() != tt.wantCount {
				t.Errorf("Count() = %d, want %d", r.Count(), tt.wantCount)
			}
			if tt.wantGone != "" {
				if _, ok := r.Get(tt.wantGone); ok {
					t.Errorf("expected %q to be gone after Unregister", tt.wantGone)
				}
			}
			for _, name := range tt.wantPresent {
				if _, ok := r.Get(name); !ok {
					t.Errorf("expected %q to still be present", name)
				}
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Table-driven tests: List
// Validates: Requirements 14.2, 14.5
// ─────────────────────────────────────────────────────────────────────────────

func TestRegistry_List(t *testing.T) {
	tests := []struct {
		name      string
		register  map[string]plugins.RegistryStatus
		wantCount int
	}{
		{
			name:      "empty registry returns empty slice",
			register:  map[string]plugins.RegistryStatus{},
			wantCount: 0,
		},
		{
			name:      "single plugin",
			register:  map[string]plugins.RegistryStatus{"vault": plugins.RegistryActive},
			wantCount: 1,
		},
		{
			name: "multiple plugins with mixed statuses",
			register: map[string]plugins.RegistryStatus{
				"vault":  plugins.RegistryActive,
				"memory": plugins.RegistryDisabled,
				"bridge": plugins.RegistryError,
			},
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := plugins.NewRegistry()
			for name, status := range tt.register {
				r.Register(name, makeEntry(name, status))
			}

			list := r.List()
			if list == nil {
				t.Fatal("List() returned nil, expected non-nil slice")
			}
			if len(list) != tt.wantCount {
				t.Errorf("List() len = %d, want %d", len(list), tt.wantCount)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Table-driven tests: ActiveNames
// Validates: Requirements 14.2
// ─────────────────────────────────────────────────────────────────────────────

func TestRegistry_ActiveNames(t *testing.T) {
	tests := []struct {
		name      string
		register  map[string]plugins.RegistryStatus
		wantNames []string
	}{
		{
			name:      "empty registry returns empty slice",
			register:  map[string]plugins.RegistryStatus{},
			wantNames: nil,
		},
		{
			name:      "all active",
			register:  map[string]plugins.RegistryStatus{"vault": plugins.RegistryActive, "memory": plugins.RegistryActive},
			wantNames: []string{"memory", "vault"},
		},
		{
			name: "mixed statuses returns only active",
			register: map[string]plugins.RegistryStatus{
				"vault":  plugins.RegistryActive,
				"memory": plugins.RegistryDisabled,
				"bridge": plugins.RegistryError,
				"search": plugins.RegistryActive,
			},
			wantNames: []string{"search", "vault"},
		},
		{
			name: "no active plugins returns empty",
			register: map[string]plugins.RegistryStatus{
				"vault":  plugins.RegistryDisabled,
				"memory": plugins.RegistryError,
			},
			wantNames: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := plugins.NewRegistry()
			for name, status := range tt.register {
				r.Register(name, makeEntry(name, status))
			}

			got := r.ActiveNames()
			sort.Strings(got)
			sort.Strings(tt.wantNames)

			if len(got) != len(tt.wantNames) {
				t.Fatalf("ActiveNames() = %v, want %v", got, tt.wantNames)
			}
			for i := range got {
				if got[i] != tt.wantNames[i] {
					t.Errorf("ActiveNames()[%d] = %q, want %q", i, got[i], tt.wantNames[i])
				}
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Table-driven tests: Count
// Validates: Requirements 14.2
// ─────────────────────────────────────────────────────────────────────────────

func TestRegistry_Count(t *testing.T) {
	tests := []struct {
		name      string
		register  []string
		wantCount int
	}{
		{
			name:      "empty registry",
			register:  nil,
			wantCount: 0,
		},
		{
			name:      "one plugin",
			register:  []string{"vault"},
			wantCount: 1,
		},
		{
			name:      "three plugins",
			register:  []string{"vault", "memory", "bridge"},
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := plugins.NewRegistry()
			for _, name := range tt.register {
				r.Register(name, makeEntry(name, plugins.RegistryActive))
			}
			if r.Count() != tt.wantCount {
				t.Errorf("Count() = %d, want %d", r.Count(), tt.wantCount)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Concurrency tests (race detector)
// Validates: Requirements 14.1, 14.6
// ─────────────────────────────────────────────────────────────────────────────

func TestRegistry_ConcurrentRegisterAndGet(t *testing.T) {
	r := plugins.NewRegistry()
	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// Half goroutines write, half read — exercises RWMutex under contention.
	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("plugin-%d", i)
			r.Register(name, makeEntry(name, plugins.RegistryActive))
		}(i)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("plugin-%d", i)
			entry, ok := r.Get(name)
			// Entry may or may not be registered yet — both are valid.
			// But if found, it must not be a partial/corrupt state.
			if ok && entry.Manifest == nil {
				t.Errorf("Get(%q) returned entry with nil Manifest (partial state)", name)
			}
		}(i)
	}

	wg.Wait()
}

func TestRegistry_ConcurrentMixedOperations(t *testing.T) {
	r := plugins.NewRegistry()
	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines * 4)

	for i := 0; i < goroutines; i++ {
		name := fmt.Sprintf("plugin-%d", i)

		go func() {
			defer wg.Done()
			r.Register(name, makeEntry(name, plugins.RegistryActive))
		}()
		go func() {
			defer wg.Done()
			r.Get(name)
		}()
		go func() {
			defer wg.Done()
			r.List()
			r.ActiveNames()
			r.Count()
		}()
		go func() {
			defer wg.Done()
			r.Unregister(name)
		}()
	}

	wg.Wait()
	// If we reach here without data race or panic, the test passes.
}

// ─────────────────────────────────────────────────────────────────────────────
// Table-driven tests: LoadFromDB
// Validates: Requirements 14.3
// ─────────────────────────────────────────────────────────────────────────────

func TestRegistry_LoadFromDB(t *testing.T) {
	tenantID := uuid.New()
	now := time.Now()

	tests := []struct {
		name           string
		tenantPlugins  []store.TenantPlugin
		catalogEntries map[string]*store.PluginCatalogEntry
		listErr        error
		catalogErr     error
		wantErr        bool
		wantCount      int
		wantActive     []string
	}{
		{
			name:          "no enabled plugins results in empty registry",
			tenantPlugins: nil,
			wantErr:       false,
			wantCount:     0,
		},
		{
			name: "loads only enabled plugins from DB",
			tenantPlugins: []store.TenantPlugin{
				{
					ID:            uuid.New(),
					TenantID:      tenantID,
					PluginName:    "vault",
					PluginVersion: "1.0.0",
					State:         "enabled",
					EnabledAt:     &now,
					Config:        json.RawMessage(`{}`),
					Permissions:   json.RawMessage(`{}`),
				},
				{
					ID:            uuid.New(),
					TenantID:      tenantID,
					PluginName:    "disabled-plugin",
					PluginVersion: "1.0.0",
					State:         "disabled",
					Config:        json.RawMessage(`{}`),
					Permissions:   json.RawMessage(`{}`),
				},
				{
					ID:            uuid.New(),
					TenantID:      tenantID,
					PluginName:    "search",
					PluginVersion: "2.0.0",
					State:         "enabled",
					EnabledAt:     &now,
					Config:        json.RawMessage(`{}`),
					Permissions:   json.RawMessage(`{}`),
				},
			},
			catalogEntries: map[string]*store.PluginCatalogEntry{
				"vault":  makeCatalogEntry("vault", "1.0.0"),
				"search": makeCatalogEntry("search", "2.0.0"),
			},
			wantErr:    false,
			wantCount:  2,
			wantActive: []string{"search", "vault"},
		},
		{
			name:    "store ListTenantPlugins error propagates",
			listErr: fmt.Errorf("database connection lost"),
			wantErr: true,
		},
		{
			name: "catalog entry not found logs warning but continues",
			tenantPlugins: []store.TenantPlugin{
				{
					ID:            uuid.New(),
					TenantID:      tenantID,
					PluginName:    "vault",
					PluginVersion: "1.0.0",
					State:         "enabled",
					EnabledAt:     &now,
					Config:        json.RawMessage(`{}`),
					Permissions:   json.RawMessage(`{}`),
				},
			},
			catalogEntries: map[string]*store.PluginCatalogEntry{},
			wantErr:        false,
			wantCount:      0,
		},
		{
			name: "installed-only plugins are skipped",
			tenantPlugins: []store.TenantPlugin{
				{
					ID:            uuid.New(),
					TenantID:      tenantID,
					PluginName:    "vault",
					PluginVersion: "1.0.0",
					State:         "installed",
					Config:        json.RawMessage(`{}`),
					Permissions:   json.RawMessage(`{}`),
				},
			},
			wantErr:   false,
			wantCount: 0,
		},
		{
			name: "error-state plugins are skipped",
			tenantPlugins: []store.TenantPlugin{
				{
					ID:            uuid.New(),
					TenantID:      tenantID,
					PluginName:    "vault",
					PluginVersion: "1.0.0",
					State:         "error",
					Config:        json.RawMessage(`{}`),
					Permissions:   json.RawMessage(`{}`),
				},
			},
			wantErr:   false,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := plugins.NewRegistry()
			ms := newMockPluginStore()
			ms.tenantPlugins = tt.tenantPlugins
			ms.listErr = tt.listErr
			ms.catalogErr = tt.catalogErr
			if tt.catalogEntries != nil {
				ms.catalogEntries = tt.catalogEntries
			}

			ctx := store.WithTenantID(context.Background(), tenantID)
			err := r.LoadFromDB(ctx, ms)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if r.Count() != tt.wantCount {
				t.Errorf("Count() = %d, want %d", r.Count(), tt.wantCount)
			}

			if tt.wantActive != nil {
				got := r.ActiveNames()
				sort.Strings(got)
				sort.Strings(tt.wantActive)
				if len(got) != len(tt.wantActive) {
					t.Fatalf("ActiveNames() = %v, want %v", got, tt.wantActive)
				}
				for i := range got {
					if got[i] != tt.wantActive[i] {
						t.Errorf("ActiveNames()[%d] = %q, want %q", i, got[i], tt.wantActive[i])
					}
				}
			}
		})
	}
}

// TestRegistry_LoadFromDB_PopulatesEntryFields verifies that LoadFromDB
// correctly populates RegistryEntry fields from the store data.
// Validates: Requirements 14.3, 14.5
func TestRegistry_LoadFromDB_PopulatesEntryFields(t *testing.T) {
	tenantID := uuid.New()
	now := time.Now()
	catalogID := uuid.New()

	ms := newMockPluginStore()
	ms.tenantPlugins = []store.TenantPlugin{
		{
			ID:            uuid.New(),
			TenantID:      tenantID,
			PluginName:    "vault",
			PluginVersion: "1.0.0",
			State:         "enabled",
			EnabledAt:     &now,
			Config:        json.RawMessage(`{}`),
			Permissions:   json.RawMessage(`{}`),
		},
	}
	ce := makeCatalogEntry("vault", "1.0.0")
	ce.BaseModel.ID = catalogID
	ms.catalogEntries["vault"] = ce

	r := plugins.NewRegistry()
	ctx := store.WithTenantID(context.Background(), tenantID)
	if err := r.LoadFromDB(ctx, ms); err != nil {
		t.Fatalf("LoadFromDB: %v", err)
	}

	entry, ok := r.Get("vault")
	if !ok {
		t.Fatal("expected 'vault' in registry after LoadFromDB")
	}

	if entry.Status != plugins.RegistryActive {
		t.Errorf("Status = %q, want %q", entry.Status, plugins.RegistryActive)
	}
	if entry.CatalogID != catalogID {
		t.Errorf("CatalogID = %v, want %v", entry.CatalogID, catalogID)
	}
	if entry.Manifest == nil {
		t.Error("Manifest is nil, expected parsed manifest")
	}
	if entry.EnabledAt.IsZero() {
		t.Error("EnabledAt is zero, expected non-zero time")
	}
}

// TestRegistry_LoadFromDB_ClearsExistingEntries verifies that LoadFromDB
// replaces any previously registered entries.
// Validates: Requirements 14.3
func TestRegistry_LoadFromDB_ClearsExistingEntries(t *testing.T) {
	tenantID := uuid.New()

	r := plugins.NewRegistry()
	// Pre-populate with an entry that should be cleared.
	r.Register("old-plugin", makeEntry("old-plugin", plugins.RegistryActive))

	ms := newMockPluginStore()
	ms.tenantPlugins = nil // No enabled plugins in DB.

	ctx := store.WithTenantID(context.Background(), tenantID)
	if err := r.LoadFromDB(ctx, ms); err != nil {
		t.Fatalf("LoadFromDB: %v", err)
	}

	if r.Count() != 0 {
		t.Errorf("Count() = %d after LoadFromDB with empty DB, want 0", r.Count())
	}
	if _, ok := r.Get("old-plugin"); ok {
		t.Error("old-plugin should have been cleared by LoadFromDB")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Property P7: concurrent Register/Get operations never return partial state.
// **Validates: Requirement 14.6**
// ─────────────────────────────────────────────────────────────────────────────

// genRegistryStatus draws a random valid RegistryStatus.
func genRegistryStatus(t *rapid.T) plugins.RegistryStatus {
	statuses := []plugins.RegistryStatus{
		plugins.RegistryActive,
		plugins.RegistryDisabled,
		plugins.RegistryError,
	}
	return statuses[rapid.IntRange(0, len(statuses)-1).Draw(t, "status")]
}

// genPluginName generates a random plugin name for registry operations.
func genPluginName(t *rapid.T) string {
	// Use a small pool of names to increase contention on the same keys.
	names := []string{"vault", "memory", "bridge", "search", "analytics", "monitor"}
	return names[rapid.IntRange(0, len(names)-1).Draw(t, "pluginName")]
}

// isCompleteEntry checks that a RegistryEntry is fully populated (not partial).
// A complete entry has: non-nil Manifest, non-zero CatalogID, valid Status, non-nil Tools.
func isCompleteEntry(entry *plugins.RegistryEntry) bool {
	if entry.Manifest == nil {
		return false
	}
	if entry.CatalogID == uuid.Nil {
		return false
	}
	if entry.Status != plugins.RegistryActive &&
		entry.Status != plugins.RegistryDisabled &&
		entry.Status != plugins.RegistryError {
		return false
	}
	if entry.Tools == nil {
		return false
	}
	return true
}

// TestRegistryThreadSafety_P7 verifies that concurrent Register and Get
// operations on the Registry never return a partially-constructed entry.
// Get must always return either nil (not found) or a complete RegistryEntry.
//
// **Validates: Requirement 14.6**
func TestRegistryThreadSafety_P7(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		r := plugins.NewRegistry()

		// Generate a random number of operations to execute concurrently.
		numOps := rapid.IntRange(10, 100).Draw(t, "numOps")

		// Build operation lists: each op is either a Register or a Get.
		type op struct {
			isRegister bool
			name       string
			status     plugins.RegistryStatus
		}

		ops := make([]op, numOps)
		for i := range ops {
			name := genPluginName(t)
			if rapid.Bool().Draw(t, fmt.Sprintf("isRegister[%d]", i)) {
				ops[i] = op{
					isRegister: true,
					name:       name,
					status:     genRegistryStatus(t),
				}
			} else {
				ops[i] = op{
					isRegister: false,
					name:       name,
				}
			}
		}

		// Execute all operations concurrently.
		var wg sync.WaitGroup
		violations := make(chan string, numOps)

		wg.Add(numOps)
		for i := range ops {
			go func(o op) {
				defer wg.Done()
				if o.isRegister {
					entry := makeEntry(o.name, o.status)
					r.Register(o.name, entry)
				} else {
					entry, ok := r.Get(o.name)
					if ok {
						// Entry was found — it must be complete, never partial.
						if !isCompleteEntry(entry) {
							violations <- fmt.Sprintf(
								"Get(%q) returned partial state: Manifest=%v, CatalogID=%v, Status=%q, Tools=%v",
								o.name, entry.Manifest != nil, entry.CatalogID, entry.Status, entry.Tools,
							)
						}
					}
					// If !ok, entry is nil — that's valid (not yet registered or was unregistered).
				}
			}(ops[i])
		}

		wg.Wait()
		close(violations)

		// Collect all violations.
		for v := range violations {
			t.Fatalf("thread-safety violation: %s", v)
		}
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Task 9.11 — Additional coverage tests for Registry refactoring
// ─────────────────────────────────────────────────────────────────────────────

func TestRegistry_SetLogger(t *testing.T) {
	r := plugins.NewRegistry()

	// SetLogger with a non-nil logger should not panic.
	customLogger := slog.Default()
	r.SetLogger(customLogger)

	// SetLogger with nil should be a no-op (keeps previous logger).
	r.SetLogger(nil)

	// Verify registry still works after SetLogger calls.
	r.Register("test", makeEntry("test", plugins.RegistryActive))
	if _, ok := r.Get("test"); !ok {
		t.Error("expected to find 'test' after SetLogger calls")
	}
}

func TestRegistry_Names(t *testing.T) {
	r := plugins.NewRegistry()

	// Empty registry returns empty slice.
	names := r.Names()
	if len(names) != 0 {
		t.Errorf("Names() on empty registry = %v, want empty", names)
	}

	// Register some plugins and verify Names returns all of them.
	r.Register("alpha", makeEntry("alpha", plugins.RegistryActive))
	r.Register("beta", makeEntry("beta", plugins.RegistryDisabled))
	r.Register("gamma", makeEntry("gamma", plugins.RegistryError))

	names = r.Names()
	if len(names) != 3 {
		t.Errorf("Names() = %d entries, want 3", len(names))
	}

	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}
	for _, expected := range []string{"alpha", "beta", "gamma"} {
		if !nameSet[expected] {
			t.Errorf("Names() missing %q", expected)
		}
	}
}
