package telemetry

import "testing"

func TestStripEndpointSchema(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"otel-collector:4317", "otel-collector:4317"},
		{"http://otel-collector:4317", "otel-collector:4317"},
		{"https://otel-collector:4317", "otel-collector:4317"},
		{"http://otel-collector.observability.svc.cluster.local:4317", "otel-collector.observability.svc.cluster.local:4317"},
		{"https://otel.example.com:4318", "otel.example.com:4318"},
		{"localhost:4317", "localhost:4317"},
		{"", ""},
		{"http://", "http://"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := stripEndpointSchema(tt.input)
			if got != tt.want {
				t.Errorf("stripEndpointSchema(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
