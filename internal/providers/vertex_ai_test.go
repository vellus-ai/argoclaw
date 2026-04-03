package providers

import (
	"fmt"
	"strings"
	"testing"
	"testing/quick"
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
	t.Parallel()
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
			projectID: "my-project-123",
			region:    "europe-west4",
			wantBase:  "https://europe-west4-aiplatform.googleapis.com/v1beta1/projects/my-project-123/locations/europe-west4/endpoints/openapi",
		},
		{
			name:      "asia-southeast1",
			projectID: "another-project",
			region:    "asia-southeast1",
			wantBase:  "https://asia-southeast1-aiplatform.googleapis.com/v1beta1/projects/another-project/locations/asia-southeast1/endpoints/openapi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ts := &mockTokenSource{token: "test-token"}
			p, err := NewVertexAIProvider(tt.projectID, tt.region, WithVertexAITokenSource(ts))
			if err != nil {
				t.Fatalf("NewVertexAIProvider() error: %v", err)
			}
			if p.APIBase() != tt.wantBase {
				t.Errorf("APIBase() = %q, want %q", p.APIBase(), tt.wantBase)
			}
		})
	}
}

func TestNewVertexAIProvider_DefaultModel(t *testing.T) {
	t.Parallel()
	ts := &mockTokenSource{token: "tok"}

	t.Run("default_is_gemini_flash", func(t *testing.T) {
		p, err := NewVertexAIProvider("my-project1", "us-central1", WithVertexAITokenSource(ts))
		if err != nil {
			t.Fatalf("NewVertexAIProvider() error: %v", err)
		}
		if p.DefaultModel() != "gemini-2.5-flash" {
			t.Errorf("DefaultModel() = %q, want %q", p.DefaultModel(), "gemini-2.5-flash")
		}
	})

	t.Run("override_with_option", func(t *testing.T) {
		p, err := NewVertexAIProvider("my-project1", "us-central1",
			WithVertexAITokenSource(ts),
			WithVertexAIDefaultModel("gemini-2.5-pro"))
		if err != nil {
			t.Fatalf("NewVertexAIProvider() error: %v", err)
		}
		if p.DefaultModel() != "gemini-2.5-pro" {
			t.Errorf("DefaultModel() = %q, want %q", p.DefaultModel(), "gemini-2.5-pro")
		}
	})
}

func TestNewVertexAIProvider_NameOption(t *testing.T) {
	t.Parallel()
	ts := &mockTokenSource{token: "tok"}

	t.Run("default_name", func(t *testing.T) {
		p, err := NewVertexAIProvider("my-project1", "us-central1", WithVertexAITokenSource(ts))
		if err != nil {
			t.Fatalf("NewVertexAIProvider() error: %v", err)
		}
		if p.Name() != "vertex-ai" {
			t.Errorf("Name() = %q, want %q", p.Name(), "vertex-ai")
		}
	})

	t.Run("custom_name", func(t *testing.T) {
		p, err := NewVertexAIProvider("my-project1", "us-central1",
			WithVertexAITokenSource(ts),
			WithVertexAIName("custom-vertex"))
		if err != nil {
			t.Fatalf("NewVertexAIProvider() error: %v", err)
		}
		if p.Name() != "custom-vertex" {
			t.Errorf("Name() = %q, want %q", p.Name(), "custom-vertex")
		}
	})
}

func TestNewVertexAIProvider_ProviderType(t *testing.T) {
	t.Parallel()
	ts := &mockTokenSource{token: "tok"}
	p, err := NewVertexAIProvider("my-project1", "us-central1", WithVertexAITokenSource(ts))
	if err != nil {
		t.Fatalf("NewVertexAIProvider() error: %v", err)
	}
	if p.ProviderType() != "vertex_ai" {
		t.Errorf("ProviderType() = %q, want %q", p.ProviderType(), "vertex_ai")
	}
}

func TestNewVertexAIProvider_TokenSourceInjected(t *testing.T) {
	t.Parallel()
	ts := &mockTokenSource{token: "injected-oauth2-token"}
	p, err := NewVertexAIProvider("my-project1", "us-central1", WithVertexAITokenSource(ts))
	if err != nil {
		t.Fatalf("NewVertexAIProvider() error: %v", err)
	}
	if p.tokenSource == nil {
		t.Fatal("tokenSource should be set")
	}
	tok, tokErr := p.tokenSource.Token()
	if tokErr != nil {
		t.Fatalf("Token() error: %v", tokErr)
	}
	if tok != "injected-oauth2-token" {
		t.Errorf("Token() = %q, want %q", tok, "injected-oauth2-token")
	}
}

// --- Input validation tests (SSRF prevention) ---

func TestNewVertexAIProvider_InvalidProjectID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		projectID string
	}{
		{"empty", ""},
		{"too_short", "ab"},
		{"starts_with_digit", "1project"},
		{"starts_with_hyphen", "-project"},
		{"contains_uppercase", "My-Project"},
		{"contains_slash", "project/evil"},
		{"contains_dot", "project.evil.com"},
		{"path_traversal", "x/../../other"},
		{"url_injection", "x.evil.com/foo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewVertexAIProvider(tt.projectID, "us-central1")
			if err == nil {
				t.Errorf("NewVertexAIProvider(%q, ...) should return error for invalid project_id", tt.projectID)
			}
		})
	}
}

func TestNewVertexAIProvider_InvalidRegion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		region string
	}{
		{"empty", ""},
		{"contains_slash", "us-central1/evil"},
		{"contains_dot", "us-central1.evil.com"},
		{"uppercase", "US-CENTRAL1"},
		{"no_hyphen", "uscentral1"},
		{"url_injection", "evil.com:443/proxy"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewVertexAIProvider("my-valid-project", tt.region)
			if err == nil {
				t.Errorf("NewVertexAIProvider(..., %q) should return error for invalid region", tt.region)
			}
		})
	}
}

func TestNewVertexAIProvider_ValidProjectIDs(t *testing.T) {
	t.Parallel()
	ts := &mockTokenSource{token: "tok"}
	valids := []string{
		"my-project",
		"vellus-ai-agent-platform",
		"project-123-test",
		"a12345",
	}
	for _, pid := range valids {
		t.Run(pid, func(t *testing.T) {
			t.Parallel()
			_, err := NewVertexAIProvider(pid, "us-central1", WithVertexAITokenSource(ts))
			if err != nil {
				t.Errorf("NewVertexAIProvider(%q, ...) should succeed but got: %v", pid, err)
			}
		})
	}
}

// --- PBT: URL construction invariants ---

func TestNewVertexAIProvider_URLInvariants_PBT(t *testing.T) {
	t.Parallel()
	ts := &mockTokenSource{token: "tok"}

	// Property: for any valid projectID and region, the constructed URL must:
	// 1. Start with https://
	// 2. Contain aiplatform.googleapis.com
	// 3. Contain the projectID in the path
	// 4. Contain the region exactly twice (subdomain + path)
	// 5. Not contain spaces
	f := func(pidSuffix, regSuffix uint8) bool {
		// Generate valid-ish projectID and region from random bytes
		pid := fmt.Sprintf("proj-%03d-test", pidSuffix)
		reg := fmt.Sprintf("us-central%d", regSuffix%10)

		// Only test if inputs pass validation
		if !vertexAIProjectIDRe.MatchString(pid) || !vertexAIRegionRe.MatchString(reg) {
			return true // skip invalid inputs
		}

		p, err := NewVertexAIProvider(pid, reg, WithVertexAITokenSource(ts))
		if err != nil {
			return false
		}
		base := p.APIBase()

		if !strings.HasPrefix(base, "https://") {
			t.Logf("FAIL: URL must start with https://, got %q", base)
			return false
		}
		if !strings.Contains(base, "aiplatform.googleapis.com") {
			t.Logf("FAIL: URL must contain aiplatform.googleapis.com, got %q", base)
			return false
		}
		if !strings.Contains(base, "/projects/"+pid+"/") {
			t.Logf("FAIL: URL must contain /projects/%s/, got %q", pid, base)
			return false
		}
		if strings.Count(base, reg) != 2 {
			t.Logf("FAIL: URL must contain region %q exactly twice, got %d in %q", reg, strings.Count(base, reg), base)
			return false
		}
		if strings.Contains(base, " ") {
			t.Logf("FAIL: URL must not contain spaces, got %q", base)
			return false
		}
		return true
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 200}); err != nil {
		t.Errorf("PBT failed: %v", err)
	}
}

// --- TokenSource tests ---

func TestOpenAIProvider_WithTokenSource(t *testing.T) {
	t.Parallel()
	ts := &mockTokenSource{token: "test-token-123"}
	p := NewOpenAIProvider("test", "", "https://example.com", "model")
	p.WithTokenSource(ts)

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
	t.Parallel()
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
	t.Parallel()
	ts := &mockTokenSource{token: "oauth2-token"}
	p := NewOpenAIProvider("test", "static-api-key", "https://example.com", "model")
	p.WithTokenSource(ts)

	tok, _ := p.tokenSource.Token()
	if tok != "oauth2-token" {
		t.Errorf("tokenSource.Token() = %q, want %q", tok, "oauth2-token")
	}
}

func TestVertexAIOptions(t *testing.T) {
	t.Parallel()
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

	t.Run("WithVertexAITokenSource", func(t *testing.T) {
		ts := &mockTokenSource{token: "test"}
		cfg := &vertexAIConfig{}
		WithVertexAITokenSource(ts)(cfg)
		if cfg.tokenSource == nil {
			t.Error("tokenSource should be set")
		}
	})
}

func TestGCPTokenSource_LazyInit(t *testing.T) {
	t.Parallel()
	ts := newGCPTokenSource()
	// Before any Token() call, src should be nil (sync.Once not yet fired)
	if ts.src != nil {
		t.Error("underlying source should be nil before first Token() call")
	}
	if ts.err != nil {
		t.Error("err should be nil before first Token() call")
	}
}

// --- Validation regex tests ---

func TestVertexAIProjectIDRegex(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		valid bool
	}{
		{"my-project", true},
		{"vellus-ai-agent-platform", true},
		{"a12345", true},
		{"project-123-test", true},
		{"", false},
		{"ab", false},
		{"1starts-with-digit", false},
		{"-starts-with-hyphen", false},
		{"ends-with-hyphen-", false},
		{"HAS-UPPERCASE", false},
		{"has.dots", false},
		{"has/slash", false},
		{"has spaces", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := vertexAIProjectIDRe.MatchString(tt.input)
			if got != tt.valid {
				t.Errorf("projectID %q: got valid=%v, want %v", tt.input, got, tt.valid)
			}
		})
	}
}

func TestVertexAIRegionRegex(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		valid bool
	}{
		{"us-central1", true},
		{"europe-west4", true},
		{"asia-southeast1", true},
		{"me-central2", true},
		{"", false},
		{"US-CENTRAL1", false},
		{"uscentral1", false},
		{"us-central1/evil", false},
		{"us-central1.evil.com", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := vertexAIRegionRe.MatchString(tt.input)
			if got != tt.valid {
				t.Errorf("region %q: got valid=%v, want %v", tt.input, got, tt.valid)
			}
		})
	}
}
