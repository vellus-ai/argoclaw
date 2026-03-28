package tools

import (
	"context"
	"testing"
)

func TestConfigureWorkspaceTool_Name(t *testing.T) {
	tool := NewConfigureWorkspaceTool()
	if tool.Name() != "configure_workspace" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "configure_workspace")
	}
}

func TestConfigureWorkspaceTool_NoStore(t *testing.T) {
	tool := NewConfigureWorkspaceTool()
	result := tool.Execute(context.Background(), map[string]any{
		"type":         "personal",
		"account_name": "Milton",
	})
	if !result.IsError {
		t.Error("expected error when store is nil")
	}
}

func TestSetBrandingTool_Name(t *testing.T) {
	tool := NewSetBrandingTool()
	if tool.Name() != "set_branding" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "set_branding")
	}
}

func TestSetBrandingTool_MissingParams(t *testing.T) {
	tool := NewSetBrandingTool()
	tool.SetOnboardingStore(nil)
	result := tool.Execute(context.Background(), map[string]any{})
	if !result.IsError {
		t.Error("expected error when no params provided")
	}
}

func TestConfigureLLMProviderTool_MissingKey(t *testing.T) {
	tool := NewConfigureLLMProviderTool()
	result := tool.Execute(context.Background(), map[string]any{
		"provider": "openai",
	})
	if !result.IsError {
		t.Error("expected error when api_key missing")
	}
}

func TestConfigureLLMProviderTool_Success(t *testing.T) {
	tool := NewConfigureLLMProviderTool()
	result := tool.Execute(context.Background(), map[string]any{
		"provider": "anthropic",
		"api_key":  "sk-ant-test-key-12345",
		"model":    "claude-sonnet-4",
	})
	if result.IsError {
		t.Errorf("unexpected error: %s", result.ForLLM)
	}
}

func TestTestLLMConnectionTool_ShortKey(t *testing.T) {
	tool := NewTestLLMConnectionTool()
	result := tool.Execute(context.Background(), map[string]any{
		"provider": "openai",
		"api_key":  "short",
	})
	if !result.IsError {
		t.Error("expected error for short API key")
	}
}

func TestCreateAgentTool_Success(t *testing.T) {
	tool := NewCreateAgentTool()
	result := tool.Execute(context.Background(), map[string]any{
		"name":   "Capitão",
		"preset": "captain",
	})
	if result.IsError {
		t.Errorf("unexpected error: %s", result.ForLLM)
	}
	if result.ForLLM == "" {
		t.Error("expected non-empty result")
	}
}

func TestCreateAgentTool_MissingName(t *testing.T) {
	tool := NewCreateAgentTool()
	result := tool.Execute(context.Background(), map[string]any{
		"preset": "captain",
	})
	if !result.IsError {
		t.Error("expected error when name missing")
	}
}

func TestConfigureChannelTool_Webchat(t *testing.T) {
	tool := NewConfigureChannelTool()
	result := tool.Execute(context.Background(), map[string]any{
		"channel": "webchat",
	})
	if result.IsError {
		t.Errorf("unexpected error: %s", result.ForLLM)
	}
}

func TestConfigureChannelTool_TelegramNoToken(t *testing.T) {
	tool := NewConfigureChannelTool()
	result := tool.Execute(context.Background(), map[string]any{
		"channel": "telegram",
	})
	if result.IsError {
		t.Error("should not error, just ask for token")
	}
}

func TestConfigureChannelTool_InvalidChannel(t *testing.T) {
	tool := NewConfigureChannelTool()
	result := tool.Execute(context.Background(), map[string]any{
		"channel": "fax",
	})
	if !result.IsError {
		t.Error("expected error for unsupported channel")
	}
}

func TestCompleteOnboardingTool_NoStore(t *testing.T) {
	tool := NewCompleteOnboardingTool()
	result := tool.Execute(context.Background(), map[string]any{})
	if !result.IsError {
		t.Error("expected error when store is nil")
	}
}

func TestGetOnboardingStatusTool_NoStore(t *testing.T) {
	tool := NewGetOnboardingStatusTool()
	result := tool.Execute(context.Background(), map[string]any{})
	if !result.IsError {
		t.Error("expected error when store is nil")
	}
}

func TestAllToolsHaveParameters(t *testing.T) {
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
		t.Run(tool.Name(), func(t *testing.T) {
			if tool.Name() == "" {
				t.Error("Name() is empty")
			}
			if tool.Description() == "" {
				t.Error("Description() is empty")
			}
			params := tool.Parameters()
			if params == nil {
				t.Error("Parameters() is nil")
			}
			if params["type"] != "object" {
				t.Errorf("Parameters().type = %v, want 'object'", params["type"])
			}
		})
	}
}
