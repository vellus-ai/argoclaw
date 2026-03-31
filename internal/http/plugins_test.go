package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/vellus-ai/argoclaw/internal/store"
)

// ─────────────────────────────────────────────────────────────────────────────
// Stub PluginStore for HTTP handler tests
// ─────────────────────────────────────────────────────────────────────────────

type stubPluginStoreHTTP struct {
	listCatalog          func(ctx context.Context) ([]store.PluginCatalogEntry, error)
	upsertCatalogEntry   func(ctx context.Context, e *store.PluginCatalogEntry) error
	getCatalogEntry      func(ctx context.Context, id uuid.UUID) (*store.PluginCatalogEntry, error)
	installPlugin        func(ctx context.Context, tp *store.TenantPlugin) error
	uninstallPlugin      func(ctx context.Context, name string, actorID *uuid.UUID) error
	getTenantPlugin      func(ctx context.Context, name string) (*store.TenantPlugin, error)
	listTenantPlugins    func(ctx context.Context) ([]store.TenantPlugin, error)
	enablePlugin         func(ctx context.Context, name string, actorID *uuid.UUID) error
	disablePlugin        func(ctx context.Context, name string, actorID *uuid.UUID) error
	updatePluginConfig   func(ctx context.Context, name string, cfg json.RawMessage, actorID *uuid.UUID) error
	setAgentPlugin       func(ctx context.Context, ap *store.AgentPlugin) error
	getAgentPlugin       func(ctx context.Context, agentID uuid.UUID, name string) (*store.AgentPlugin, error)
	listAgentPlugins     func(ctx context.Context, agentID uuid.UUID) ([]store.AgentPlugin, error)
	listAuditLog         func(ctx context.Context, name string, limit int) ([]store.PluginAuditEntry, error)
	// data proxy operations
	putData      func(ctx context.Context, plugin, col, key string, val json.RawMessage, exp *time.Time) error
	getData      func(ctx context.Context, plugin, col, key string) (*store.PluginDataEntry, error)
	listDataKeys func(ctx context.Context, plugin, col, prefix string, limit, offset int) ([]string, error)
	deleteData   func(ctx context.Context, plugin, col, key string) error
}

func (s *stubPluginStoreHTTP) ListCatalog(ctx context.Context) ([]store.PluginCatalogEntry, error) {
	if s.listCatalog != nil {
		return s.listCatalog(ctx)
	}
	return nil, nil
}
func (s *stubPluginStoreHTTP) UpsertCatalogEntry(ctx context.Context, e *store.PluginCatalogEntry) error {
	if s.upsertCatalogEntry != nil {
		return s.upsertCatalogEntry(ctx, e)
	}
	return nil
}
func (s *stubPluginStoreHTTP) GetCatalogEntry(ctx context.Context, id uuid.UUID) (*store.PluginCatalogEntry, error) {
	if s.getCatalogEntry != nil {
		return s.getCatalogEntry(ctx, id)
	}
	return nil, store.ErrPluginNotFound
}
func (s *stubPluginStoreHTTP) GetCatalogEntryByName(ctx context.Context, name string) (*store.PluginCatalogEntry, error) {
	return nil, store.ErrPluginNotFound
}
func (s *stubPluginStoreHTTP) InstallPlugin(ctx context.Context, tp *store.TenantPlugin) error {
	if s.installPlugin != nil {
		return s.installPlugin(ctx, tp)
	}
	return nil
}
func (s *stubPluginStoreHTTP) EnablePlugin(ctx context.Context, name string, actorID *uuid.UUID) error {
	if s.enablePlugin != nil {
		return s.enablePlugin(ctx, name, actorID)
	}
	return nil
}
func (s *stubPluginStoreHTTP) DisablePlugin(ctx context.Context, name string, actorID *uuid.UUID) error {
	if s.disablePlugin != nil {
		return s.disablePlugin(ctx, name, actorID)
	}
	return nil
}
func (s *stubPluginStoreHTTP) UninstallPlugin(ctx context.Context, name string, actorID *uuid.UUID) error {
	if s.uninstallPlugin != nil {
		return s.uninstallPlugin(ctx, name, actorID)
	}
	return nil
}
func (s *stubPluginStoreHTTP) GetTenantPlugin(ctx context.Context, name string) (*store.TenantPlugin, error) {
	if s.getTenantPlugin != nil {
		return s.getTenantPlugin(ctx, name)
	}
	return nil, store.ErrPluginNotFound
}
func (s *stubPluginStoreHTTP) ListTenantPlugins(ctx context.Context) ([]store.TenantPlugin, error) {
	if s.listTenantPlugins != nil {
		return s.listTenantPlugins(ctx)
	}
	return nil, nil
}
func (s *stubPluginStoreHTTP) UpdatePluginConfig(ctx context.Context, name string, cfg json.RawMessage, actorID *uuid.UUID) error {
	if s.updatePluginConfig != nil {
		return s.updatePluginConfig(ctx, name, cfg, actorID)
	}
	return nil
}
func (s *stubPluginStoreHTTP) SetPluginError(_ context.Context, _, _ string) error { return nil }
func (s *stubPluginStoreHTTP) SetAgentPlugin(ctx context.Context, ap *store.AgentPlugin) error {
	if s.setAgentPlugin != nil {
		return s.setAgentPlugin(ctx, ap)
	}
	return nil
}
func (s *stubPluginStoreHTTP) GetAgentPlugin(ctx context.Context, agentID uuid.UUID, name string) (*store.AgentPlugin, error) {
	if s.getAgentPlugin != nil {
		return s.getAgentPlugin(ctx, agentID, name)
	}
	return nil, store.ErrPluginNotFound
}
func (s *stubPluginStoreHTTP) ListAgentPlugins(ctx context.Context, agentID uuid.UUID) ([]store.AgentPlugin, error) {
	if s.listAgentPlugins != nil {
		return s.listAgentPlugins(ctx, agentID)
	}
	return nil, nil
}
func (s *stubPluginStoreHTTP) IsPluginEnabledForAgent(_ context.Context, _ uuid.UUID, _ string) (bool, error) {
	return true, nil
}
func (s *stubPluginStoreHTTP) PutData(ctx context.Context, plugin, col, key string, val json.RawMessage, exp *time.Time) error {
	if s.putData != nil {
		return s.putData(ctx, plugin, col, key, val, exp)
	}
	return nil
}
func (s *stubPluginStoreHTTP) GetData(ctx context.Context, plugin, col, key string) (*store.PluginDataEntry, error) {
	if s.getData != nil {
		return s.getData(ctx, plugin, col, key)
	}
	return nil, store.ErrPluginNotFound
}
func (s *stubPluginStoreHTTP) ListDataKeys(ctx context.Context, plugin, col, prefix string, limit, offset int) ([]string, error) {
	if s.listDataKeys != nil {
		return s.listDataKeys(ctx, plugin, col, prefix, limit, offset)
	}
	return nil, nil
}
func (s *stubPluginStoreHTTP) DeleteData(ctx context.Context, plugin, col, key string) error {
	if s.deleteData != nil {
		return s.deleteData(ctx, plugin, col, key)
	}
	return nil
}
func (s *stubPluginStoreHTTP) DeleteCollectionData(_ context.Context, _, _ string) error { return nil }
func (s *stubPluginStoreHTTP) LogAudit(_ context.Context, _ *store.PluginAuditEntry) error {
	return nil
}
func (s *stubPluginStoreHTTP) ListAuditLog(ctx context.Context, name string, limit int) ([]store.PluginAuditEntry, error) {
	if s.listAuditLog != nil {
		return s.listAuditLog(ctx, name, limit)
	}
	return nil, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Test helpers
// ─────────────────────────────────────────────────────────────────────────────

const testToken = "test-gateway-token"

func newPluginMux(s store.PluginStore) *http.ServeMux {
	mux := http.NewServeMux()
	h := NewPluginHandler(s, testToken, nil)
	h.RegisterRoutes(mux)
	return mux
}

func doRequest(mux *http.ServeMux, method, path string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	r := httptest.NewRequest(method, path, &buf)
	r.Header.Set("Authorization", "Bearer "+testToken)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	return w
}

func doRequestNoAuth(mux *http.ServeMux, method, path string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	return w
}

// ─────────────────────────────────────────────────────────────────────────────
// Catalog tests
// ─────────────────────────────────────────────────────────────────────────────

func TestPluginHandler_ListCatalog_RequiresAuth(t *testing.T) {
	mux := newPluginMux(&stubPluginStoreHTTP{})
	w := doRequestNoAuth(mux, "GET", "/v1/plugins/catalog")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestPluginHandler_ListCatalog_ReturnsEntries(t *testing.T) {
	stub := &stubPluginStoreHTTP{
		listCatalog: func(_ context.Context) ([]store.PluginCatalogEntry, error) {
			return []store.PluginCatalogEntry{
				{Name: "vault", Version: "1.0.0"},
				{Name: "memory", Version: "0.5.0"},
			}, nil
		},
	}
	mux := newPluginMux(stub)
	w := doRequest(mux, "GET", "/v1/plugins/catalog", nil)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	entries, ok := resp["catalog"].([]any)
	if !ok || len(entries) != 2 {
		t.Errorf("expected 2 catalog entries, got: %v", resp)
	}
}

func TestPluginHandler_CreateCatalogEntry_RequiresAuth(t *testing.T) {
	mux := newPluginMux(&stubPluginStoreHTTP{})
	w := doRequestNoAuth(mux, "POST", "/v1/plugins/catalog")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestPluginHandler_CreateCatalogEntry_MissingFields(t *testing.T) {
	mux := newPluginMux(&stubPluginStoreHTTP{})
	// Missing name and version
	w := doRequest(mux, "POST", "/v1/plugins/catalog", map[string]any{
		"display_name": "Vault",
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing fields, got %d", w.Code)
	}
}

func TestPluginHandler_CreateCatalogEntry_InvalidPermissions(t *testing.T) {
	// G4 boundary check at HTTP layer: manifest with core:* write → 400
	mux := newPluginMux(&stubPluginStoreHTTP{})
	entry := map[string]any{
		"name":    "evil-plugin",
		"version": "1.0.0",
		"manifest": map[string]any{
			"name":    "evil-plugin",
			"version": "1.0.0",
			"runtime": map[string]any{"transport": "stdio"},
			"permissions": map[string]any{
				"data": map[string]any{
					"write": []string{"core:agents"},
				},
			},
		},
	}
	w := doRequest(mux, "POST", "/v1/plugins/catalog", entry)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for forbidden permission, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPluginHandler_CreateCatalogEntry_Success(t *testing.T) {
	called := false
	stub := &stubPluginStoreHTTP{
		upsertCatalogEntry: func(_ context.Context, _ *store.PluginCatalogEntry) error {
			called = true
			return nil
		},
	}
	mux := newPluginMux(stub)
	entry := map[string]any{
		"name":    "prompt-vault",
		"version": "1.0.0",
	}
	w := doRequest(mux, "POST", "/v1/plugins/catalog", entry)
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if !called {
		t.Error("expected UpsertCatalogEntry to be called")
	}
}

func TestPluginHandler_GetCatalogEntry_NotFound(t *testing.T) {
	mux := newPluginMux(&stubPluginStoreHTTP{})
	id := uuid.New()
	w := doRequest(mux, "GET", "/v1/plugins/catalog/"+id.String(), nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestPluginHandler_GetCatalogEntry_InvalidUUID(t *testing.T) {
	mux := newPluginMux(&stubPluginStoreHTTP{})
	w := doRequest(mux, "GET", "/v1/plugins/catalog/not-a-uuid", nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestPluginHandler_GetCatalogEntry_Found(t *testing.T) {
	id := uuid.New()
	stub := &stubPluginStoreHTTP{
		getCatalogEntry: func(_ context.Context, gotID uuid.UUID) (*store.PluginCatalogEntry, error) {
			if gotID == id {
				return &store.PluginCatalogEntry{Name: "vault", Version: "1.0.0"}, nil
			}
			return nil, store.ErrPluginNotFound
		},
	}
	mux := newPluginMux(stub)
	w := doRequest(mux, "GET", "/v1/plugins/catalog/"+id.String(), nil)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Installed plugins tests
// ─────────────────────────────────────────────────────────────────────────────

func TestPluginHandler_ListInstalled_RequiresAuth(t *testing.T) {
	mux := newPluginMux(&stubPluginStoreHTTP{})
	w := doRequestNoAuth(mux, "GET", "/v1/plugins/installed")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestPluginHandler_ListInstalled_ReturnsPlugins(t *testing.T) {
	stub := &stubPluginStoreHTTP{
		listTenantPlugins: func(_ context.Context) ([]store.TenantPlugin, error) {
			return []store.TenantPlugin{
				{PluginName: "vault", State: store.PluginStateEnabled},
			}, nil
		},
	}
	mux := newPluginMux(stub)
	w := doRequest(mux, "GET", "/v1/plugins/installed", nil)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	plugins, ok := resp["plugins"].([]any)
	if !ok || len(plugins) != 1 {
		t.Errorf("expected 1 plugin, got: %v", resp)
	}
}

func TestPluginHandler_InstallPlugin_RequiresAuth(t *testing.T) {
	mux := newPluginMux(&stubPluginStoreHTTP{})
	w := doRequestNoAuth(mux, "POST", "/v1/plugins/install")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestPluginHandler_InstallPlugin_MissingPluginName(t *testing.T) {
	mux := newPluginMux(&stubPluginStoreHTTP{})
	w := doRequest(mux, "POST", "/v1/plugins/install", map[string]any{})
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing plugin_name, got %d", w.Code)
	}
}

func TestPluginHandler_InstallPlugin_Success(t *testing.T) {
	called := false
	stub := &stubPluginStoreHTTP{
		installPlugin: func(_ context.Context, tp *store.TenantPlugin) error {
			called = true
			return nil
		},
	}
	mux := newPluginMux(stub)
	w := doRequest(mux, "POST", "/v1/plugins/install", map[string]any{
		"plugin_name":    "prompt-vault",
		"plugin_version": "1.0.0",
	})
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if !called {
		t.Error("expected InstallPlugin to be called")
	}
}

func TestPluginHandler_InstallPlugin_AlreadyInstalled(t *testing.T) {
	stub := &stubPluginStoreHTTP{
		installPlugin: func(_ context.Context, _ *store.TenantPlugin) error {
			return store.ErrPluginAlreadyInstalled
		},
	}
	mux := newPluginMux(stub)
	w := doRequest(mux, "POST", "/v1/plugins/install", map[string]any{
		"plugin_name": "prompt-vault",
	})
	if w.Code != http.StatusConflict {
		t.Errorf("expected 409 for already installed, got %d", w.Code)
	}
}

func TestPluginHandler_UninstallPlugin_RequiresAuth(t *testing.T) {
	mux := newPluginMux(&stubPluginStoreHTTP{})
	w := doRequestNoAuth(mux, "DELETE", "/v1/plugins/installed/prompt-vault")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestPluginHandler_UninstallPlugin_Success(t *testing.T) {
	called := false
	stub := &stubPluginStoreHTTP{
		uninstallPlugin: func(_ context.Context, name string, _ *uuid.UUID) error {
			called = true
			return nil
		},
	}
	mux := newPluginMux(stub)
	w := doRequest(mux, "DELETE", "/v1/plugins/installed/prompt-vault", nil)
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
	if !called {
		t.Error("expected UninstallPlugin to be called")
	}
}

func TestPluginHandler_UninstallPlugin_NotFound(t *testing.T) {
	stub := &stubPluginStoreHTTP{
		uninstallPlugin: func(_ context.Context, _ string, _ *uuid.UUID) error {
			return store.ErrPluginNotFound
		},
	}
	mux := newPluginMux(stub)
	w := doRequest(mux, "DELETE", "/v1/plugins/installed/nonexistent", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestPluginHandler_UpdatePluginConfig_Success(t *testing.T) {
	called := false
	stub := &stubPluginStoreHTTP{
		updatePluginConfig: func(_ context.Context, name string, cfg json.RawMessage, _ *uuid.UUID) error {
			called = true
			return nil
		},
	}
	mux := newPluginMux(stub)
	w := doRequest(mux, "PUT", "/v1/plugins/installed/prompt-vault", map[string]any{
		"config": map[string]any{"maxPrompts": 100},
	})
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !called {
		t.Error("expected UpdatePluginConfig to be called")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Agent grant tests
// ─────────────────────────────────────────────────────────────────────────────

func TestPluginHandler_GrantAgent_RequiresAuth(t *testing.T) {
	mux := newPluginMux(&stubPluginStoreHTTP{})
	agentID := uuid.New()
	w := doRequestNoAuth(mux, "POST", "/v1/plugins/agents/"+agentID.String()+"/grant")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestPluginHandler_GrantAgent_InvalidAgentID(t *testing.T) {
	mux := newPluginMux(&stubPluginStoreHTTP{})
	w := doRequest(mux, "POST", "/v1/plugins/agents/not-a-uuid/grant", map[string]any{
		"plugin_name": "vault",
		"enabled":     true,
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid agent ID, got %d", w.Code)
	}
}

func TestPluginHandler_GrantAgent_Success(t *testing.T) {
	called := false
	stub := &stubPluginStoreHTTP{
		setAgentPlugin: func(_ context.Context, _ *store.AgentPlugin) error {
			called = true
			return nil
		},
	}
	mux := newPluginMux(stub)
	agentID := uuid.New()
	w := doRequest(mux, "POST", "/v1/plugins/agents/"+agentID.String()+"/grant", map[string]any{
		"plugin_name": "vault",
		"enabled":     true,
	})
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if !called {
		t.Error("expected SetAgentPlugin to be called")
	}
}

func TestPluginHandler_RevokeAgent_RequiresAuth(t *testing.T) {
	mux := newPluginMux(&stubPluginStoreHTTP{})
	agentID := uuid.New()
	w := doRequestNoAuth(mux, "DELETE", "/v1/plugins/agents/"+agentID.String()+"/vault")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestPluginHandler_RevokeAgent_Success(t *testing.T) {
	called := false
	stub := &stubPluginStoreHTTP{
		setAgentPlugin: func(_ context.Context, ap *store.AgentPlugin) error {
			called = true
			if ap.Enabled {
				t.Error("revoke should set Enabled=false")
			}
			return nil
		},
	}
	mux := newPluginMux(stub)
	agentID := uuid.New()
	w := doRequest(mux, "DELETE", "/v1/plugins/agents/"+agentID.String()+"/vault", nil)
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
	if !called {
		t.Error("expected SetAgentPlugin to be called")
	}
}

func TestPluginHandler_EnablePlugin_Success(t *testing.T) {
	called := false
	stub := &stubPluginStoreHTTP{
		enablePlugin: func(_ context.Context, name string, _ *uuid.UUID) error {
			called = true
			return nil
		},
	}
	mux := newPluginMux(stub)
	w := doRequest(mux, "POST", "/v1/plugins/installed/vault/enable", nil)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !called {
		t.Error("expected EnablePlugin to be called")
	}
}

func TestPluginHandler_EnablePlugin_NotFound(t *testing.T) {
	stub := &stubPluginStoreHTTP{
		enablePlugin: func(_ context.Context, _ string, _ *uuid.UUID) error {
			return store.ErrPluginNotFound
		},
	}
	mux := newPluginMux(stub)
	w := doRequest(mux, "POST", "/v1/plugins/installed/nonexistent/enable", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestPluginHandler_DisablePlugin_Success(t *testing.T) {
	called := false
	stub := &stubPluginStoreHTTP{
		disablePlugin: func(_ context.Context, name string, _ *uuid.UUID) error {
			called = true
			return nil
		},
	}
	mux := newPluginMux(stub)
	w := doRequest(mux, "POST", "/v1/plugins/installed/vault/disable", nil)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !called {
		t.Error("expected DisablePlugin to be called")
	}
}

func TestPluginHandler_DisablePlugin_NotFound(t *testing.T) {
	stub := &stubPluginStoreHTTP{
		disablePlugin: func(_ context.Context, _ string, _ *uuid.UUID) error {
			return store.ErrPluginNotFound
		},
	}
	mux := newPluginMux(stub)
	w := doRequest(mux, "POST", "/v1/plugins/installed/nonexistent/disable", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestPluginHandler_AuditLog_Success(t *testing.T) {
	stub := &stubPluginStoreHTTP{
		listAuditLog: func(_ context.Context, name string, _ int) ([]store.PluginAuditEntry, error) {
			return []store.PluginAuditEntry{
				{PluginName: name, Action: store.AuditInstall},
			}, nil
		},
	}
	mux := newPluginMux(stub)
	w := doRequest(mux, "GET", "/v1/plugins/installed/vault/audit", nil)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
