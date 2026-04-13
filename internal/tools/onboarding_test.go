package tools

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"testing/quick"

	"github.com/google/uuid"
	"github.com/vellus-ai/argoclaw/internal/store"
)

// --- Mock OnboardingStore ---

type mockOnboardingStore struct {
	settings        map[string]map[string]any // tenantID → key → value
	branding        map[string][2]string      // tenantID → [color, name]
	completed       map[string]bool           // tenantID → completed
	failOnUpdate    bool
	failOnBranding  bool
	failOnComplete  bool
	failOnGetStatus bool
}

func newMockStore() *mockOnboardingStore {
	return &mockOnboardingStore{
		settings:  make(map[string]map[string]any),
		branding:  make(map[string][2]string),
		completed: make(map[string]bool),
	}
}

func (m *mockOnboardingStore) UpdateTenantSettings(ctx context.Context, tenantID, key string, value any) error {
	if m.failOnUpdate {
		return fmt.Errorf("store error")
	}
	if m.settings[tenantID] == nil {
		m.settings[tenantID] = make(map[string]any)
	}
	m.settings[tenantID][key] = value
	return nil
}

func (m *mockOnboardingStore) UpdateTenantBranding(ctx context.Context, tenantID, primaryColor, productName string) error {
	if m.failOnBranding {
		return fmt.Errorf("branding error")
	}
	m.branding[tenantID] = [2]string{primaryColor, productName}
	return nil
}

func (m *mockOnboardingStore) GetOnboardingStatus(ctx context.Context, tenantID string) (map[string]any, error) {
	if m.failOnGetStatus {
		return nil, fmt.Errorf("status error")
	}
	return map[string]any{
		"workspace_configured": true,
		"branding_set":         m.branding[tenantID] != [2]string{},
		"onboarding_complete":  m.completed[tenantID],
	}, nil
}

func (m *mockOnboardingStore) CompleteOnboarding(ctx context.Context, tenantID string) error {
	if m.failOnComplete {
		return fmt.Errorf("complete error")
	}
	m.completed[tenantID] = true
	return nil
}

func (m *mockOnboardingStore) UpdateLastCompletedState(ctx context.Context, tenantID string, state string) error {
	if m.failOnUpdate {
		return fmt.Errorf("store error")
	}
	return nil
}

// --- Helper: context with tenant ---

func ctxWithTenant(tenantID uuid.UUID) context.Context {
	return store.WithTenantID(context.Background(), tenantID)
}

// ===== ConfigureWorkspaceTool =====

func TestConfigureWorkspaceTool_NoStore(t *testing.T) {
	t.Parallel()
	tool := NewConfigureWorkspaceTool()
	result := tool.Execute(ctxWithTenant(uuid.New()), map[string]any{
		"type": "personal", "account_name": "Test",
	})
	if !result.IsError {
		t.Error("expected error when store is nil")
	}
}

func TestConfigureWorkspaceTool_MissingParams(t *testing.T) {
	t.Parallel()
	ms := newMockStore()
	tool := NewConfigureWorkspaceTool()
	tool.SetOnboardingStore(ms)

	tests := []struct {
		name string
		args map[string]any
	}{
		{"missing type", map[string]any{"account_name": "Test"}},
		{"missing name", map[string]any{"type": "personal"}},
		{"empty name", map[string]any{"type": "business", "account_name": ""}},
		{"empty both", map[string]any{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tool.Execute(ctxWithTenant(uuid.New()), tt.args)
			if !result.IsError {
				t.Errorf("expected error for %s", tt.name)
			}
		})
	}
}

func TestConfigureWorkspaceTool_InvalidType(t *testing.T) {
	t.Parallel()
	ms := newMockStore()
	tool := NewConfigureWorkspaceTool()
	tool.SetOnboardingStore(ms)

	result := tool.Execute(ctxWithTenant(uuid.New()), map[string]any{
		"type": "enterprise", "account_name": "Test",
	})
	if !result.IsError {
		t.Error("expected error for invalid account type")
	}
	if !strings.Contains(result.ForLLM, "personal") {
		t.Error("error should mention valid types")
	}
}

func TestConfigureWorkspaceTool_NameTooLong(t *testing.T) {
	t.Parallel()
	ms := newMockStore()
	tool := NewConfigureWorkspaceTool()
	tool.SetOnboardingStore(ms)

	result := tool.Execute(ctxWithTenant(uuid.New()), map[string]any{
		"type": "business", "account_name": strings.Repeat("x", 256),
	})
	if !result.IsError {
		t.Error("expected error for name > 255 chars")
	}
}

func TestConfigureWorkspaceTool_NoTenantContext(t *testing.T) {
	t.Parallel()
	ms := newMockStore()
	tool := NewConfigureWorkspaceTool()
	tool.SetOnboardingStore(ms)

	result := tool.Execute(context.Background(), map[string]any{
		"type": "personal", "account_name": "Test",
	})
	if !result.IsError {
		t.Error("expected error when tenant context missing")
	}
	if !strings.Contains(result.ForLLM, "tenant context") {
		t.Error("error should mention tenant context")
	}
}

func TestConfigureWorkspaceTool_HappyPath(t *testing.T) {
	t.Parallel()
	ms := newMockStore()
	tool := NewConfigureWorkspaceTool()
	tool.SetOnboardingStore(ms)
	tid := uuid.New()

	result := tool.Execute(ctxWithTenant(tid), map[string]any{
		"type": "business", "account_name": "Vellus Tech", "industry": "Technology", "team_size": "small",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ForLLM)
	}
	if !strings.Contains(result.ForLLM, "Vellus Tech") {
		t.Error("result should contain account name")
	}

	// Verify store received correct tenant ID and settings
	tidStr := tid.String()
	if ms.settings[tidStr]["account_type"] != "business" {
		t.Errorf("account_type = %v, want business", ms.settings[tidStr]["account_type"])
	}
	if ms.settings[tidStr]["account_name"] != "Vellus Tech" {
		t.Errorf("account_name = %v, want Vellus Tech", ms.settings[tidStr]["account_name"])
	}
	if ms.settings[tidStr]["industry"] != "Technology" {
		t.Errorf("industry = %v, want Technology", ms.settings[tidStr]["industry"])
	}
}

func TestConfigureWorkspaceTool_StoreError(t *testing.T) {
	t.Parallel()
	ms := newMockStore()
	ms.failOnUpdate = true
	tool := NewConfigureWorkspaceTool()
	tool.SetOnboardingStore(ms)

	result := tool.Execute(ctxWithTenant(uuid.New()), map[string]any{
		"type": "personal", "account_name": "Test",
	})
	if !result.IsError {
		t.Error("expected error when store fails")
	}
}

// ===== SetBrandingTool =====

func TestSetBrandingTool_NoStore(t *testing.T) {
	t.Parallel()
	tool := NewSetBrandingTool()
	result := tool.Execute(ctxWithTenant(uuid.New()), map[string]any{"primary_color": "#3B82F6"})
	if !result.IsError {
		t.Error("expected error when store is nil")
	}
}

func TestSetBrandingTool_MissingParams(t *testing.T) {
	t.Parallel()
	ms := newMockStore()
	tool := NewSetBrandingTool()
	tool.SetOnboardingStore(ms)

	result := tool.Execute(ctxWithTenant(uuid.New()), map[string]any{})
	if !result.IsError {
		t.Error("expected error when no params provided")
	}
}

func TestSetBrandingTool_InvalidHexColor(t *testing.T) {
	t.Parallel()
	ms := newMockStore()
	tool := NewSetBrandingTool()
	tool.SetOnboardingStore(ms)

	invalid := []string{"red", "invalid", "#GGG", "#12345", "3B82F6", "#fff; background: url(x)"}
	for _, color := range invalid {
		t.Run(color, func(t *testing.T) {
			result := tool.Execute(ctxWithTenant(uuid.New()), map[string]any{"primary_color": color})
			if !result.IsError {
				t.Errorf("expected error for invalid color %q", color)
			}
		})
	}
}

func TestSetBrandingTool_ValidHexColors(t *testing.T) {
	t.Parallel()
	ms := newMockStore()
	tool := NewSetBrandingTool()
	tool.SetOnboardingStore(ms)

	valid := []string{"#3B82F6", "#fff", "#000", "#ABC", "#abcdef"}
	for _, color := range valid {
		t.Run(color, func(t *testing.T) {
			result := tool.Execute(ctxWithTenant(uuid.New()), map[string]any{"primary_color": color})
			if result.IsError {
				t.Errorf("unexpected error for valid color %q: %s", color, result.ForLLM)
			}
		})
	}
}

func TestSetBrandingTool_HappyPath_ColorOnly(t *testing.T) {
	t.Parallel()
	ms := newMockStore()
	tool := NewSetBrandingTool()
	tool.SetOnboardingStore(ms)
	tid := uuid.New()

	result := tool.Execute(ctxWithTenant(tid), map[string]any{"primary_color": "#3B82F6"})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ForLLM)
	}
	// Store should receive color but empty name (partial update)
	b := ms.branding[tid.String()]
	if b[0] != "#3B82F6" {
		t.Errorf("color = %q, want #3B82F6", b[0])
	}
}

func TestSetBrandingTool_HappyPath_BothFields(t *testing.T) {
	t.Parallel()
	ms := newMockStore()
	tool := NewSetBrandingTool()
	tool.SetOnboardingStore(ms)
	tid := uuid.New()

	result := tool.Execute(ctxWithTenant(tid), map[string]any{
		"primary_color": "#10B981", "product_name": "MyBrand",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ForLLM)
	}
	b := ms.branding[tid.String()]
	if b[0] != "#10B981" || b[1] != "MyBrand" {
		t.Errorf("branding = %v, want [#10B981, MyBrand]", b)
	}
}

// ===== ConfigureLLMProviderTool =====

func TestConfigureLLMProviderTool_MissingKey(t *testing.T) {
	t.Parallel()
	tool := NewConfigureLLMProviderTool()
	result := tool.Execute(context.Background(), map[string]any{"provider": "openai"})
	if !result.IsError {
		t.Error("expected error when api_key missing")
	}
}

func TestConfigureLLMProviderTool_ShortKey(t *testing.T) {
	t.Parallel()
	tool := NewConfigureLLMProviderTool()
	result := tool.Execute(context.Background(), map[string]any{
		"provider": "anthropic", "api_key": "short",
	})
	if !result.IsError {
		t.Error("expected error for short API key")
	}
}

func TestConfigureLLMProviderTool_MasksKey(t *testing.T) {
	t.Parallel()
	tool := NewConfigureLLMProviderTool()
	result := tool.Execute(context.Background(), map[string]any{
		"provider": "anthropic", "api_key": "sk-ant-secret-key-1234567890",
		"model": "claude-sonnet-4",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ForLLM)
	}
	// Result must contain masked key, never full key
	if strings.Contains(result.ForLLM, "sk-ant-secret-key-1234567890") {
		t.Error("result must NOT contain full API key")
	}
	if !strings.Contains(result.ForLLM, "sk-a***") {
		t.Errorf("result should contain masked key, got: %s", result.ForLLM)
	}
	// Must mention dashboard setup
	if !strings.Contains(result.ForLLM, "dashboard") {
		t.Error("result should mention completing setup via dashboard")
	}
}

// ===== TestLLMConnectionTool =====

func TestTestLLMConnectionTool_ShortKey(t *testing.T) {
	t.Parallel()
	tool := NewTestLLMConnectionTool()
	result := tool.Execute(context.Background(), map[string]any{
		"provider": "openai", "api_key": "short",
	})
	if !result.IsError {
		t.Error("expected error for short API key")
	}
}

func TestTestLLMConnectionTool_MasksKey(t *testing.T) {
	t.Parallel()
	tool := NewTestLLMConnectionTool()
	result := tool.Execute(context.Background(), map[string]any{
		"provider": "openai", "api_key": "sk-1234567890abcdef",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ForLLM)
	}
	if strings.Contains(result.ForLLM, "sk-1234567890abcdef") {
		t.Error("result must NOT contain full API key")
	}
	if !strings.Contains(result.ForLLM, "sk-1***") {
		t.Error("result should contain masked key")
	}
}

// ===== CreateAgentTool =====

func TestCreateAgentTool_MissingName(t *testing.T) {
	t.Parallel()
	tool := NewCreateAgentTool()
	result := tool.Execute(context.Background(), map[string]any{"preset": "captain"})
	if !result.IsError {
		t.Error("expected error when name missing")
	}
}

func TestCreateAgentTool_ValidPreset(t *testing.T) {
	t.Parallel()
	tool := NewCreateAgentTool()
	result := tool.Execute(context.Background(), map[string]any{
		"name": "Capitão", "preset": "captain",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ForLLM)
	}
	if !strings.Contains(result.ForLLM, "dashboard") {
		t.Error("result should mention completing via dashboard")
	}
}

func TestCreateAgentTool_CustomPreset(t *testing.T) {
	t.Parallel()
	tool := NewCreateAgentTool()
	result := tool.Execute(context.Background(), map[string]any{
		"name": "MyBot", "preset": "custom", "persona": "A friendly assistant",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ForLLM)
	}
	if !strings.Contains(result.ForLLM, "A friendly assistant") {
		t.Error("result should contain custom persona")
	}
}

// ===== ConfigureChannelTool =====

func TestConfigureChannelTool_Webchat(t *testing.T) {
	t.Parallel()
	tool := NewConfigureChannelTool()
	result := tool.Execute(context.Background(), map[string]any{"channel": "webchat"})
	if result.IsError {
		t.Errorf("unexpected error: %s", result.ForLLM)
	}
}

func TestConfigureChannelTool_TelegramGuidance(t *testing.T) {
	t.Parallel()
	tool := NewConfigureChannelTool()
	result := tool.Execute(context.Background(), map[string]any{"channel": "telegram"})
	if result.IsError {
		t.Error("should provide guidance, not error")
	}
	if !strings.Contains(result.ForLLM, "BotFather") {
		t.Error("should mention BotFather for Telegram setup")
	}
	if !strings.Contains(result.ForLLM, "dashboard") {
		t.Error("should direct to dashboard for token entry")
	}
}

func TestConfigureChannelTool_NoBotTokenInResponse(t *testing.T) {
	t.Parallel()
	tool := NewConfigureChannelTool()
	// Channel tool no longer accepts bot_token — only guides to dashboard
	result := tool.Execute(context.Background(), map[string]any{"channel": "telegram"})
	if result.IsError {
		t.Error("should not error")
	}
	if strings.Contains(result.ForLLM, "bot_token") {
		t.Error("response should not contain bot_token field name")
	}
}

func TestConfigureChannelTool_InvalidChannel(t *testing.T) {
	t.Parallel()
	tool := NewConfigureChannelTool()
	result := tool.Execute(context.Background(), map[string]any{"channel": "fax"})
	if !result.IsError {
		t.Error("expected error for unsupported channel")
	}
}

// ===== CompleteOnboardingTool =====

func TestCompleteOnboardingTool_NoStore(t *testing.T) {
	t.Parallel()
	tool := NewCompleteOnboardingTool()
	result := tool.Execute(ctxWithTenant(uuid.New()), map[string]any{})
	if !result.IsError {
		t.Error("expected error when store is nil")
	}
}

func TestCompleteOnboardingTool_NoTenant(t *testing.T) {
	t.Parallel()
	ms := newMockStore()
	tool := NewCompleteOnboardingTool()
	tool.SetOnboardingStore(ms)
	result := tool.Execute(context.Background(), map[string]any{})
	if !result.IsError {
		t.Error("expected error when tenant context missing")
	}
}

func TestCompleteOnboardingTool_HappyPath(t *testing.T) {
	t.Parallel()
	ms := newMockStore()
	tool := NewCompleteOnboardingTool()
	tool.SetOnboardingStore(ms)
	tid := uuid.New()

	result := tool.Execute(ctxWithTenant(tid), map[string]any{})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ForLLM)
	}
	if !ms.completed[tid.String()] {
		t.Error("store should mark onboarding as complete for correct tenant")
	}
}

func TestCompleteOnboardingTool_StoreError(t *testing.T) {
	t.Parallel()
	ms := newMockStore()
	ms.failOnComplete = true
	tool := NewCompleteOnboardingTool()
	tool.SetOnboardingStore(ms)

	result := tool.Execute(ctxWithTenant(uuid.New()), map[string]any{})
	if !result.IsError {
		t.Error("expected error when store fails")
	}
}

// ===== GetOnboardingStatusTool =====

func TestGetOnboardingStatusTool_NoStore(t *testing.T) {
	t.Parallel()
	tool := NewGetOnboardingStatusTool()
	result := tool.Execute(ctxWithTenant(uuid.New()), map[string]any{})
	if !result.IsError {
		t.Error("expected error when store is nil")
	}
}

func TestGetOnboardingStatusTool_HappyPath(t *testing.T) {
	t.Parallel()
	ms := newMockStore()
	tool := NewGetOnboardingStatusTool()
	tool.SetOnboardingStore(ms)
	tid := uuid.New()

	result := tool.Execute(ctxWithTenant(tid), map[string]any{})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ForLLM)
	}
	if !strings.Contains(result.ForLLM, "workspace_configured") {
		t.Error("status should contain workspace_configured field")
	}
}

func TestGetOnboardingStatusTool_StoreError(t *testing.T) {
	t.Parallel()
	ms := newMockStore()
	ms.failOnGetStatus = true
	tool := NewGetOnboardingStatusTool()
	tool.SetOnboardingStore(ms)

	result := tool.Execute(ctxWithTenant(uuid.New()), map[string]any{})
	if !result.IsError {
		t.Error("expected error when store fails")
	}
}

// ===== Tenant Isolation =====

func TestTenantIsolation_DifferentTenantsIndependent(t *testing.T) {
	t.Parallel()
	ms := newMockStore()
	tool := NewConfigureWorkspaceTool()
	tool.SetOnboardingStore(ms)

	tid1 := uuid.New()
	tid2 := uuid.New()

	tool.Execute(ctxWithTenant(tid1), map[string]any{"type": "personal", "account_name": "Tenant1"})
	tool.Execute(ctxWithTenant(tid2), map[string]any{"type": "business", "account_name": "Tenant2"})

	if ms.settings[tid1.String()]["account_name"] != "Tenant1" {
		t.Error("tenant1 should have its own data")
	}
	if ms.settings[tid2.String()]["account_name"] != "Tenant2" {
		t.Error("tenant2 should have its own data")
	}
	if ms.settings[tid1.String()]["account_name"] == ms.settings[tid2.String()]["account_name"] {
		t.Error("tenants should have independent data")
	}
}

func TestTenantIsolation_EmptyContextRejectsAll(t *testing.T) {
	t.Parallel()
	ms := newMockStore()

	tools := []interface {
		Tool
		SetOnboardingStore(OnboardingStore)
	}{
		NewConfigureWorkspaceTool(),
		NewSetBrandingTool(),
		NewCompleteOnboardingTool(),
		NewGetOnboardingStatusTool(),
	}

	for _, tool := range tools {
		tool.SetOnboardingStore(ms)
		t.Run(tool.Name(), func(t *testing.T) {
			args := map[string]any{}
			if tool.Name() == "configure_workspace" {
				args = map[string]any{"type": "personal", "account_name": "Test"}
			}
			if tool.Name() == "set_branding" {
				args = map[string]any{"primary_color": "#FFF"}
			}
			result := tool.Execute(context.Background(), args)
			if !result.IsError {
				t.Errorf("%s should reject when no tenant in context", tool.Name())
			}
		})
	}
}

// ===== PBT =====

func TestPBT_HexColorValidation(t *testing.T) {
	t.Parallel()
	// Property: any string NOT matching ^#[0-9A-Fa-f]{3,6}$ should be rejected
	f := func(s string) bool {
		if s == "" {
			return true // empty is handled by "at least one param required"
		}
		isValid := hexColorRe.MatchString(s)
		ms := newMockStore()
		tool := NewSetBrandingTool()
		tool.SetOnboardingStore(ms)
		result := tool.Execute(ctxWithTenant(uuid.New()), map[string]any{"primary_color": s})
		if isValid {
			return !result.IsError // valid hex → success
		}
		return result.IsError // invalid hex → error
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 5000}); err != nil {
		t.Errorf("PBT hex color: %v", err)
	}
}

func TestPBT_APIKeyMasking(t *testing.T) {
	t.Parallel()
	// Property: for any api_key with len >= 10, the full key never appears in result
	f := func(key string) bool {
		if len(key) < 10 {
			return true // short keys are rejected, not our property
		}
		tool := NewConfigureLLMProviderTool()
		result := tool.Execute(context.Background(), map[string]any{
			"provider": "openai", "api_key": key,
		})
		return !strings.Contains(result.ForLLM, key)
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 5000}); err != nil {
		t.Errorf("PBT api key masking: %v", err)
	}
}

func TestPBT_AllToolsHaveNonEmptyIdentity(t *testing.T) {
	t.Parallel()
	tools := []Tool{
		NewConfigureWorkspaceTool(),
		NewSetBrandingTool(),
		NewConfigureLLMProviderTool(),
		NewTestLLMConnectionTool(),
		NewCreateAgentTool(),
		NewConfigureChannelTool(),
		NewCompleteOnboardingTool(),
		NewGetOnboardingStatusTool(),
	}
	for _, tool := range tools {
		if tool.Name() == "" {
			t.Errorf("tool has empty Name()")
		}
		if tool.Description() == "" {
			t.Errorf("%s has empty Description()", tool.Name())
		}
		params := tool.Parameters()
		if params == nil || params["type"] != "object" {
			t.Errorf("%s Parameters().type != object", tool.Name())
		}
	}
}
