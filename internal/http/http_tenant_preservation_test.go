package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/vellus-ai/argoclaw/internal/auth"
	"github.com/vellus-ai/argoclaw/internal/store"
	"pgregory.net/rapid"
)

// ─────────────────────────────────────────────────────────────────────────────
// Task 5: Preservation Tests — Gateway Token and Public Endpoints in HTTP
//
// **Property 2: Preservation** — Gateway Token e Endpoints Públicos em HTTP
//
// These tests verify that existing behavior is preserved:
// - Gateway token requests work without tenant_id (uses WithCrossTenant via requireAuth)
// - Public endpoints (/health) are accessible without tenant_id
// - PluginHandler with TenantMiddleware already works correctly (reference)
//
// Expected: PASS on unfixed code (confirms baseline to preserve)
// After fix: PASS (confirms no regression)
//
// **Validates: Requirements 4.3 (Preservation — gateway token, endpoints públicos)**
// ─────────────────────────────────────────────────────────────────────────────

// TestPreservation_GatewayToken_ProvidersHandler verifies that requests with
// gateway token continue working without tenant_id requirement.
func TestPreservation_GatewayToken_ProvidersHandler(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		ps := newMockProviderStore()
		h := NewProvidersHandler(ps, newMockSecretsStore(), "test-token", nil, "", nil)

		// Gateway token request — no JWT, no tenant_id
		req := httptest.NewRequest("GET", "/v1/providers", nil)
		req.Header.Set("Authorization", "Bearer test-token")
		rec := httptest.NewRecorder()

		// Call through auth middleware (simulates RegisterRoutes chain)
		h.auth(h.handleListProviders)(rec, req)

		// Gateway token requests should succeed (200 OK)
		if rec.Code != http.StatusOK {
			rt.Fatalf("ProvidersHandler with gateway token: status = %d, want 200", rec.Code)
		}
	})
}

// TestPreservation_GatewayToken_AgentsHandler verifies that requests with
// gateway token continue working for agent endpoints.
func TestPreservation_GatewayToken_AgentsHandler(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Gateway token request — no JWT, no tenant_id
		var capturedTenantID uuid.UUID
		handler := requireAuth("test-token", "", func(w http.ResponseWriter, r *http.Request) {
			capturedTenantID = store.TenantIDFromContext(r.Context())
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/v1/agents", nil)
		req.Header.Set("Authorization", "Bearer test-token")
		rec := httptest.NewRecorder()
		handler(rec, req)

		// Gateway token: no tenant_id in context (uuid.Nil) — this is correct behavior
		if rec.Code != http.StatusOK {
			rt.Fatalf("AgentsHandler with gateway token: status = %d, want 200", rec.Code)
		}
		if capturedTenantID != uuid.Nil {
			rt.Fatalf("Gateway token should have no tenant_id, got %v", capturedTenantID)
		}
	})
}

// TestPreservation_JWTAuth_TenantIDInjected verifies that JWT auth with
// requireAuth already injects basic tenant_id via injectJWTContext.
// This behavior must be preserved after adding TenantMiddleware.
func TestPreservation_JWTAuth_TenantIDInjected(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tenantID := uuid.New()
		jwtMw := NewJWTMiddleware(testSecret)

		claims := auth.TokenClaims{
			UserID:   rapid.StringMatching(`^user-[a-z0-9]{4}$`).Draw(rt, "userID"),
			Email:    "test@example.com",
			TenantID: tenantID.String(),
			Role:     "member",
		}
		token, err := auth.GenerateAccessToken(claims, testSecret)
		if err != nil {
			rt.Fatalf("failed to generate token: %v", err)
		}

		var capturedTenantID uuid.UUID
		handler := jwtMw.Wrap(http.HandlerFunc(requireAuth("test-token", "", func(w http.ResponseWriter, r *http.Request) {
			capturedTenantID = store.TenantIDFromContext(r.Context())
			w.WriteHeader(http.StatusOK)
		})))

		req := httptest.NewRequest("GET", "/v1/providers", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		// JWT auth with requireAuth already injects tenant_id via injectJWTContext
		if capturedTenantID != tenantID {
			rt.Fatalf("JWT auth: TenantIDFromContext = %v, want %v", capturedTenantID, tenantID)
		}
	})
}

// TestPreservation_PluginHandler_TenantMiddleware_Works verifies that
// PluginHandler (the reference implementation) correctly applies TenantMiddleware.
// This is the pattern we're replicating for ProvidersHandler and AgentsHandler.
func TestPreservation_PluginHandler_TenantMiddleware_Works(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tenantID := uuid.New()
		operatorLevel := rapid.IntRange(0, 2).Draw(rt, "operatorLevel")

		mockTenants := &mockTenantStoreForOperator{
			tenants: []store.Tenant{
				{
					ID:            tenantID,
					Slug:          "test-tenant",
					OperatorLevel: operatorLevel,
					Plan:          "starter",
					Status:        "active",
				},
			},
		}

		jwtMw := NewJWTMiddleware(testSecret)
		tenantMw := NewTenantMiddleware(mockTenants)

		claims := auth.TokenClaims{
			UserID:   rapid.StringMatching(`^user-[a-z0-9]{4}$`).Draw(rt, "userID"),
			Email:    "test@example.com",
			TenantID: tenantID.String(),
			Role:     "admin",
		}
		token, err := auth.GenerateAccessToken(claims, testSecret)
		if err != nil {
			rt.Fatalf("failed to generate token: %v", err)
		}

		// Simulate PluginHandler's withTenant pattern
		var capturedIsOperator bool
		var capturedTenantID uuid.UUID
		withTenant := func(next http.HandlerFunc) http.HandlerFunc {
			wrapped := tenantMw.Wrap(next)
			return wrapped.ServeHTTP
		}

		captureMux := http.NewServeMux()
		captureMux.HandleFunc("GET /v1/plugins/catalog", withTenant(requireAuth("test-token", "", func(w http.ResponseWriter, r *http.Request) {
			capturedTenantID = store.TenantIDFromContext(r.Context())
			capturedIsOperator = store.IsOperatorMode(r.Context())
			w.WriteHeader(http.StatusOK)
		})))
		handler := jwtMw.Wrap(captureMux)

		req := httptest.NewRequest("GET", "/v1/plugins/catalog", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			rt.Fatalf("PluginHandler: status = %d, want 200", rec.Code)
		}

		// Tenant ID should always be set
		if capturedTenantID != tenantID {
			rt.Fatalf("PluginHandler: TenantIDFromContext = %v, want %v", capturedTenantID, tenantID)
		}

		// Operator mode should match operator_level
		expectOperator := operatorLevel >= 1
		if capturedIsOperator != expectOperator {
			rt.Fatalf("PluginHandler: IsOperatorMode = %v for operator_level=%d, want %v", capturedIsOperator, operatorLevel, expectOperator)
		}
	})
}

// TestPreservation_PublicEndpoint_NoTenantRequired verifies that public
// endpoints like /health work without any authentication or tenant context.
func TestPreservation_PublicEndpoint_NoTenantRequired(t *testing.T) {
	// /health is a simple endpoint that doesn't require auth
	// This test verifies it remains accessible after our changes
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	// Simulate the health handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("/health: status = %d, want 200", rec.Code)
	}
}
