package plugins

import (
	"sync"

	"github.com/google/uuid"
)

// Plugin status constants for the in-memory registry.
const (
	StatusActive   = "active"
	StatusDisabled = "disabled"
	StatusError    = "error"
)

// PluginState holds the runtime state of a loaded plugin.
type PluginState struct {
	// Manifest is the parsed plugin.yaml for this plugin.
	Manifest *PluginManifest
	// CatalogID is the UUID of the catalog entry this plugin was loaded from.
	CatalogID uuid.UUID
	// Status is the current runtime status: active, disabled, error.
	Status string
	// ErrorMsg holds the last error message when Status == StatusError.
	ErrorMsg string
}

// Registry is a thread-safe in-memory registry that maps plugin names
// to their current runtime state. It is the source of truth for which
// plugins are currently loaded and active in a running gateway process.
//
// The registry does NOT own plugin lifecycle — the Lifecycle Controller does.
// The registry only reflects what has been registered by the controller.
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]*PluginState
}

// NewRegistry creates a new empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		plugins: make(map[string]*PluginState),
	}
}

// Register adds or replaces the state for a named plugin.
// If the plugin is already registered, its state is overwritten.
func (r *Registry) Register(name string, state *PluginState) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.plugins[name] = state
}

// Unregister removes a plugin from the registry.
// If the plugin is not registered, this is a no-op.
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.plugins, name)
}

// Get returns the state for a named plugin.
// Returns (nil, false) if the plugin is not registered.
func (r *Registry) Get(name string) (*PluginState, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.plugins[name]
	return s, ok
}

// List returns a snapshot of all registered plugin states.
// The returned slice is never nil (may be empty).
// Callers must not modify the returned PluginState values.
func (r *Registry) List() []*PluginState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*PluginState, 0, len(r.plugins))
	for _, s := range r.plugins {
		result = append(result, s)
	}
	return result
}

// Count returns the number of registered plugins.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.plugins)
}

// Names returns the names of all registered plugins.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.plugins))
	for name := range r.plugins {
		names = append(names, name)
	}
	return names
}

// ActiveNames returns the names of all plugins with Status == StatusActive.
func (r *Registry) ActiveNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var names []string
	for name, s := range r.plugins {
		if s.Status == StatusActive {
			names = append(names, name)
		}
	}
	return names
}
