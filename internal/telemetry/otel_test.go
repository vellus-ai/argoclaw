package telemetry_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/vellus-ai/argoclaw/internal/telemetry"
)

func TestSetup_NoopWhenNoEndpoint(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	shutdown, err := telemetry.Setup(context.Background(), telemetry.Config{
		ServiceName: "test-argoclaw",
		Environment: "test",
	})
	if err != nil {
		t.Fatalf("Setup with no endpoint should succeed (noop): %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("noop Shutdown failed: %v", err)
	}
}

func TestGenAI_Constants_MatchSpec(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"AttrGenAISystem", telemetry.AttrGenAISystem, "gen_ai.system"},
		{"AttrGenAIOperationName", telemetry.AttrGenAIOperationName, "gen_ai.operation.name"},
		{"AttrGenAIRequestModel", telemetry.AttrGenAIRequestModel, "gen_ai.request.model"},
		{"AttrGenAIRequestMaxTokens", telemetry.AttrGenAIRequestMaxTokens, "gen_ai.request.max_tokens"},
		{"AttrGenAIUsageInputTokens", telemetry.AttrGenAIUsageInputTokens, "gen_ai.usage.input_tokens"},
		{"AttrGenAIUsageOutputTokens", telemetry.AttrGenAIUsageOutputTokens, "gen_ai.usage.output_tokens"},
		{"AttrGenAIResponseFinishReasons", telemetry.AttrGenAIResponseFinishReasons, "gen_ai.response.finish_reasons"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
			}
		})
	}
}

func TestGenAI_RecordLLMCall_NoOp(t *testing.T) {
	ctx := context.Background()
	err := telemetry.RecordLLMCall(ctx, telemetry.GenAIAttrs{
		System:    "anthropic",
		Model:     "claude-3-5-sonnet",
		Operation: "chat",
	}, func(ctx context.Context) error {
		return nil
	})
	if err != nil {
		t.Fatalf("RecordLLMCall should not return error for successful fn: %v", err)
	}
}

func TestGenAI_RecordLLMCall_PropagatesError(t *testing.T) {
	ctx := context.Background()
	sentinel := fmt.Errorf("llm api error")
	err := telemetry.RecordLLMCall(ctx, telemetry.GenAIAttrs{
		System: "google",
		Model:  "gemini-2.0-flash",
	}, func(ctx context.Context) error {
		return sentinel
	})
	if err != sentinel {
		t.Errorf("RecordLLMCall should propagate fn error; got %v, want %v", err, sentinel)
	}
}
