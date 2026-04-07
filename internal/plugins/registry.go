package plugins

import (
	"sync"
)

// Backward-compatible aliases for registry status constants.
// Prefer using RegistryActive, RegistryDisabled, RegistryError from models.go.
const (
	StatusActive   = RegistryActive
	StatusDisabled = RegistryDisabled
	StatusError    = RegistryError
)

// Registry is a thread-safe in-memory registry that maps plugin names
// to their current runtime state. It is the source of truth for which
// plugins are currently loaded and active in a running gateway process.
//
// The registry does NOT own plugin lifecycle — the Lifecycle Controller does.
// The registry only reflects what has been registered by the controller.
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]*RegistryEntry
}

// NewRegistry creates a new empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		plugins: make(map[string]*RegistryEntry),
	}
}

// Register adds or replaces the state for a named plugin.
// If the plugin is already registered, its state is overwritten.
func (r *Registry) Register(name string, state *RegistryEntry) {
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
func (r *Registry) Get(name string) (*RegistryEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.plugins[name]
	return s, ok
}

// List returns a snapshot of all registered plugin states.
// The returned slice is never nil (may be empty).
// Callers must not modify the returned RegistryEntry values.
func (r *Registry) List() []*RegistryEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*RegistryEntry, 0, len(r.plugins))
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

// ActiveNames returns the names of all plugins with Status == RegistryActive.
func (r *Registry) ActiveNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var names []string
	for name, s := range r.plugins {
		if s.Status == RegistryActive {
			names = append(names, name)
		}
	}
	return names
}
