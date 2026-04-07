package plugins_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/vellus-ai/argoclaw/internal/plugins"
	"github.com/vellus-ai/argoclaw/internal/store"
)

// ─────────────────────────────────────────────────────────────────────────────
// Mock RedisPublishSubscriber
// ─────────────────────────────────────────────────────────────────────────────

// mockRedisPubSub implements plugins.RedisPublishSubscriber for testing.
type mockRedisPubSub struct {
	mu           sync.Mutex
	published    []publishedMsg
	publishErr   error
	subscribeErr error
	msgCh        chan string // channel returned by Subscribe
	cancelCalled bool
}

type publishedMsg struct {
	Channel string
	Message string
}

func newMockRedisPubSub() *mockRedisPubSub {
	return &mockRedisPubSub{
		msgCh: make(chan string, 100),
	}
}

func (m *mockRedisPubSub) Publish(_ context.Context, channel string, message interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.publishErr != nil {
		return m.publishErr
	}
	m.published = append(m.published, publishedMsg{
		Channel: channel,
		Message: fmt.Sprintf("%v", message),
	})
	return nil
}

func (m *mockRedisPubSub) Subscribe(_ context.Context, channel string) (<-chan string, func(), error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.subscribeErr != nil {
		return nil, nil, m.subscribeErr
	}
	cancel := func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.cancelCalled = true
	}
	return m.msgCh, cancel, nil
}

func (m *mockRedisPubSub) getPublished() []publishedMsg {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]publishedMsg, len(m.published))
	copy(cp, m.published)
	return cp
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests: PublishChange
// Validates: Requirements 14.3, 14.4
// ─────────────────────────────────────────────────────────────────────────────

func TestRegistry_PublishChange(t *testing.T) {
	tests := []struct {
		name        string
		rps         *mockRedisPubSub // nil means no Redis configured
		pluginName  string
		action      string
		publishErr  error
		wantPublish int
	}{
		{
			name:        "publishes to Redis channel",
			rps:         newMockRedisPubSub(),
			pluginName:  "vault",
			action:      "enable",
			wantPublish: 1,
		},
		{
			name:        "no-op when Redis not configured",
			rps:         nil,
			pluginName:  "vault",
			action:      "enable",
			wantPublish: 0,
		},
		{
			name:        "publish error is logged but does not fail",
			rps:         newMockRedisPubSub(),
			pluginName:  "vault",
			action:      "disable",
			publishErr:  fmt.Errorf("connection refused"),
			wantPublish: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := plugins.NewRegistry()

			if tt.rps != nil {
				tt.rps.publishErr = tt.publishErr
				r.SetSync(tt.rps, "plugin_registry_sync", 30*time.Second, newMockPluginStore())
			}

			// PublishChange should never panic or return error.
			r.PublishChange(context.Background(), tt.pluginName, tt.action)

			if tt.rps != nil {
				published := tt.rps.getPublished()
				if len(published) != tt.wantPublish {
					t.Errorf("published %d messages, want %d", len(published), tt.wantPublish)
				}
				if tt.wantPublish > 0 {
					if published[0].Channel != "plugin_registry_sync" {
						t.Errorf("channel = %q, want %q", published[0].Channel, "plugin_registry_sync")
					}
					// Verify the message is valid JSON with expected fields.
					var msg struct {
						PluginName string `json:"plugin_name"`
						Action     string `json:"action"`
						Timestamp  int64  `json:"timestamp"`
					}
					if err := json.Unmarshal([]byte(published[0].Message), &msg); err != nil {
						t.Fatalf("published message is not valid JSON: %v", err)
					}
					if msg.PluginName != tt.pluginName {
						t.Errorf("message plugin_name = %q, want %q", msg.PluginName, tt.pluginName)
					}
					if msg.Action != tt.action {
						t.Errorf("message action = %q, want %q", msg.Action, tt.action)
					}
					if msg.Timestamp == 0 {
						t.Error("message timestamp is zero")
					}
				}
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests: StartSync / subscribeChanges with Redis
// Validates: Requirements 14.3, 14.4
// ─────────────────────────────────────────────────────────────────────────────

func TestRegistry_StartSync_ReceivesRedisNotification(t *testing.T) {
	tenantID := uuid.New()
	now := time.Now()

	ms := newMockPluginStore()
	ms.tenantPlugins = []store.TenantPlugin{
		{
			ID:            uuid.New(),
			TenantID:      tenantID,
			PluginName:    "vault",
			PluginVersion: "1.0.0",
			State:         "enabled",
			EnabledAt:     &now,
			Config:        json.RawMessage(`{}`),
			Permissions:   json.RawMessage(`{}`),
		},
	}
	ms.catalogEntries["vault"] = makeCatalogEntry("vault", "1.0.0")

	rps := newMockRedisPubSub()
	r := plugins.NewRegistry()
	r.SetSync(rps, "plugin_registry_sync", 30*time.Second, ms)

	ctx := store.WithTenantID(context.Background(), tenantID)
	r.StartSync(ctx)
	defer r.StopSync()

	// Send a sync notification via the mock Redis channel.
	syncMsg := `{"plugin_name":"vault","action":"enable","timestamp":1234567890}`
	rps.msgCh <- syncMsg

	// Wait for the registry to reload from DB.
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for registry to reload from DB after Redis notification")
		default:
			if r.Count() > 0 {
				entry, ok := r.Get("vault")
				if ok && entry.Status == plugins.RegistryActive {
					return // success
				}
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestRegistry_StartSync_StopSyncCancelsGoroutine(t *testing.T) {
	rps := newMockRedisPubSub()
	ms := newMockPluginStore()

	r := plugins.NewRegistry()
	r.SetSync(rps, "plugin_registry_sync", 30*time.Second, ms)

	ctx := context.Background()
	r.StartSync(ctx)

	// Give the goroutine time to start.
	time.Sleep(50 * time.Millisecond)

	// StopSync should cancel the goroutine without panic.
	r.StopSync()

	// Calling StopSync again should be safe (no-op).
	r.StopSync()
}

func TestRegistry_StartSync_NoStoreDisablesSync(t *testing.T) {
	r := plugins.NewRegistry()
	// Don't call SetSync — store is nil.

	// StartSync should be a no-op when store is not configured.
	r.StartSync(context.Background())
	r.StopSync() // should not panic
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests: DB polling fallback
// Validates: Requirements 14.3, 14.4
// ─────────────────────────────────────────────────────────────────────────────

func TestRegistry_PollFallback_WhenRedisUnavailable(t *testing.T) {
	tenantID := uuid.New()
	now := time.Now()

	ms := newMockPluginStore()
	ms.tenantPlugins = []store.TenantPlugin{
		{
			ID:            uuid.New(),
			TenantID:      tenantID,
			PluginName:    "vault",
			PluginVersion: "1.0.0",
			State:         "enabled",
			EnabledAt:     &now,
			Config:        json.RawMessage(`{}`),
			Permissions:   json.RawMessage(`{}`),
		},
	}
	ms.catalogEntries["vault"] = makeCatalogEntry("vault", "1.0.0")

	r := plugins.NewRegistry()
	// No Redis configured — should fall back to DB polling.
	r.SetSync(nil, "plugin_registry_sync", 100*time.Millisecond, ms)

	ctx := store.WithTenantID(context.Background(), tenantID)
	r.StartSync(ctx)
	defer r.StopSync()

	// Wait for at least one poll cycle to complete.
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for DB poll to populate registry")
		default:
			if r.Count() > 0 {
				entry, ok := r.Get("vault")
				if ok && entry.Status == plugins.RegistryActive {
					return // success — DB poll loaded the plugin
				}
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestRegistry_PollFallback_WhenRedisSubscribeFails(t *testing.T) {
	tenantID := uuid.New()
	now := time.Now()

	ms := newMockPluginStore()
	ms.tenantPlugins = []store.TenantPlugin{
		{
			ID:            uuid.New(),
			TenantID:      tenantID,
			PluginName:    "search",
			PluginVersion: "1.0.0",
			State:         "enabled",
			EnabledAt:     &now,
			Config:        json.RawMessage(`{}`),
			Permissions:   json.RawMessage(`{}`),
		},
	}
	ms.catalogEntries["search"] = makeCatalogEntry("search", "1.0.0")

	rps := newMockRedisPubSub()
	rps.subscribeErr = fmt.Errorf("connection refused")

	r := plugins.NewRegistry()
	r.SetSync(rps, "plugin_registry_sync", 100*time.Millisecond, ms)

	ctx := store.WithTenantID(context.Background(), tenantID)
	r.StartSync(ctx)
	defer r.StopSync()

	// Redis subscribe fails → should fall back to DB polling.
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for DB poll fallback after Redis subscribe failure")
		default:
			if r.Count() > 0 {
				entry, ok := r.Get("search")
				if ok && entry.Status == plugins.RegistryActive {
					return // success
				}
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests: handleSyncMessage (via Redis notification)
// Validates: Requirements 14.3, 14.4
// ─────────────────────────────────────────────────────────────────────────────

func TestRegistry_HandleSyncMessage_InvalidJSON(t *testing.T) {
	rps := newMockRedisPubSub()
	ms := newMockPluginStore()

	r := plugins.NewRegistry()
	r.SetSync(rps, "plugin_registry_sync", 30*time.Second, ms)

	ctx := context.Background()
	r.StartSync(ctx)
	defer r.StopSync()

	// Send invalid JSON — should be logged as warning but not crash.
	rps.msgCh <- "not-valid-json"

	// Give time for the message to be processed.
	time.Sleep(100 * time.Millisecond)

	// Registry should still be empty (no crash, no load).
	if r.Count() != 0 {
		t.Errorf("Count() = %d after invalid message, want 0", r.Count())
	}
}

func TestRegistry_HandleSyncMessage_DBLoadError(t *testing.T) {
	rps := newMockRedisPubSub()
	ms := newMockPluginStore()
	ms.listErr = fmt.Errorf("database unavailable")

	r := plugins.NewRegistry()
	r.SetSync(rps, "plugin_registry_sync", 30*time.Second, ms)

	ctx := context.Background()
	r.StartSync(ctx)
	defer r.StopSync()

	// Send a valid sync message — DB load will fail.
	syncMsg := `{"plugin_name":"vault","action":"enable","timestamp":1234567890}`
	rps.msgCh <- syncMsg

	// Give time for the message to be processed.
	time.Sleep(100 * time.Millisecond)

	// Registry should still be empty (DB load failed gracefully).
	if r.Count() != 0 {
		t.Errorf("Count() = %d after DB load error, want 0", r.Count())
	}
}

func TestRegistry_SubscriptionLost_FallsBackToPoll(t *testing.T) {
	tenantID := uuid.New()
	now := time.Now()

	ms := newMockPluginStore()
	ms.tenantPlugins = []store.TenantPlugin{
		{
			ID:            uuid.New(),
			TenantID:      tenantID,
			PluginName:    "vault",
			PluginVersion: "1.0.0",
			State:         "enabled",
			EnabledAt:     &now,
			Config:        json.RawMessage(`{}`),
			Permissions:   json.RawMessage(`{}`),
		},
	}
	ms.catalogEntries["vault"] = makeCatalogEntry("vault", "1.0.0")

	rps := newMockRedisPubSub()
	r := plugins.NewRegistry()
	r.SetSync(rps, "plugin_registry_sync", 100*time.Millisecond, ms)

	ctx := store.WithTenantID(context.Background(), tenantID)
	r.StartSync(ctx)
	defer r.StopSync()

	// Give the goroutine time to subscribe.
	time.Sleep(50 * time.Millisecond)

	// Close the message channel to simulate subscription loss.
	close(rps.msgCh)

	// After subscription loss, should fall back to DB polling.
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for DB poll fallback after subscription loss")
		default:
			if r.Count() > 0 {
				entry, ok := r.Get("vault")
				if ok && entry.Status == plugins.RegistryActive {
					return // success
				}
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}
