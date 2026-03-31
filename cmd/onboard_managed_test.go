//go:build !integration

package cmd

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"pgregory.net/rapid"

	"github.com/vellus-ai/argoclaw/internal/store"
)

// --- in-memory stub of store.ProviderStore for onboard seeding tests ---

type stubProviderStore struct {
	providers map[string]*store.LLMProviderData
	seedErr   error // if set, SeedOnboardProvider returns this error
}

func newStubProviderStore() *stubProviderStore {
	return &stubProviderStore{providers: make(map[string]*store.LLMProviderData)}
}

func (s *stubProviderStore) CreateProvider(_ context.Context, p *store.LLMProviderData) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	s.providers[p.Name] = p
	return nil
}

// SeedOnboardProvider is the idempotent variant used by onboarding.
// Mirrors ON CONFLICT (name, tenant_id) DO NOTHING: a second call for the same
// name is a silent no-op, exactly as the Postgres implementation behaves.
func (s *stubProviderStore) SeedOnboardProvider(_ context.Context, p *store.LLMProviderData) error {
	if s.seedErr != nil {
		return s.seedErr
	}
	if _, exists := s.providers[p.Name]; exists {
		return nil // idempotent — DO NOTHING semantics
	}
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	s.providers[p.Name] = p
	return nil
}

func (s *stubProviderStore) GetProvider(_ context.Context, id uuid.UUID) (*store.LLMProviderData, error) {
	for _, p := range s.providers {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func (s *stubProviderStore) GetProviderByName(_ context.Context, name string) (*store.LLMProviderData, error) {
	if p, ok := s.providers[name]; ok {
		return p, nil
	}
	return nil, fmt.Errorf("not found")
}

func (s *stubProviderStore) ListProviders(_ context.Context) ([]store.LLMProviderData, error) {
	out := make([]store.LLMProviderData, 0, len(s.providers))
	for _, p := range s.providers {
		out = append(out, *p)
	}
	return out, nil
}

func (s *stubProviderStore) UpdateProvider(_ context.Context, _ uuid.UUID, _ map[string]any) error {
	return nil
}

func (s *stubProviderStore) DeleteProvider(_ context.Context, id uuid.UUID) error {
	for name, p := range s.providers {
		if p.ID == id {
			delete(s.providers, name)
			return nil
		}
	}
	return fmt.Errorf("not found")
}

// --- tests ---

// TestSeedPlaceholdersWithStore_SeedsAllDefaults verifies that all
// defaultPlaceholderProviders are inserted into an empty store.
func TestSeedPlaceholdersWithStore_SeedsAllDefaults(t *testing.T) {
	stub := newStubProviderStore()
	if err := seedPlaceholdersWithStore(context.Background(), stub); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := len(stub.providers), len(defaultPlaceholderProviders); got != want {
		t.Errorf("seeded %d providers, want %d", got, want)
	}
}

// TestSeedPlaceholdersWithStore_Idempotent verifies that calling the function twice
// does not duplicate any provider — SeedOnboardProvider must be DO NOTHING.
func TestSeedPlaceholdersWithStore_Idempotent(t *testing.T) {
	stub := newStubProviderStore()

	for i := range 2 {
		if err := seedPlaceholdersWithStore(context.Background(), stub); err != nil {
			t.Fatalf("call %d unexpected error: %v", i+1, err)
		}
	}

	if got, want := len(stub.providers), len(defaultPlaceholderProviders); got != want {
		t.Errorf("after 2 calls: got %d providers, want %d (no duplicates)", got, want)
	}
}

// TestSeedPlaceholdersWithStore_SkipsExistingAPIBase verifies that a placeholder whose
// api_base already exists (user-configured provider) is skipped.
func TestSeedPlaceholdersWithStore_SkipsExistingAPIBase(t *testing.T) {
	stub := newStubProviderStore()

	// Pre-populate with a provider sharing the openrouter api_base.
	existing := &store.LLMProviderData{
		BaseModel:    store.BaseModel{ID: uuid.New()},
		Name:         "my-openrouter",
		ProviderType: store.ProviderOpenRouter,
		APIBase:      "https://openrouter.ai/api/v1",
		Enabled:      true,
	}
	stub.providers[existing.Name] = existing

	if err := seedPlaceholdersWithStore(context.Background(), stub); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The placeholder "openrouter" must NOT be inserted because its api_base is taken.
	if _, ok := stub.providers["openrouter"]; ok {
		t.Error("placeholder 'openrouter' should have been skipped (api_base already in use)")
	}
}

// TestSeedPlaceholdersWithStore_SeedErrorDoesNotAbort verifies that a transient error
// from SeedOnboardProvider for one provider does not abort seeding of the rest.
func TestSeedPlaceholdersWithStore_SeedErrorDoesNotAbort(t *testing.T) {
	// Return an error on every SeedOnboardProvider call.
	stub := &stubProviderStore{
		providers: make(map[string]*store.LLMProviderData),
		seedErr:   fmt.Errorf("simulated DB error"),
	}

	// Must not return an error — individual seed failures are logged and skipped.
	if err := seedPlaceholdersWithStore(context.Background(), stub); err != nil {
		t.Fatalf("seedPlaceholdersWithStore should tolerate individual seed errors, got: %v", err)
	}

	if len(stub.providers) != 0 {
		t.Errorf("expected no providers seeded on error, got %d", len(stub.providers))
	}
}

// TestPBT_SeedPlaceholders_NeverPanics is a property-based test that verifies
// seedPlaceholdersWithStore never panics regardless of pre-existing store state.
func TestPBT_SeedPlaceholders_NeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		stub := newStubProviderStore()

		// Pre-populate with a random subset of providers already present.
		names := rapid.SliceOfDistinct(
			rapid.SampledFrom(defaultPlaceholderProviders),
			func(p store.LLMProviderData) string { return p.Name },
		).Draw(rt, "preexisting")

		for _, p := range names {
			cp := p
			cp.ID = uuid.New()
			stub.providers[cp.Name] = &cp
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					rt.Fatalf("seedPlaceholdersWithStore panicked: %v", r)
				}
			}()
			_ = seedPlaceholdersWithStore(context.Background(), stub)
		}()
	})
}
