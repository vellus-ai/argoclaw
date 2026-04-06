package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/vellus-ai/argoclaw/internal/store"
)

// --- Onboarding Store Interface ---

// OnboardingStore defines the persistence operations needed by onboarding tools.
// Implemented by the PostgreSQL store layer.
type OnboardingStore interface {
	UpdateTenantSettings(ctx context.Context, tenantID string, key string, value any) error
	UpdateTenantBranding(ctx context.Context, tenantID string, primaryColor, productName string) error
	GetOnboardingStatus(ctx context.Context, tenantID string) (map[string]any, error)
	CompleteOnboarding(ctx context.Context, tenantID string) error
}

// OnboardingStoreAware tools receive an OnboardingStore for tenant configuration.
type OnboardingStoreAware interface {
	SetOnboardingStore(OnboardingStore)
}

// hexColorRe validates hex color format: #RGB or #RRGGBB
var hexColorRe = regexp.MustCompile(`^#[0-9A-Fa-f]{3}([0-9A-Fa-f]{3})?$`)

// validAccountTypes are the allowed values for workspace type.
var validAccountTypes = map[string]bool{"personal": true, "business": true}

// --- 1. ConfigureWorkspaceTool ---

type ConfigureWorkspaceTool struct {
	store OnboardingStore
}

func NewConfigureWorkspaceTool() *ConfigureWorkspaceTool { return &ConfigureWorkspaceTool{} }
func (t *ConfigureWorkspaceTool) SetOnboardingStore(s OnboardingStore) { t.store = s }

func (t *ConfigureWorkspaceTool) Name() string { return "configure_workspace" }

func (t *ConfigureWorkspaceTool) Description() string {
	return `Configure the workspace type and basic info. Call this after determining if the user is setting up for personal use or business. Parameters: type ("personal" or "business"), account_name (company or personal name), industry (optional), team_size (optional).`
}

func (t *ConfigureWorkspaceTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"type":         map[string]any{"type": "string", "enum": []string{"personal", "business"}, "description": "Account type"},
			"account_name": map[string]any{"type": "string", "description": "Company name or personal account name"},
			"industry":     map[string]any{"type": "string", "description": "Industry sector (business only)"},
			"team_size":    map[string]any{"type": "string", "description": "Team size: solo, small, medium, large, enterprise"},
		},
		"required": []string{"type", "account_name"},
	}
}

func (t *ConfigureWorkspaceTool) Execute(ctx context.Context, args map[string]any) *Result {
	if t.store == nil {
		return ErrorResult("onboarding store not available")
	}

	accType, _ := args["type"].(string)
	accName, _ := args["account_name"].(string)
	accName = strings.TrimSpace(accName)
	if accType == "" || accName == "" {
		return ErrorResult("type and account_name are required")
	}
	if !validAccountTypes[accType] {
		return ErrorResult("type must be 'personal' or 'business'")
	}
	if len(accName) > 255 {
		return ErrorResult("account_name must be 255 characters or fewer")
	}

	tenantID := tenantIDFromCtx(ctx)
	if tenantID == "" {
		return ErrorResult("tenant context not available")
	}

	settings := map[string]any{"account_type": accType, "account_name": accName}
	if industry, ok := args["industry"].(string); ok && industry != "" {
		settings["industry"] = industry
	}
	if size, ok := args["team_size"].(string); ok && size != "" {
		settings["team_size"] = size
	}

	for k, v := range settings {
		if err := t.store.UpdateTenantSettings(ctx, tenantID, k, v); err != nil {
			return ErrorResult(fmt.Sprintf("failed to update %s: %v", k, err))
		}
	}

	return NewResult(fmt.Sprintf("Workspace configured: type=%s, name=%s", accType, accName))
}

// --- 2. SetBrandingTool ---

type SetBrandingTool struct {
	store OnboardingStore
}

func NewSetBrandingTool() *SetBrandingTool { return &SetBrandingTool{} }
func (t *SetBrandingTool) SetOnboardingStore(s OnboardingStore) { t.store = s }

func (t *SetBrandingTool) Name() string { return "set_branding" }

func (t *SetBrandingTool) Description() string {
	return `Set the workspace branding: primary color (hex) and product name. Parameters: primary_color (hex like "#3B82F6"), product_name (default "ARGO"). Only provided fields are updated — omitted fields keep their current value.`
}

func (t *SetBrandingTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"primary_color": map[string]any{"type": "string", "description": "Primary color in hex (e.g. #3B82F6)"},
			"product_name":  map[string]any{"type": "string", "description": "Product name (default: ARGO)"},
		},
	}
}

func (t *SetBrandingTool) Execute(ctx context.Context, args map[string]any) *Result {
	if t.store == nil {
		return ErrorResult("onboarding store not available")
	}

	tenantID := tenantIDFromCtx(ctx)
	if tenantID == "" {
		return ErrorResult("tenant context not available")
	}

	color, _ := args["primary_color"].(string)
	name, _ := args["product_name"].(string)
	if color == "" && name == "" {
		return ErrorResult("at least primary_color or product_name is required")
	}

	// Validate hex color format
	if color != "" && !hexColorRe.MatchString(color) {
		return ErrorResult("primary_color must be a valid hex color (e.g. #3B82F6)")
	}

	if err := t.store.UpdateTenantBranding(ctx, tenantID, color, name); err != nil {
		return ErrorResult(fmt.Sprintf("failed to update branding: %v", err))
	}

	var parts []string
	if color != "" {
		parts = append(parts, fmt.Sprintf("color=%s", color))
	}
	if name != "" {
		parts = append(parts, fmt.Sprintf("name=%s", name))
	}

	return NewResult(fmt.Sprintf("Branding updated: %s", strings.Join(parts, ", ")))
}

// --- 3. ConfigureLLMProviderTool ---
// NOTE: This tool is a STUB. It validates input but does NOT persist the provider.
// The actual provider creation requires the HTTP API /v1/providers which handles
// encryption and DB persistence. This tool exists so the Imediato can guide the
// user through the setup flow and collect the information.

type ConfigureLLMProviderTool struct{}

func NewConfigureLLMProviderTool() *ConfigureLLMProviderTool { return &ConfigureLLMProviderTool{} }

func (t *ConfigureLLMProviderTool) Name() string { return "configure_llm_provider" }

func (t *ConfigureLLMProviderTool) Description() string {
	return `Collect LLM provider configuration. This validates the input but does NOT persist yet — the user must complete setup via the dashboard settings. Supported providers: anthropic, openai, google. Parameters: provider (name), api_key (the key), model (preferred model).`
}

func (t *ConfigureLLMProviderTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"provider": map[string]any{"type": "string", "enum": []string{"anthropic", "openai", "google"}, "description": "LLM provider"},
			"api_key":  map[string]any{"type": "string", "description": "API key for the provider"},
			"model":    map[string]any{"type": "string", "description": "Preferred model (e.g. claude-sonnet-4, gpt-4o, gemini-2.5-flash)"},
		},
		"required": []string{"provider", "api_key"},
	}
}

func (t *ConfigureLLMProviderTool) Execute(ctx context.Context, args map[string]any) *Result {
	provider, _ := args["provider"].(string)
	apiKey, _ := args["api_key"].(string)
	model, _ := args["model"].(string)

	if provider == "" || apiKey == "" {
		return ErrorResult("provider and api_key are required")
	}

	if len(apiKey) < 10 {
		return ErrorResult("API key appears too short — please check and try again")
	}

	// Mask the key in response — never expose full key in LLM context
	masked := apiKey[:4] + "***"

	return NewResult(fmt.Sprintf("Provider info collected: provider=%s, model=%s, key=%s. NOTE: To activate this provider, go to Settings > Providers in your dashboard and complete the setup there. The API key will be encrypted at rest.", provider, model, masked))
}

// --- 4. TestLLMConnectionTool ---

type TestLLMConnectionTool struct{}

func NewTestLLMConnectionTool() *TestLLMConnectionTool { return &TestLLMConnectionTool{} }

func (t *TestLLMConnectionTool) Name() string { return "test_llm_connection" }

func (t *TestLLMConnectionTool) Description() string {
	return `Validate an LLM API key format. Does NOT make an actual API call — full connection testing happens when the provider is activated via the dashboard. Parameters: provider, api_key.`
}

func (t *TestLLMConnectionTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"provider": map[string]any{"type": "string", "description": "LLM provider to test"},
			"api_key":  map[string]any{"type": "string", "description": "API key to validate"},
		},
		"required": []string{"provider", "api_key"},
	}
}

func (t *TestLLMConnectionTool) Execute(ctx context.Context, args map[string]any) *Result {
	provider, _ := args["provider"].(string)
	apiKey, _ := args["api_key"].(string)

	if provider == "" || apiKey == "" {
		return ErrorResult("provider and api_key are required")
	}

	if len(apiKey) < 10 {
		return ErrorResult("API key appears too short — please check and try again")
	}

	// Mask key in response
	masked := apiKey[:4] + "***"

	return NewResult(fmt.Sprintf("Key format validated: provider=%s, key=%s. Full connection test will run when the provider is activated via the dashboard.", provider, masked))
}

// --- 5. CreateAgentTool ---
// NOTE: This tool is a STUB. It collects agent configuration but does NOT persist.
// The actual agent creation happens via the agent management API.

type CreateAgentTool struct{}

func NewCreateAgentTool() *CreateAgentTool { return &CreateAgentTool{} }

func (t *CreateAgentTool) Name() string { return "create_agent" }

func (t *CreateAgentTool) Description() string {
	return `Collect configuration for a new AI agent. This does NOT create the agent yet — the user must finalize via the dashboard. Available presets: captain (direct leader), helmsman (navigator/planner), lookout (researcher/analyst), gunner (executor/implementer), navigator (strategist), blacksmith (builder/creator), custom (define your own). Parameters: name, preset, persona (custom description).`
}

func (t *CreateAgentTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":    map[string]any{"type": "string", "description": "Display name for the agent"},
			"preset":  map[string]any{"type": "string", "enum": []string{"captain", "helmsman", "lookout", "gunner", "navigator", "blacksmith", "custom"}, "description": "Personality preset"},
			"persona": map[string]any{"type": "string", "description": "Custom persona description (for custom preset)"},
		},
		"required": []string{"name", "preset"},
	}
}

func (t *CreateAgentTool) Execute(ctx context.Context, args map[string]any) *Result {
	name, _ := args["name"].(string)
	preset, _ := args["preset"].(string)
	persona, _ := args["persona"].(string)

	if name == "" || preset == "" {
		return ErrorResult("name and preset are required")
	}

	presetDescriptions := map[string]string{
		"captain":    "Direct, decisive leader focused on results and clear communication",
		"helmsman":   "Navigator and planner who charts the course and manages timelines",
		"lookout":    "Researcher and analyst who gathers intelligence and identifies opportunities",
		"gunner":     "Executor and implementer who gets things done with precision",
		"navigator":  "Strategist who analyzes data and provides actionable insights",
		"blacksmith": "Builder and creator who crafts solutions and tools",
	}

	description := persona
	if description == "" {
		if desc, ok := presetDescriptions[preset]; ok {
			description = desc
		} else {
			description = "Custom AI agent"
		}
	}

	return NewResult(fmt.Sprintf("Agent config collected: name=%s, preset=%s, description=%s. NOTE: To create this agent, go to Agents in your dashboard. The Imediato (your Chief of Staff) is already active.", name, preset, description))
}

// --- 6. ConfigureChannelTool ---

type ConfigureChannelTool struct{}

func NewConfigureChannelTool() *ConfigureChannelTool { return &ConfigureChannelTool{} }

func (t *ConfigureChannelTool) Name() string { return "configure_channel" }

func (t *ConfigureChannelTool) Description() string {
	return `Guide channel configuration. Webchat is always enabled. Other channels (telegram, whatsapp, discord, slack) require setup via the dashboard Settings > Channels. This tool provides guidance — it does NOT store credentials.`
}

func (t *ConfigureChannelTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"channel": map[string]any{"type": "string", "enum": []string{"webchat", "telegram", "whatsapp", "discord", "slack"}, "description": "Channel type"},
		},
		"required": []string{"channel"},
	}
}

func (t *ConfigureChannelTool) Execute(ctx context.Context, args map[string]any) *Result {
	channel, _ := args["channel"].(string)
	if channel == "" {
		return ErrorResult("channel type is required")
	}

	switch channel {
	case "webchat":
		return NewResult("Webchat is always enabled by default. Your users can access it at the Ponte de Comando URL.")
	case "telegram":
		return NewResult("To connect Telegram: 1) Create a bot via @BotFather, 2) Copy the bot token, 3) Go to Settings > Channels in your dashboard and paste the token there. The token will be encrypted at rest.")
	case "whatsapp":
		return NewResult("WhatsApp integration requires a Meta Business account and WhatsApp Business API. Go to Settings > Channels in your dashboard for the step-by-step setup guide.")
	case "discord", "slack":
		return NewResult(fmt.Sprintf("%s integration is available on Pro plan and above. Go to Settings > Channels in your dashboard to set it up.", channel))
	default:
		return ErrorResult(fmt.Sprintf("unsupported channel: %s", channel))
	}
}

// --- 7. CompleteOnboardingTool ---

type CompleteOnboardingTool struct {
	store OnboardingStore
}

func NewCompleteOnboardingTool() *CompleteOnboardingTool { return &CompleteOnboardingTool{} }
func (t *CompleteOnboardingTool) SetOnboardingStore(s OnboardingStore) { t.store = s }

func (t *CompleteOnboardingTool) Name() string { return "complete_onboarding" }

func (t *CompleteOnboardingTool) Description() string {
	return `Mark the onboarding as complete and transition the Imediato from onboarding mode to normal Chief of Staff mode. Call this when the user has finished the initial setup or explicitly wants to skip remaining steps.`
}

func (t *CompleteOnboardingTool) Parameters() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (t *CompleteOnboardingTool) Execute(ctx context.Context, args map[string]any) *Result {
	if t.store == nil {
		return ErrorResult("onboarding store not available")
	}

	tenantID := tenantIDFromCtx(ctx)
	if tenantID == "" {
		return ErrorResult("tenant context not available")
	}

	if err := t.store.CompleteOnboarding(ctx, tenantID); err != nil {
		return ErrorResult(fmt.Sprintf("failed to complete onboarding: %v", err))
	}

	return NewResult("Onboarding complete! The workspace is now fully operational. Transitioning to normal Chief of Staff mode.")
}

// --- 8. GetOnboardingStatusTool ---

type GetOnboardingStatusTool struct {
	store OnboardingStore
}

func NewGetOnboardingStatusTool() *GetOnboardingStatusTool { return &GetOnboardingStatusTool{} }
func (t *GetOnboardingStatusTool) SetOnboardingStore(s OnboardingStore) { t.store = s }

func (t *GetOnboardingStatusTool) Name() string { return "get_onboarding_status" }

func (t *GetOnboardingStatusTool) Description() string {
	return `Get the current onboarding progress. Returns what has been configured (workspace, branding, provider, agents, channels) and what remains.`
}

func (t *GetOnboardingStatusTool) Parameters() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (t *GetOnboardingStatusTool) Execute(ctx context.Context, args map[string]any) *Result {
	if t.store == nil {
		return ErrorResult("onboarding store not available")
	}

	tenantID := tenantIDFromCtx(ctx)
	if tenantID == "" {
		return ErrorResult("tenant context not available")
	}

	status, err := t.store.GetOnboardingStatus(ctx, tenantID)
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to get status: %v", err))
	}

	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to serialize status: %v", err))
	}
	return NewResult(string(data))
}

// --- Helper ---

// tenantIDFromCtx extracts the tenant UUID from the request context.
// Uses store.TenantIDFromContext which reads the argoclaw_tenant_id key
// set by the JWT middleware or provisioning flow.
func tenantIDFromCtx(ctx context.Context) string {
	id := store.TenantIDFromContext(ctx)
	if id == uuid.Nil {
		return ""
	}
	return id.String()
}
