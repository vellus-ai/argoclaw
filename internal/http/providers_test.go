package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/vellus-ai/argoclaw/internal/providers"
	"github.com/vellus-ai/argoclaw/internal/store"
)

// newTestProvidersHandler creates a ProvidersHandler with minimal deps for unit tests.
// It reuses mockProviderStore and mockSecretsStore defined in oauth_test.go.
func newTestProvidersHandler(ps *mockProviderStore, reg *providers.Registry) *ProvidersHandler {
	ss := newMockSecretsStore()
	return NewProvidersHandler(ps, ss, "test-token", reg, "")
}

// makeAuthRequest wraps an httptest request with the gateway token bearer.
func makeAuthRequest(method, path string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	r.Header.Set("Authorization", "Bearer test-token")
	return r
}

// TestHandleListProviders_NoRegistry returns only DB providers when registry is nil.
func TestHandleListProviders_NoRegistry(t *testing.T) {
	ps := newMockProviderStore()
	dbID := uuid.New()
	dbProvider := &store.LLMProviderData{Name: "my-provider", ProviderType: "openai_compat", Enabled: true}
	dbProvider.ID = dbID
	ps.providers["my-provider"] = dbProvider

	h := NewProvidersHandler(ps, newMockSecretsStore(), "test-token", nil, "")

	w := httptest.NewRecorder()
	h.handleListProviders(w, makeAuthRequest("GET", "/v1/providers"))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp struct {
		Providers []store.LLMProviderData `json:"providers"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Providers) != 1 {
		t.Fatalf("providers count = %d, want 1", len(resp.Providers))
	}
	if resp.Providers[0].Name != "my-provider" {
		t.Errorf("name = %q, want %q", resp.Providers[0].Name, "my-provider")
	}
	if resp.Providers[0].HostManaged {
		t.Error("DB provider should not be host_managed")
	}
}

// TestHandleListProviders_DeduplicatesRegistryProvider ensures that a provider
// already in the DB is not duplicated even when the registry also has it.
func TestHandleListProviders_DeduplicatesRegistryProvider(t *testing.T) {
	ps := newMockProviderStore()
	dbID := uuid.New()
	dbProvider := &store.LLMProviderData{Name: "anthropic", ProviderType: "anthropic_native", Enabled: true}
	dbProvider.ID = dbID
	ps.providers["anthropic"] = dbProvider

	reg := providers.NewRegistry()
	// Register a provider with the same name as the DB entry.
	reg.Register(&stubProvider{name: "anthropic"})

	h := newTestProvidersHandler(ps, reg)

	w := httptest.NewRecorder()
	h.handleListProviders(w, makeAuthRequest("GET", "/v1/providers"))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp struct {
		Providers []store.LLMProviderData `json:"providers"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// Must have exactly 1 entry — no duplication.
	if len(resp.Providers) != 1 {
		t.Errorf("providers count = %d, want 1 (deduplication failed)", len(resp.Providers))
	}
	if resp.Providers[0].ID != dbID {
		t.Errorf("ID = %v, want DB ID %v", resp.Providers[0].ID, dbID)
	}
}

// TestHandleListProviders_VertexAI_MappedTypeAndDeterministicUUID checks that
// a registry entry named "vertex-ai" gets provider_type "vertex_ai" and a
// stable, non-zero deterministic UUID.
func TestHandleListProviders_VertexAI_MappedTypeAndDeterministicUUID(t *testing.T) {
	ps := newMockProviderStore() // empty DB
	reg := providers.NewRegistry()
	reg.Register(&stubProvider{name: "vertex-ai"})

	h := newTestProvidersHandler(ps, reg)

	// Call twice to confirm determinism.
	var firstID uuid.UUID
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		h.handleListProviders(w, makeAuthRequest("GET", "/v1/providers"))

		if w.Code != http.StatusOK {
			t.Fatalf("iter %d: status = %d, want 200", i, w.Code)
		}

		var resp struct {
			Providers []store.LLMProviderData `json:"providers"`
		}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("iter %d decode: %v", i, err)
		}
		if len(resp.Providers) != 1 {
			t.Fatalf("iter %d: providers count = %d, want 1", i, len(resp.Providers))
		}
		p := resp.Providers[0]

		if p.Name != "vertex-ai" {
			t.Errorf("name = %q, want %q", p.Name, "vertex-ai")
		}
		if p.ProviderType != store.ProviderVertexAI {
			t.Errorf("provider_type = %q, want %q", p.ProviderType, store.ProviderVertexAI)
		}
		if p.ID == uuid.Nil {
			t.Error("synthetic UUID must not be nil")
		}
		if !p.HostManaged {
			t.Error("synthetic entry must have host_managed=true")
		}
		if i == 0 {
			firstID = p.ID
		} else if p.ID != firstID {
			t.Errorf("UUID is not deterministic: first=%v second=%v", firstID, p.ID)
		}
	}
}

// TestHandleListProviders_GenericRegistryEntry verifies that a generic registry
// name gets a deterministic non-zero UUID and the provider_type equals the name.
func TestHandleListProviders_GenericRegistryEntry(t *testing.T) {
	ps := newMockProviderStore()
	reg := providers.NewRegistry()
	reg.Register(&stubProvider{name: "custom-llm"})

	h := newTestProvidersHandler(ps, reg)

	w := httptest.NewRecorder()
	h.handleListProviders(w, makeAuthRequest("GET", "/v1/providers"))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp struct {
		Providers []store.LLMProviderData `json:"providers"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Providers) != 1 {
		t.Fatalf("providers count = %d, want 1", len(resp.Providers))
	}
	p := resp.Providers[0]

	if p.ProviderType != "custom-llm" {
		t.Errorf("provider_type = %q, want %q", p.ProviderType, "custom-llm")
	}
	if p.ID == uuid.Nil {
		t.Error("synthetic UUID must not be nil")
	}
	// Verify determinism independently.
	expected := uuid.NewSHA1(uuid.NameSpaceDNS, []byte("custom-llm"))
	if p.ID != expected {
		t.Errorf("UUID = %v, want deterministic %v", p.ID, expected)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Minimal Provider stub for registry tests
// ─────────────────────────────────────────────────────────────────────────────

// stubProvider implements providers.Provider with the minimal required methods.
type stubProvider struct {
	name string
}

func (s *stubProvider) Name() string { return s.name }
func (s *stubProvider) DefaultModel() string { return "" }
func (s *stubProvider) Chat(_ context.Context, _ providers.ChatRequest) (*providers.ChatResponse, error) {
	return nil, nil
}
func (s *stubProvider) ChatStream(_ context.Context, _ providers.ChatRequest, _ func(providers.StreamChunk)) (*providers.ChatResponse, error) {
	return nil, nil
}
