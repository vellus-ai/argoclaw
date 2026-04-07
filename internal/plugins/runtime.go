package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ─────────────────────────────────────────────────────────────────────────────
// MCP abstraction interfaces — enable testing without real MCP processes
// ─────────────────────────────────────────────────────────────────────────────

// MCPToolInfo holds basic tool information discovered from an MCP server.
type MCPToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// MCPCallToolResult holds the result of an MCP tool call.
type MCPCallToolResult struct {
	Content string `json:"content"`
	IsError bool   `json:"is_error"`
}

// MCPClient abstracts an MCP client connection for testing.
type MCPClient interface {
	Initialize(ctx context.Context) error
	ListTools(ctx context.Context) ([]MCPToolInfo, error)
	CallTool(ctx context.Context, toolName string, args json.RawMessage) (*MCPCallToolResult, error)
	Ping(ctx context.Context) error
	Close() error
}

// MCPClientFactory creates an MCP client from command, args, and env.
type MCPClientFactory func(command string, args []string, env map[string]string) (MCPClient, error)

// ─────────────────────────────────────────────────────────────────────────────
// pluginProcess — tracks a running plugin MCP process
// ─────────────────────────────────────────────────────────────────────────────

type pluginProcess struct {
	name     string
	client   MCPClient
	manifest *PluginManifest
	tenantID uuid.UUID
	enabled  bool
	cancel   context.CancelFunc

	// In-flight tracking for WaitInFlight.
	inflight sync.WaitGroup
}

// ─────────────────────────────────────────────────────────────────────────────
// RuntimeManager
// ─────────────────────────────────────────────────────────────────────────────

// RuntimeManager manages MCP plugin processes: starting, stopping, and routing
// tool calls through circuit breakers.
type RuntimeManager struct {
	cfg      Config
	factory  MCPClientFactory
	logger   *slog.Logger

	mu        sync.Mutex
	processes map[string]*pluginProcess
	circuits  map[string]*CircuitBreaker
}

// NewRuntimeManager creates a RuntimeManager with the given config and MCP
// client factory.
func NewRuntimeManager(cfg Config, factory MCPClientFactory, logger *slog.Logger) *RuntimeManager {
	if logger == nil {
		logger = slog.Default()
	}
	return &RuntimeManager{
		cfg:       cfg,
		factory:   factory,
		logger:    logger,
		processes: make(map[string]*pluginProcess),
		circuits:  make(map[string]*CircuitBreaker),
	}
}

// StartPlugin validates the manifest command sandbox, creates an MCP client,
// initializes the MCP connection, discovers tools, and returns them.
// ARGO_TENANT_ID is propagated to the plugin process via environment variables.
func (rm *RuntimeManager) StartPlugin(ctx context.Context, manifest *PluginManifest, tenantID uuid.UUID) ([]DiscoveredTool, error) {
	name := manifest.Metadata.Name

	// 1. Validate command sandbox.
	if err := rm.validateCommand(manifest.Spec.Runtime); err != nil {
		return nil, fmt.Errorf("start plugin %s: %w", name, err)
	}

	// 2. Build environment, propagating ARGO_TENANT_ID.
	env := make(map[string]string, len(manifest.Spec.Runtime.Env)+1)
	for k, v := range manifest.Spec.Runtime.Env {
		env[k] = v
	}
	env["ARGO_TENANT_ID"] = tenantID.String()

	// 3. Create MCP client via factory.
	client, err := rm.factory(manifest.Spec.Runtime.Command, manifest.Spec.Runtime.Args, env)
	if err != nil {
		return nil, fmt.Errorf("start plugin %s: create client: %w", name, err)
	}

	// 4. MCP Initialize handshake.
	if err := client.Initialize(ctx); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("start plugin %s: initialize: %w", name, err)
	}

	// 5. MCP tools/list.
	mcpTools, err := client.ListTools(ctx)
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("start plugin %s: list tools: %w", name, err)
	}

	// 6. Convert to DiscoveredTool.
	discovered := make([]DiscoveredTool, len(mcpTools))
	for i, t := range mcpTools {
		discovered[i] = DiscoveredTool{
			Name:        t.Name,
			Description: t.Description,
		}
	}

	// 7. Create process context for lifecycle management.
	_, cancel := context.WithCancel(context.Background())

	proc := &pluginProcess{
		name:     name,
		client:   client,
		manifest: manifest,
		tenantID: tenantID,
		enabled:  true,
		cancel:   cancel,
	}

	// 8. Store process and create circuit breaker.
	rm.mu.Lock()
	rm.processes[name] = proc
	rm.circuits[name] = NewCircuitBreaker(name, rm.cfg.CircuitBreakerThreshold, rm.cfg.CircuitBreakerResetTimeout)
	rm.mu.Unlock()

	rm.logger.Info("plugins.runtime.started",
		"plugin", name,
		"tools", len(discovered),
		"tenant_id", tenantID.String(),
	)

	return discovered, nil
}

// StopPlugin stops a running plugin process gracefully. It cancels the
// process context, closes the MCP client, and removes the process from the
// manager.
func (rm *RuntimeManager) StopPlugin(ctx context.Context, name string, timeout time.Duration) error {
	rm.mu.Lock()
	proc, ok := rm.processes[name]
	if !ok {
		rm.mu.Unlock()
		return fmt.Errorf("stop plugin %s: %w", name, ErrPluginNotFound)
	}
	delete(rm.processes, name)
	delete(rm.circuits, name)
	rm.mu.Unlock()

	// Cancel the process context.
	if proc.cancel != nil {
		proc.cancel()
	}

	// Close the MCP client.
	if proc.client != nil {
		if err := proc.client.Close(); err != nil {
			rm.logger.Warn("plugins.runtime.close_error",
				"plugin", name,
				"error", err,
			)
		}
	}

	rm.logger.Info("plugins.runtime.stopped", "plugin", name)
	return nil
}

// StopAll stops all running plugin processes with a total timeout.
func (rm *RuntimeManager) StopAll(timeout time.Duration) error {
	rm.mu.Lock()
	names := make([]string, 0, len(rm.processes))
	for name := range rm.processes {
		names = append(names, name)
	}
	rm.mu.Unlock()

	if len(names) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var errs []string
	for _, name := range names {
		if err := rm.StopPlugin(ctx, name, timeout); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", name, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("stop all plugins: %s", strings.Join(errs, "; "))
	}
	return nil
}

// WaitInFlight waits for all in-flight tool calls on a plugin to complete,
// with the given timeout.
func (rm *RuntimeManager) WaitInFlight(ctx context.Context, name string, timeout time.Duration) error {
	rm.mu.Lock()
	proc, ok := rm.processes[name]
	rm.mu.Unlock()

	if !ok {
		return fmt.Errorf("wait inflight %s: %w", name, ErrPluginNotFound)
	}

	done := make(chan struct{})
	go func() {
		proc.inflight.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("wait inflight %s: %w", name, ErrPluginTimeout)
	case <-ctx.Done():
		return fmt.Errorf("wait inflight %s: %w", name, ctx.Err())
	}
}

// CallTool routes a tool call to the appropriate plugin process through its
// circuit breaker. Returns an error if the plugin is not found, not enabled,
// or if the circuit breaker is open.
func (rm *RuntimeManager) CallTool(ctx context.Context, name, toolName string, args json.RawMessage) (*MCPCallToolResult, error) {
	rm.mu.Lock()
	proc, ok := rm.processes[name]
	if !ok {
		rm.mu.Unlock()
		return nil, fmt.Errorf("call tool %s/%s: %w", name, toolName, ErrPluginNotFound)
	}
	if !proc.enabled {
		rm.mu.Unlock()
		return nil, fmt.Errorf("call tool %s/%s: %w", name, toolName, ErrPluginNotEnabled)
	}
	cb, cbOK := rm.circuits[name]
	rm.mu.Unlock()

	// Check circuit breaker.
	if cbOK && !cb.Allow() {
		return nil, fmt.Errorf("call tool %s/%s: %w", name, toolName, ErrCircuitOpen)
	}

	// Track in-flight calls.
	proc.inflight.Add(1)
	defer proc.inflight.Done()

	// Execute the MCP tool call.
	result, err := proc.client.CallTool(ctx, toolName, args)
	if err != nil {
		if cbOK {
			cb.RecordFailure()
		}
		return nil, fmt.Errorf("call tool %s/%s: %w", name, toolName, err)
	}

	if cbOK {
		cb.RecordSuccess()
	}

	return result, nil
}

// GetCircuitBreaker returns the circuit breaker for a plugin (for health monitor).
func (rm *RuntimeManager) GetCircuitBreaker(name string) *CircuitBreaker {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	return rm.circuits[name]
}

// GetClient returns the MCP client for a plugin (for health monitor ping).
func (rm *RuntimeManager) GetClient(name string) MCPClient {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	proc, ok := rm.processes[name]
	if !ok {
		return nil
	}
	return proc.client
}

// ─────────────────────────────────────────────────────────────────────────────
// Command sandbox validation
// ─────────────────────────────────────────────────────────────────────────────

// validateCommand checks the runtime command against the sandbox policy.
func (rm *RuntimeManager) validateCommand(runtime ManifestRuntime) error {
	cmd := runtime.Command

	// Path traversal check.
	if strings.Contains(cmd, "..") {
		return fmt.Errorf("%w: command contains path traversal sequence: %q", ErrPathTraversal, cmd)
	}

	// Must be relative (not start with /).
	if strings.HasPrefix(cmd, "/") {
		return fmt.Errorf("%w: command must be a relative path, got absolute path: %q", ErrPathTraversal, cmd)
	}

	// Command allowlist.
	allowed := false
	for _, ac := range rm.cfg.AllowedCommands {
		if cmd == ac {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("%w: %q is not in the allowed commands list", ErrCommandNotAllowed, cmd)
	}

	// Blocked env vars.
	for envKey := range runtime.Env {
		if rm.cfg.BlockedEnvVars[envKey] {
			return fmt.Errorf("%w: environment variable %q cannot be overridden by plugins",
				ErrBlockedEnvVar, envKey)
		}
	}

	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Backoff calculation
// ─────────────────────────────────────────────────────────────────────────────

// backoffDuration calculates exponential backoff: base * 2^attempt, capped at maxBackoff.
func backoffDuration(attempt int, base, maxBackoff time.Duration) time.Duration {
	d := base
	for i := 0; i < attempt; i++ {
		d *= 2
		if d > maxBackoff {
			return maxBackoff
		}
	}
	return d
}
