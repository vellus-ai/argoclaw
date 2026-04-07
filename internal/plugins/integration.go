package plugins

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// ─────────────────────────────────────────────────────────────────────────────
// Store subset interfaces — avoid full store.PluginStore dependency
// ─────────────────────────────────────────────────────────────────────────────

// TenantPluginInfo is a lightweight struct for plugin loading at startup.
// It avoids importing the full store.TenantPlugin which includes fields
// not needed for initial registry population.
type TenantPluginInfo struct {
	PluginName    string `json:"plugin_name"`
	PluginVersion string `json:"plugin_version"`
}

// PluginStoreSubset abstracts the store operations needed by GatewayPluginBridge.
// This is a subset of store.PluginStore to keep the dependency minimal
// and make testing straightforward.
type PluginStoreSubset interface {
	// ListEnabledPlugins returns all plugins in "enabled" state across tenants.
	ListEnabledPlugins(ctx context.Context) ([]TenantPluginInfo, error)

	// LogAudit appends an immutable audit entry for a plugin operation.
	LogAudit(ctx context.Context, entry *AuditLogEntry) error

	// IsPluginEnabledForAgent checks if a plugin is active for a given agent.
	// Returns true if the plugin is enabled at tenant level AND not explicitly
	// disabled at agent level (opt-out model).
	IsPluginEnabledForAgent(ctx context.Context, agentID uuid.UUID, pluginName string) (bool, error)
}

// ToolRegistrySubset abstracts the tool group registration operations.
// Maps to tools.RegisterToolGroup / tools.UnregisterToolGroup.
type ToolRegistrySubset interface {
	RegisterGroup(name string, members []string)
	UnregisterGroup(name string)
}

// ─────────────────────────────────────────────────────────────────────────────
// AuditLogEntry — lightweight audit entry for integration layer
// ─────────────────────────────────────────────────────────────────────────────

// AuditLogEntry represents an audit log record for plugin operations.
// This is the integration layer's view — the store layer maps it to its
// own PluginAuditEntry type.
type AuditLogEntry struct {
	ID         uuid.UUID       `json:"id"`
	TenantID   uuid.UUID       `json:"tenant_id"`
	PluginName string          `json:"plugin_name"`
	Action     string          `json:"action"`
	ActorID    uuid.UUID       `json:"actor_id"`
	ActorType  string          `json:"actor_type"` // user | system | agent
	Details    json.RawMessage `json:"details"`
	CreatedAt  time.Time       `json:"created_at"`
}

// ─────────────────────────────────────────────────────────────────────────────
// 19.3 — AuditLogger for tool call audit logging
// ─────────────────────────────────────────────────────────────────────────────

// LogToolCallParams holds the parameters for recording a tool call in the
// audit log. Each field maps to a column or detail field in the audit entry.
type LogToolCallParams struct {
	TenantID   uuid.UUID
	PluginName string
	ToolName   string // full prefixed name: plugin_{name}__{tool}
	AgentID    uuid.UUID
	DurationMs int64
	Status     string // "success" or "error"
	ErrorMsg   string // non-empty when Status == "error"
}

// AuditLogger records plugin tool calls in the persistent audit log.
// It wraps the store's LogAudit method, adding structured detail fields
// specific to tool call auditing. Store errors are logged but never
// propagated — audit failures must not break the tool call flow.
type AuditLogger struct {
	store  PluginStoreSubset
	logger *slog.Logger
}

// NewAuditLogger creates an AuditLogger backed by the given store.
func NewAuditLogger(store PluginStoreSubset, logger *slog.Logger) *AuditLogger {
	if logger == nil {
		logger = slog.Default()
	}
	return &AuditLogger{store: store, logger: logger}
}

// LogToolCall records a single tool call in the audit log.
// This method is fire-and-forget: store errors are logged at WARN level
// but never returned, ensuring audit failures do not impact the tool
// call execution path.
func (al *AuditLogger) LogToolCall(ctx context.Context, params LogToolCallParams) {
	details := map[string]interface{}{
		"tool_name":   params.ToolName,
		"duration_ms": params.DurationMs,
		"status":      params.Status,
	}
	if params.ErrorMsg != "" {
		details["error"] = params.ErrorMsg
	}

	detailsJSON, err := json.Marshal(details)
	if err != nil {
		al.logger.Warn("plugins.audit.marshal_failed",
			"plugin", params.PluginName,
			"tool", params.ToolName,
			"error", err,
		)
		return
	}

	entry := &AuditLogEntry{
		ID:         uuid.New(),
		TenantID:   params.TenantID,
		PluginName: params.PluginName,
		Action:     "tool_call",
		ActorID:    params.AgentID,
		ActorType:  "agent",
		Details:    detailsJSON,
		CreatedAt:  time.Now().UTC(),
	}

	if err := al.store.LogAudit(ctx, entry); err != nil {
		al.logger.Warn("plugins.audit.tool_call_failed",
			"plugin", params.PluginName,
			"tool", params.ToolName,
			"error", err,
		)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 19.5 + 19.6 — GatewayPluginBridge: gateway startup/shutdown integration
// ─────────────────────────────────────────────────────────────────────────────

// IntegrationConfig holds all configuration for the Plugin Host gateway
// integration, including the feature flag.
type IntegrationConfig struct {
	// PluginSystemEnabled is the master feature flag. When false, Init returns
	// immediately without loading plugins, no routes are registered, and
	// Shutdown is a no-op. This enables gradual rollout.
	PluginSystemEnabled bool `json:"plugin_system_enabled"`

	// ShutdownTimeout is the maximum time to wait for all plugin processes
	// to stop during gateway shutdown. After this timeout, remaining processes
	// are force-killed.
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`

	// RoutePrefix is the HTTP route prefix for plugin management endpoints.
	// Default: "/v1/plugins".
	RoutePrefix string `json:"route_prefix"`
}

// DefaultIntegrationConfig returns production-safe defaults.
func DefaultIntegrationConfig() IntegrationConfig {
	return IntegrationConfig{
		PluginSystemEnabled: true,
		ShutdownTimeout:     30 * time.Second,
		RoutePrefix:         "/v1/plugins",
	}
}

// GatewayBridgeDeps holds all dependencies injected into the GatewayPluginBridge.
// Using a struct avoids long constructor parameter lists and makes
// adding new dependencies backward-compatible.
type GatewayBridgeDeps struct {
	Config       IntegrationConfig
	Store        PluginStoreSubset
	ToolRegistry ToolRegistrySubset
	Logger       *slog.Logger
}

// GatewayPluginBridge orchestrates the plugin subsystem lifecycle within the gateway.
// It is the single entry point for gateway startup/shutdown code to interact
// with the plugin system.
//
// Startup flow (Init):
//  1. Check feature flag — return immediately if disabled.
//  2. Load all enabled plugins from the store.
//  3. Register tool groups for each plugin (empty initially — populated
//     when MCP processes connect in a future work package).
//
// Shutdown flow (Shutdown):
//  1. Unregister all tool groups.
//  2. Stop all running MCP processes (future WP).
//  3. Clear the in-memory registry.
//
// Integration in cmd/ startup:
//
//	host := plugins.NewGatewayBridge(plugins.GatewayBridgeDeps{...})
//	if err := host.Init(ctx); err != nil {
//	    slog.Warn("plugins.init_failed", "error", err)
//	    // Continue gateway startup — plugin failures are non-fatal
//	}
//	defer host.Shutdown()
type GatewayPluginBridge struct {
	config       IntegrationConfig
	store        PluginStoreSubset
	toolRegistry ToolRegistrySubset
	logger       *slog.Logger
	enabled      bool
	plugins      []string // names of loaded plugins
}

// NewGatewayBridge creates a GatewayPluginBridge with the given dependencies.
func NewGatewayBridge(deps GatewayBridgeDeps) *GatewayPluginBridge {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &GatewayPluginBridge{
		config:       deps.Config,
		store:        deps.Store,
		toolRegistry: deps.ToolRegistry,
		logger:       logger,
	}
}

// Init loads enabled plugins and registers their tool groups.
// When the feature flag is disabled, Init returns nil immediately.
// Store errors are logged as warnings and do NOT prevent the gateway
// from starting — this is intentional to ensure plugin failures are
// non-fatal.
func (h *GatewayPluginBridge) Init(ctx context.Context) error {
	if !h.config.PluginSystemEnabled {
		h.logger.Info("plugins.host.disabled", "reason", "feature_flag")
		return nil
	}

	h.enabled = true
	h.logger.Info("plugins.host.init_start")

	if h.store == nil {
		h.logger.Warn("plugins.host.no_store", "reason", "store dependency not provided")
		return nil
	}

	plugins, err := h.store.ListEnabledPlugins(ctx)
	if err != nil {
		h.logger.Warn("plugins.host.load_failed",
			"error", err,
			"reason", "continuing without plugins",
		)
		return nil
	}

	for _, p := range plugins {
		h.plugins = append(h.plugins, p.PluginName)
		if h.toolRegistry != nil {
			h.toolRegistry.RegisterGroup("plugin:"+p.PluginName, []string{})
		}
		h.logger.Info("plugins.host.loaded",
			"plugin", p.PluginName,
			"version", p.PluginVersion,
		)
	}

	h.logger.Info("plugins.host.init_complete", "loaded", len(h.plugins))
	return nil
}

// Shutdown stops all plugin processes and unregisters tool groups.
// It is safe to call even when the plugin system is disabled.
func (h *GatewayPluginBridge) Shutdown() {
	if !h.enabled {
		return
	}

	h.logger.Info("plugins.host.shutdown_start",
		"plugins", len(h.plugins),
		"timeout", h.config.ShutdownTimeout,
	)

	for _, name := range h.plugins {
		if h.toolRegistry != nil {
			h.toolRegistry.UnregisterGroup("plugin:" + name)
		}
		h.logger.Info("plugins.host.unregistered", "plugin", name)
	}

	h.plugins = nil
	h.enabled = false

	h.logger.Info("plugins.host.shutdown_complete")
}

// IsEnabled returns whether the plugin system is currently active.
func (h *GatewayPluginBridge) IsEnabled() bool {
	return h.enabled
}

// ActivePluginNames returns the names of all currently loaded plugins.
func (h *GatewayPluginBridge) ActivePluginNames() []string {
	result := make([]string, len(h.plugins))
	copy(result, h.plugins)
	return result
}
