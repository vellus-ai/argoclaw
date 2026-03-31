package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/vellus-ai/argoclaw/internal/store"
)

// ─────────────────────────────────────────────────────────────────────────────
// Setup helpers for data proxy HTTP tests
// (reuses stubPluginStoreHTTP defined in plugins_test.go)
// ─────────────────────────────────────────────────────────────────────────────

func newPluginDataMux(s store.PluginStore) *http.ServeMux {
	mux := http.NewServeMux()
	h := NewPluginDataHandler(s, testToken)
	h.RegisterRoutes(mux)
	return mux
}

func doDataRequest(mux *http.ServeMux, method, path string, body any) *httptest.ResponseRecorder {
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

func doDataRequestNoAuth(mux *http.ServeMux, method, path string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	return w
}

// ─────────────────────────────────────────────────────────────────────────────
// List keys
// ─────────────────────────────────────────────────────────────────────────────

func TestPluginDataHandler_ListKeys_RequiresAuth(t *testing.T) {
	mux := newPluginDataMux(&stubPluginStoreHTTP{})
	w := doDataRequestNoAuth(mux, "GET", "/v1/plugins/vault/data/prompts")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestPluginDataHandler_ListKeys_EmptyResultIsNotNull(t *testing.T) {
	mux := newPluginDataMux(&stubPluginStoreHTTP{})
	w := doDataRequest(mux, "GET", "/v1/plugins/vault/data/prompts", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	keys, ok := resp["keys"]
	if !ok {
		t.Fatal("expected 'keys' field in response")
	}
	if keys == nil {
		t.Error("expected non-null keys array (empty slice, not null)")
	}
}

func TestPluginDataHandler_ListKeys_ReturnsKeys(t *testing.T) {
	stub := &stubPluginStoreHTTP{
		listDataKeys: func(_ context.Context, _, _, _ string, _, _ int) ([]string, error) {
			return []string{"key1", "key2"}, nil
		},
	}
	mux := newPluginDataMux(stub)
	w := doDataRequest(mux, "GET", "/v1/plugins/vault/data/prompts", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	rawKeys, _ := resp["keys"].([]any)
	if len(rawKeys) != 2 {
		t.Errorf("expected 2 keys, got: %v", resp["keys"])
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Get value
// ─────────────────────────────────────────────────────────────────────────────

func TestPluginDataHandler_GetValue_RequiresAuth(t *testing.T) {
	mux := newPluginDataMux(&stubPluginStoreHTTP{})
	w := doDataRequestNoAuth(mux, "GET", "/v1/plugins/vault/data/prompts/my-key")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestPluginDataHandler_GetValue_NotFound(t *testing.T) {
	// Default stub returns ErrPluginNotFound for GetData.
	mux := newPluginDataMux(&stubPluginStoreHTTP{})
	w := doDataRequest(mux, "GET", "/v1/plugins/vault/data/prompts/missing-key", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPluginDataHandler_GetValue_Found(t *testing.T) {
	stub := &stubPluginStoreHTTP{
		getData: func(_ context.Context, _, _, key string) (*store.PluginDataEntry, error) {
			return &store.PluginDataEntry{Key: key, Value: json.RawMessage(`"hello"`)}, nil
		},
	}
	mux := newPluginDataMux(stub)
	w := doDataRequest(mux, "GET", "/v1/plugins/vault/data/prompts/my-key", nil)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["key"] != "my-key" {
		t.Errorf("expected key=my-key in response, got: %v", resp)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Put value
// ─────────────────────────────────────────────────────────────────────────────

func TestPluginDataHandler_PutValue_RequiresAuth(t *testing.T) {
	mux := newPluginDataMux(&stubPluginStoreHTTP{})
	w := doDataRequestNoAuth(mux, "PUT", "/v1/plugins/vault/data/prompts/my-key")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestPluginDataHandler_PutValue_InvalidJSON(t *testing.T) {
	mux := newPluginDataMux(&stubPluginStoreHTTP{})
	r := httptest.NewRequest("PUT", "/v1/plugins/vault/data/prompts/my-key", bytes.NewBufferString("not-json-at-all!!!"))
	r.Header.Set("Authorization", "Bearer "+testToken)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", w.Code)
	}
}

func TestPluginDataHandler_PutValue_Success(t *testing.T) {
	called := false
	stub := &stubPluginStoreHTTP{
		putData: func(_ context.Context, plugin, col, key string, val json.RawMessage, _ *time.Time) error {
			called = true
			return nil
		},
	}
	mux := newPluginDataMux(stub)
	body := map[string]any{"value": "hello world"}
	w := doDataRequest(mux, "PUT", "/v1/plugins/vault/data/prompts/my-key", body)
	if w.Code != http.StatusOK && w.Code != http.StatusNoContent {
		t.Errorf("expected 200 or 204, got %d: %s", w.Code, w.Body.String())
	}
	if !called {
		t.Error("expected PutData to be called")
	}
}

func TestPluginDataHandler_PutValue_EmptyBodyAllowed(t *testing.T) {
	// An empty JSON object {} should be a valid value.
	mux := newPluginDataMux(&stubPluginStoreHTTP{})
	w := doDataRequest(mux, "PUT", "/v1/plugins/vault/data/prompts/my-key", map[string]any{})
	if w.Code != http.StatusOK && w.Code != http.StatusNoContent {
		t.Errorf("expected success for empty JSON body, got %d: %s", w.Code, w.Body.String())
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Delete value
// ─────────────────────────────────────────────────────────────────────────────

func TestPluginDataHandler_DeleteValue_RequiresAuth(t *testing.T) {
	mux := newPluginDataMux(&stubPluginStoreHTTP{})
	w := doDataRequestNoAuth(mux, "DELETE", "/v1/plugins/vault/data/prompts/my-key")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestPluginDataHandler_DeleteValue_Success(t *testing.T) {
	mux := newPluginDataMux(&stubPluginStoreHTTP{})
	w := doDataRequest(mux, "DELETE", "/v1/plugins/vault/data/prompts/my-key", nil)
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPluginDataHandler_DeleteValue_StoreError(t *testing.T) {
	stub := &stubPluginStoreHTTP{
		deleteData: func(_ context.Context, _, _, _ string) error {
			return errors.New("db connection failed")
		},
	}
	// Override DeleteData for the stub
	_ = stub
	// This test verifies that a store error maps to 500.
	// Since the stub above doesn't wire deleteData into the stub struct (the struct
	// uses a function field "deleteData"), it's already covered by the field defined
	// in plugins_test.go. The default no-op returns nil, so let's verify the error path.
	mux := newPluginDataMux(&stubPluginStoreHTTP{
		deleteData: func(_ context.Context, _, _, _ string) error {
			return errors.New("db connection failed")
		},
	})
	w := doDataRequest(mux, "DELETE", "/v1/plugins/vault/data/prompts/my-key", nil)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for store error, got %d", w.Code)
	}
}
