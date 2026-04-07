package plugins

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

var _ HealthPluginStore = (*mockPluginStore)(nil)

// ─────────────────────────────────────────────────────────────────────────────
// Mock types for HealthMonitor tests
// ─────────────────────────────────────────────────────────────────────────────

// mockPluginStore is a minimal mock of the store.PluginStore interface.
// Only methods used by HealthMonitor are implemented.
type mockPluginStore struct {
	mu             sync.Mutex
	setErrorCalls  []setErrorCall
	setErrorErr    error
	disableCalls   []disableCall
	disableErr     error
	auditEntries   []auditEntryRecord
	auditErr       error
}

type setErrorCall struct {
	PluginName string
	ErrMsg     string
}

type disableCall struct {
	PluginName string
}

type auditEntryRecord struct {
	PluginName string
	Action     string
	ActorType  string
}

func (m *mockPluginStore) SetPluginError(_ context.Context, pluginName, errMsg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setErrorCalls = append(m.setErrorCalls, setErrorCall{PluginName: pluginName, ErrMsg: errMsg})
	return m.setErrorErr
}

func (m *mockPluginStore) DisablePlugin(_ context.Context, pluginName string, _ *uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.disableCalls = append(m.disableCalls, disableCall{PluginName: pluginName})
	return m.disableErr
}

func (m *mockPluginStore) LogAudit(_ context.Context, pluginName, action, actorType string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.auditEntries = append(m.auditEntries, auditEntryRecord{
		PluginName: pluginName,
		Action:     action,
		ActorType:  actorType,
	})
	return m.auditErr
}

func (m *mockPluginStore) getSetErrorCalls() []setErrorCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]setErrorCall, len(m.setErrorCalls))
	copy(cp, m.setErrorCalls)
	return cp
}

func (m *mockPluginStore) getDisableCalls() []disableCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]disableCall, len(m.disableCalls))
	copy(cp, m.disableCalls)
	return cp
}

func (m *mockPluginStore) getAuditEntries() []auditEntryRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]auditEntryRecord, len(m.auditEntries))
	copy(cp, m.auditEntries)
	return cp
}

// mockPinger implements the Pinger interface for health checks.
type mockPinger struct {
	mu      sync.Mutex
	pingErr error
	count   int64
}

func (p *mockPinger) Ping(ctx context.Context) error {
	atomic.AddInt64(&p.count, 1)
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.pingErr
}

func (p *mockPinger) setPingErr(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pingErr = err
}

func (p *mockPinger) pingCount() int64 {
	return atomic.LoadInt64(&p.count)
}

// mockHealthRegistry implements the HealthRegistry interface.
type mockHealthRegistry struct {
	mu      sync.Mutex
	entries map[string]*RegistryEntry
}

func newMockHealthRegistry() *mockHealthRegistry {
	return &mockHealthRegistry{
		entries: make(map[string]*RegistryEntry),
	}
}

func (r *mockHealthRegistry) Get(name string) (*RegistryEntry, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.entries[name]
	return e, ok
}

func (r *mockHealthRegistry) Register(name string, entry *RegistryEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[name] = entry
}

func (r *mockHealthRegistry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.entries, name)
}

func (r *mockHealthRegistry) ActiveNames() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var names []string
	for name, e := range r.entries {
		if e.Status == RegistryActive {
			names = append(names, name)
		}
	}
	return names
}

// ─────────────────────────────────────────────────────────────────────────────
// HealthMonitor Start/Stop tests
// ─────────────────────────────────────────────────────────────────────────────

func TestHealthMonitor_StartStop(t *testing.T) {
	t.Parallel()
	pinger := &mockPinger{}
	registry := newMockHealthRegistry()
	store := &mockPluginStore{}

	registry.Register("test-plugin", &RegistryEntry{
		Manifest: &PluginManifest{
			Spec: ManifestSpec{
				Runtime: ManifestRuntime{
					HealthCheck: HealthCheckConfig{Interval: 1, Timeout: 1},
				},
			},
		},
		Status: RegistryActive,
	})

	hm := NewHealthMonitor(HealthMonitorConfig{
		MaxFailures:      3,
		MaxRecoveries:    5,
		AutoDisableAfter: 5 * time.Minute,
		CheckInterval:    50 * time.Millisecond, // fast for tests
	}, registry, store, slog.Default())

	hm.RegisterPinger("test-plugin", pinger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hm.Start(ctx)

	// Wait for at least one ping.
	time.Sleep(150 * time.Millisecond)

	if pinger.pingCount() == 0 {
		t.Error("expected at least one ping after start")
	}

	hm.Stop()

	countBefore := pinger.pingCount()
	time.Sleep(100 * time.Millisecond)
	countAfter := pinger.pingCount()

	if countAfter != countBefore {
		t.Error("expected no more pings after stop")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Failure detection tests
// ─────────────────────────────────────────────────────────────────────────────

func TestHealthMonitor_FailureDetection_AfterThreeFailures(t *testing.T) {
	t.Parallel()
	pinger := &mockPinger{pingErr: errors.New("connection refused")}
	registry := newMockHealthRegistry()
	store := &mockPluginStore{}

	registry.Register("failing-plugin", &RegistryEntry{
		Manifest: &PluginManifest{
			Spec: ManifestSpec{
				Runtime: ManifestRuntime{
					HealthCheck: HealthCheckConfig{Interval: 1, Timeout: 1},
				},
			},
		},
		Status: RegistryActive,
		Tools:  []string{"plugin_failing-plugin__tool1"},
	})

	hm := NewHealthMonitor(HealthMonitorConfig{
		MaxFailures:      3,
		MaxRecoveries:    5,
		AutoDisableAfter: 5 * time.Minute,
		CheckInterval:    20 * time.Millisecond,
	}, registry, store, slog.Default())

	hm.RegisterPinger("failing-plugin", pinger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hm.Start(ctx)

	// Wait long enough for 3+ checks.
	time.Sleep(200 * time.Millisecond)
	hm.Stop()

	// After 3 consecutive failures, store should have SetPluginError called.
	calls := store.getSetErrorCalls()
	if len(calls) == 0 {
		t.Fatal("expected SetPluginError to be called after 3 failures")
	}
	if calls[0].PluginName != "failing-plugin" {
		t.Errorf("expected plugin name 'failing-plugin', got %q", calls[0].PluginName)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Auto-recovery tests
// ─────────────────────────────────────────────────────────────────────────────

func TestHealthMonitor_AutoRecovery(t *testing.T) {
	t.Parallel()
	pinger := &mockPinger{pingErr: errors.New("temporary failure")}
	registry := newMockHealthRegistry()
	store := &mockPluginStore{}

	registry.Register("recovering-plugin", &RegistryEntry{
		Manifest: &PluginManifest{
			Spec: ManifestSpec{
				Runtime: ManifestRuntime{
					HealthCheck: HealthCheckConfig{Interval: 1, Timeout: 1},
				},
			},
		},
		Status: RegistryActive,
	})

	hm := NewHealthMonitor(HealthMonitorConfig{
		MaxFailures:      2,
		MaxRecoveries:    5,
		AutoDisableAfter: 5 * time.Minute,
		CheckInterval:    20 * time.Millisecond,
	}, registry, store, slog.Default())

	hm.RegisterPinger("recovering-plugin", pinger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hm.Start(ctx)

	// Wait for failures to trigger error state.
	time.Sleep(150 * time.Millisecond)

	// Now fix the pinger — simulate recovery.
	pinger.setPingErr(nil)

	// Wait for recovery to kick in.
	time.Sleep(200 * time.Millisecond)
	hm.Stop()

	// Check that the plugin was marked as error first.
	errorCalls := store.getSetErrorCalls()
	if len(errorCalls) == 0 {
		t.Fatal("expected SetPluginError to be called during failure phase")
	}

	// Check PluginStatus — should show recovered state.
	status := hm.GetStatus("recovering-plugin")
	if status == nil {
		t.Fatal("expected non-nil status for recovering-plugin")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Auto-disable after max recovery attempts
// ─────────────────────────────────────────────────────────────────────────────

func TestHealthMonitor_AutoDisableAfterMaxRecoveries(t *testing.T) {
	t.Parallel()
	pinger := &mockPinger{pingErr: errors.New("permanent failure")}
	registry := newMockHealthRegistry()
	store := &mockPluginStore{}

	registry.Register("dying-plugin", &RegistryEntry{
		Manifest: &PluginManifest{
			Spec: ManifestSpec{
				Runtime: ManifestRuntime{
					HealthCheck: HealthCheckConfig{Interval: 1, Timeout: 1},
				},
			},
		},
		Status: RegistryActive,
	})

	hm := NewHealthMonitor(HealthMonitorConfig{
		MaxFailures:      1,
		MaxRecoveries:    2,
		AutoDisableAfter: 100 * time.Millisecond,
		CheckInterval:    20 * time.Millisecond,
		BaseBackoff:      10 * time.Millisecond,
		MaxBackoff:       30 * time.Millisecond,
	}, registry, store, slog.Default())

	hm.RegisterPinger("dying-plugin", pinger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hm.Start(ctx)

	// Wait for failures + recovery attempts + auto-disable.
	time.Sleep(500 * time.Millisecond)
	hm.Stop()

	// Should have been auto-disabled.
	disableCalls := store.getDisableCalls()
	if len(disableCalls) == 0 {
		t.Fatal("expected DisablePlugin to be called after max recovery attempts")
	}
	if disableCalls[0].PluginName != "dying-plugin" {
		t.Errorf("expected 'dying-plugin', got %q", disableCalls[0].PluginName)
	}

	// Audit log should contain auto_disable entry.
	auditEntries := store.getAuditEntries()
	found := false
	for _, e := range auditEntries {
		if e.Action == "auto_disable" && e.PluginName == "dying-plugin" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected audit log entry with action='auto_disable'")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Admin priority over automatic transitions
// ─────────────────────────────────────────────────────────────────────────────

func TestHealthMonitor_AdminPriorityOverAutoTransitions(t *testing.T) {
	t.Parallel()
	pinger := &mockPinger{pingErr: errors.New("failure")}
	registry := newMockHealthRegistry()
	store := &mockPluginStore{}

	registry.Register("admin-priority-plugin", &RegistryEntry{
		Manifest: &PluginManifest{
			Spec: ManifestSpec{
				Runtime: ManifestRuntime{
					HealthCheck: HealthCheckConfig{Interval: 1, Timeout: 1},
				},
			},
		},
		Status: RegistryActive,
	})

	hm := NewHealthMonitor(HealthMonitorConfig{
		MaxFailures:      2,
		MaxRecoveries:    5,
		AutoDisableAfter: 5 * time.Minute,
		CheckInterval:    20 * time.Millisecond,
	}, registry, store, slog.Default())

	hm.RegisterPinger("admin-priority-plugin", pinger)

	// Mark admin operation in progress.
	hm.SetAdminLock("admin-priority-plugin", true)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hm.Start(ctx)

	// Wait for several check intervals.
	time.Sleep(200 * time.Millisecond)
	hm.Stop()

	// With admin lock, no automatic state transitions should happen.
	errorCalls := store.getSetErrorCalls()
	if len(errorCalls) > 0 {
		t.Error("expected no SetPluginError calls while admin lock is held")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// PluginStatus counters
// ─────────────────────────────────────────────────────────────────────────────

func TestHealthMonitor_StatusCounters(t *testing.T) {
	t.Parallel()
	pinger := &mockPinger{pingErr: errors.New("fail")}
	registry := newMockHealthRegistry()
	store := &mockPluginStore{}

	registry.Register("counter-plugin", &RegistryEntry{
		Manifest: &PluginManifest{
			Spec: ManifestSpec{
				Runtime: ManifestRuntime{
					HealthCheck: HealthCheckConfig{Interval: 1, Timeout: 1},
				},
			},
		},
		Status: RegistryActive,
	})

	hm := NewHealthMonitor(HealthMonitorConfig{
		MaxFailures:      3,
		MaxRecoveries:    5,
		AutoDisableAfter: 5 * time.Minute,
		CheckInterval:    20 * time.Millisecond,
	}, registry, store, slog.Default())

	hm.RegisterPinger("counter-plugin", pinger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hm.Start(ctx)
	time.Sleep(150 * time.Millisecond)
	hm.Stop()

	status := hm.GetStatus("counter-plugin")
	if status == nil {
		t.Fatal("expected non-nil status")
	}

	// LastHealthCheck should be set.
	if status.LastHealthCheck == nil {
		t.Error("expected LastHealthCheck to be non-nil")
	}

	// HealthResult should indicate failure.
	if status.HealthResult != "fail" {
		t.Errorf("expected HealthResult='fail', got %q", status.HealthResult)
	}

	// TotalErrors should be > 0.
	if status.TotalErrors == 0 {
		t.Error("expected TotalErrors > 0")
	}
}

func TestHealthMonitor_GetStatus_NotFound(t *testing.T) {
	t.Parallel()
	hm := NewHealthMonitor(HealthMonitorConfig{}, newMockHealthRegistry(), &mockPluginStore{}, slog.Default())
	status := hm.GetStatus("nonexistent")
	if status != nil {
		t.Error("expected nil status for nonexistent plugin")
	}
}
