package telemetry

import (
	"context"
	"sync"
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
	metricsOnce     sync.Once
	genAITokenUsage metric.Int64Histogram
	genAIOpDuration metric.Float64Histogram
)

// InitMetrics initializes GenAI metrics. Call after Setup().
// Safe to call multiple times — initialization runs exactly once via sync.Once.
func InitMetrics() error {
	var initErr error
	metricsOnce.Do(func() {
		meter := otel.GetMeterProvider().Meter("argoclaw/genai")

		var err error
		genAITokenUsage, err = meter.Int64Histogram(
			"gen_ai.client.token.usage",
			metric.WithDescription("Number of tokens used per GenAI request"),
			metric.WithUnit("{token}"),
		)
		if err != nil {
			initErr = err
			return
		}

		genAIOpDuration, err = meter.Float64Histogram(
			"gen_ai.client.operation.duration",
			metric.WithDescription("Duration of GenAI client operations in milliseconds"),
			metric.WithUnit("ms"),
			metric.WithExplicitBucketBoundaries(0, 100, 250, 500, 1000, 2500, 5000, 10000, 30000),
		)
		if err != nil {
			initErr = err
			return
		}
	})
	return initErr
}

// RecordLLMCall wraps an LLM API call with OpenTelemetry tracing and metrics.
// attrs is passed as a pointer so callers can update InputTokens/OutputTokens/FinishReason
// after fn returns (post-call callback pattern) and have those values reflected in metrics.
func RecordLLMCall(ctx context.Context, attrs *GenAIAttrs, fn func(context.Context) error) error {
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
		if genAITokenUsage != nil {
			total := int64(attrs.InputTokens + attrs.OutputTokens)
			genAITokenUsage.Record(ctx, total,
				metric.WithAttributeSet(commonAttrs))
		}
	}

	if genAIOpDuration != nil {
		genAIOpDuration.Record(ctx, durationMs,
			metric.WithAttributeSet(commonAttrs))
	}

	if attrs.FinishReason != "" {
		span.SetAttributes(attribute.String(AttrGenAIResponseFinishReasons, attrs.FinishReason))
	}

	return callErr
}
