package tenant_isolation_test

import (
	"context"
	"math/rand"
	"strings"
	"testing"
	"testing/quick"
	"time"

	"github.com/google/uuid"
	"github.com/vellus-ai/argoclaw/internal/store"
)

// =============================================================================
// TDD: Agent Store — Cross-Tenant Data Isolation
// =============================================================================

// Test: Agent created by Tenant A is invisible to Tenant B.
func TestAgentIsolation_CreateAndGetByKey(t *testing.T) {
	ctx := context.Background()

	agentKey := "iso-agent-" + uuid.New().String()[:8]
	agent := &store.AgentData{
		AgentKey:    agentKey,
		DisplayName: "Tenant A Secret Agent",
		AgentType:   "predefined",
		Status:      "active",
		Provider:    "anthropic",
		Model:       "claude-sonnet-4-20250514",
	}

	// Create agent in Tenant A context
	ctxA := ctxForTenant(env.tenantA.ID)
	if err := env.agentStore.Create(ctxA, agent); err != nil {
		t.Fatalf("create agent for tenant A: %v", err)
	}
	t.Cleanup(func() {
		env.db.ExecContext(ctx, `DELETE FROM agents WHERE id = $1`, agent.ID)
	})

	// Tenant A can see it
	got, err := env.agentStore.GetByKey(ctxA, agentKey)
	if err != nil {
		t.Fatalf("tenant A should see own agent: %v", err)
	}
	if got.AgentKey != agentKey {
		t.Errorf("expected agent_key=%q, got %q", agentKey, got.AgentKey)
	}

	// Tenant B MUST NOT see it
	ctxB := ctxForTenant(env.tenantB.ID)
	gotB, err := env.agentStore.GetByKey(ctxB, agentKey)
	if err == nil && gotB != nil {
		t.Fatalf("SECURITY VIOLATION: Tenant B accessed Tenant A's agent (key=%s, id=%s)", agentKey, gotB.ID)
	}
}

// Test: Agent created by Tenant A is invisible to Tenant B via GetByID.
func TestAgentIsolation_GetByID_CrossTenant(t *testing.T) {
	ctx := context.Background()

	agent := &store.AgentData{
		AgentKey:    "iso-byid-" + uuid.New().String()[:8],
		DisplayName: "Tenant A GetByID Agent",
		AgentType:   "predefined",
		Status:      "active",
		Provider:    "anthropic",
		Model:       "claude-sonnet-4-20250514",
	}

	ctxA := ctxForTenant(env.tenantA.ID)
	if err := env.agentStore.Create(ctxA, agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	t.Cleanup(func() {
		env.db.ExecContext(ctx, `DELETE FROM agents WHERE id = $1`, agent.ID)
	})

	// Tenant A can access by ID
	got, err := env.agentStore.GetByID(ctxA, agent.ID)
	if err != nil {
		t.Fatalf("tenant A should access own agent by ID: %v", err)
	}
	if got.ID != agent.ID {
		t.Errorf("expected id=%s, got %s", agent.ID, got.ID)
	}

	// Tenant B MUST NOT access by ID — even knowing the UUID
	ctxB := ctxForTenant(env.tenantB.ID)
	gotB, err := env.agentStore.GetByID(ctxB, agent.ID)
	if err == nil && gotB != nil {
		t.Fatalf("SECURITY VIOLATION: Tenant B accessed Tenant A's agent by UUID (id=%s)", agent.ID)
	}
}

// Test: Agent update is scoped to the owning tenant.
func TestAgentIsolation_Update_CrossTenant(t *testing.T) {
	ctx := context.Background()

	agent := &store.AgentData{
		AgentKey:    "iso-update-" + uuid.New().String()[:8],
		DisplayName: "Before Update",
		AgentType:   "predefined",
		Status:      "active",
		Provider:    "anthropic",
		Model:       "claude-sonnet-4-20250514",
	}

	ctxA := ctxForTenant(env.tenantA.ID)
	if err := env.agentStore.Create(ctxA, agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	t.Cleanup(func() {
		env.db.ExecContext(ctx, `DELETE FROM agents WHERE id = $1`, agent.ID)
	})

	// Tenant B tries to update Tenant A's agent — MUST be a no-op
	ctxB := ctxForTenant(env.tenantB.ID)
	err := env.agentStore.Update(ctxB, agent.ID, map[string]any{
		"display_name": "HACKED BY TENANT B",
	})
	// Update may not return error (0 rows affected), but data MUST NOT change
	if err != nil {
		t.Logf("update returned error (OK): %v", err)
	}

	// Verify agent is unchanged
	got, err := env.agentStore.GetByID(ctxA, agent.ID)
	if err != nil {
		t.Fatalf("get agent after cross-tenant update attempt: %v", err)
	}
	if got.DisplayName == "HACKED BY TENANT B" {
		t.Fatal("SECURITY VIOLATION: Tenant B modified Tenant A's agent display_name")
	}
	if got.DisplayName != "Before Update" {
		t.Errorf("expected display_name=%q, got %q", "Before Update", got.DisplayName)
	}
}

// Test: Soft-delete is scoped to the owning tenant.
func TestAgentIsolation_SoftDelete_CrossTenant(t *testing.T) {
	ctx := context.Background()

	agent := &store.AgentData{
		AgentKey:    "iso-delete-" + uuid.New().String()[:8],
		DisplayName: "Should Not Be Deleted",
		AgentType:   "predefined",
		Status:      "active",
		Provider:    "anthropic",
		Model:       "claude-sonnet-4-20250514",
	}

	ctxA := ctxForTenant(env.tenantA.ID)
	if err := env.agentStore.Create(ctxA, agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	t.Cleanup(func() {
		env.db.ExecContext(ctx, `DELETE FROM agents WHERE id = $1`, agent.ID)
	})

	// Tenant B tries to soft-delete Tenant A's agent
	ctxB := ctxForTenant(env.tenantB.ID)
	_ = env.agentStore.Update(ctxB, agent.ID, map[string]any{
		"deleted_at": time.Now(),
	})

	// Verify agent is still alive for Tenant A
	got, err := env.agentStore.GetByID(ctxA, agent.ID)
	if err != nil || got == nil {
		t.Fatal("SECURITY VIOLATION: Tenant B soft-deleted Tenant A's agent")
	}
	if got.DisplayName != "Should Not Be Deleted" {
		t.Errorf("agent was modified: got display_name=%q", got.DisplayName)
	}
}

// =============================================================================
// TDD: Tenant Branding — Cross-Tenant Isolation
// =============================================================================

// Test: Branding for Tenant A is invisible to Tenant B.
func TestBrandingIsolation_CrossTenant(t *testing.T) {
	ctx := context.Background()

	branding := &store.TenantBranding{
		TenantID:     env.tenantA.ID,
		PrimaryColor: "#FF0000",
		ProductName:  "Alpha Product",
		LogoURL:      "https://alpha.test/logo.png",
	}

	if err := env.tenantStore.UpsertBranding(ctx, branding); err != nil {
		t.Fatalf("upsert branding: %v", err)
	}
	t.Cleanup(func() {
		env.db.ExecContext(ctx, `DELETE FROM tenant_branding WHERE tenant_id = $1`, env.tenantA.ID)
	})

	// Tenant A can retrieve their branding
	gotA, err := env.tenantStore.GetBranding(ctx, env.tenantA.ID)
	if err != nil || gotA == nil {
		t.Fatalf("tenant A should see own branding: %v", err)
	}
	if gotA.PrimaryColor != "#FF0000" {
		t.Errorf("expected primary_color=#FF0000, got %s", gotA.PrimaryColor)
	}

	// Tenant B queries their own branding — MUST NOT see Tenant A's
	gotB, err := env.tenantStore.GetBranding(ctx, env.tenantB.ID)
	if err != nil {
		t.Fatalf("unexpected error querying tenant B branding: %v", err)
	}
	if gotB != nil && gotB.PrimaryColor == "#FF0000" {
		t.Fatal("SECURITY VIOLATION: Tenant B sees Tenant A's branding")
	}
}

// Test: Tenant membership is isolated.
func TestMembershipIsolation_CrossTenant(t *testing.T) {
	ctx := context.Background()

	// List users of Tenant A
	usersA, err := env.tenantStore.ListUsers(ctx, env.tenantA.ID)
	if err != nil {
		t.Fatalf("list users A: %v", err)
	}

	// List users of Tenant B
	usersB, err := env.tenantStore.ListUsers(ctx, env.tenantB.ID)
	if err != nil {
		t.Fatalf("list users B: %v", err)
	}

	// Ensure no cross-contamination
	for _, u := range usersA {
		if u.TenantID != env.tenantA.ID {
			t.Fatalf("SECURITY VIOLATION: user in tenant A list has wrong tenant_id=%s", u.TenantID)
		}
		for _, uB := range usersB {
			if u.UserID == uB.UserID {
				t.Fatalf("SECURITY VIOLATION: same user %s appears in both tenants", u.UserID)
			}
		}
	}
}

// =============================================================================
// TDD: Direct SQL Injection Attempts
// =============================================================================

// Test: agent_key with SQL injection payload does not bypass tenant filter.
func TestAgentIsolation_SQLInjection_AgentKey(t *testing.T) {
	payloads := []string{
		"' OR 1=1 --",
		"' OR tenant_id IS NOT NULL --",
		"'; DROP TABLE agents; --",
		"' UNION SELECT * FROM agents --",
		"\\'; DELETE FROM agents WHERE ''='",
	}

	ctxA := ctxForTenant(env.tenantA.ID)
	for _, payload := range payloads {
		t.Run(payload[:min(len(payload), 30)], func(t *testing.T) {
			got, err := env.agentStore.GetByKey(ctxA, payload)
			if err == nil && got != nil {
				t.Fatalf("SECURITY VIOLATION: SQL injection payload returned data: %+v", got)
			}
		})
	}
}

// =============================================================================
// PBT: Property-Based Testing — Tenant Isolation Invariants
// =============================================================================

// Property: For ANY random agent_key, Tenant B NEVER sees agents owned by Tenant A.
func TestPBT_AgentKeyNeverLeaksCrossTenant(t *testing.T) {
	ctx := context.Background()
	ctxA := ctxForTenant(env.tenantA.ID)
	ctxB := ctxForTenant(env.tenantB.ID)

	// Create N agents for Tenant A
	const numAgents = 5
	agentIDs := make([]uuid.UUID, 0, numAgents)
	for i := 0; i < numAgents; i++ {
		agent := &store.AgentData{
			AgentKey:    "pbt-agent-" + uuid.New().String()[:8],
			DisplayName: "PBT Agent " + uuid.New().String()[:4],
			AgentType:   "predefined",
			Status:      "active",
			Provider:    "anthropic",
			Model:       "claude-sonnet-4-20250514",
		}
		if err := env.agentStore.Create(ctxA, agent); err != nil {
			t.Fatalf("create PBT agent: %v", err)
		}
		agentIDs = append(agentIDs, agent.ID)
	}
	t.Cleanup(func() {
		for _, id := range agentIDs {
			env.db.ExecContext(ctx, `DELETE FROM agents WHERE id = $1`, id)
		}
	})

	// Build a set of Tenant A agent IDs for O(1) lookup
	agentIDSet := make(map[uuid.UUID]bool, numAgents)
	for _, id := range agentIDs {
		agentIDSet[id] = true
	}

	// Property: random agent_key lookup by Tenant B never returns a Tenant A agent
	f := func(seed int64) bool {
		r := rand.New(rand.NewSource(seed))
		keyLen := r.Intn(30) + 3
		chars := "abcdefghijklmnopqrstuvwxyz0123456789-_"
		var sb strings.Builder
		for i := 0; i < keyLen; i++ {
			sb.WriteByte(chars[r.Intn(len(chars))])
		}
		randomKey := sb.String()

		got, err := env.agentStore.GetByKey(ctxB, randomKey)
		if err != nil {
			return true // not found = OK
		}
		if got == nil {
			return true
		}
		// If Tenant B found an agent, it MUST NOT be one of Tenant A's agents
		return !agentIDSet[got.ID]
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 200}); err != nil {
		t.Errorf("PBT VIOLATION: cross-tenant agent leak detected: %v", err)
	}
}

// Property: For ANY random UUID, Tenant B cannot access Tenant A's agents by ID.
func TestPBT_RandomUUID_NeverAccessesCrossTenantAgent(t *testing.T) {
	ctxB := ctxForTenant(env.tenantB.ID)

	f := func() bool {
		randomID := uuid.New()
		got, err := env.agentStore.GetByID(ctxB, randomID)
		if err != nil {
			return true
		}
		return got == nil
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 500}); err != nil {
		t.Errorf("PBT VIOLATION: random UUID returned agent for wrong tenant: %v", err)
	}
}

// =============================================================================
// TDD: Tenant Store — Slug Uniqueness & Isolation
// =============================================================================

// Test: Two tenants cannot share the same slug.
func TestTenantSlugUniqueness(t *testing.T) {
	ctx := context.Background()

	duplicateTenant := &store.Tenant{
		ID:     uuid.New(),
		Slug:   env.tenantA.Slug, // same slug as Tenant A
		Name:   "Duplicate Slug Tenant",
		Plan:   "trial",
		Status: "active",
	}

	err := env.tenantStore.CreateTenant(ctx, duplicateTenant)
	if err == nil {
		t.Cleanup(func() {
			env.db.ExecContext(ctx, `DELETE FROM tenants WHERE id = $1`, duplicateTenant.ID)
		})
		t.Fatal("INTEGRITY VIOLATION: duplicate slug was allowed")
	}
}

// Test: Tenant A cannot see Tenant B via GetBySlug.
func TestTenantSlugIsolation(t *testing.T) {
	ctx := context.Background()

	gotA, err := env.tenantStore.GetBySlug(ctx, env.tenantA.Slug)
	if err != nil || gotA == nil {
		t.Fatalf("tenant A not found by slug: %v", err)
	}
	if gotA.ID != env.tenantA.ID {
		t.Fatalf("GetBySlug returned wrong tenant: expected %s, got %s", env.tenantA.ID, gotA.ID)
	}

	gotB, err := env.tenantStore.GetBySlug(ctx, env.tenantB.Slug)
	if err != nil || gotB == nil {
		t.Fatalf("tenant B not found by slug: %v", err)
	}
	if gotB.ID == env.tenantA.ID {
		t.Fatal("SECURITY VIOLATION: Tenant B slug returned Tenant A's data")
	}
}

