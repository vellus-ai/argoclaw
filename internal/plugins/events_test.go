package plugins_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/vellus-ai/argoclaw/internal/plugins"
)

// ─────────────────────────────────────────────────────────────────────────────
// Fake MessageBus for testing
// ─────────────────────────────────────────────────────────────────────────────

type fakeSubscription struct {
	topic   string
	handler func(event interface{})
}

type fakeMessageBus struct {
	mu            sync.Mutex
	subscriptions []fakeSubscription
	unsubCalls    int
}

func (f *fakeMessageBus) Subscribe(topic string, handler func(event interface{})) (func(), error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	idx := len(f.subscriptions)
	f.subscriptions = append(f.subscriptions, fakeSubscription{topic: topic, handler: handler})
	return func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.unsubCalls++
		// Mark as removed by clearing topic.
		if idx < len(f.subscriptions) {
			f.subscriptions[idx].topic = ""
		}
	}, nil
}

func (f *fakeMessageBus) Publish(topic string, event interface{}) error {
	return nil
}

func (f *fakeMessageBus) activeTopics() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var topics []string
	for _, s := range f.subscriptions {
		if s.topic != "" {
			topics = append(topics, s.topic)
		}
	}
	return topics
}

// ─────────────────────────────────────────────────────────────────────────────
// Helper — build a manifest with event permissions
// ─────────────────────────────────────────────────────────────────────────────

func makeManifestWithEvents(name string, subscribe, publish []string) *plugins.PluginManifest {
	return &plugins.PluginManifest{
		Name: name,
		Metadata: plugins.ManifestMetadata{
			Name:    name,
			Version: "1.0.0",
		},
		Spec: plugins.ManifestSpec{
			Permissions: plugins.ManifestPermissions{
				Events: plugins.EventPermissions{
					Subscribe: subscribe,
					Publish:   publish,
				},
			},
		},
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests — Connect
// ─────────────────────────────────────────────────────────────────────────────

func TestEventBridge_Connect_SubscribesPerManifest(t *testing.T) {
	bus := &fakeMessageBus{}
	bridge := plugins.NewEventBridge(bus, nil)

	manifest := makeManifestWithEvents("prompt-vault",
		[]string{"agent.activity", "plugin.prompt-vault.cache_hit"},
		[]string{"plugin.prompt-vault.version_created"},
	)

	err := bridge.Connect(context.Background(), "prompt-vault", manifest)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	topics := bus.activeTopics()
	if len(topics) != 2 {
		t.Fatalf("expected 2 subscriptions, got %d: %v", len(topics), topics)
	}

	// Verify topics match subscribe list.
	want := map[string]bool{
		"agent.activity":                    true,
		"plugin.prompt-vault.cache_hit":     true,
	}
	for _, topic := range topics {
		if !want[topic] {
			t.Errorf("unexpected subscription topic: %q", topic)
		}
	}
}

func TestEventBridge_Connect_NoSubscriptions(t *testing.T) {
	bus := &fakeMessageBus{}
	bridge := plugins.NewEventBridge(bus, nil)

	manifest := makeManifestWithEvents("my-plugin", nil, nil)

	err := bridge.Connect(context.Background(), "my-plugin", manifest)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	topics := bus.activeTopics()
	if len(topics) != 0 {
		t.Fatalf("expected 0 subscriptions, got %d", len(topics))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests — Disconnect
// ─────────────────────────────────────────────────────────────────────────────

func TestEventBridge_Disconnect_RemovesAllSubscriptions(t *testing.T) {
	bus := &fakeMessageBus{}
	bridge := plugins.NewEventBridge(bus, nil)

	manifest := makeManifestWithEvents("prompt-vault",
		[]string{"agent.activity", "core.config_changed"},
		nil,
	)

	if err := bridge.Connect(context.Background(), "prompt-vault", manifest); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	bridge.Disconnect("prompt-vault")

	bus.mu.Lock()
	unsubCalls := bus.unsubCalls
	bus.mu.Unlock()

	if unsubCalls != 2 {
		t.Errorf("expected 2 unsubscribe calls, got %d", unsubCalls)
	}
}

func TestEventBridge_Disconnect_Idempotent(t *testing.T) {
	bus := &fakeMessageBus{}
	bridge := plugins.NewEventBridge(bus, nil)

	manifest := makeManifestWithEvents("prompt-vault",
		[]string{"agent.activity"},
		nil,
	)

	if err := bridge.Connect(context.Background(), "prompt-vault", manifest); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// First disconnect.
	bridge.Disconnect("prompt-vault")
	// Second disconnect — should not panic or error.
	bridge.Disconnect("prompt-vault")

	bus.mu.Lock()
	unsubCalls := bus.unsubCalls
	bus.mu.Unlock()

	// Only 1 real unsubscribe call (from first Disconnect).
	if unsubCalls != 1 {
		t.Errorf("expected 1 unsubscribe call (idempotent), got %d", unsubCalls)
	}
}

func TestEventBridge_Disconnect_NeverConnected(t *testing.T) {
	bus := &fakeMessageBus{}
	bridge := plugins.NewEventBridge(bus, nil)

	// Should not panic.
	bridge.Disconnect("nonexistent-plugin")
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests — ValidatePublish
// ─────────────────────────────────────────────────────────────────────────────

func TestEventBridge_ValidatePublish_OwnNamespace(t *testing.T) {
	bridge := plugins.NewEventBridge(&fakeMessageBus{}, nil)

	err := bridge.ValidatePublish("my-plugin", "plugin.my-plugin.event_created")
	if err != nil {
		t.Fatalf("expected publish to own namespace to succeed, got: %v", err)
	}
}

func TestEventBridge_ValidatePublish_OwnNamespaceDeepEvent(t *testing.T) {
	bridge := plugins.NewEventBridge(&fakeMessageBus{}, nil)

	err := bridge.ValidatePublish("prompt-vault", "plugin.prompt-vault.version.published")
	if err != nil {
		t.Fatalf("expected publish to own deep namespace to succeed, got: %v", err)
	}
}

func TestEventBridge_ValidatePublish_RejectOtherPlugin(t *testing.T) {
	bridge := plugins.NewEventBridge(&fakeMessageBus{}, nil)

	err := bridge.ValidatePublish("my-plugin", "plugin.other-plugin.event")
	if err == nil {
		t.Fatal("expected error for publishing to other plugin namespace")
	}
	if !errors.Is(err, plugins.ErrEventNamespaceViolation) {
		t.Errorf("expected ErrEventNamespaceViolation, got: %v", err)
	}
}

func TestEventBridge_ValidatePublish_RejectCoreEvent(t *testing.T) {
	bridge := plugins.NewEventBridge(&fakeMessageBus{}, nil)

	err := bridge.ValidatePublish("my-plugin", "core.config_changed")
	if err == nil {
		t.Fatal("expected error for publishing core event")
	}
	if !errors.Is(err, plugins.ErrEventNamespaceViolation) {
		t.Errorf("expected ErrEventNamespaceViolation, got: %v", err)
	}
}

func TestEventBridge_ValidatePublish_RejectEmptyEventType(t *testing.T) {
	bridge := plugins.NewEventBridge(&fakeMessageBus{}, nil)

	err := bridge.ValidatePublish("my-plugin", "")
	if err == nil {
		t.Fatal("expected error for empty event type")
	}
	if !errors.Is(err, plugins.ErrEventNamespaceViolation) {
		t.Errorf("expected ErrEventNamespaceViolation, got: %v", err)
	}
}

func TestEventBridge_ValidatePublish_RejectBarePluginPrefix(t *testing.T) {
	bridge := plugins.NewEventBridge(&fakeMessageBus{}, nil)

	// "plugin.my-plugin" without trailing dot and event name should fail.
	err := bridge.ValidatePublish("my-plugin", "plugin.my-plugin")
	if err == nil {
		t.Fatal("expected error for bare plugin prefix without event name")
	}
}

func TestEventBridge_ValidatePublish_RejectAgentEvent(t *testing.T) {
	bridge := plugins.NewEventBridge(&fakeMessageBus{}, nil)

	err := bridge.ValidatePublish("my-plugin", "agent.activity")
	if err == nil {
		t.Fatal("expected error for agent event")
	}
	if !errors.Is(err, plugins.ErrEventNamespaceViolation) {
		t.Errorf("expected ErrEventNamespaceViolation, got: %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests — Concurrency
// ─────────────────────────────────────────────────────────────────────────────

func TestEventBridge_ConcurrentConnectDisconnect(t *testing.T) {
	bus := &fakeMessageBus{}
	bridge := plugins.NewEventBridge(bus, nil)

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			name := "test-plugin"
			manifest := makeManifestWithEvents(name,
				[]string{"agent.activity"},
				[]string{"plugin.test-plugin.done"},
			)

			if idx%2 == 0 {
				_ = bridge.Connect(context.Background(), name, manifest)
			} else {
				bridge.Disconnect(name)
			}
		}(i)
	}

	wg.Wait()
	// No race detector failure = pass.
}
