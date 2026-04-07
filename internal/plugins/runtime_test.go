package plugins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

// ─────────────────────────────────────────────────────────────────────────────
// Mock types for RuntimeManager tests
// ─────────────────────────────────────────────────────────────────────────────

// mockMCPClient simulates an MCP client for testing.
type mockMCPClient struct {
	mu           sync.Mutex
	initErr      error
	listToolsErr error
	callToolErr  error
	callToolFn   func(ctx context.Context, toolName string, args json.RawMessage) (*MCPCallToolResult, error)
	pingErr      error
	tools        []MCPToolInfo
	closed       bool
	closeErr     error
	callCount    int64
	pingCount    int64
}

func (m *mockMCPClient) Initialize(ctx context.Context) error {
	return m.initErr
}

func (m *mockMCPClient) ListTools(ctx context.Context) ([]MCPToolInfo, error) {
	if m.listToolsErr != nil {
		return nil, m.listToolsErr
	}
	return m.tools, nil
}

func (m *mockMCPClient) CallTool(ctx context.Context, toolName string, args json.RawMessage) (*MCPCallToolResult, error) {
	atomic.AddInt64(&m.callCount, 1)
	if m.callToolFn != nil {
		return m.callToolFn(ctx, toolName, args)
	}
	if m.callToolErr != nil {
		return nil, m.callToolErr
	}
	return &MCPCallToolResult{Content: "ok"}, nil
}

func (m *mockMCPClient) Ping(ctx context.Context) error {
	atomic.AddInt64(&m.pingCount, 1)
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.pingErr
}

func (m *mockMCPClient) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return m.closeErr
}

// mockMCPClientFactory returns a factory function that produces mockMCPClients.
type mockMCPClientFactory struct {
	mu      sync.Mutex
	clients []*mockMCPClient
	nextErr error
}

func (f *mockMCPClientFactory) create(command string, args []string, env map[string]string) (MCPClient, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.nextErr != nil {
		return nil, f.nextErr
	}
	client := &mockMCPClient{
		tools: []MCPToolInfo{
			{Name: "tool1", Description: "Test tool 1"},
			{Name: "tool2", Description: "Test tool 2"},
		},
	}
	f.clients = append(f.clients, client)
	return client, nil
}

func (f *mockMCPClientFactory) lastClient() *mockMCPClient {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.clients) == 0 {
		return nil
	}
	return f.clients[len(f.clients)-1]
}

// ─────────────────────────────────────────────────────────────────────────────
// Helper to build a valid test manifest
// ─────────────────────────────────────────────────────────────────────────────

func testManifest(name, command string) *PluginManifest {
	return &PluginManifest{
		Metadata: ManifestMetadata{
			Name:    name,
			Version: "1.0.0",
		},
		Spec: ManifestSpec{
			Type: "tool",
			Runtime: ManifestRuntime{
				Transport: "stdio",
				Command:   command,
				Args:      []string{"--mode", "mcp"},
				Env:       map[string]string{"CUSTOM_VAR": "value"},
				HealthCheck: HealthCheckConfig{
					Interval: 60,
					Timeout:  5,
				},
			},
			Permissions: ManifestPermissions{
				Tools: ToolPermissions{
					Provide: []string{"tool1", "tool2"},
				},
			},
		},
		Name:    name,
		Version: "1.0.0",
	}
}

func newTestRuntimeManager(factory MCPClientFactory) *RuntimeManager {
	cfg := DefaultConfig()
	return NewRuntimeManager(cfg, factory, slog.Default())
}

// ─────────────────────────────────────────────────────────────────────────────
// StartPlugin tests
// ─────────────────────────────────────────────────────────────────────────────

func TestStartPlugin_Success(t *testing.T) {
	t.Parallel()
	factory := &mockMCPClientFactory{}
	rm := newTestRuntimeManager(factory.create)

	manifest := testManifest("test-plugin", "./server")
	tenantID := uuid.New()

	tools, err := rm.StartPlugin(context.Background(), manifest, tenantID)
	if err != nil {
		t.Fatalf("StartPlugin failed: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
	if tools[0].Name != "tool1" {
		t.Errorf("expected tool1, got %s", tools[0].Name)
	}
	if tools[1].Name != "tool2" {
		t.Errorf("expected tool2, got %s", tools[1].Name)
	}
}

func TestStartPlugin_CommandValidation_Allowed(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		command string
		wantErr error
	}{
		{"allowed ./server", "./server", nil},
		{"allowed ./plugin", "./plugin", nil},
		{"allowed ./bin/server", "./bin/server", nil},
		{"absolute path /usr/bin/bash", "/usr/bin/bash", ErrPathTraversal},
		{"disallowed python", "python", ErrCommandNotAllowed},
		{"path traversal ../server", "../server", ErrPathTraversal},
		{"path traversal ./../../bin/sh", "./../../bin/sh", ErrPathTraversal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			factory := &mockMCPClientFactory{}
			rm := newTestRuntimeManager(factory.create)

			manifest := testManifest("test-plugin", tt.command)
			_, err := rm.StartPlugin(context.Background(), manifest, uuid.New())

			if tt.wantErr == nil && err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected error %v, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestStartPlugin_TenantIDPropagation(t *testing.T) {
	t.Parallel()
	var capturedEnv map[string]string
	factory := func(command string, args []string, env map[string]string) (MCPClient, error) {
		capturedEnv = env
		return &mockMCPClient{
			tools: []MCPToolInfo{{Name: "t1", Description: "test"}},
		}, nil
	}

	rm := NewRuntimeManager(DefaultConfig(), factory, slog.Default())
	manifest := testManifest("test-plugin", "./server")
	tenantID := uuid.New()

	_, err := rm.StartPlugin(context.Background(), manifest, tenantID)
	if err != nil {
		t.Fatalf("StartPlugin failed: %v", err)
	}

	if got, ok := capturedEnv["ARGO_TENANT_ID"]; !ok {
		t.Fatal("ARGO_TENANT_ID not set in env")
	} else if got != tenantID.String() {
		t.Errorf("ARGO_TENANT_ID = %q, want %q", got, tenantID.String())
	}
}

func TestStartPlugin_MCPInitializeError(t *testing.T) {
	t.Parallel()
	factory := func(command string, args []string, env map[string]string) (MCPClient, error) {
		return &mockMCPClient{initErr: errors.New("init failed")}, nil
	}

	rm := NewRuntimeManager(DefaultConfig(), factory, slog.Default())
	manifest := testManifest("test-plugin", "./server")

	_, err := rm.StartPlugin(context.Background(), manifest, uuid.New())
	if err == nil {
		t.Fatal("expected error from MCP initialize failure")
	}
	if !containsString(err.Error(), "initialize") {
		t.Errorf("error should mention initialize: %v", err)
	}
}

func TestStartPlugin_MCPListToolsError(t *testing.T) {
	t.Parallel()
	factory := func(command string, args []string, env map[string]string) (MCPClient, error) {
		return &mockMCPClient{listToolsErr: errors.New("tools/list failed")}, nil
	}

	rm := NewRuntimeManager(DefaultConfig(), factory, slog.Default())
	manifest := testManifest("test-plugin", "./server")

	_, err := rm.StartPlugin(context.Background(), manifest, uuid.New())
	if err == nil {
		t.Fatal("expected error from tools/list failure")
	}
}

func TestStartPlugin_FactoryError(t *testing.T) {
	t.Parallel()
	factory := func(command string, args []string, env map[string]string) (MCPClient, error) {
		return nil, errors.New("cannot create client")
	}

	rm := NewRuntimeManager(DefaultConfig(), factory, slog.Default())
	manifest := testManifest("test-plugin", "./server")

	_, err := rm.StartPlugin(context.Background(), manifest, uuid.New())
	if err == nil {
		t.Fatal("expected error from factory failure")
	}
}

func TestStartPlugin_BlockedEnvVar(t *testing.T) {
	t.Parallel()
	factory := &mockMCPClientFactory{}
	rm := newTestRuntimeManager(factory.create)

	manifest := testManifest("test-plugin", "./server")
	manifest.Spec.Runtime.Env["PATH"] = "/evil/path"

	_, err := rm.StartPlugin(context.Background(), manifest, uuid.New())
	if !errors.Is(err, ErrBlockedEnvVar) {
		t.Fatalf("expected ErrBlockedEnvVar, got: %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// StopPlugin tests
// ─────────────────────────────────────────────────────────────────────────────

func TestStopPlugin_Graceful(t *testing.T) {
	t.Parallel()
	factory := &mockMCPClientFactory{}
	rm := newTestRuntimeManager(factory.create)

	manifest := testManifest("test-plugin", "./server")
	_, err := rm.StartPlugin(context.Background(), manifest, uuid.New())
	if err != nil {
		t.Fatalf("StartPlugin: %v", err)
	}

	err = rm.StopPlugin(context.Background(), "test-plugin", 5*time.Second)
	if err != nil {
		t.Fatalf("StopPlugin: %v", err)
	}

	client := factory.lastClient()
	if client == nil {
		t.Fatal("no client created")
	}
	client.mu.Lock()
	closed := client.closed
	client.mu.Unlock()
	if !closed {
		t.Error("expected client to be closed after stop")
	}
}

func TestStopPlugin_NotFound(t *testing.T) {
	t.Parallel()
	factory := &mockMCPClientFactory{}
	rm := newTestRuntimeManager(factory.create)

	err := rm.StopPlugin(context.Background(), "nonexistent", 5*time.Second)
	if !errors.Is(err, ErrPluginNotFound) {
		t.Fatalf("expected ErrPluginNotFound, got: %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// StopAll tests
// ─────────────────────────────────────────────────────────────────────────────

func TestStopAll(t *testing.T) {
	t.Parallel()
	factory := &mockMCPClientFactory{}
	rm := newTestRuntimeManager(factory.create)

	for i := 0; i < 3; i++ {
		name := fmt.Sprintf("plugin-%d", i)
		manifest := testManifest(name, "./server")
		_, err := rm.StartPlugin(context.Background(), manifest, uuid.New())
		if err != nil {
			t.Fatalf("StartPlugin %s: %v", name, err)
		}
	}

	err := rm.StopAll(5 * time.Second)
	if err != nil {
		t.Fatalf("StopAll: %v", err)
	}

	// Verify all clients were closed.
	factory.mu.Lock()
	for _, c := range factory.clients {
		c.mu.Lock()
		if !c.closed {
			t.Error("expected all clients to be closed")
		}
		c.mu.Unlock()
	}
	factory.mu.Unlock()
}

func TestStopAll_EmptyIsNoop(t *testing.T) {
	t.Parallel()
	factory := &mockMCPClientFactory{}
	rm := newTestRuntimeManager(factory.create)

	err := rm.StopAll(5 * time.Second)
	if err != nil {
		t.Fatalf("StopAll on empty should not error, got: %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// WaitInFlight tests
// ─────────────────────────────────────────────────────────────────────────────

func TestWaitInFlight_NoInflight(t *testing.T) {
	t.Parallel()
	factory := &mockMCPClientFactory{}
	rm := newTestRuntimeManager(factory.create)

	manifest := testManifest("test-plugin", "./server")
	_, err := rm.StartPlugin(context.Background(), manifest, uuid.New())
	if err != nil {
		t.Fatalf("StartPlugin: %v", err)
	}

	err = rm.WaitInFlight(context.Background(), "test-plugin", 2*time.Second)
	if err != nil {
		t.Fatalf("WaitInFlight with no inflight calls should succeed, got: %v", err)
	}
}

func TestWaitInFlight_NotFound(t *testing.T) {
	t.Parallel()
	factory := &mockMCPClientFactory{}
	rm := newTestRuntimeManager(factory.create)

	err := rm.WaitInFlight(context.Background(), "nonexistent", 2*time.Second)
	if !errors.Is(err, ErrPluginNotFound) {
		t.Fatalf("expected ErrPluginNotFound, got: %v", err)
	}
}

func TestWaitInFlight_WaitsForCompletion(t *testing.T) {
	t.Parallel()
	callStarted := make(chan struct{})
	callDone := make(chan struct{})

	factory := func(command string, args []string, env map[string]string) (MCPClient, error) {
		return &mockMCPClient{
			tools: []MCPToolInfo{{Name: "slow-tool", Description: "slow"}},
			callToolFn: func(ctx context.Context, toolName string, args json.RawMessage) (*MCPCallToolResult, error) {
				close(callStarted)
				<-callDone
				return &MCPCallToolResult{Content: "done"}, nil
			},
		}, nil
	}

	rm := NewRuntimeManager(DefaultConfig(), factory, slog.Default())
	manifest := testManifest("test-plugin", "./server")
	_, err := rm.StartPlugin(context.Background(), manifest, uuid.New())
	if err != nil {
		t.Fatalf("StartPlugin: %v", err)
	}

	// Start a slow tool call in a goroutine.
	go func() {
		_, _ = rm.CallTool(context.Background(), "test-plugin", "slow-tool", nil)
	}()

	// Wait for the call to start.
	<-callStarted

	// WaitInFlight should block until the call completes.
	waitDone := make(chan error, 1)
	go func() {
		waitDone <- rm.WaitInFlight(context.Background(), "test-plugin", 5*time.Second)
	}()

	// Let the call finish.
	time.Sleep(50 * time.Millisecond)
	close(callDone)

	select {
	case err := <-waitDone:
		if err != nil {
			t.Fatalf("WaitInFlight should succeed, got: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("WaitInFlight timed out")
	}
}

func TestWaitInFlight_Timeout(t *testing.T) {
	t.Parallel()
	callStarted := make(chan struct{})

	factory := func(command string, args []string, env map[string]string) (MCPClient, error) {
		return &mockMCPClient{
			tools: []MCPToolInfo{{Name: "blocking-tool", Description: "blocks forever"}},
			callToolFn: func(ctx context.Context, toolName string, args json.RawMessage) (*MCPCallToolResult, error) {
				close(callStarted)
				<-ctx.Done() // block until context cancelled
				return nil, ctx.Err()
			},
		}, nil
	}

	rm := NewRuntimeManager(DefaultConfig(), factory, slog.Default())
	manifest := testManifest("test-plugin", "./server")
	_, err := rm.StartPlugin(context.Background(), manifest, uuid.New())
	if err != nil {
		t.Fatalf("StartPlugin: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start a blocking tool call.
	go func() {
		_, _ = rm.CallTool(ctx, "test-plugin", "blocking-tool", nil)
	}()
	<-callStarted

	// WaitInFlight with a very short timeout should fail.
	err = rm.WaitInFlight(context.Background(), "test-plugin", 100*time.Millisecond)
	if !errors.Is(err, ErrPluginTimeout) {
		t.Fatalf("expected ErrPluginTimeout, got: %v", err)
	}

	cancel() // clean up the blocking call
}

// ─────────────────────────────────────────────────────────────────────────────
// CallTool tests
// ─────────────────────────────────────────────────────────────────────────────

func TestCallTool_Success(t *testing.T) {
	t.Parallel()
	factory := &mockMCPClientFactory{}
	rm := newTestRuntimeManager(factory.create)

	manifest := testManifest("test-plugin", "./server")
	_, err := rm.StartPlugin(context.Background(), manifest, uuid.New())
	if err != nil {
		t.Fatalf("StartPlugin: %v", err)
	}

	result, err := rm.CallTool(context.Background(), "test-plugin", "tool1", json.RawMessage(`{"key":"value"}`))
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Content != "ok" {
		t.Errorf("expected content 'ok', got %q", result.Content)
	}
}

func TestCallTool_PluginNotFound(t *testing.T) {
	t.Parallel()
	factory := &mockMCPClientFactory{}
	rm := newTestRuntimeManager(factory.create)

	_, err := rm.CallTool(context.Background(), "nonexistent", "tool1", nil)
	if !errors.Is(err, ErrPluginNotFound) {
		t.Fatalf("expected ErrPluginNotFound, got: %v", err)
	}
}

func TestCallTool_PluginNotEnabled(t *testing.T) {
	t.Parallel()
	factory := &mockMCPClientFactory{}
	rm := newTestRuntimeManager(factory.create)

	manifest := testManifest("test-plugin", "./server")
	_, err := rm.StartPlugin(context.Background(), manifest, uuid.New())
	if err != nil {
		t.Fatalf("StartPlugin: %v", err)
	}

	// Manually set plugin state to disabled.
	rm.mu.Lock()
	rm.processes["test-plugin"].enabled = false
	rm.mu.Unlock()

	_, err = rm.CallTool(context.Background(), "test-plugin", "tool1", nil)
	if !errors.Is(err, ErrPluginNotEnabled) {
		t.Fatalf("expected ErrPluginNotEnabled, got: %v", err)
	}
}

func TestCallTool_CircuitBreakerOpen(t *testing.T) {
	t.Parallel()
	factory := func(command string, args []string, env map[string]string) (MCPClient, error) {
		return &mockMCPClient{
			tools:       []MCPToolInfo{{Name: "tool1", Description: "test"}},
			callToolErr: errors.New("server error"),
		}, nil
	}

	cfg := DefaultConfig()
	cfg.CircuitBreakerThreshold = 2
	cfg.CircuitBreakerResetTimeout = 1 * time.Hour // long timeout so it stays open
	rm := NewRuntimeManager(cfg, factory, slog.Default())

	manifest := testManifest("test-plugin", "./server")
	_, err := rm.StartPlugin(context.Background(), manifest, uuid.New())
	if err != nil {
		t.Fatalf("StartPlugin: %v", err)
	}

	// Trip the circuit breaker with consecutive failures.
	for i := 0; i < 2; i++ {
		_, err := rm.CallTool(context.Background(), "test-plugin", "tool1", nil)
		if err == nil {
			t.Fatal("expected error from CallTool")
		}
	}

	// Next call should be rejected by circuit breaker.
	_, err = rm.CallTool(context.Background(), "test-plugin", "tool1", nil)
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got: %v", err)
	}
}

func TestCallTool_CircuitBreakerClosed_Allows(t *testing.T) {
	t.Parallel()
	callCount := int64(0)
	factory := func(command string, args []string, env map[string]string) (MCPClient, error) {
		return &mockMCPClient{
			tools: []MCPToolInfo{{Name: "tool1", Description: "test"}},
			callToolFn: func(ctx context.Context, toolName string, args json.RawMessage) (*MCPCallToolResult, error) {
				atomic.AddInt64(&callCount, 1)
				return &MCPCallToolResult{Content: "ok"}, nil
			},
		}, nil
	}

	rm := NewRuntimeManager(DefaultConfig(), factory, slog.Default())
	manifest := testManifest("test-plugin", "./server")
	_, err := rm.StartPlugin(context.Background(), manifest, uuid.New())
	if err != nil {
		t.Fatalf("StartPlugin: %v", err)
	}

	_, err = rm.CallTool(context.Background(), "test-plugin", "tool1", nil)
	if err != nil {
		t.Fatalf("CallTool should succeed with closed circuit: %v", err)
	}
	if atomic.LoadInt64(&callCount) != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestCallTool_PluginCrashedDuringCall(t *testing.T) {
	t.Parallel()
	factory := func(command string, args []string, env map[string]string) (MCPClient, error) {
		return &mockMCPClient{
			tools: []MCPToolInfo{{Name: "tool1", Description: "test"}},
			callToolFn: func(ctx context.Context, toolName string, args json.RawMessage) (*MCPCallToolResult, error) {
				return nil, errors.New("broken pipe: plugin process exited")
			},
		}, nil
	}

	rm := NewRuntimeManager(DefaultConfig(), factory, slog.Default())
	manifest := testManifest("test-plugin", "./server")
	_, err := rm.StartPlugin(context.Background(), manifest, uuid.New())
	if err != nil {
		t.Fatalf("StartPlugin: %v", err)
	}

	_, err = rm.CallTool(context.Background(), "test-plugin", "tool1", nil)
	if err == nil {
		t.Fatal("expected error when plugin crashes")
	}
	// Error should be descriptive.
	if !containsString(err.Error(), "test-plugin") {
		t.Errorf("error should mention plugin name: %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Backoff calculation test
// ─────────────────────────────────────────────────────────────────────────────

func TestBackoffDuration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 1 * time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{4, 16 * time.Second},
		{5, 30 * time.Second}, // capped at max
		{6, 30 * time.Second},
		{10, 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("attempt_%d", tt.attempt), func(t *testing.T) {
			got := backoffDuration(tt.attempt, 1*time.Second, 30*time.Second)
			if got != tt.want {
				t.Errorf("backoffDuration(%d) = %v, want %v", tt.attempt, got, tt.want)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Helper
// ─────────────────────────────────────────────────────────────────────────────

func containsString(s, substr string) bool {
	return strings.Contains(s, substr)
}
