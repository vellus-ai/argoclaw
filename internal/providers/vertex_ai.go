package providers

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// VertexAIOption configures a Vertex AI provider.
type VertexAIOption func(*vertexAIConfig)

type vertexAIConfig struct {
	name         string
	defaultModel string
}

// WithVertexAIName overrides the provider name (default: "vertex-ai").
func WithVertexAIName(name string) VertexAIOption {
	return func(c *vertexAIConfig) { c.name = name }
}

// WithVertexAIDefaultModel overrides the default model (default: "gemini-2.5-flash").
func WithVertexAIDefaultModel(model string) VertexAIOption {
	return func(c *vertexAIConfig) { c.defaultModel = model }
}

// NewVertexAIProvider creates a provider that calls Gemini (and other models) via
// Vertex AI's OpenAI-compatible endpoint.
//
// Authentication uses Application Default Credentials (ADC):
//   - On GKE: automatic via Workload Identity (zero config)
//   - On VMs: via attached service account
//   - Locally: via `gcloud auth application-default login`
//
// The provider reuses OpenAIProvider with a dynamic TokenSource for OAuth2 bearer tokens.
func NewVertexAIProvider(projectID, region string, opts ...VertexAIOption) *OpenAIProvider {
	cfg := &vertexAIConfig{
		name:         "vertex-ai",
		defaultModel: "gemini-2.5-flash",
	}
	for _, opt := range opts {
		opt(cfg)
	}

	// Vertex AI OpenAI-compatible endpoint:
	// https://{REGION}-aiplatform.googleapis.com/v1beta1/projects/{PROJECT}/locations/{REGION}/endpoints/openapi
	base := fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1beta1/projects/%s/locations/%s/endpoints/openapi",
		region, projectID, region)

	p := NewOpenAIProvider(cfg.name, "", base, cfg.defaultModel)
	p.WithProviderType("vertex_ai")
	p.WithTokenSource(newGCPTokenSource())
	return p
}

// gcpTokenSource wraps an oauth2.TokenSource to implement our TokenSource interface.
// It caches the underlying token and handles auto-refresh.
type gcpTokenSource struct {
	mu  sync.Mutex
	src oauth2.TokenSource // lazy-initialized
}

// newGCPTokenSource creates a TokenSource that obtains OAuth2 access tokens
// via Google Application Default Credentials (ADC).
func newGCPTokenSource() *gcpTokenSource {
	return &gcpTokenSource{}
}

// Token returns a valid OAuth2 access token, refreshing if expired.
func (g *gcpTokenSource) Token() (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.src == nil {
		// Initialize on first call. Uses ADC which auto-detects:
		// 1. GOOGLE_APPLICATION_CREDENTIALS env var (service account key file)
		// 2. gcloud CLI credentials (gcloud auth application-default login)
		// 3. GCE/GKE metadata server (Workload Identity, attached SA)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		ts, err := google.DefaultTokenSource(ctx, "https://www.googleapis.com/auth/cloud-platform")
		if err != nil {
			return "", fmt.Errorf("vertex-ai: ADC not available: %w", err)
		}
		// oauth2.ReuseTokenSource handles caching and auto-refresh
		g.src = oauth2.ReuseTokenSource(nil, ts)
		slog.Info("vertex-ai: initialized GCP token source via ADC")
	}

	tok, err := g.src.Token()
	if err != nil {
		return "", fmt.Errorf("vertex-ai: token refresh failed: %w", err)
	}
	return tok.AccessToken, nil
}
