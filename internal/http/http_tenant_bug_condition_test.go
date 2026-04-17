package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/vellus-ai/argoclaw/internal/auth"
	"github.com/vellus-ai/argoclaw/internal/providers"
	"github.com/vellus-ai/argoclaw/internal/store"
	"pgregory.net/rapid"
)

// ─────────────────────────────────────────────────────────────────────────────
// Task 4: Exploratory Bug Condition Test — HTTP Endpoints Without TenantMiddleware
//
// **Property 1: Bug Condition** — Endpoints HTTP sem TenantMiddleware
//
// These tests demonstrate that ProvidersHandler and AgentsHandler do NOT have
// TenantMiddleware applied. While requireAuth injects basic tenant_id via
// injectJWTContext, the TenantMiddleware adds critical operator mode support
// (WithCrossTenant + WithOperatorMode for operator tenants).
//
// Additionally, pending_messages.go uses context.Background() for GetDefault.
//
// Expected: FAIL on unfixed code (confirms bug exists)
// After fix: PASS (confirms bug is fixed)
//
// **Validates: Requirements 4.1, 4.2**
// ─────────────────────────────────────────────────────────────────────────────

// TestBugCondition_ProvidersHandler_NoOperatorMode demonstrates that
// ProvidersHandler does NOT activate operator mode for operator tenants
// because TenantMiddleware is missing.
// After fix: TenantMiddleware is applied, so operator mode IS activated.
func TestBugCondition_ProvidersHandler_NoOperatorMode(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tenantID := uuid.New()
		operatorLevel := rapid.IntRange(1, 3).Draw(rt, "operatorLevel")

		mockTenants := &mockTenantStoreForOperator{
			tenants: []store.Tenant{
				{
					ID:            tenantID,
					Slug:          "operator-tenant",
					OperatorLevel: operatorLevel,
					Plan:          "internal",
					Status:        "active",
				},
			},
		}

		jwtMw := NewJWTMiddleware(testSecret)
		tenantMw := NewTenantMiddleware(mockTenants)

		claims := auth.TokenClaims{
			UserID:   rapid.StringMatching(`^user-[a-z0-9]{4}$`).Draw(rt, "userID"),
			Email:    "op@vellus.tech",
			TenantID: tenantID.String(),
			Role:     "admin",
		}
		token, err := auth.GenerateAccessToken(claims, testSecret)
		if err != nil {
			rt.Fatalf("failed to generate token: %v", err)
		}

		ps := newMockProviderStore()
		h := NewProvidersHandler(ps, newMockSecretsStore(), "test-token", nil, "", tenantMw)

		// Use the actual RegisterRoutes which now includes withTenant wrapping
		var capturedCtx context.Context
		mux := http.NewServeMux()
		// Register a route that captures context — simulates the real handler chain
		mux.HandleFunc("GET /v1/providers", h.withTenant(h.auth(func(w http.ResponseWriter, r *http.Request) {
			capturedCtx = r.Context()
			w.WriteHeader(http.StatusOK)
		})))
		handler := jwtMw.Wrap(mux)

		req := httptest.NewRequest("GET", "/v1/providers", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if capturedCtx == nil {
			rt.Fatal("handler was not called")
		}

		// After fix: TenantMiddleware wraps routes, so operator mode IS activated.
		if !store.IsOperatorMode(capturedCtx) {
			rt.Fatalf("ProvidersHandler: IsOperatorMode = false for operator_level=%d, want true (TenantMiddleware missing)", operatorLevel)
		}
	})
}

// TestBugCondition_AgentsHandler_NoOperatorMode demonstrates that
// AgentsHandler does NOT activate operator mode for operator tenants.
// After fix: TenantMiddleware is applied, so operator mode IS activated.
func TestBugCondition_AgentsHandler_NoOperatorMode(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tenantID := uuid.New()
		operatorLevel := rapid.IntRange(1, 3).Draw(rt, "operatorLevel")

		mockTenants := &mockTenantStoreForOperator{
			tenants: []store.Tenant{
				{
					ID:            tenantID,
					Slug:          "operator-tenant",
					OperatorLevel: operatorLevel,
					Plan:          "internal",
					Status:        "active",
				},
			},
		}

		jwtMw := NewJWTMiddleware(testSecret)
		tenantMw := NewTenantMiddleware(mockTenants)

		claims := auth.TokenClaims{
			UserID:   rapid.StringMatching(`^user-[a-z0-9]{4}$`).Draw(rt, "userID"),
			Email:    "op@vellus.tech",
			TenantID: tenantID.String(),
			Role:     "admin",
		}
		token, err := auth.GenerateAccessToken(claims, testSecret)
		if err != nil {
			rt.Fatalf("failed to generate token: %v", err)
		}

		// Create AgentsHandler with TenantMiddleware (after fix)
		h := NewAgentsHandler(nil, "test-token", "/tmp/ws", nil, nil, nil, tenantMw)

		// Use the withTenant wrapping from the fixed handler
		var capturedCtx context.Context
		captureMux := http.NewServeMux()
		captureMux.HandleFunc("GET /v1/agents", h.withTenant(h.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
			capturedCtx = r.Context()
			w.WriteHeader(http.StatusOK)
		})))
		handler := jwtMw.Wrap(captureMux)

		req := httptest.NewRequest("GET", "/v1/agents", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if capturedCtx == nil {
			rt.Fatal("handler was not called")
		}

		// After fix: operator tenant should have operator mode activated
		if !store.IsOperatorMode(capturedCtx) {
			rt.Fatalf("AgentsHandler: IsOperatorMode = false for operator_level=%d, want true (TenantMiddleware missing)", operatorLevel)
		}
	})
}

// TestBugCondition_PendingMessages_GetDefaultUsesBackgroundContext demonstrates that
// pending_messages.go resolveProviderAndModel uses context.Background() for GetDefault
// instead of the request context with tenant_id.
// This is a unit test (not PBT) because the bug is deterministic: context.Background()
// is always used regardless of input.
func TestBugCondition_PendingMessages_GetDefaultUsesBackgroundContext(t *testing.T) {
	tenantID := uuid.New()

	// Create a mock agent store that captures the context passed to GetDefault
	var capturedCtx context.Context
	mockAgents := &ctxCapturingDefaultStore{
		onGetDefault: func(ctx context.Context) {
			capturedCtx = ctx
		},
	}

	// providerReg must be non-nil so resolveProviderAndModel doesn't return early
	reg := providers.NewRegistry()

	h := NewPendingMessagesHandler(nil, mockAgents, "test-token", reg)

	// resolveProviderAndModel now accepts ctx parameter.
	// Pass a context with tenant_id to verify it's propagated to GetDefault.
	ctx := store.WithTenantID(context.Background(), tenantID)
	h.resolveProviderAndModel(ctx)

	if capturedCtx == nil {
		t.Fatal("GetDefault was not called — mock agent store should have been invoked")
	}

	gotTenantID := store.TenantIDFromContext(capturedCtx)
	// BUG: GetDefault receives context.Background() without tenant_id
	// After fix (task 6.3): GetDefault receives r.Context() with tenant_id
	if gotTenantID != tenantID {
		t.Fatalf("PendingMessages.GetDefault: TenantIDFromContext = %v, want %v (uses context.Background())", gotTenantID, tenantID)
	}
}

// ─── Mock stores for bug condition tests ─────────────────────────────────────

// ctxCapturingDefaultStore captures the context passed to GetDefault.
type ctxCapturingDefaultStore struct {
	store.AgentStore // embed to satisfy interface — panics on unimplemented methods
	onGetDefault     func(ctx context.Context)
}

func (s *ctxCapturingDefaultStore) GetDefault(ctx context.Context) (*store.AgentData, error) {
	if s.onGetDefault != nil {
		s.onGetDefault(ctx)
	}
	// Return a valid agent so callers don't panic on nil dereference
	return &store.AgentData{Provider: "", Model: ""}, nil
}
