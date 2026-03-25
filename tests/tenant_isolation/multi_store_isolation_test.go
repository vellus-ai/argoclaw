package tenant_isolation_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/vellus-ai/argoclaw/internal/store"
	"github.com/vellus-ai/argoclaw/internal/store/pg"
)

// =============================================================================
// TDD: LLM Provider Store — Cross-Tenant Isolation
// =============================================================================

// Test: Provider created by Tenant A is invisible to Tenant B via ListProviders.
func TestProviderIsolation_ListProviders_CrossTenant(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	providerStore := pg.NewPGProviderStore(env.db, "")

	provider := &store.LLMProviderData{
		Name:         "iso-provider-" + uuid.New().String()[:8],
		DisplayName:  "Tenant A Provider",
		ProviderType: "openai_compatible",
		APIBase:      "https://api.example.com/v1",
		APIKey:       "sk-test-fake-key",
		Enabled:      true,
	}
	provider.ID = uuid.New()

	ctxA := ctxForTenant(env.tenantA.ID)
	if err := providerStore.CreateProvider(ctxA, provider); err != nil {
		t.Fatalf("create provider: %v", err)
	}
	t.Cleanup(func() {
		env.db.ExecContext(ctx, `DELETE FROM llm_providers WHERE id = $1`, provider.ID)
	})

	// Tenant A lists — should include their provider
	listA, err := providerStore.ListProviders(ctxA)
	if err != nil {
		t.Fatalf("list providers A: %v", err)
	}
	found := false
	for _, p := range listA {
		if p.ID == provider.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("tenant A should see own provider in list")
	}

	// Tenant B lists — MUST NOT include Tenant A's provider
	ctxB := ctxForTenant(env.tenantB.ID)
	listB, err := providerStore.ListProviders(ctxB)
	if err != nil {
		t.Fatalf("list providers B: %v", err)
	}
	for _, p := range listB {
		if p.ID == provider.ID {
			t.Fatal("SECURITY VIOLATION: Tenant B sees Tenant A's LLM provider in list")
		}
	}
}

// Test: Provider GetByName is tenant-scoped.
func TestProviderIsolation_GetByName_CrossTenant(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	providerStore := pg.NewPGProviderStore(env.db, "")

	providerName := "iso-named-" + uuid.New().String()[:8]
	provider := &store.LLMProviderData{
		Name:         providerName,
		DisplayName:  "Named Provider A",
		ProviderType: "anthropic",
		APIKey:       "sk-ant-test-fake",
		Enabled:      true,
	}
	provider.ID = uuid.New()

	ctxA := ctxForTenant(env.tenantA.ID)
	if err := providerStore.CreateProvider(ctxA, provider); err != nil {
		t.Fatalf("create provider: %v", err)
	}
	t.Cleanup(func() {
		env.db.ExecContext(ctx, `DELETE FROM llm_providers WHERE id = $1`, provider.ID)
	})

	// Tenant A finds by name
	got, err := providerStore.GetProviderByName(ctxA, providerName)
	if err != nil || got == nil {
		t.Fatalf("tenant A should find provider by name: %v", err)
	}

	// Tenant B MUST NOT find it
	ctxB := ctxForTenant(env.tenantB.ID)
	gotB, err := providerStore.GetProviderByName(ctxB, providerName)
	if err == nil && gotB != nil {
		t.Fatal("SECURITY VIOLATION: Tenant B found Tenant A's provider by name")
	}
}

// =============================================================================
// TDD: Agent Teams Store — Cross-Tenant Isolation
// =============================================================================

// Test: Team created by Tenant A is invisible to Tenant B via ListTeams.
func TestTeamIsolation_ListTeams_CrossTenant(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	teamStore := pg.NewPGTeamStore(env.db)

	// First create a lead agent for the team
	leadAgent := &store.AgentData{
		AgentKey:    "team-lead-" + uuid.New().String()[:8],
		DisplayName: "Team Lead Agent",
		AgentType:   "predefined",
		Status:      "active",
		Provider:    "anthropic",
		Model:       "claude-sonnet-4-20250514",
	}
	ctxA := ctxForTenant(env.tenantA.ID)
	if err := env.agentStore.Create(ctxA, leadAgent); err != nil {
		t.Fatalf("create lead agent: %v", err)
	}
	t.Cleanup(func() {
		env.db.ExecContext(ctx, `DELETE FROM agents WHERE id = $1`, leadAgent.ID)
	})

	team := &store.TeamData{
		Name:        "iso-team-" + uuid.New().String()[:8],
		LeadAgentID: leadAgent.ID,
		Description: "Tenant A Team",
		Status:      "active",
		CreatedBy:   env.userA.String(),
	}
	if err := teamStore.CreateTeam(ctxA, team); err != nil {
		t.Fatalf("create team: %v", err)
	}
	t.Cleanup(func() {
		env.db.ExecContext(ctx, `DELETE FROM agent_teams WHERE id = $1`, team.ID)
	})

	// Tenant A lists — should include their team
	listA, err := teamStore.ListTeams(ctxA)
	if err != nil {
		t.Fatalf("list teams A: %v", err)
	}
	found := false
	for _, tm := range listA {
		if tm.ID == team.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("tenant A should see own team in list")
	}

	// Tenant B lists — MUST NOT include Tenant A's team
	ctxB := ctxForTenant(env.tenantB.ID)
	listB, err := teamStore.ListTeams(ctxB)
	if err != nil {
		t.Fatalf("list teams B: %v", err)
	}
	for _, tm := range listB {
		if tm.ID == team.ID {
			t.Fatal("SECURITY VIOLATION: Tenant B sees Tenant A's team in list")
		}
	}
}

// =============================================================================
// TDD: Agent List — Cross-Tenant Isolation
// =============================================================================

// Test: Agent List only returns agents for the requesting tenant.
func TestAgentIsolation_List_CrossTenant(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Create agents for both tenants
	agentA := &store.AgentData{
		AgentKey:    "list-iso-a-" + uuid.New().String()[:8],
		DisplayName: "List Test Agent A",
		AgentType:   "predefined",
		Status:      "active",
		Provider:    "anthropic",
		Model:       "claude-sonnet-4-20250514",
		OwnerID:     env.userA.String(),
	}
	agentB := &store.AgentData{
		AgentKey:    "list-iso-b-" + uuid.New().String()[:8],
		DisplayName: "List Test Agent B",
		AgentType:   "predefined",
		Status:      "active",
		Provider:    "anthropic",
		Model:       "claude-sonnet-4-20250514",
		OwnerID:     env.userB.String(),
	}

	ctxA := ctxForTenant(env.tenantA.ID)
	ctxB := ctxForTenant(env.tenantB.ID)

	if err := env.agentStore.Create(ctxA, agentA); err != nil {
		t.Fatalf("create agent A: %v", err)
	}
	if err := env.agentStore.Create(ctxB, agentB); err != nil {
		t.Fatalf("create agent B: %v", err)
	}
	t.Cleanup(func() {
		env.db.ExecContext(ctx, `DELETE FROM agents WHERE id = $1`, agentA.ID)
		env.db.ExecContext(ctx, `DELETE FROM agents WHERE id = $1`, agentB.ID)
	})

	// List from Tenant A context
	listA, err := env.agentStore.List(ctxA, "")
	if err != nil {
		t.Fatalf("list agents A: %v", err)
	}
	for _, a := range listA {
		if a.ID == agentB.ID {
			t.Fatal("SECURITY VIOLATION: Tenant A List returned Tenant B's agent")
		}
	}
	foundA := false
	for _, a := range listA {
		if a.ID == agentA.ID {
			foundA = true
			break
		}
	}
	if !foundA {
		t.Error("tenant A should see own agent in List")
	}

	// List from Tenant B context
	listB, err := env.agentStore.List(ctxB, "")
	if err != nil {
		t.Fatalf("list agents B: %v", err)
	}
	for _, a := range listB {
		if a.ID == agentA.ID {
			t.Fatal("SECURITY VIOLATION: Tenant B List returned Tenant A's agent")
		}
	}
}

// =============================================================================
// TDD: Custom Tools Store — Cross-Tenant Isolation
// =============================================================================

// Test: Custom tool created by Tenant A is invisible to Tenant B via ListAll.
func TestCustomToolIsolation_ListAll_CrossTenant(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	toolStore := pg.NewPGCustomToolStore(env.db, "")

	tool := &store.CustomToolDef{
		Name:        "iso-tool-" + uuid.New().String()[:8],
		Description: "Tenant A Secret Tool",
		Command:     "echo test",
		Enabled:     true,
		CreatedBy:   env.userA.String(),
	}
	tool.ID = uuid.New()

	ctxA := ctxForTenant(env.tenantA.ID)
	if err := toolStore.Create(ctxA, tool); err != nil {
		t.Fatalf("create custom tool: %v", err)
	}
	t.Cleanup(func() {
		env.db.ExecContext(ctx, `DELETE FROM custom_tools WHERE id = $1`, tool.ID)
	})

	// Tenant B lists all — MUST NOT include Tenant A's tool
	ctxB := ctxForTenant(env.tenantB.ID)
	listB, err := toolStore.ListAll(ctxB)
	if err != nil {
		t.Fatalf("list tools B: %v", err)
	}
	for _, tl := range listB {
		if tl.ID == tool.ID {
			t.Fatal("SECURITY VIOLATION: Tenant B sees Tenant A's custom tool in ListAll")
		}
	}
}
