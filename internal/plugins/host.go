package plugins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/vellus-ai/argoclaw/internal/store"
)

// ─────────────────────────────────────────────────────────────────────────────
// Interfaces for core ArgoClaw components
// ─────────────────────────────────────────────────────────────────────────────

// ToolRegistry abstracts the core tool registry for plugin tool registration.
type ToolRegistry interface {
	Register(name string, tool interface{}) error
	Unregister(name string) error
}

// PolicyEngine abstracts the core policy engine for tool group management.
type PolicyEngine interface {
	RegisterToolGroup(name string, members []string) error
	UnregisterToolGroup(name string) error
}

// CronStore abstracts the core cron store for scheduled job management.
type CronStore interface {
	RegisterJob(name, schedule string, handler func(ctx context.Context) error) error
	DisableJob(name string) error
}

// ─────────────────────────────────────────────────────────────────────────────
// Plan hierarchy for eligibility checks
// ─────────────────────────────────────────────────────────────────────────────

var planHierarchy = map[string]int{
	"starter":    0,
	"pro":        1,
	"enterprise": 2,
}

// PlatformVersion is the current gateway version. Set at build time or config.
var PlatformVersion = "1.0.0"

// ─────────────────────────────────────────────────────────────────────────────
// PluginHost — central lifecycle controller
// ─────────────────────────────────────────────────────────────────────────────

// PluginHost is the central coordinator that ties all plugin subsystems together.
// It manages the full lifecycle of plugins: install, enable, disable, uninstall,
// config updates, and gateway init/shutdown.
type PluginHost struct {
	store     store.PluginStore
	registry  *Registry
	runtime   *RuntimeManager
	health    *HealthMonitor
	migration *MigrationRunner
	dataProxy *DataProxy
	events    *EventBridge
	toolReg   ToolRegistry
	policy    PolicyEngine
	cronStore CronStore
	logger    *slog.Logger
	mu        sync.Mutex
	config    Config
	parser    *ManifestParser
}

// NewPluginHost creates a PluginHost with all required dependencies.
func NewPluginHost(
	cfg Config,
	s store.PluginStore,
	registry *Registry,
	runtime *RuntimeManager,
	health *HealthMonitor,
	migration *MigrationRunner,
	dataProxy *DataProxy,
	events *EventBridge,
	toolReg ToolRegistry,
	policy PolicyEngine,
	cronStore CronStore,
	logger *slog.Logger,
) *PluginHost {
	if logger == nil {
		logger = slog.Default()
	}
	return &PluginHost{
		store:     s,
		registry:  registry,
		runtime:   runtime,
		health:    health,
		migration: migration,
		dataProxy: dataProxy,
		events:    events,
		toolReg:   toolReg,
		policy:    policy,
		cronStore: cronStore,
		logger:    logger,
		config:    cfg,
		parser:    NewManifestParser(cfg),
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Init and Shutdown
// ─────────────────────────────────────────────────────────────────────────────

// Init loads enabled plugins from the database, starts their MCP processes,
// registers tools, and starts the health monitor and registry sync.
// Errors in individual plugins are logged as warnings but do NOT prevent
// the gateway from starting.
func (h *PluginHost) Init(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	plugins, err := h.store.ListTenantPlugins(ctx)
	if err != nil {
		return fmt.Errorf("plugin host init: list plugins: %w", err)
	}

	for _, tp := range plugins {
		if tp.State != store.PluginStateEnabled {
			continue
		}

		if err := h.enablePluginInternal(ctx, tp.PluginName); err != nil {
			h.logger.Warn("plugin host init: failed to enable plugin",
				"plugin", tp.PluginName,
				"error", err,
			)
			continue
		}
	}

	// Start health monitor.
	if h.health != nil && h.config.HealthCheckEnabled {
		h.health.Start(ctx)
	}

	// Start registry sync.
	if h.registry != nil {
		h.registry.StartSync(ctx)
	}

	h.logger.Info("plugin host init: complete")
	return nil
}

// Shutdown stops the health monitor, all running plugin processes, and clears
// the registry. Processes that don't terminate within 30s are force-killed.
func (h *PluginHost) Shutdown() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Stop health monitor.
	if h.health != nil {
		h.health.Stop()
	}

	// Stop all plugin processes.
	var runtimeErr error
	if h.runtime != nil {
		runtimeErr = h.runtime.StopAll(30 * time.Second)
	}

	// Clear registry.
	if h.registry != nil {
		h.registry.StopSync()
		for _, name := range h.registry.Names() {
			h.registry.Unregister(name)
		}
	}

	h.logger.Info("plugin host shutdown: complete")
	return runtimeErr
}

// ─────────────────────────────────────────────────────────────────────────────
// Install
// ─────────────────────────────────────────────────────────────────────────────

// Install installs a plugin for the current tenant. The operation is atomic:
// catalog lookup, manifest parsing, eligibility checks, migrations, and DB
// insert all happen in a single transaction. Failure at any step rolls back.
func (h *PluginHost) Install(ctx context.Context, pluginName string) (*store.TenantPlugin, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// 1. Get catalog entry.
	catalog, err := h.store.GetCatalogEntryByName(ctx, pluginName)
	if err != nil {
		if errors.Is(err, store.ErrPluginNotFound) {
			return nil, fmt.Errorf("install %s: %w", pluginName, ErrPluginNotFound)
		}
		return nil, fmt.Errorf("install %s: get catalog: %w", pluginName, err)
	}

	// 2. Parse manifest.
	manifest, err := h.parser.Parse(catalog.Manifest)
	if err != nil {
		return nil, fmt.Errorf("install %s: parse manifest: %w", pluginName, err)
	}

	// 3. Check plan eligibility.
	if err := h.checkPlanEligibility(ctx, manifest); err != nil {
		return nil, fmt.Errorf("install %s: %w", pluginName, err)
	}

	// 4. Check platform version.
	if err := h.checkPlatformVersion(manifest); err != nil {
		return nil, fmt.Errorf("install %s: %w", pluginName, err)
	}

	// 5. Check dependencies.
	if err := h.checkDependencies(ctx, manifest); err != nil {
		return nil, fmt.Errorf("install %s: %w", pluginName, err)
	}

	// 6. Check features.
	if err := h.checkFeatures(manifest); err != nil {
		return nil, fmt.Errorf("install %s: %w", pluginName, err)
	}

	// 7. Install in DB (store handles TX + audit).
	tenantID := store.TenantIDFromContext(ctx)
	tp := &store.TenantPlugin{
		ID:            uuid.New(),
		TenantID:      tenantID,
		PluginName:    pluginName,
		PluginVersion: catalog.Version,
		State:         store.PluginStateInstalled,
		Config:        json.RawMessage("{}"),
		Permissions:   catalog.Manifest,
		InstalledBy:   nil, // set by store from context if available
	}

	if err := h.store.InstallPlugin(ctx, tp); err != nil {
		if errors.Is(err, store.ErrPluginAlreadyInstalled) {
			return nil, fmt.Errorf("install %s: %w", pluginName, ErrPluginAlreadyInstalled)
		}
		return nil, fmt.Errorf("install %s: %w", pluginName, err)
	}

	h.logger.Info("plugin host: installed",
		"plugin", pluginName,
		"version", catalog.Version,
		"tenant_id", tenantID.String(),
	)

	// Return the installed record.
	installed, err := h.store.GetTenantPlugin(ctx, pluginName)
	if err != nil {
		return tp, nil // best effort
	}
	return installed, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Enable
// ─────────────────────────────────────────────────────────────────────────────

// Enable activates a plugin for the current tenant. It starts the MCP process,
// discovers tools, registers them in the tool registry and policy engine,
// connects events, registers cron jobs, and updates the DB.
func (h *PluginHost) Enable(ctx context.Context, pluginName string) (*store.TenantPlugin, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// 1. Get tenant plugin and validate state.
	tp, err := h.store.GetTenantPlugin(ctx, pluginName)
	if err != nil {
		if errors.Is(err, store.ErrPluginNotFound) {
			return nil, fmt.Errorf("enable %s: %w", pluginName, ErrPluginNotInstalled)
		}
		return nil, fmt.Errorf("enable %s: get plugin: %w", pluginName, err)
	}

	// 2. Validate state transition.
	currentState := PluginState(tp.State)
	if !isValidTransition(currentState, StateEnabled) {
		return nil, fmt.Errorf("enable %s: %w: cannot transition from %s to enabled",
			pluginName, ErrInvalidState, tp.State)
	}

	// 3. Enable internal.
	if err := h.enablePluginInternal(ctx, pluginName); err != nil {
		return nil, fmt.Errorf("enable %s: %w", pluginName, err)
	}

	// 4. Update DB.
	if err := h.store.EnablePlugin(ctx, pluginName, nil); err != nil {
		return nil, fmt.Errorf("enable %s: update db: %w", pluginName, err)
	}

	h.logger.Info("plugin host: enabled", "plugin", pluginName)

	// Return updated record.
	updated, err := h.store.GetTenantPlugin(ctx, pluginName)
	if err != nil {
		return tp, nil
	}
	return updated, nil
}

// enablePluginInternal performs the runtime enable sequence without locking
// or DB state changes. Used by both Enable and Init.
func (h *PluginHost) enablePluginInternal(ctx context.Context, pluginName string) error {
	// Get catalog entry for manifest.
	catalog, err := h.store.GetCatalogEntryByName(ctx, pluginName)
	if err != nil {
		return fmt.Errorf("get catalog: %w", err)
	}

	// Parse manifest.
	manifest, err := h.parser.Parse(catalog.Manifest)
	if err != nil {
		return fmt.Errorf("parse manifest: %w", err)
	}

	tenantID := store.TenantIDFromContext(ctx)

	// Start MCP process and discover tools.
	var discoveredTools []DiscoveredTool
	if h.runtime != nil {
		discovered, err := h.runtime.StartPlugin(ctx, manifest, tenantID)
		if err != nil {
			return fmt.Errorf("start mcp: %w", err)
		}
		discoveredTools = discovered
	}

	// Register tools with prefix in ToolRegistry.
	toolNames := make([]string, 0, len(discoveredTools))
	for _, tool := range discoveredTools {
		prefixedName := toolPrefix(pluginName, tool.Name)
		toolNames = append(toolNames, prefixedName)
		if h.toolReg != nil {
			if err := h.toolReg.Register(prefixedName, tool); err != nil {
				h.logger.Warn("plugin host: failed to register tool",
					"plugin", pluginName,
					"tool", prefixedName,
					"error", err,
				)
			}
		}
	}

	// Register tool group in PolicyEngine.
	if h.policy != nil {
		groupName := "plugin:" + pluginName
		if err := h.policy.RegisterToolGroup(groupName, toolNames); err != nil {
			h.logger.Warn("plugin host: failed to register tool group",
				"plugin", pluginName,
				"error", err,
			)
		}
	}

	// Connect events.
	if h.events != nil {
		if err := h.events.Connect(ctx, pluginName, manifest); err != nil {
			h.logger.Warn("plugin host: failed to connect events",
				"plugin", pluginName,
				"error", err,
			)
		}
	}

	// Register cron jobs.
	if h.cronStore != nil && len(manifest.Spec.Permissions.Cron) > 0 {
		for _, job := range manifest.Spec.Permissions.Cron {
			jobName := fmt.Sprintf("plugin:%s:%s", pluginName, job.Name)
			toolName := job.Tool
			if err := h.cronStore.RegisterJob(jobName, job.Schedule, func(ctx context.Context) error {
				_, err := h.runtime.CallTool(ctx, pluginName, toolName, nil)
				return err
			}); err != nil {
				h.logger.Warn("plugin host: failed to register cron job",
					"plugin", pluginName,
					"job", jobName,
					"error", err,
				)
			}
		}
	}

	// Register health pinger.
	if h.health != nil && h.runtime != nil {
		client := h.runtime.GetClient(pluginName)
		if client != nil {
			h.health.RegisterPinger(pluginName, client)
		}
	}

	// Register in in-memory registry.
	entry := &RegistryEntry{
		Manifest:  manifest,
		CatalogID: catalog.ID,
		Status:    RegistryActive,
		Tools:     toolNames,
		EnabledAt: time.Now(),
	}
	h.registry.Register(pluginName, entry)

	// Publish change via Redis.
	h.registry.PublishChange(ctx, pluginName, "enable")

	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Disable
// ─────────────────────────────────────────────────────────────────────────────

// Disable deactivates a plugin for the current tenant. It waits for in-flight
// tool calls, unregisters tools and events, stops the MCP process, and updates
// the DB. All data is preserved.
func (h *PluginHost) Disable(ctx context.Context, pluginName string) (*store.TenantPlugin, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// 1. Get tenant plugin and validate state.
	tp, err := h.store.GetTenantPlugin(ctx, pluginName)
	if err != nil {
		if errors.Is(err, store.ErrPluginNotFound) {
			return nil, fmt.Errorf("disable %s: %w", pluginName, ErrPluginNotInstalled)
		}
		return nil, fmt.Errorf("disable %s: get plugin: %w", pluginName, err)
	}

	// 2. Validate state transition.
	currentState := PluginState(tp.State)
	if !isValidTransition(currentState, StateDisabled) {
		return nil, fmt.Errorf("disable %s: %w: cannot transition from %s to disabled",
			pluginName, ErrInvalidState, tp.State)
	}

	// 3. Disable internal.
	h.disablePluginInternal(ctx, pluginName)

	// 4. Update DB.
	if err := h.store.DisablePlugin(ctx, pluginName, nil); err != nil {
		return nil, fmt.Errorf("disable %s: update db: %w", pluginName, err)
	}

	h.logger.Info("plugin host: disabled", "plugin", pluginName)

	// Return updated record.
	updated, err := h.store.GetTenantPlugin(ctx, pluginName)
	if err != nil {
		return tp, nil
	}
	return updated, nil
}

// disablePluginInternal performs the runtime disable sequence without locking
// or DB state changes. Used by Disable and Uninstall.
func (h *PluginHost) disablePluginInternal(ctx context.Context, pluginName string) {
	// Wait for in-flight tool calls.
	if h.runtime != nil {
		_ = h.runtime.WaitInFlight(ctx, pluginName, 5*time.Second)
	}

	// Unregister tools from ToolRegistry.
	if h.toolReg != nil {
		if entry, ok := h.registry.Get(pluginName); ok {
			for _, toolName := range entry.Tools {
				_ = h.toolReg.Unregister(toolName)
			}
		}
	}

	// Unregister tool group from PolicyEngine.
	if h.policy != nil {
		groupName := "plugin:" + pluginName
		_ = h.policy.UnregisterToolGroup(groupName)
	}

	// Disconnect events.
	if h.events != nil {
		h.events.Disconnect(pluginName)
	}

	// Disable cron jobs.
	if h.cronStore != nil {
		if entry, ok := h.registry.Get(pluginName); ok && entry.Manifest != nil {
			for _, job := range entry.Manifest.Spec.Permissions.Cron {
				jobName := fmt.Sprintf("plugin:%s:%s", pluginName, job.Name)
				_ = h.cronStore.DisableJob(jobName)
			}
		}
	}

	// Stop MCP process.
	if h.runtime != nil {
		_ = h.runtime.StopPlugin(ctx, pluginName, 5*time.Second)
	}

	// Unregister from registry and publish change.
	h.registry.Unregister(pluginName)
	h.registry.PublishChange(ctx, pluginName, "disable")
}

// ─────────────────────────────────────────────────────────────────────────────
// Uninstall
// ─────────────────────────────────────────────────────────────────────────────

// Uninstall removes a plugin for the current tenant. If other plugins depend
// on this one, the operation is rejected with 409. If the plugin is enabled,
// it is disabled first. The uninstall is atomic: audit log, data cleanup, and
// record deletion all happen in a single transaction.
func (h *PluginHost) Uninstall(ctx context.Context, pluginName string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// 1. Get tenant plugin.
	tp, err := h.store.GetTenantPlugin(ctx, pluginName)
	if err != nil {
		if errors.Is(err, store.ErrPluginNotFound) {
			return fmt.Errorf("uninstall %s: %w", pluginName, ErrPluginNotInstalled)
		}
		return fmt.Errorf("uninstall %s: get plugin: %w", pluginName, err)
	}

	// 2. Check for dependents.
	if err := h.checkDependents(ctx, pluginName); err != nil {
		return fmt.Errorf("uninstall %s: %w", pluginName, err)
	}

	// 3. Disable if currently enabled.
	if tp.State == store.PluginStateEnabled {
		h.disablePluginInternal(ctx, pluginName)
	}

	// 4. Uninstall from DB (store handles TX: audit → cleanup → delete).
	if err := h.store.UninstallPlugin(ctx, pluginName, nil); err != nil {
		return fmt.Errorf("uninstall %s: %w", pluginName, err)
	}

	h.logger.Info("plugin host: uninstalled", "plugin", pluginName)
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// UpdateConfig
// ─────────────────────────────────────────────────────────────────────────────

// UpdateConfig validates and updates the per-tenant configuration for a plugin.
// If the plugin is enabled, it restarts the MCP process to apply the new config.
func (h *PluginHost) UpdateConfig(ctx context.Context, pluginName string, config json.RawMessage) (*store.TenantPlugin, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// 1. Get tenant plugin.
	tp, err := h.store.GetTenantPlugin(ctx, pluginName)
	if err != nil {
		if errors.Is(err, store.ErrPluginNotFound) {
			return nil, fmt.Errorf("update config %s: %w", pluginName, ErrPluginNotInstalled)
		}
		return nil, fmt.Errorf("update config %s: get plugin: %w", pluginName, err)
	}

	// 2. Get catalog for config schema.
	catalog, err := h.store.GetCatalogEntryByName(ctx, pluginName)
	if err != nil {
		return nil, fmt.Errorf("update config %s: get catalog: %w", pluginName, err)
	}

	// 3. Parse manifest to get config schema.
	manifest, err := h.parser.Parse(catalog.Manifest)
	if err != nil {
		return nil, fmt.Errorf("update config %s: parse manifest: %w", pluginName, err)
	}

	// 4. Validate config against schema.
	if len(manifest.Spec.ConfigSchema) > 0 && string(manifest.Spec.ConfigSchema) != "null" {
		if err := validateJSONSchema(manifest.Spec.ConfigSchema.RawMessage(), config); err != nil {
			return nil, fmt.Errorf("update config %s: %w: %v", pluginName, ErrConfigInvalid, err)
		}
	}

	// 5. Persist config.
	if err := h.store.UpdatePluginConfig(ctx, pluginName, config, nil); err != nil {
		return nil, fmt.Errorf("update config %s: persist: %w", pluginName, err)
	}

	// 6. Restart MCP process if enabled.
	if tp.State == store.PluginStateEnabled {
		h.disablePluginInternal(ctx, pluginName)
		if err := h.enablePluginInternal(ctx, pluginName); err != nil {
			h.logger.Warn("plugin host: failed to restart after config change",
				"plugin", pluginName,
				"error", err,
			)
		}
	}

	h.logger.Info("plugin host: config updated", "plugin", pluginName)

	// Return updated record.
	updated, err := h.store.GetTenantPlugin(ctx, pluginName)
	if err != nil {
		return tp, nil
	}
	return updated, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Eligibility checks
// ─────────────────────────────────────────────────────────────────────────────

// checkPlanEligibility verifies the tenant's plan meets the plugin's minimum requirement.
func (h *PluginHost) checkPlanEligibility(ctx context.Context, manifest *PluginManifest) error {
	requiredPlan := manifest.Spec.Requires.Plan
	if requiredPlan == "" {
		return nil // no plan requirement
	}

	// For now, tenant plan is derived from context or defaults to "starter".
	// In production, this would come from the tenant record.
	tenantPlan := tenantPlanFromContext(ctx)

	reqLevel, ok := planHierarchy[requiredPlan]
	if !ok {
		return nil // unknown plan requirement, allow
	}
	tenantLevel, ok := planHierarchy[tenantPlan]
	if !ok {
		tenantLevel = 0 // default to starter
	}

	if tenantLevel < reqLevel {
		return fmt.Errorf("%w: requires %s plan, tenant has %s",
			ErrPlanInsufficient, requiredPlan, tenantPlan)
	}
	return nil
}

// checkPlatformVersion verifies the gateway version satisfies the plugin's
// minimum platform requirement.
func (h *PluginHost) checkPlatformVersion(manifest *PluginManifest) error {
	required := manifest.Spec.Requires.Platform
	if required == "" {
		return nil
	}

	if !semverSatisfies(PlatformVersion, required) {
		return fmt.Errorf("%w: requires platform %s, current is %s",
			ErrPlatformVersion, required, PlatformVersion)
	}
	return nil
}

// checkDependencies verifies all required plugins are installed.
func (h *PluginHost) checkDependencies(ctx context.Context, manifest *PluginManifest) error {
	for _, dep := range manifest.Spec.Requires.Plugins {
		_, err := h.store.GetTenantPlugin(ctx, dep)
		if err != nil {
			if errors.Is(err, store.ErrPluginNotFound) {
				return fmt.Errorf("%w: required plugin %q is not installed",
					ErrDependencyMissing, dep)
			}
			return fmt.Errorf("check dependency %s: %w", dep, err)
		}
	}
	return nil
}

// checkFeatures verifies all required gateway features are available.
func (h *PluginHost) checkFeatures(manifest *PluginManifest) error {
	for _, feature := range manifest.Spec.Requires.Features {
		if !isFeatureAvailable(feature) {
			return fmt.Errorf("%w: required feature %q is not available",
				ErrDependencyMissing, feature)
		}
	}
	return nil
}

// checkDependents verifies no other installed plugins depend on this plugin.
func (h *PluginHost) checkDependents(ctx context.Context, pluginName string) error {
	plugins, err := h.store.ListTenantPlugins(ctx)
	if err != nil {
		return fmt.Errorf("check dependents: %w", err)
	}

	for _, tp := range plugins {
		if tp.PluginName == pluginName {
			continue
		}
		catalog, err := h.store.GetCatalogEntryByName(ctx, tp.PluginName)
		if err != nil {
			continue // skip plugins with missing catalog entries
		}
		manifest, err := h.parser.Parse(catalog.Manifest)
		if err != nil {
			continue
		}
		for _, dep := range manifest.Spec.Requires.Plugins {
			if dep == pluginName {
				return fmt.Errorf("%w: plugin %q depends on %q",
					ErrDependentExists, tp.PluginName, pluginName)
			}
		}
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Helper functions
// ─────────────────────────────────────────────────────────────────────────────

// toolPrefix generates the prefixed tool name: plugin_{name}__{tool}
func toolPrefix(pluginName, toolName string) string {
	underscored := strings.ReplaceAll(pluginName, "-", "_")
	return fmt.Sprintf("plugin_%s__%s", underscored, toolName)
}

// isValidTransition checks if a state transition is allowed.
func isValidTransition(from, to PluginState) bool {
	allowed, ok := ValidTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// tenantPlanFromContext extracts the tenant plan from context.
// Returns "starter" as default if not set.
func tenantPlanFromContext(ctx context.Context) string {
	if plan, ok := ctx.Value(ctxKeyTenantPlan).(string); ok && plan != "" {
		return plan
	}
	return "starter"
}

type contextKey string

const ctxKeyTenantPlan contextKey = "tenant_plan"

// WithTenantPlan sets the tenant plan in context.
func WithTenantPlan(ctx context.Context, plan string) context.Context {
	return context.WithValue(ctx, ctxKeyTenantPlan, plan)
}

// semverSatisfies checks if currentVersion >= requiredVersion.
// Simple semver comparison: major.minor.patch as integers.
func semverSatisfies(current, required string) bool {
	cMajor, cMinor, cPatch := parseSemver(current)
	rMajor, rMinor, rPatch := parseSemver(required)

	if cMajor != rMajor {
		return cMajor > rMajor
	}
	if cMinor != rMinor {
		return cMinor > rMinor
	}
	return cPatch >= rPatch
}

// parseSemver extracts major.minor.patch from a semver string.
func parseSemver(v string) (major, minor, patch int) {
	// Strip pre-release suffix.
	if idx := strings.IndexByte(v, '-'); idx >= 0 {
		v = v[:idx]
	}
	// Strip build metadata.
	if idx := strings.IndexByte(v, '+'); idx >= 0 {
		v = v[:idx]
	}

	parts := strings.Split(v, ".")
	if len(parts) >= 1 {
		fmt.Sscanf(parts[0], "%d", &major)
	}
	if len(parts) >= 2 {
		fmt.Sscanf(parts[1], "%d", &minor)
	}
	if len(parts) >= 3 {
		fmt.Sscanf(parts[2], "%d", &patch)
	}
	return
}

// isFeatureAvailable checks if a gateway feature is enabled.
// For now, returns true for known features, false for unknown.
var availableFeatures = map[string]bool{
	"memory":   true,
	"sandbox":  true,
	"webhooks": true,
}

func isFeatureAvailable(feature string) bool {
	return availableFeatures[feature]
}

// validateJSONSchema validates a JSON document against a JSON Schema.
// This is a simplified implementation that checks required fields and types.
// For production, use a full JSON Schema library.
func validateJSONSchema(schema, doc json.RawMessage) error {
	var schemaObj map[string]json.RawMessage
	if err := json.Unmarshal(schema, &schemaObj); err != nil {
		return fmt.Errorf("invalid schema: %w", err)
	}

	var docObj map[string]json.RawMessage
	if err := json.Unmarshal(doc, &docObj); err != nil {
		return fmt.Errorf("config must be a JSON object")
	}

	// Check required fields.
	if reqRaw, ok := schemaObj["required"]; ok {
		var required []string
		if err := json.Unmarshal(reqRaw, &required); err == nil {
			for _, field := range required {
				if _, exists := docObj[field]; !exists {
					return fmt.Errorf("missing required field: %s", field)
				}
			}
		}
	}

	return nil
}
