package agent

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"pgregory.net/rapid"

	"github.com/vellus-ai/argoclaw/internal/store"
)

// =============================================================================
// Preservation Property Tests — Baseline behavior that must NOT change after fix
// =============================================================================
//
// **Validates: Requirements 1.3, 2.3, 3.4 (Preservation)**
//
// These tests capture the CURRENT correct behavior of the Router on unfixed code.
// They must PASS now and continue to PASS after the fix is applied.

// =============================================================================
// Preservation 1 — Gateway Token (WithCrossTenant) resolves without tenant_id
// =============================================================================
//
// **Validates: Requirements 1.3, 2.3**
//
// When a request uses a gateway token, the context is marked with
// store.WithCrossTenant(ctx). In this mode, store.TenantIDFromContext returns
// uuid.Nil and store.IsCrossTenant returns true. The Router.Get(ctx, agentKey)
// call should resolve the agent successfully without requiring a tenant_id.
//
// This behavior must be preserved after the fix — gateway token requests must
// continue to work without tenant_id filtering.

func TestPreservation_GatewayTokenResolvesWithoutTenantID(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		agentKey := rapid.StringMatching(`[a-z][a-z0-9\-]{2,20}`).Draw(t, "agentKey")

		// Simulate gateway token context: WithCrossTenant marks cross-tenant access
		ctx := store.WithCrossTenant(context.Background())

		// Verify preconditions: cross-tenant context has uuid.Nil tenant and IsCrossTenant=true
		if tid := store.TenantIDFromContext(ctx); tid != uuid.Nil {
			t.Fatalf("precondition: WithCrossTenant ctx should have tenant_id=uuid.Nil, got %s", tid)
		}
		if !store.IsCrossTenant(ctx) {
			t.Fatal("precondition: WithCrossTenant ctx should have IsCrossTenant=true")
		}

		resolver := func(ctx context.Context, key string, opts ResolveOpts) (Agent, error) {
			return &mockAgent{id: key}, nil
		}

		router := NewRouter()
		router.SetResolver(resolver)

		// Router.Get with cross-tenant ctx should resolve successfully
		ag, err := router.Get(ctx, agentKey)
		if err != nil {
			t.Fatalf("Router.Get(%q) with gateway token context failed: %v", agentKey, err)
		}
		if ag == nil {
			t.Fatal("Router.Get returned nil agent for gateway token request")
		}
		if ag.ID() != agentKey {
			t.Errorf("Router.Get returned agent with ID=%q, want %q", ag.ID(), agentKey)
		}
	})
}

// =============================================================================
// Preservation 2 — Cache TTL expiration and re-resolution
// =============================================================================
//
// **Validates: Requirements 2.3**
//
// The Router caches agents with a TTL. When the TTL expires, the next Get()
// call should re-resolve the agent via the resolver. This behavior must be
// preserved after the fix introduces composite cache keys (tenantID:agentKey).

func TestPreservation_CacheTTLExpirationTriggersReResolve(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		agentKey := rapid.StringMatching(`[a-z][a-z0-9\-]{2,20}`).Draw(t, "agentKey")
		tenantID := uuid.New()
		ctx := store.WithTenantID(context.Background(), tenantID)

		resolveCount := 0
		resolver := func(ctx context.Context, key string, opts ResolveOpts) (Agent, error) {
			resolveCount++
			return &mockAgent{id: fmt.Sprintf("%s-v%d", key, resolveCount)}, nil
		}

		router := NewRouter()
		router.SetResolver(resolver)
		// Set a very short TTL for testing
		router.ttl = 1 * time.Millisecond

		// First call: resolver is called, agent is cached
		ag1, err := router.Get(ctx, agentKey)
		if err != nil {
			t.Fatalf("first Router.Get(%q) failed: %v", agentKey, err)
		}
		if resolveCount != 1 {
			t.Fatalf("expected 1 resolve call after first Get, got %d", resolveCount)
		}

		// Wait for TTL to expire
		time.Sleep(5 * time.Millisecond)

		// Second call: TTL expired, resolver should be called again
		ag2, err := router.Get(ctx, agentKey)
		if err != nil {
			t.Fatalf("second Router.Get(%q) after TTL expired failed: %v", agentKey, err)
		}
		if resolveCount < 2 {
			t.Errorf("expected resolver to be called again after TTL expiry, got %d calls", resolveCount)
		}

		// The re-resolved agent should have a different ID (new version)
		if ag1.ID() == ag2.ID() {
			t.Errorf("expected different agent after TTL expiry, both have ID=%q", ag1.ID())
		}
	})
}

// =============================================================================
// Preservation 3 — Background goroutine pattern (context.Background() has no tenant)
// =============================================================================
//
// **Validates: Requirements 3.4**
//
// Legitimate background goroutines (GenerateTitle, sessions.Save, cron jobs)
// use context.Background() intentionally. This test confirms the pattern:
// store.TenantIDFromContext(context.Background()) returns uuid.Nil.
// This behavior must be preserved — background goroutines should NOT be
// required to have a tenant_id.

func TestPreservation_BackgroundContextHasNoTenantID(t *testing.T) {
	t.Parallel()

	// Simple assertion — not PBT because context.Background() is deterministic.
	// This confirms the pattern used by legitimate background goroutines.
	bgCtx := context.Background()
	tid := store.TenantIDFromContext(bgCtx)
	if tid != uuid.Nil {
		t.Errorf("context.Background() should have tenant_id=uuid.Nil, got %s", tid)
	}

	// Also verify that IsCrossTenant is false for plain background context
	if store.IsCrossTenant(bgCtx) {
		t.Error("context.Background() should NOT be marked as cross-tenant")
	}
}

// =============================================================================
// Preservation 4 — Router.Register() caches agent directly, Get() returns from cache
// =============================================================================
//
// **Validates: Requirements 2.3**
//
// When an agent is registered directly via Router.Register(ag), subsequent
// Router.Get(ctx, agentKey) calls should return it from cache without calling the
// resolver. Register uses agentKey as-is (no tenant prefix) for backward compat.

func TestPreservation_RegisteredAgentReturnedFromCacheWithoutResolver(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		agentKey := rapid.StringMatching(`[a-z][a-z0-9\-]{2,20}`).Draw(t, "agentKey")

		resolverCalled := false
		resolver := func(ctx context.Context, key string, opts ResolveOpts) (Agent, error) {
			resolverCalled = true
			return &mockAgent{id: "resolver-" + key}, nil
		}

		router := NewRouter()
		router.SetResolver(resolver)

		// Register agent directly (uses agentKey as cache key for backward compat)
		registered := &mockAgent{id: agentKey}
		router.Register(registered)

		// Get with uuid.Nil tenant (no tenant context) should find the registered agent
		// because Register stores with key "00000000-0000-0000-0000-000000000000:agentKey"
		ctx := context.Background() // uuid.Nil tenant
		ag, err := router.Get(ctx, agentKey)
		if err != nil {
			t.Fatalf("Router.Get(%q) after Register failed: %v", agentKey, err)
		}
		if ag == nil {
			t.Fatal("Router.Get returned nil for registered agent")
		}
		if ag.ID() != agentKey {
			t.Errorf("Router.Get returned agent ID=%q, want %q (registered agent)", ag.ID(), agentKey)
		}
		if resolverCalled {
			t.Error("resolver should NOT be called when agent is already registered in cache")
		}
	})
}
