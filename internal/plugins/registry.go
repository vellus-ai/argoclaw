package plugins

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/vellus-ai/argoclaw/internal/store"
)

// Backward-compatible aliases for registry status constants.
// Prefer using RegistryActive, RegistryDisabled, RegistryError from models.go.
const (
	StatusActive   = RegistryActive
	StatusDisabled = RegistryDisabled
	StatusError    = RegistryError
)

// RedisPublishSubscriber abstracts the Redis pub/sub operations needed by the
// Registry for cross-instance synchronization. This allows testing without a
// real Redis connection and avoids a hard dependency on the redis package.
type RedisPublishSubscriber interface {
	// Publish sends a message to a Redis channel. Returns the number of
	// clients that received the message.
	Publish(ctx context.Context, channel string, message interface{}) error
	// Subscribe returns a channel that receives messages published to the
	// given Redis channel. The returned cancel function stops the subscription.
	Subscribe(ctx context.Context, channel string) (messages <-chan string, cancel func(), err error)
}

// registrySyncMessage is the JSON payload published via Redis pub/sub
// when a plugin state changes.
type registrySyncMessage struct {
	PluginName string `json:"plugin_name"`
	Action     string `json:"action"` // e.g. "register", "unregister", "enable", "disable"
	Timestamp  int64  `json:"timestamp"`
}

// Registry is a thread-safe in-memory registry that maps plugin names
// to their current runtime state. It is the source of truth for which
// plugins are currently loaded and active in a running gateway process.
//
// The registry does NOT own plugin lifecycle — the Lifecycle Controller does.
// The registry only reflects what has been registered by the controller.
//
// When a RedisPublishSubscriber is configured, the registry synchronizes
// state changes across gateway instances via pub/sub on the configured
// channel. If Redis is unavailable, it falls back to periodic DB polling.
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]*RegistryEntry

	// Redis pub/sub for cross-instance sync (optional — nil means no sync).
	redisPubSub  RedisPublishSubscriber
	syncChannel  string
	pollInterval time.Duration
	store        store.PluginStore
	logger       *slog.Logger
	cancelSync   context.CancelFunc // stops subscribeChanges goroutine
}

// NewRegistry creates a new empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		plugins: make(map[string]*RegistryEntry),
		logger:  slog.Default(),
	}
}

// SetSync configures Redis pub/sub synchronization for the registry.
// When set, state changes are published to the channel and other instances
// are notified to reload from the database. If rps is nil, sync is disabled
// and the registry operates in local-only mode.
//
// The store is used for reloading state when a sync notification is received
// and as the data source for the DB polling fallback.
func (r *Registry) SetSync(rps RedisPublishSubscriber, channel string, pollInterval time.Duration, s store.PluginStore) {
	r.redisPubSub = rps
	r.syncChannel = channel
	r.pollInterval = pollInterval
	r.store = s
}

// SetLogger overrides the default logger for the registry.
func (r *Registry) SetLogger(l *slog.Logger) {
	if l != nil {
		r.logger = l
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

// LoadFromDB loads plugins with state="enabled" from the database into the registry.
// Called during gateway Init to populate the in-memory registry from persistent state.
// Clears any existing entries before loading.
func (r *Registry) LoadFromDB(ctx context.Context, s store.PluginStore) error {
	// 1. Fetch all tenant plugins from the store.
	tenantPlugins, err := s.ListTenantPlugins(ctx)
	if err != nil {
		return err
	}

	// 2. Clear existing entries.
	r.mu.Lock()
	r.plugins = make(map[string]*RegistryEntry)
	r.mu.Unlock()

	// 3. Filter for enabled plugins and load each one.
	for _, tp := range tenantPlugins {
		if tp.State != store.PluginStateEnabled {
			continue
		}

		// Fetch catalog entry for this plugin.
		catalogEntry, err := s.GetCatalogEntryByName(ctx, tp.PluginName)
		if err != nil {
			// If catalog entry not found, skip with warning.
			slog.Warn("LoadFromDB: catalog entry not found, skipping",
				"plugin", tp.PluginName, "error", err)
			continue
		}

		// Parse the manifest from the catalog entry.
		var manifest PluginManifest
		if err := json.Unmarshal(catalogEntry.Manifest, &manifest); err != nil {
			slog.Warn("LoadFromDB: failed to parse manifest, skipping",
				"plugin", tp.PluginName, "error", err)
			continue
		}

		// Set backward-compat fields.
		manifest.Name = manifest.Metadata.Name
		manifest.Version = manifest.Metadata.Version

		// Build the registry entry.
		entry := &RegistryEntry{
			Manifest:  &manifest,
			CatalogID: catalogEntry.ID,
			Status:    RegistryActive,
			Tools:     []string{}, // Tools are discovered at Enable time, not during DB load.
		}
		if tp.EnabledAt != nil {
			entry.EnabledAt = *tp.EnabledAt
		}

		r.Register(tp.PluginName, entry)
	}

	return nil
}

// PublishChange notifies other gateway instances that a plugin state has changed.
// The notification is best-effort: if Redis is unavailable the error is logged
// and the local operation is not affected.
func (r *Registry) PublishChange(ctx context.Context, pluginName, action string) {
	if r.redisPubSub == nil {
		return
	}

	msg := registrySyncMessage{
		PluginName: pluginName,
		Action:     action,
		Timestamp:  time.Now().UnixMilli(),
	}
	data, err := json.Marshal(msg)
	if err != nil {
		r.logger.Warn("registry sync: failed to marshal message",
			"plugin", pluginName, "action", action, "error", err)
		return
	}

	if err := r.redisPubSub.Publish(ctx, r.syncChannel, string(data)); err != nil {
		r.logger.Warn("registry sync: publish failed (Redis may be unavailable)",
			"channel", r.syncChannel, "plugin", pluginName, "error", err)
	}
}

// StartSync begins listening for cross-instance sync notifications.
// It runs a dedicated goroutine that subscribes to the Redis pub/sub channel.
// If Redis is unavailable or the subscription fails, it falls back to polling
// the database at the configured interval (default 30s).
//
// Call StopSync to stop the background goroutine.
func (r *Registry) StartSync(ctx context.Context) {
	if r.store == nil {
		r.logger.Debug("registry sync: no store configured, sync disabled")
		return
	}

	ctx, cancel := context.WithCancel(ctx)
	r.cancelSync = cancel

	go r.subscribeChanges(ctx)
}

// StopSync stops the background sync goroutine started by StartSync.
func (r *Registry) StopSync() {
	if r.cancelSync != nil {
		r.cancelSync()
		r.cancelSync = nil
	}
}

// subscribeChanges listens for Redis pub/sub notifications on the sync channel.
// When a notification is received, it reloads the registry from the database.
// If Redis is unavailable, it falls back to polling the DB at pollInterval.
func (r *Registry) subscribeChanges(ctx context.Context) {
	for {
		if err := r.subscribeRedis(ctx); err != nil {
			if ctx.Err() != nil {
				return // context cancelled, shutting down
			}
			r.logger.Warn("registry sync: Redis subscription failed, falling back to DB polling",
				"error", err)
		}

		// Fallback: poll DB at the configured interval until Redis recovers
		// or the context is cancelled.
		if r.pollFallback(ctx) {
			return // context cancelled
		}
	}
}

// subscribeRedis attempts to subscribe to the Redis channel and process
// incoming messages. Returns an error if the subscription cannot be
// established or is lost.
func (r *Registry) subscribeRedis(ctx context.Context) error {
	if r.redisPubSub == nil {
		return errNoRedis
	}

	messages, cancel, err := r.redisPubSub.Subscribe(ctx, r.syncChannel)
	if err != nil {
		return err
	}
	defer cancel()

	r.logger.Info("registry sync: subscribed to Redis channel", "channel", r.syncChannel)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msgStr, ok := <-messages:
			if !ok {
				// Channel closed — subscription lost.
				return errSubscriptionLost
			}
			r.handleSyncMessage(ctx, msgStr)
		}
	}
}

// errNoRedis is a sentinel used internally when Redis is not configured.
var errNoRedis = &syncError{msg: "redis pub/sub not configured"}

// syncError is a simple error type for internal sync errors.
type syncError struct{ msg string }

func (e *syncError) Error() string { return e.msg }

// errSubscriptionLost indicates the Redis subscription channel was closed.
var errSubscriptionLost = &syncError{msg: "redis subscription lost"}

// handleSyncMessage processes a single sync notification by reloading from DB.
func (r *Registry) handleSyncMessage(ctx context.Context, raw string) {
	var msg registrySyncMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		r.logger.Warn("registry sync: invalid message", "raw", raw, "error", err)
		return
	}

	r.logger.Debug("registry sync: received notification",
		"plugin", msg.PluginName, "action", msg.Action)

	if err := r.LoadFromDB(ctx, r.store); err != nil {
		r.logger.Error("registry sync: failed to reload from DB",
			"plugin", msg.PluginName, "error", err)
	}
}

// pollFallback polls the database at pollInterval to keep the registry in sync.
// Returns true if the context was cancelled (caller should exit).
func (r *Registry) pollFallback(ctx context.Context) bool {
	interval := r.pollInterval
	if interval <= 0 {
		interval = 30 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return true
		case <-ticker.C:
			if err := r.LoadFromDB(ctx, r.store); err != nil {
				r.logger.Warn("registry sync: DB poll failed", "error", err)
			}

			// Try to re-establish Redis subscription.
			if r.redisPubSub != nil {
				return false // break out to retry subscribeRedis
			}
		}
	}
}
