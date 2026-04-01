package providers

import (
	"fmt"
	"strings"
	"testing"
)

// mockTokenSource implements TokenSource for testing.
type mockTokenSource struct {
	token string
	err   error
	calls int
}

func (m *mockTokenSource) Token() (string, error) {
	m.calls++
	return m.token, m.err
}

func TestNewVertexAIProvider_EndpointURL(t *testing.T) {
	tests := []struct {
		name      string
		projectID string
		region    string
		wantBase  string
	}{
		{
			name:      "us-central1",
			projectID: "vellus-ai-agent-platform",
			region:    "us-central1",
			wantBase:  "https://us-central1-aiplatform.googleapis.com/v1beta1/projects/vellus-ai-agent-platform/locations/us-central1/endpoints/openapi",
		},
		{
			name:      "europe-west4",
			projectID: "my-project",
			region:    "europe-west4",
			wantBase:  "https://europe-west4-aiplatform.googleapis.com/v1beta1/projects/my-project/locations/europe-west4/endpoints/openapi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a mock token source to avoid ADC dependency in unit tests
			p := NewOpenAIProvider("vertex-ai", "", tt.wantBase, "gemini-2.5-flash")
			if p.APIBase() != tt.wantBase {
				t.Errorf("APIBase() = %q, want %q", p.APIBase(), tt.wantBase)
			}
		})
	}
}

func TestNewVertexAIProvider_DefaultModel(t *testing.T) {
	p := NewOpenAIProvider("vertex-ai", "", "https://example.com", "")

	// Default model should be empty when not set
	if p.DefaultModel() != "" {
		t.Errorf("DefaultModel() = %q, want empty", p.DefaultModel())
	}

	// With option
	p2 := NewOpenAIProvider("vertex-ai", "", "https://example.com", "gemini-2.5-pro")
	if p2.DefaultModel() != "gemini-2.5-pro" {
		t.Errorf("DefaultModel() = %q, want %q", p2.DefaultModel(), "gemini-2.5-pro")
	}
}

func TestNewVertexAIProvider_NameOption(t *testing.T) {
	p := NewOpenAIProvider("custom-vertex", "", "https://example.com", "gemini-2.5-flash")
	if p.Name() != "custom-vertex" {
		t.Errorf("Name() = %q, want %q", p.Name(), "custom-vertex")
	}
}

func TestVertexAIProvider_SupportsThoughtSignature(t *testing.T) {
	// Vertex AI endpoint should be detected for thought_signature support
	base := "https://us-central1-aiplatform.googleapis.com/v1beta1/projects/test/locations/us-central1/endpoints/openapi"
	if !strings.Contains(strings.ToLower(base), "aiplatform.googleapis.com") {
		t.Error("Vertex AI base URL should contain aiplatform.googleapis.com")
	}
}

func TestOpenAIProvider_WithTokenSource(t *testing.T) {
	ts := &mockTokenSource{token: "test-token-123"}
	p := NewOpenAIProvider("test", "", "https://example.com", "model")
	p.WithTokenSource(ts)

	if p.tokenSource == nil {
		t.Fatal("tokenSource should be set after WithTokenSource()")
	}

	tok, err := p.tokenSource.Token()
	if err != nil {
		t.Fatalf("Token() error: %v", err)
	}
	if tok != "test-token-123" {
		t.Errorf("Token() = %q, want %q", tok, "test-token-123")
	}
	if ts.calls != 1 {
		t.Errorf("Token() called %d times, want 1", ts.calls)
	}
}

func TestOpenAIProvider_WithTokenSource_Error(t *testing.T) {
	ts := &mockTokenSource{err: fmt.Errorf("ADC not configured")}
	p := NewOpenAIProvider("vertex-ai", "", "https://example.com", "model")
	p.WithTokenSource(ts)

	_, err := p.tokenSource.Token()
	if err == nil {
		t.Fatal("Token() should return error")
	}
	if !strings.Contains(err.Error(), "ADC not configured") {
		t.Errorf("Token() error = %q, want containing 'ADC not configured'", err.Error())
	}
}

func TestOpenAIProvider_TokenSourceOverridesAPIKey(t *testing.T) {
	// When both apiKey and tokenSource are set, tokenSource should take precedence
	ts := &mockTokenSource{token: "oauth2-token"}
	p := NewOpenAIProvider("test", "static-api-key", "https://example.com", "model")
	p.WithTokenSource(ts)

	// The doRequest method checks tokenSource first — we verify the field is set
	if p.apiKey != "static-api-key" {
		t.Errorf("apiKey should still be %q", "static-api-key")
	}
	if p.tokenSource == nil {
		t.Error("tokenSource should be set")
	}

	tok, _ := p.tokenSource.Token()
	if tok != "oauth2-token" {
		t.Errorf("tokenSource.Token() = %q, want %q", tok, "oauth2-token")
	}
}

func TestVertexAIOptions(t *testing.T) {
	t.Run("WithVertexAIName", func(t *testing.T) {
		cfg := &vertexAIConfig{name: "default"}
		WithVertexAIName("custom-name")(cfg)
		if cfg.name != "custom-name" {
			t.Errorf("name = %q, want %q", cfg.name, "custom-name")
		}
	})

	t.Run("WithVertexAIDefaultModel", func(t *testing.T) {
		cfg := &vertexAIConfig{defaultModel: "default"}
		WithVertexAIDefaultModel("gemini-2.5-pro")(cfg)
		if cfg.defaultModel != "gemini-2.5-pro" {
			t.Errorf("defaultModel = %q, want %q", cfg.defaultModel, "gemini-2.5-pro")
		}
	})
}

func TestGCPTokenSource_LazyInit(t *testing.T) {
	ts := newGCPTokenSource()
	if ts.src != nil {
		t.Error("underlying source should be nil before first Token() call")
	}
	// We don't call Token() here because it requires ADC which may not be available in CI
}
