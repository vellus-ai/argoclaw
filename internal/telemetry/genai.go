package telemetry

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// OpenTelemetry GenAI Semantic Conventions v1.28+
// https://opentelemetry.io/docs/specs/semconv/gen-ai/
const (
	AttrGenAISystem               = "gen_ai.system"
	AttrGenAIOperationName        = "gen_ai.operation.name"
	AttrGenAIRequestModel         = "gen_ai.request.model"
	AttrGenAIRequestMaxTokens     = "gen_ai.request.max_tokens"
	AttrGenAIUsageInputTokens     = "gen_ai.usage.input_tokens"
	AttrGenAIUsageOutputTokens    = "gen_ai.usage.output_tokens"
	AttrGenAIResponseFinishReasons = "gen_ai.response.finish_reasons"
)

// GenAIAttrs contains attributes for a single GenAI operation.
type GenAIAttrs struct {
	System       string // e.g. "anthropic", "google"
	Model        string // e.g. "claude-3-5-sonnet", "gemini-2.0-flash"
	Operation    string // e.g. "chat", "embeddings" (default: "chat")
	MaxTokens    int
	InputTokens  int
	OutputTokens int
	FinishReason string
}

var (
	genAITokenUsage    metric.Int64Histogram
	genAIOpDuration    metric.Float64Histogram
	metricsInitialized bool
)

// InitMetrics initializes GenAI metrics. Call after Setup().
func InitMetrics() error {
	meter := otel.GetMeterProvider().Meter("argoclaw/genai")

	var err error
	genAITokenUsage, err = meter.Int64Histogram(
		"gen_ai.client.token.usage",
		metric.WithDescription("Number of tokens used per GenAI request"),
		metric.WithUnit("{token}"),
	)
	if err != nil {
		return err
	}

	genAIOpDuration, err = meter.Float64Histogram(
		"gen_ai.client.operation.duration",
		metric.WithDescription("Duration of GenAI client operations in milliseconds"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(0, 100, 250, 500, 1000, 2500, 5000, 10000, 30000),
	)
	if err != nil {
		return err
	}

	metricsInitialized = true
	return nil
}

// RecordLLMCall wraps an LLM API call with OpenTelemetry tracing and metrics.
// Attrs.InputTokens and Attrs.OutputTokens should be set before or after fn returns
// by updating the GenAIAttrs pointer if using a post-call callback pattern.
func RecordLLMCall(ctx context.Context, attrs GenAIAttrs, fn func(context.Context) error) error {
	op := attrs.Operation
	if op == "" {
		op = "chat"
	}

	tracer := otel.Tracer("argoclaw/genai")
	ctx, span := tracer.Start(ctx, attrs.System+"."+op,
		trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	span.SetAttributes(
		attribute.String(AttrGenAISystem, attrs.System),
		attribute.String(AttrGenAIOperationName, op),
		attribute.String(AttrGenAIRequestModel, attrs.Model),
	)
	if attrs.MaxTokens > 0 {
		span.SetAttributes(attribute.Int(AttrGenAIRequestMaxTokens, attrs.MaxTokens))
	}

	start := time.Now()
	callErr := fn(ctx)
	durationMs := float64(time.Since(start).Milliseconds())

	commonAttrs := attribute.NewSet(
		attribute.String(AttrGenAISystem, attrs.System),
		attribute.String(AttrGenAIRequestModel, attrs.Model),
	)

	if attrs.InputTokens > 0 || attrs.OutputTokens > 0 {
		span.SetAttributes(
			attribute.Int(AttrGenAIUsageInputTokens, attrs.InputTokens),
			attribute.Int(AttrGenAIUsageOutputTokens, attrs.OutputTokens),
		)
		if metricsInitialized && genAITokenUsage != nil {
			total := int64(attrs.InputTokens + attrs.OutputTokens)
			genAITokenUsage.Record(ctx, total,
				metric.WithAttributeSet(commonAttrs))
		}
	}

	if metricsInitialized && genAIOpDuration != nil {
		genAIOpDuration.Record(ctx, durationMs,
			metric.WithAttributeSet(commonAttrs))
	}

	if attrs.FinishReason != "" {
		span.SetAttributes(attribute.String(AttrGenAIResponseFinishReasons, attrs.FinishReason))
	}

	return callErr
}
