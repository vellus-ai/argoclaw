package plugins

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
)

// ─────────────────────────────────────────────────────────────────────────────
// MessageBus interface — minimal abstraction over ArgoClaw's internal bus
// ─────────────────────────────────────────────────────────────────────────────

// PluginMessageBus abstracts the subscribe/publish operations needed by EventBridge.
// The ArgoClaw bus.MessageBus does not expose a topic-based Subscribe that returns
// an unsubscribe func, so we define this minimal interface for plugin event bridging.
type PluginMessageBus interface {
	// Subscribe registers a handler for a topic and returns an unsubscribe function.
	Subscribe(topic string, handler func(event interface{})) (unsubscribe func(), err error)

	// Publish sends an event to all subscribers of the given topic.
	Publish(topic string, event interface{}) error
}

// ─────────────────────────────────────────────────────────────────────────────
// Sentinel errors
// ─────────────────────────────────────────────────────────────────────────────

// ErrEventNamespaceViolation is returned when a plugin attempts to publish
// events outside its own namespace (plugin.{name}.*).
var ErrEventNamespaceViolation = fmt.Errorf("event namespace violation")

// ─────────────────────────────────────────────────────────────────────────────
// Subscription tracking
// ─────────────────────────────────────────────────────────────────────────────

// subscription tracks a single event subscription with its unsubscribe handle.
type subscription struct {
	topic       string
	unsubscribe func()
}

// ─────────────────────────────────────────────────────────────────────────────
// EventBridge
// ─────────────────────────────────────────────────────────────────────────────

// EventBridge connects plugin event subscriptions to the ArgoClaw message bus.
// It manages the lifecycle of event subscriptions per plugin, ensuring plugins
// can only publish events within their own namespace (plugin.{name}.*).
type EventBridge struct {
	bus    PluginMessageBus
	logger *slog.Logger
	mu     sync.Mutex
	subs   map[string][]subscription // key: pluginName → active subscriptions
}

// NewEventBridge creates a new EventBridge connected to the given message bus.
// If logger is nil, a default no-op logger is used.
func NewEventBridge(bus PluginMessageBus, logger *slog.Logger) *EventBridge {
	if logger == nil {
		logger = slog.Default()
	}
	return &EventBridge{
		bus:    bus,
		logger: logger,
		subs:   make(map[string][]subscription),
	}
}

// Connect subscribes to all events declared in the plugin manifest's
// permissions.events.subscribe list. Each subscription is tracked so it
// can be cleanly removed on Disconnect.
func (eb *EventBridge) Connect(ctx context.Context, name string, manifest *PluginManifest) error {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	// Disconnect any existing subscriptions first (reconnect scenario).
	eb.disconnectLocked(name)

	topics := manifest.Spec.Permissions.Events.Subscribe
	if len(topics) == 0 {
		eb.logger.Info("event_bridge.connect: no subscriptions declared",
			"plugin", name)
		return nil
	}

	subs := make([]subscription, 0, len(topics))
	for _, topic := range topics {
		unsub, err := eb.bus.Subscribe(topic, func(event interface{}) {
			eb.logger.Debug("event_bridge.received",
				"plugin", name,
				"topic", topic,
			)
		})
		if err != nil {
			// Rollback all subscriptions made so far.
			for _, s := range subs {
				s.unsubscribe()
			}
			return fmt.Errorf("event_bridge.connect: subscribe to %q: %w", topic, err)
		}
		subs = append(subs, subscription{topic: topic, unsubscribe: unsub})
	}

	eb.subs[name] = subs
	eb.logger.Info("event_bridge.connect: subscriptions active",
		"plugin", name,
		"count", len(subs),
	)
	return nil
}

// Disconnect removes all event subscriptions for the named plugin.
// It is idempotent — calling Disconnect on an already-disconnected or
// never-connected plugin is a no-op.
func (eb *EventBridge) Disconnect(name string) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.disconnectLocked(name)
}

// disconnectLocked removes subscriptions while the mutex is already held.
func (eb *EventBridge) disconnectLocked(name string) {
	subs, ok := eb.subs[name]
	if !ok {
		return
	}
	for _, s := range subs {
		s.unsubscribe()
	}
	delete(eb.subs, name)
	eb.logger.Info("event_bridge.disconnect: subscriptions removed",
		"plugin", name,
		"count", len(subs),
	)
}

// ValidatePublish checks that the event type falls within the plugin's
// allowed namespace: plugin.{name}.* (e.g., plugin.prompt-vault.version_created).
// Returns ErrEventNamespaceViolation if the event type is outside the namespace.
func (eb *EventBridge) ValidatePublish(name string, eventType string) error {
	requiredPrefix := pluginEventPrefix + name + "."
	if eventType == "" || !strings.HasPrefix(eventType, requiredPrefix) || len(eventType) <= len(requiredPrefix) {
		return fmt.Errorf("%w: plugin %q can only publish events matching %q, got %q",
			ErrEventNamespaceViolation, name, pluginEventPrefix+name+".*", eventType)
	}
	return nil
}
