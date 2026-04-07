package plugins

import (
	"context"
	"log/slog"

	"github.com/vellus-ai/argoclaw/internal/store"
	"github.com/vellus-ai/argoclaw/internal/tools"
)

// Lifecycle manages the runtime lifecycle of plugins for a gateway process.
//
// It is responsible for:
//   - Loading enabled plugins from the store into the in-memory Registry
//   - Registering/unregistering tool groups for each plugin
//   - Clearing the registry on Stop
//
// The Lifecycle does NOT manage the actual MCP process — process management
// is a future responsibility. For Phase 0, it provides the scaffolding.
type Lifecycle struct {
	store    store.PluginStore
	registry *Registry
}

// NewLifecycle creates a new Lifecycle controller.
func NewLifecycle(s store.PluginStore, reg *Registry) *Lifecycle {
	return &Lifecycle{store: s, registry: reg}
}

// LoadAll loads all enabled tenant plugins into the registry and registers
// their tool groups. It is called at gateway startup.
//
// Plugins in states other than PluginStateEnabled are skipped.
// A store error aborts the load and returns an error.
func (l *Lifecycle) LoadAll(ctx context.Context) error {
	plugins, err := l.store.ListTenantPlugins(ctx)
	if err != nil {
		return err
	}

	loaded := 0
	for _, tp := range plugins {
		if tp.State != store.PluginStateEnabled {
			continue
		}
		state := &RegistryEntry{
			Manifest: &PluginManifest{
				Metadata: ManifestMetadata{
					Name:    tp.PluginName,
					Version: tp.PluginVersion,
				},
				Name:    tp.PluginName,
				Version: tp.PluginVersion,
			},
			Status: RegistryActive,
		}
		l.registry.Register(tp.PluginName, state)
		// Register an empty tool group for the plugin. Tools are populated by
		// the MCP manager when the plugin process connects (future WP).
		tools.RegisterToolGroup("plugin:"+tp.PluginName, []string{})
		loaded++
		slog.Info("plugins.lifecycle.loaded", "plugin", tp.PluginName, "version", tp.PluginVersion)
	}

	slog.Info("plugins.lifecycle.load_complete", "loaded", loaded, "total", len(plugins))
	return nil
}

// RegisterPlugin adds a plugin to the in-memory registry and registers its tool group.
// Called when a plugin is enabled at runtime (after the gateway is running).
func (l *Lifecycle) RegisterPlugin(name string, state *RegistryEntry) {
	l.registry.Register(name, state)
	tools.RegisterToolGroup("plugin:"+name, []string{})
	slog.Info("plugins.lifecycle.registered", "plugin", name)
}

// UnregisterPlugin removes a plugin from the registry and unregisters its tool group.
// Called when a plugin is disabled at runtime.
func (l *Lifecycle) UnregisterPlugin(name string) {
	l.registry.Unregister(name)
	tools.UnregisterToolGroup("plugin:" + name)
	slog.Info("plugins.lifecycle.unregistered", "plugin", name)
}

// ActivePluginNames returns the names of all currently active plugins.
func (l *Lifecycle) ActivePluginNames() []string {
	return l.registry.ActiveNames()
}

// Stop clears all plugins from the registry and unregisters their tool groups.
// Called during gateway shutdown.
func (l *Lifecycle) Stop() {
	for _, name := range l.registry.Names() {
		tools.UnregisterToolGroup("plugin:" + name)
		l.registry.Unregister(name)
	}
	slog.Info("plugins.lifecycle.stopped")
}
