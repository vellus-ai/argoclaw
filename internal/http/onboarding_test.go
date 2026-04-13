package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/vellus-ai/argoclaw/internal/auth"
	"github.com/vellus-ai/argoclaw/internal/store"
	"github.com/vellus-ai/argoclaw/internal/tools"
)

// --- Mock OnboardingStore ---

type mockOnbStore struct {
	status             map[string]any
	lastCompletedState map[string]string
	failOnGetStatus    bool
	failOnComplete     bool
	failOnUpdate       bool
}

func newMockOnbStore() *mockOnbStore {
	return &mockOnbStore{
		status: map[string]any{
			"onboarding_complete":  false,
			"workspace_configured": false,
			"branding_set":         false,
		},
		lastCompletedState: make(map[string]string),
	}
}

func (m *mockOnbStore) UpdateTenantSettings(_ context.Context, tenantID, key string, value any) error {
	if m.failOnUpdate {
		return fmt.Errorf("store error")
	}
	return nil
}

func (m *mockOnbStore) UpdateTenantBranding(_ context.Context, tenantID, primaryColor, productName string) error {
	return nil
}

func (m *mockOnbStore) GetOnboardingStatus(_ context.Context, tenantID string) (map[string]any, error) {
	if m.failOnGetStatus {
		return nil, fmt.Errorf("status error")
	}
	return m.status, nil
}

func (m *mockOnbStore) CompleteOnboarding(_ context.Context, tenantID string) error {
	if m.failOnComplete {
		return fmt.Errorf("complete error")
	}
	return nil
}

func (m *mockOnbStore) UpdateLastCompletedState(_ context.Context, tenantID, state string) error {
	if m.failOnUpdate {
		return fmt.Errorf("store error")
	}
	m.lastCompletedState[tenantID] = state
	return nil
}

// --- Test Helpers ---

const testJWTSecret = "test-secret-key-for-jwt-min-32-chars!!"
const testGatewayToken = "test-gateway-token"

// jwtForTenant generates a valid JWT with the given tenant and user IDs.
func jwtForTenant(t *testing.T, tenantID, userID string) string {
	t.Helper()
	token, err := auth.GenerateAccessToken(auth.TokenClaims{
		UserID:   userID,
		Email:    "test@example.com",
		TenantID: tenantID,
		Role:     "admin",
	}, testJWTSecret)
	if err != nil {
		t.Fatalf("generate JWT: %v", err)
	}
	return token
}

// setupOnboardingHandler creates a handler with mock store and registry, returning
// the mux ready to serve requests.
func setupOnboardingHandler(t *testing.T, ms *mockOnbStore) *http.ServeMux {
	t.Helper()

	// Build a registry with onboarding tools wired to the mock store
	reg := tools.NewRegistry()
	onbTools := []tools.Tool{
		tools.NewConfigureWorkspaceTool(),
		tools.NewSetBrandingTool(),
		tools.NewConfigureLLMProviderTool(),
		tools.NewTestLLMConnectionTool(),
		tools.NewCreateAgentTool(),
		tools.NewConfigureChannelTool(),
		tools.NewCompleteOnboardingTool(),
		tools.NewGetOnboardingStatusTool(),
	}
	for _, tool := range onbTools {
		if aware, ok := tool.(tools.OnboardingStoreAware); ok {
			aware.SetOnboardingStore(ms)
		}
		reg.Register(tool)
	}

	h := NewOnboardingHandler(ms, reg, testGatewayToken)
	mux := http.NewServeMux()

	// Wrap with JWT middleware (same as production)
	jwtMW := NewJWTMiddleware(testJWTSecret)
	wrappedMux := jwtMW.Wrap(mux)
	h.RegisterRoutes(mux)

	// Return a mux that serves through JWT middleware
	finalMux := http.NewServeMux()
	finalMux.Handle("/", wrappedMux)
	return finalMux
}

func postJSON(t *testing.T, url string, body any) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	return bytes.NewBuffer(b)
}

func decodeResponse(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v (body: %s)", err, rec.Body.String())
	}
	return result
}

// ===== Task 1.2: OnboardingHandler Tests =====

func TestGetStatus_ReturnsOnboardingStatus(t *testing.T) {
	t.Parallel()
	ms := newMockOnbStore()
	mux := setupOnboardingHandler(t, ms)
	tenantID := uuid.New().String()

	req := httptest.NewRequest("GET", "/v1/onboarding/status", nil)
	req.Header.Set("Authorization", "Bearer "+jwtForTenant(t, tenantID, "user-1"))
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	body := decodeResponse(t, rec)
	if body["onboarding_complete"] != false {
		t.Errorf("onboarding_complete = %v, want false", body["onboarding_complete"])
	}
}

func TestGetStatus_Unauthorized(t *testing.T) {
	t.Parallel()
	ms := newMockOnbStore()
	mux := setupOnboardingHandler(t, ms)

	req := httptest.NewRequest("GET", "/v1/onboarding/status", nil)
	// No Authorization header
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestAction_WhitelistedTool(t *testing.T) {
	t.Parallel()
	ms := newMockOnbStore()
	mux := setupOnboardingHandler(t, ms)
	tenantID := uuid.New().String()

	body := postJSON(t, "", map[string]any{
		"tool": "configure_workspace",
		"args": map[string]any{
			"type":         "business",
			"account_name": "Vellus Tech",
		},
	})

	req := httptest.NewRequest("POST", "/v1/onboarding/action", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwtForTenant(t, tenantID, "user-1"))
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	result := decodeResponse(t, rec)
	if result["ok"] != true {
		t.Errorf("ok = %v, want true; body: %s", result["ok"], rec.Body.String())
	}
}

func TestAction_BlockedTool(t *testing.T) {
	t.Parallel()
	ms := newMockOnbStore()
	mux := setupOnboardingHandler(t, ms)
	tenantID := uuid.New().String()

	body := postJSON(t, "", map[string]any{
		"tool": "create_agent",
		"args": map[string]any{"name": "test", "preset": "captain"},
	})

	req := httptest.NewRequest("POST", "/v1/onboarding/action", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwtForTenant(t, tenantID, "user-1"))
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403; body: %s", rec.Code, rec.Body.String())
	}
}

func TestAction_InvalidArgs(t *testing.T) {
	t.Parallel()
	ms := newMockOnbStore()
	mux := setupOnboardingHandler(t, ms)
	tenantID := uuid.New().String()

	body := postJSON(t, "", map[string]any{
		"tool": "set_branding",
		"args": map[string]any{
			"primary_color": "not-a-hex-color",
		},
	})

	req := httptest.NewRequest("POST", "/v1/onboarding/action", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwtForTenant(t, tenantID, "user-1"))
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	// Tool returns error result — handler should relay it
	result := decodeResponse(t, rec)
	if result["ok"] == true {
		t.Error("expected ok=false for invalid args")
	}
}

func TestAction_MissingToolField(t *testing.T) {
	t.Parallel()
	ms := newMockOnbStore()
	mux := setupOnboardingHandler(t, ms)
	tenantID := uuid.New().String()

	body := postJSON(t, "", map[string]any{
		"args": map[string]any{},
	})

	req := httptest.NewRequest("POST", "/v1/onboarding/action", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwtForTenant(t, tenantID, "user-1"))
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400; body: %s", rec.Code, rec.Body.String())
	}
}

func TestAction_InvalidJSON(t *testing.T) {
	t.Parallel()
	ms := newMockOnbStore()
	mux := setupOnboardingHandler(t, ms)
	tenantID := uuid.New().String()

	req := httptest.NewRequest("POST", "/v1/onboarding/action", strings.NewReader("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwtForTenant(t, tenantID, "user-1"))
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestAction_PersistsLastCompletedState(t *testing.T) {
	t.Parallel()
	ms := newMockOnbStore()
	mux := setupOnboardingHandler(t, ms)
	tenantID := uuid.New().String()

	body := postJSON(t, "", map[string]any{
		"tool": "configure_workspace",
		"args": map[string]any{
			"type":         "personal",
			"account_name": "Test User",
		},
		"completed_state": "workspace_type",
	})

	req := httptest.NewRequest("POST", "/v1/onboarding/action", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwtForTenant(t, tenantID, "user-1"))
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	// Verify last_completed_state was persisted
	if ms.lastCompletedState[tenantID] != "workspace_type" {
		t.Errorf("last_completed_state = %q, want %q", ms.lastCompletedState[tenantID], "workspace_type")
	}
}

func TestGetStatus_StoreError(t *testing.T) {
	t.Parallel()
	ms := newMockOnbStore()
	ms.failOnGetStatus = true
	mux := setupOnboardingHandler(t, ms)
	tenantID := uuid.New().String()

	req := httptest.NewRequest("GET", "/v1/onboarding/status", nil)
	req.Header.Set("Authorization", "Bearer "+jwtForTenant(t, tenantID, "user-1"))
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

// ===== Task 1.3: Security & Input Validation Tests =====

func TestAction_TenantIsolation_JWTSetsContext(t *testing.T) {
	t.Parallel()
	ms := newMockOnbStore()
	mux := setupOnboardingHandler(t, ms)

	tenantA := uuid.New().String()
	tenantB := uuid.New().String()

	// Tenant A configures workspace
	bodyA := postJSON(t, "", map[string]any{
		"tool": "configure_workspace",
		"args": map[string]any{
			"type":         "business",
			"account_name": "Tenant A Corp",
		},
		"completed_state": "workspace_type",
	})
	reqA := httptest.NewRequest("POST", "/v1/onboarding/action", bodyA)
	reqA.Header.Set("Content-Type", "application/json")
	reqA.Header.Set("Authorization", "Bearer "+jwtForTenant(t, tenantA, "user-a"))
	recA := httptest.NewRecorder()
	mux.ServeHTTP(recA, reqA)

	if recA.Code != http.StatusOK {
		t.Fatalf("tenant A status = %d, want 200", recA.Code)
	}

	// Verify state was saved for correct tenant
	if ms.lastCompletedState[tenantA] != "workspace_type" {
		t.Errorf("tenant A state = %q, want workspace_type", ms.lastCompletedState[tenantA])
	}
	if _, exists := ms.lastCompletedState[tenantB]; exists {
		t.Error("tenant B should not have state from tenant A")
	}
}

func TestAction_AccountNameTrimmed(t *testing.T) {
	t.Parallel()
	ms := newMockOnbStore()
	mux := setupOnboardingHandler(t, ms)
	tenantID := uuid.New().String()

	// The tool itself handles trimming — we verify it doesn't crash with whitespace
	body := postJSON(t, "", map[string]any{
		"tool": "configure_workspace",
		"args": map[string]any{
			"type":         "personal",
			"account_name": "  Test User  ",
		},
	})
	req := httptest.NewRequest("POST", "/v1/onboarding/action", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwtForTenant(t, tenantID, "user-1"))
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}

func TestAction_HexColorValidation(t *testing.T) {
	t.Parallel()
	ms := newMockOnbStore()
	mux := setupOnboardingHandler(t, ms)
	tenantID := uuid.New().String()

	tests := []struct {
		name    string
		color   string
		wantErr bool
	}{
		{"valid 6-digit", "#3B82F6", false},
		{"valid 3-digit", "#FFF", false},
		{"invalid no hash", "3B82F6", true},
		{"invalid word", "red", true},
		{"xss attempt", "#FFF; background: url(evil)", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := postJSON(t, "", map[string]any{
				"tool": "set_branding",
				"args": map[string]any{"primary_color": tt.color},
			})
			req := httptest.NewRequest("POST", "/v1/onboarding/action", body)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+jwtForTenant(t, tenantID, "user-1"))
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			result := decodeResponse(t, rec)
			isErr := result["ok"] != true
			if isErr != tt.wantErr {
				t.Errorf("color %q: got error=%v, want error=%v; body: %s", tt.color, isErr, tt.wantErr, rec.Body.String())
			}
		})
	}
}

func TestAction_APIKeyNeverInResponse(t *testing.T) {
	t.Parallel()
	ms := newMockOnbStore()
	mux := setupOnboardingHandler(t, ms)
	tenantID := uuid.New().String()

	secretKey := "sk-ant-api03-very-secret-key-1234567890abcdef"
	body := postJSON(t, "", map[string]any{
		"tool": "configure_llm_provider",
		"args": map[string]any{
			"provider": "anthropic",
			"api_key":  secretKey,
			"model":    "claude-sonnet-4",
		},
	})

	req := httptest.NewRequest("POST", "/v1/onboarding/action", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwtForTenant(t, tenantID, "user-1"))
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	responseBody := rec.Body.String()
	if strings.Contains(responseBody, secretKey) {
		t.Error("SECURITY: full API key must NEVER appear in response body")
	}
}

func TestAction_Unauthorized_NoToken(t *testing.T) {
	t.Parallel()
	ms := newMockOnbStore()
	mux := setupOnboardingHandler(t, ms)

	body := postJSON(t, "", map[string]any{
		"tool": "configure_workspace",
		"args": map[string]any{"type": "personal", "account_name": "Test"},
	})

	req := httptest.NewRequest("POST", "/v1/onboarding/action", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestAction_CompleteOnboarding_Whitelisted(t *testing.T) {
	t.Parallel()
	ms := newMockOnbStore()
	mux := setupOnboardingHandler(t, ms)
	tenantID := uuid.New().String()

	body := postJSON(t, "", map[string]any{
		"tool": "complete_onboarding",
		"args": map[string]any{},
	})

	req := httptest.NewRequest("POST", "/v1/onboarding/action", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwtForTenant(t, tenantID, "user-1"))
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}

// Suppress unused import warnings — store is used by context injection
var _ = store.WithTenantID
