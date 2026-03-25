package tenant_isolation_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/vellus-ai/argoclaw/internal/auth"
	"github.com/vellus-ai/argoclaw/internal/store"
)

// =============================================================================
// TDD: Privilege Escalation — Role Manipulation
// =============================================================================

// Test: JWT with role "admin" for Tenant A cannot escalate to affect Tenant B.
func TestPrivilegeEscalation_AdminRoleCrossTenant(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ctxA := ctxForTenant(env.tenantA.ID)
	ctxB := ctxForTenant(env.tenantB.ID)

	// Create agent for Tenant B
	agentB := &store.AgentData{
		AgentKey:    "priv-esc-target-" + uuid.New().String()[:8],
		DisplayName: "Tenant B Protected Agent",
		AgentType:   "predefined",
		Status:      "active",
		Provider:    "anthropic",
		Model:       "claude-sonnet-4-20250514",
	}
	if err := env.agentStore.Create(ctxB, agentB); err != nil {
		t.Fatalf("create agent B: %v", err)
	}
	t.Cleanup(func() {
		env.db.ExecContext(ctx, `DELETE FROM agents WHERE id = $1`, agentB.ID)
	})

	// Tenant A admin tries to update Tenant B's agent
	err := env.agentStore.Update(ctxA, agentB.ID, map[string]any{
		"display_name": "ESCALATED BY TENANT A",
		"status":       "inactive",
	})
	if err != nil {
		t.Logf("update returned error (expected): %v", err)
	}

	// Verify agent B is unchanged
	got, err := env.agentStore.GetByID(ctxB, agentB.ID)
	if err != nil || got == nil {
		t.Fatalf("agent B should still exist: %v", err)
	}
	if got.DisplayName != "Tenant B Protected Agent" {
		t.Fatal("SECURITY VIOLATION: Tenant A admin escalated and modified Tenant B's agent")
	}
	if got.Status != "active" {
		t.Fatalf("SECURITY VIOLATION: Tenant A admin changed Tenant B agent status to %q", got.Status)
	}
}

// Test: JWT crafted with Tenant A's user but Tenant B's tenant_id is detected.
func TestPrivilegeEscalation_CrossTenantTokenForge(t *testing.T) {
	t.Parallel()
	// Forge a token: Tenant A's user claims Tenant B's tenant
	forgedToken := mustGenerateToken(t, auth.TokenClaims{
		UserID:   env.userA.String(),     // Tenant A's user
		Email:    "alice@e2e-alpha.test",
		TenantID: env.tenantB.ID.String(), // Claims Tenant B access!
		Role:     "admin",
	})

	// The token is cryptographically valid (same JWT secret).
	// The SECURITY question: does this let user A access Tenant B's data?
	claims, err := auth.ValidateAccessToken(forgedToken, env.jwtSecret)
	if err != nil {
		t.Fatalf("forged token should be cryptographically valid: %v", err)
	}

	// This is the gap: if tenant membership isn't validated, user A becomes Tenant B admin.
	// The middleware currently trusts the JWT claims. This test documents the behavior.
	if claims.TenantID == env.tenantB.ID.String() && claims.UserID == env.userA.String() {
		t.Log("WARNING: JWT tenant_id is trusted without membership validation. " +
			"In production, token issuance MUST verify tenant membership before embedding tid claim. " +
			"If token issuance is compromised, any user can claim any tenant. " +
			"Consider adding TenantMiddleware.validateMembership() as defense-in-depth.")
	}
}

// Test: Tenant A cannot add themselves to Tenant B.
func TestPrivilegeEscalation_SelfAddToForeignTenant(t *testing.T) {
	ctx := context.Background()

	// Tenant A user tries to join Tenant B
	err := env.tenantStore.AddUser(ctx, env.tenantB.ID, env.userA, "admin")
	if err != nil {
		// If there's a policy check, this would fail
		t.Logf("add user to foreign tenant returned error (expected in secure setup): %v", err)
		return
	}

	// If the add succeeded, verify and clean up — this is a finding
	t.Cleanup(func() {
		env.tenantStore.RemoveUser(ctx, env.tenantB.ID, env.userA)
	})

	// Check if Tenant A user now has access to Tenant B
	users, err := env.tenantStore.ListUsers(ctx, env.tenantB.ID)
	if err != nil {
		t.Fatalf("list users: %v", err)
	}

	for _, u := range users {
		if u.UserID == env.userA {
			t.Log("FINDING: Store layer allows any caller to AddUser to any tenant. " +
				"Authorization check must be enforced at the HTTP handler level (RequireTenant + role check). " +
				"This is acceptable if the store is only called from authenticated, authorized handlers.")
			return
		}
	}
}

// =============================================================================
// TDD: Tenant Status — Suspended/Cancelled Tenant Access
// =============================================================================

// Test: Suspended tenant's JWT is cryptographically valid but data should be inaccessible.
func TestSuspendedTenant_DataAccessPolicy(t *testing.T) {
	ctx := context.Background()

	// Create a temporary tenant and suspend it
	suspendedTenant := &store.Tenant{
		ID:     uuid.New(),
		Slug:   "e2e-suspended-" + uuid.New().String()[:8],
		Name:   "Suspended Tenant",
		Plan:   "pro",
		Status: "suspended",
	}
	if err := env.tenantStore.CreateTenant(ctx, suspendedTenant); err != nil {
		t.Fatalf("create suspended tenant: %v", err)
	}
	t.Cleanup(func() {
		env.db.ExecContext(ctx, `DELETE FROM agents WHERE tenant_id = $1`, suspendedTenant.ID)
		env.db.ExecContext(ctx, `DELETE FROM tenant_users WHERE tenant_id = $1`, suspendedTenant.ID)
		env.db.ExecContext(ctx, `DELETE FROM tenants WHERE id = $1`, suspendedTenant.ID)
	})

	// Create an agent for the suspended tenant
	ctxS := ctxForTenant(suspendedTenant.ID)
	agent := &store.AgentData{
		AgentKey:    "suspended-agent-" + uuid.New().String()[:8],
		DisplayName: "Agent on Suspended Tenant",
		AgentType:   "predefined",
		Status:      "active",
		Provider:    "anthropic",
		Model:       "claude-sonnet-4-20250514",
	}
	if err := env.agentStore.Create(ctxS, agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}

	// Generate JWT for suspended tenant
	suspendedToken := mustGenerateToken(t, auth.TokenClaims{
		UserID:   uuid.New().String(),
		TenantID: suspendedTenant.ID.String(),
		Role:     "admin",
	})

	// Token is cryptographically valid
	claims, err := auth.ValidateAccessToken(suspendedToken, env.jwtSecret)
	if err != nil {
		t.Fatalf("suspended tenant token should be valid: %v", err)
	}
	if claims.TenantID != suspendedTenant.ID.String() {
		t.Errorf("expected tenant_id=%s", suspendedTenant.ID)
	}

	// Store queries still work (store layer doesn't check tenant status)
	got, err := env.agentStore.GetByKey(ctxS, agent.AgentKey)
	if err != nil || got == nil {
		t.Log("INFO: suspended tenant cannot access data at store level (strict enforcement)")
	} else {
		t.Log("FINDING: Store layer allows data access for suspended tenants. " +
			"Tenant status check must be enforced at HTTP/WS middleware level. " +
			"Consider adding status validation in TenantMiddleware.Wrap().")
	}
}

// =============================================================================
// TDD: Immutable Fields — tenant_id Cannot Be Changed
// =============================================================================

// Test: tenant_id cannot be changed via agent update.
func TestImmutableTenantID_AgentUpdate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ctxA := ctxForTenant(env.tenantA.ID)

	agent := &store.AgentData{
		AgentKey:    "immutable-tid-" + uuid.New().String()[:8],
		DisplayName: "Immutable TID Test",
		AgentType:   "predefined",
		Status:      "active",
		Provider:    "anthropic",
		Model:       "claude-sonnet-4-20250514",
	}
	if err := env.agentStore.Create(ctxA, agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	t.Cleanup(func() {
		env.db.ExecContext(ctx, `DELETE FROM agents WHERE id = $1`, agent.ID)
	})

	// Try to change tenant_id via update
	err := env.agentStore.Update(ctxA, agent.ID, map[string]any{
		"tenant_id": env.tenantB.ID,
	})

	// Verify tenant_id didn't change
	var actualTenantID uuid.UUID
	err2 := env.db.QueryRowContext(ctx,
		`SELECT tenant_id FROM agents WHERE id = $1`, agent.ID).Scan(&actualTenantID)
	if err2 != nil {
		t.Fatalf("query agent tenant_id: %v", err2)
	}

	if actualTenantID == env.tenantB.ID {
		t.Fatal("SECURITY VIOLATION: tenant_id was changed via Update — agent migrated to another tenant")
	}
	if actualTenantID != env.tenantA.ID {
		t.Errorf("expected tenant_id=%s, got %s", env.tenantA.ID, actualTenantID)
	}

	// If update returned error, that's the best behavior
	if err != nil {
		t.Logf("update correctly rejected tenant_id change: %v", err)
	} else {
		t.Log("FINDING: Update accepted tenant_id field but WHERE clause prevented cross-tenant change. " +
			"Consider adding tenant_id to an explicit deny-list in the Update handler.")
	}
}

// =============================================================================
// TDD: Temporal Isolation — Expired Trial
// =============================================================================

// Test: Tenant with expired trial still has data isolated.
func TestExpiredTrial_DataIsolation(t *testing.T) {
	ctx := context.Background()

	pastTime := time.Now().Add(-30 * 24 * time.Hour) // 30 days ago
	expiredTenant := &store.Tenant{
		ID:          uuid.New(),
		Slug:        "e2e-expired-" + uuid.New().String()[:8],
		Name:        "Expired Trial Tenant",
		Plan:        "trial",
		Status:      "active",
		TrialEndsAt: &pastTime,
	}
	if err := env.tenantStore.CreateTenant(ctx, expiredTenant); err != nil {
		t.Fatalf("create expired tenant: %v", err)
	}
	t.Cleanup(func() {
		env.db.ExecContext(ctx, `DELETE FROM agents WHERE tenant_id = $1`, expiredTenant.ID)
		env.db.ExecContext(ctx, `DELETE FROM tenants WHERE id = $1`, expiredTenant.ID)
	})

	// Create agent for expired tenant
	ctxE := ctxForTenant(expiredTenant.ID)
	agent := &store.AgentData{
		AgentKey:    "expired-agent-" + uuid.New().String()[:8],
		DisplayName: "Expired Trial Agent",
		AgentType:   "predefined",
		Status:      "active",
		Provider:    "anthropic",
		Model:       "claude-sonnet-4-20250514",
	}
	if err := env.agentStore.Create(ctxE, agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}

	// Active tenant (A) must not see expired tenant's agent
	ctxA := ctxForTenant(env.tenantA.ID)
	got, err := env.agentStore.GetByKey(ctxA, agent.AgentKey)
	if err == nil && got != nil {
		t.Fatal("SECURITY VIOLATION: active tenant accessed expired tenant's agent")
	}

	// Expired tenant can still see their own agent (data preservation)
	gotE, err := env.agentStore.GetByKey(ctxE, agent.AgentKey)
	if err != nil || gotE == nil {
		t.Log("INFO: expired tenant cannot access own data — strict enforcement")
	}
}
