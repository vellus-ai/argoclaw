package telemetry

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Config for OpenTelemetry setup.
type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	// OTLPEndpoint overrides OTEL_EXPORTER_OTLP_ENDPOINT env var.
	// If both are empty, telemetry is a no-op.
	OTLPEndpoint string
}

// Setup initializes OpenTelemetry SDK (tracer provider + meter provider).
// Returns a shutdown function. If no OTLP endpoint is configured, returns no-op shutdown.
func Setup(ctx context.Context, cfg Config) (shutdown func(context.Context) error, err error) {
	endpoint := cfg.OTLPEndpoint
	if endpoint == "" {
		endpoint = os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	}

	if endpoint == "" {
		return func(context.Context) error { return nil }, nil
	}

	serviceName := cfg.ServiceName
	if v := os.Getenv("OTEL_SERVICE_NAME"); serviceName == "" && v != "" {
		serviceName = v
	}
	if serviceName == "" {
		serviceName = "argoclaw"
	}

	version := cfg.ServiceVersion
	if version == "" {
		version = "dev"
	}

	environment := cfg.Environment
	if environment == "" {
		environment = "production"
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(version),
			semconv.DeploymentEnvironment(environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create OTel resource: %w", err)
	}

	conn, err := grpc.NewClient(endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to OTLP endpoint %s: %w", endpoint, err)
	}

	traceExp, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("create trace exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(0.1))),
	)
	otel.SetTracerProvider(tp)

	metricExp, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithGRPCConn(conn))
	if err != nil {
		tp.Shutdown(ctx)
		conn.Close()
		return nil, fmt.Errorf("create metric exporter: %w", err)
	}

	mp := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(metricExp,
			metric.WithInterval(15*time.Second))),
		metric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	return func(ctx context.Context) error {
		var errs []error
		if e := tp.Shutdown(ctx); e != nil {
			errs = append(errs, e)
		}
		if e := mp.Shutdown(ctx); e != nil {
			errs = append(errs, e)
		}
		if e := conn.Close(); e != nil {
			errs = append(errs, e)
		}
		if len(errs) > 0 {
			return fmt.Errorf("OTel shutdown: %v", errs)
		}
		return nil
	}, nil
}
