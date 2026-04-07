package plugins

import "time"

// ─────────────────────────────────────────────────────────────────────────────
// Plugin Host Configuration
// ─────────────────────────────────────────────────────────────────────────────

// Config holds all configurable parameters for the Plugin Host subsystem.
type Config struct {
	// AllowedCommands is the allowlist of executable commands for the plugin sandbox.
	// Only commands in this list can be spawned by plugins.
	AllowedCommands []string `json:"allowed_commands"`

	// BlockedEnvVars are environment variables that plugins cannot override.
	// Includes security-sensitive vars like PATH, LD_PRELOAD, HOME, USER, SHELL, etc.
	BlockedEnvVars map[string]bool `json:"blocked_env_vars"`

	// MaxPluginsPerTenant is the maximum number of plugins a single tenant can install.
	MaxPluginsPerTenant int `json:"max_plugins_per_tenant"`

	// HealthCheckEnabled controls whether periodic health checks run for active plugins.
	HealthCheckEnabled bool `json:"health_check_enabled"`

	// CircuitBreakerThreshold is the number of consecutive failures before the
	// circuit breaker opens for a plugin.
	CircuitBreakerThreshold int `json:"circuit_breaker_threshold"`

	// CircuitBreakerResetTimeout is the duration the circuit stays open before
	// transitioning to half-open.
	CircuitBreakerResetTimeout time.Duration `json:"circuit_breaker_reset_timeout"`

	// AutoDisableAfter is the duration a plugin can remain in error state without
	// successful recovery before being automatically disabled.
	AutoDisableAfter time.Duration `json:"auto_disable_after"`

	// MaxRecoveryAttempts is the maximum number of recovery attempts before
	// the plugin is automatically disabled.
	MaxRecoveryAttempts int `json:"max_recovery_attempts"`

	// RegistrySyncChannel is the Redis pub/sub channel name used to synchronize
	// the in-memory registry across gateway instances.
	RegistrySyncChannel string `json:"registry_sync_channel"`

	// RegistryPollInterval is the fallback polling interval for reloading
	// registry state from the database when Redis is unavailable.
	RegistryPollInterval time.Duration `json:"registry_poll_interval"`

	// DataMaxValueSize is the maximum size in bytes for a single value in the
	// plugin KV store.
	DataMaxValueSize int `json:"data_max_value_size"`

	// AuditRetentionDays is the number of days to retain audit log entries.
	// Records older than this are eligible for archival.
	AuditRetentionDays int `json:"audit_retention_days"`
}

// DefaultConfig returns a Config populated with production-safe default values.
func DefaultConfig() Config {
	return Config{
		AllowedCommands: []string{"./server", "./plugin", "./bin/server"},
		BlockedEnvVars: map[string]bool{
			"PATH":       true,
			"LD_PRELOAD": true,
			"HOME":       true,
			"USER":       true,
			"SHELL":      true,
			"LD_LIBRARY_PATH": true,
		},
		MaxPluginsPerTenant:        50,
		HealthCheckEnabled:         true,
		CircuitBreakerThreshold:    5,
		CircuitBreakerResetTimeout: 30 * time.Second,
		AutoDisableAfter:           5 * time.Minute,
		MaxRecoveryAttempts:        5,
		RegistrySyncChannel:        "plugin_registry_sync",
		RegistryPollInterval:       30 * time.Second,
		DataMaxValueSize:           1048576, // 1MB
		AuditRetentionDays:         90,
	}
}
