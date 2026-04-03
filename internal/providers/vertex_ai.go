package providers

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// vertexAIScope is the minimum OAuth2 scope for Vertex AI prediction calls.
// Using aiplatform scope instead of cloud-platform to follow least-privilege.
const vertexAIScope = "https://www.googleapis.com/auth/aiplatform"

// vertexAIProjectIDRe validates GCP project IDs (6-30 chars, lowercase+digits+hyphens).
var vertexAIProjectIDRe = regexp.MustCompile(`^[a-z][a-z0-9-]{4,28}[a-z0-9]$`)

// vertexAIRegionRe validates GCP region identifiers (e.g. us-central1, europe-west4).
var vertexAIRegionRe = regexp.MustCompile(`^[a-z]+-[a-z]+\d+[a-z]?$`)

// VertexAIOption configures a Vertex AI provider.
type VertexAIOption func(*vertexAIConfig)

type vertexAIConfig struct {
	name         string
	defaultModel string
	tokenSource  TokenSource // injectable for testing
}

// WithVertexAIName overrides the provider name (default: "vertex-ai").
func WithVertexAIName(name string) VertexAIOption {
	return func(c *vertexAIConfig) { c.name = name }
}

// WithVertexAIDefaultModel overrides the default model (default: "gemini-2.5-flash").
func WithVertexAIDefaultModel(model string) VertexAIOption {
	return func(c *vertexAIConfig) { c.defaultModel = model }
}

// WithVertexAITokenSource overrides the default GCP ADC token source (for testing).
func WithVertexAITokenSource(ts TokenSource) VertexAIOption {
	return func(c *vertexAIConfig) { c.tokenSource = ts }
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
// projectID and region are validated to prevent SSRF via URL injection.
func NewVertexAIProvider(projectID, region string, opts ...VertexAIOption) (*OpenAIProvider, error) {
	if !vertexAIProjectIDRe.MatchString(projectID) {
		return nil, fmt.Errorf("vertex-ai: invalid project_id %q (must match %s)", projectID, vertexAIProjectIDRe.String())
	}
	if !vertexAIRegionRe.MatchString(region) {
		return nil, fmt.Errorf("vertex-ai: invalid region %q (must match %s)", region, vertexAIRegionRe.String())
	}

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

	ts := cfg.tokenSource
	if ts == nil {
		ts = newGCPTokenSource()
	}

	p := NewOpenAIProvider(cfg.name, "", base, cfg.defaultModel)
	p.WithProviderType("vertex_ai")
	p.WithTokenSource(ts)
	return p, nil
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
// SECURITY: Never log the returned token value.
func (g *gcpTokenSource) Token() (string, error) {
	g.mu.Lock()
	if g.src == nil {
		// Initialize on first call. Uses ADC which auto-detects:
		// 1. GOOGLE_APPLICATION_CREDENTIALS env var (service account key file)
		// 2. gcloud CLI credentials (gcloud auth application-default login)
		// 3. GCE/GKE metadata server (Workload Identity, attached SA)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		ts, err := google.DefaultTokenSource(ctx, vertexAIScope)
		if err != nil {
			g.mu.Unlock()
			return "", fmt.Errorf("vertex-ai: ADC not available: %w", err)
		}
		// oauth2.ReuseTokenSource handles caching and auto-refresh (thread-safe)
		g.src = oauth2.ReuseTokenSource(nil, ts)
		slog.Info("vertex-ai: initialized GCP token source via ADC")
	}
	src := g.src
	g.mu.Unlock()

	// Call Token() outside the lock — oauth2.ReuseTokenSource is thread-safe.
	tok, err := src.Token()
	if err != nil {
		return "", fmt.Errorf("vertex-ai: token refresh failed: %w", err)
	}
	return tok.AccessToken, nil
}
