package plugins_test

import (
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/vellus-ai/argoclaw/internal/plugins"
)

func makeState(name string) *plugins.RegistryEntry {
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
		Status:    plugins.RegistryActive,
	}
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := plugins.NewRegistry()

	r.Register("vault", makeState("vault"))

	got, ok := r.Get("vault")
	if !ok {
		t.Fatal("expected to find 'vault' in registry")
	}
	if got.Manifest.Name != "vault" {
		t.Errorf("name: got %q want 'vault'", got.Manifest.Name)
	}
}

func TestRegistry_GetNotFound(t *testing.T) {
	r := plugins.NewRegistry()
	_, ok := r.Get("nonexistent")
	if ok {
		t.Error("expected false for nonexistent plugin")
	}
}

func TestRegistry_Unregister(t *testing.T) {
	r := plugins.NewRegistry()
	r.Register("vault", makeState("vault"))
	r.Unregister("vault")

	_, ok := r.Get("vault")
	if ok {
		t.Error("expected plugin to be gone after Unregister")
	}
}

func TestRegistry_Unregister_Idempotent(t *testing.T) {
	r := plugins.NewRegistry()
	// Should not panic if the plugin does not exist.
	r.Unregister("nonexistent")
}

func TestRegistry_List(t *testing.T) {
	r := plugins.NewRegistry()
	r.Register("vault", makeState("vault"))
	r.Register("memory", makeState("memory"))
	r.Register("bridge", makeState("bridge"))

	list := r.List()
	if len(list) != 3 {
		t.Errorf("expected 3 plugins, got %d", len(list))
	}
}

func TestRegistry_List_Empty(t *testing.T) {
	r := plugins.NewRegistry()
	list := r.List()
	if list == nil {
		t.Error("expected non-nil empty slice")
	}
	if len(list) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(list))
	}
}

func TestRegistry_Register_Overwrites(t *testing.T) {
	r := plugins.NewRegistry()
	s1 := makeState("vault")
	s1.Status = plugins.RegistryActive
	r.Register("vault", s1)

	s2 := makeState("vault")
	s2.Status = plugins.RegistryError
	r.Register("vault", s2)

	got, _ := r.Get("vault")
	if got.Status != plugins.RegistryError {
		t.Errorf("expected overwritten state to have StatusError")
	}
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	r := plugins.NewRegistry()
	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			name := "plugin-" + uuid.New().String()[:8]
			r.Register(name, makeState(name))
			r.Get(name)
			r.List()
			r.Unregister(name)
		}(i)
	}

	wg.Wait()
	// If we get here without data race or panic, the test passes.
}

func TestRegistry_Count(t *testing.T) {
	r := plugins.NewRegistry()
	if r.Count() != 0 {
		t.Errorf("expected 0 initially, got %d", r.Count())
	}
	r.Register("a", makeState("a"))
	r.Register("b", makeState("b"))
	if r.Count() != 2 {
		t.Errorf("expected 2, got %d", r.Count())
	}
	r.Unregister("a")
	if r.Count() != 1 {
		t.Errorf("expected 1 after unregister, got %d", r.Count())
	}
}
