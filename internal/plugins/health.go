package plugins

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ─────────────────────────────────────────────────────────────────────────────
// Interfaces for HealthMonitor dependencies
// ─────────────────────────────────────────────────────────────────────────────

// Pinger is the interface used by HealthMonitor to ping a plugin process.
type Pinger interface {
	Ping(ctx context.Context) error
}

// HealthPluginStore abstracts the store operations needed by HealthMonitor.
// This is a subset of store.PluginStore to avoid importing the full interface.
type HealthPluginStore interface {
	SetPluginError(ctx context.Context, pluginName, errMsg string) error
	DisablePlugin(ctx context.Context, pluginName string, actorID *uuid.UUID) error
	LogAudit(ctx context.Context, pluginName, action, actorType string) error
}

// HealthRegistry abstracts the plugin registry for health monitoring.
type HealthRegistry interface {
	Get(name string) (*RegistryEntry, bool)
	ActiveNames() []string
	Unregister(name string)
}

// ─────────────────────────────────────────────────────────────────────────────
// HealthMonitorConfig
// ─────────────────────────────────────────────────────────────────────────────

// HealthMonitorConfig holds configuration for the HealthMonitor.
type HealthMonitorConfig struct {
	MaxFailures      int           // consecutive failures before error state (default 3)
	MaxRecoveries    int           // max recovery attempts before auto-disable (default 5)
	AutoDisableAfter time.Duration // duration in error before auto-disable (default 5min)
	CheckInterval    time.Duration // override manifest interval for all plugins (0 = use manifest)
	BaseBackoff      time.Duration // base backoff for recovery attempts (default 1s)
	MaxBackoff       time.Duration // max backoff cap (default 30s)
}

// ─────────────────────────────────────────────────────────────────────────────
// pluginHealthState — per-plugin health tracking
// ─────────────────────────────────────────────────────────────────────────────

type pluginHealthState struct {
	mu               sync.Mutex
	consecutiveFails int
	recoveryAttempts int
	inErrorState     bool
	errorSince       time.Time
	lastCheck        time.Time
	lastResult       string
	totalErrors      int64
	adminLock        bool
}

// ─────────────────────────────────────────────────────────────────────────────
// HealthMonitor
// ─────────────────────────────────────────────────────────────────────────────

// HealthMonitor runs periodic health checks on active plugins. It detects
// failures, attempts recovery with exponential backoff, and auto-disables
// plugins that cannot recover.
type HealthMonitor struct {
	cfg      HealthMonitorConfig
	registry HealthRegistry
	store    HealthPluginStore
	logger   *slog.Logger

	mu      sync.Mutex
	pingers map[string]Pinger
	states  map[string]*pluginHealthState
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewHealthMonitor creates a HealthMonitor with the given configuration.
func NewHealthMonitor(cfg HealthMonitorConfig, registry HealthRegistry, store HealthPluginStore, logger *slog.Logger) *HealthMonitor {
	if cfg.MaxFailures <= 0 {
		cfg.MaxFailures = 3
	}
	if cfg.MaxRecoveries <= 0 {
		cfg.MaxRecoveries = 5
	}
	if cfg.AutoDisableAfter <= 0 {
		cfg.AutoDisableAfter = 5 * time.Minute
	}
	if cfg.BaseBackoff <= 0 {
		cfg.BaseBackoff = 1 * time.Second
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = 30 * time.Second
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &HealthMonitor{
		cfg:      cfg,
		registry: registry,
		store:    store,
		logger:   logger,
		pingers:  make(map[string]Pinger),
		states:   make(map[string]*pluginHealthState),
	}
}

// RegisterPinger registers a Pinger for a named plugin. Must be called before Start.
func (hm *HealthMonitor) RegisterPinger(name string, p Pinger) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.pingers[name] = p
	if _, ok := hm.states[name]; !ok {
		hm.states[name] = &pluginHealthState{}
	}
}

// SetAdminLock sets or clears the admin lock for a plugin. When the lock is
// held, automatic state transitions (error, auto-disable) are suppressed.
func (hm *HealthMonitor) SetAdminLock(name string, locked bool) {
	hm.mu.Lock()
	state, ok := hm.states[name]
	if !ok {
		state = &pluginHealthState{}
		hm.states[name] = state
	}
	hm.mu.Unlock()

	state.mu.Lock()
	state.adminLock = locked
	state.mu.Unlock()
}

// Start begins health check goroutines for all registered pingers.
func (hm *HealthMonitor) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	hm.cancel = cancel

	hm.mu.Lock()
	pingers := make(map[string]Pinger, len(hm.pingers))
	for name, p := range hm.pingers {
		pingers[name] = p
	}
	hm.mu.Unlock()

	for name, pinger := range pingers {
		hm.wg.Add(1)
		go hm.checkLoop(ctx, name, pinger)
	}

	hm.logger.Info("plugins.health.started", "plugins", len(pingers))
}

// Stop halts all health check goroutines and waits for them to finish.
func (hm *HealthMonitor) Stop() {
	if hm.cancel != nil {
		hm.cancel()
	}
	hm.wg.Wait()
	hm.logger.Info("plugins.health.stopped")
}

// GetStatus returns the current health status for a plugin.
func (hm *HealthMonitor) GetStatus(name string) *PluginStatus {
	hm.mu.Lock()
	state, ok := hm.states[name]
	hm.mu.Unlock()

	if !ok {
		return nil
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	var lastCheck *time.Time
	if !state.lastCheck.IsZero() {
		t := state.lastCheck
		lastCheck = &t
	}

	return &PluginStatus{
		LastHealthCheck: lastCheck,
		HealthResult:    state.lastResult,
		TotalErrors:     state.totalErrors,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Health check loop
// ─────────────────────────────────────────────────────────────────────────────

func (hm *HealthMonitor) checkLoop(ctx context.Context, name string, pinger Pinger) {
	defer hm.wg.Done()

	interval := hm.cfg.CheckInterval
	if interval <= 0 {
		interval = 60 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hm.checkPlugin(ctx, name, pinger)
		}
	}
}

func (hm *HealthMonitor) checkPlugin(ctx context.Context, name string, pinger Pinger) {
	hm.mu.Lock()
	state, ok := hm.states[name]
	if !ok {
		state = &pluginHealthState{}
		hm.states[name] = state
	}
	hm.mu.Unlock()

	// Perform ping with timeout.
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err := pinger.Ping(pingCtx)

	now := time.Now()
	state.mu.Lock()
	state.lastCheck = now

	if state.adminLock {
		// Admin lock held — skip automatic transitions.
		if err != nil {
			state.lastResult = "fail"
		} else {
			state.lastResult = "ok"
		}
		state.mu.Unlock()
		return
	}

	if err != nil {
		state.consecutiveFails++
		state.totalErrors++
		state.lastResult = "fail"

		fails := state.consecutiveFails
		inError := state.inErrorState
		recoveryAttempts := state.recoveryAttempts
		errorSince := state.errorSince
		state.mu.Unlock()

		hm.logger.Warn("plugins.health.check_failed",
			"plugin", name,
			"consecutive_failures", fails,
			"error", err.Error(),
		)

		if !inError && fails >= hm.cfg.MaxFailures {
			// Transition to error state.
			hm.transitionToError(ctx, name, state, err)
		} else if inError {
			// Already in error state — check for auto-disable.
			if recoveryAttempts >= hm.cfg.MaxRecoveries || time.Since(errorSince) >= hm.cfg.AutoDisableAfter {
				hm.autoDisable(ctx, name, state)
			} else {
				// Increment recovery attempt.
				state.mu.Lock()
				state.recoveryAttempts++
				state.mu.Unlock()
			}
		}
	} else {
		// Success — reset failure counters.
		wasInError := state.inErrorState
		state.consecutiveFails = 0
		state.lastResult = "ok"
		if wasInError {
			state.inErrorState = false
			state.recoveryAttempts = 0
			hm.logger.Info("plugins.health.recovered", "plugin", name)
		}
		state.mu.Unlock()
	}
}

// transitionToError marks a plugin as in error state and calls SetPluginError.
func (hm *HealthMonitor) transitionToError(ctx context.Context, name string, state *pluginHealthState, pingErr error) {
	state.mu.Lock()
	state.inErrorState = true
	state.errorSince = time.Now()
	state.recoveryAttempts = 0
	state.mu.Unlock()

	errMsg := "health check failed: " + pingErr.Error()
	if err := hm.store.SetPluginError(ctx, name, errMsg); err != nil {
		hm.logger.Error("plugins.health.set_error_failed",
			"plugin", name,
			"error", err,
		)
	}

	hm.logger.Warn("plugins.health.error_state",
		"plugin", name,
		"reason", errMsg,
	)
}

// autoDisable disables a plugin after exhausting recovery attempts.
func (hm *HealthMonitor) autoDisable(ctx context.Context, name string, state *pluginHealthState) {
	state.mu.Lock()
	state.inErrorState = false // cleared since we're disabling
	state.mu.Unlock()

	hm.logger.Warn("plugins.health.auto_disable",
		"plugin", name,
		"reason", "max recovery attempts exceeded or auto-disable timeout",
	)

	// Unregister from registry.
	hm.registry.Unregister(name)

	// Disable in store.
	if err := hm.store.DisablePlugin(ctx, name, nil); err != nil {
		hm.logger.Error("plugins.health.disable_failed",
			"plugin", name,
			"error", err,
		)
	}

	// Write audit log entry.
	if err := hm.store.LogAudit(ctx, name, "auto_disable", "system"); err != nil {
		hm.logger.Error("plugins.health.audit_failed",
			"plugin", name,
			"error", err,
		)
	}
}
