package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"errors"

	"github.com/google/uuid"
	"github.com/vellus-ai/argoclaw/internal/auth"
	"github.com/vellus-ai/argoclaw/internal/store"
	"pgregory.net/rapid"
)

func TestTenantMiddleware_InjectsTenantID(t *testing.T) {
	mw := NewTenantMiddleware(nil) // store not needed for context injection
	jwtMw := NewJWTMiddleware(testSecret)
	tenantID := uuid.New()

	claims := auth.TokenClaims{UserID: "u1", Email: "t@t.com", TenantID: tenantID.String(), Role: "member"}
	token, _ := auth.GenerateAccessToken(claims, testSecret)

	var gotTenantID uuid.UUID
	handler := jwtMw.Wrap(mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTenantID = TenantIDFromRequest(r.Context())
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest("GET", "/v1/agents", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if gotTenantID != tenantID {
		t.Errorf("TenantID = %v, want %v", gotTenantID, tenantID)
	}
}

func TestTenantMiddleware_NoJWT_PassThrough(t *testing.T) {
	mw := NewTenantMiddleware(nil)
	called := false

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		tid := TenantIDFromRequest(r.Context())
		if tid != uuid.Nil {
			t.Errorf("expected nil tenant without JWT, got %v", tid)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/agents", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("handler should be called (pass-through)")
	}
}

func TestRequireTenant_RejectsWithoutTenant(t *testing.T) {
	requireTenant := RequireTenant()

	handler := requireTenant(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	}))

	req := httptest.NewRequest("GET", "/v1/agents", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestRequireTenant_AllowsWithTenant(t *testing.T) {
	requireTenant := RequireTenant()
	tenantID := uuid.New()

	handler := requireTenant(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/agents", nil)
	ctx := store.WithTenantID(req.Context(), tenantID)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestTenantIDFromRequest_NilWhenEmpty(t *testing.T) {
	ctx := context.Background()
	if got := TenantIDFromRequest(ctx); got != uuid.Nil {
		t.Errorf("expected uuid.Nil, got %v", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Task 3.2: TenantMiddleware HTTP — Operator Mode Propagation
//
// Unit tests:
//   (a) HTTP request with tenant operator_level >= 1 → context has operator mode
//   (b) HTTP request with tenant operator_level = 0 → normal context, no operator mode
//   (c) No additional DB call needed — Tenant struct already loaded by middleware
//
// PBT:
//   ∀ tenant T: (T.OperatorLevel >= 1) ↔ (IsOperatorMode(ctx) = true) after middleware
//
// **Validates: Requirements 2.2, 2.6**
// ─────────────────────────────────────────────────────────────────────────────

func TestTenantMiddleware_OperatorTenant_ActivatesOperatorMode(t *testing.T) {
	tenantID := uuid.New()
	mockStore := &mockTenantStoreForOperator{
		tenants: []store.Tenant{
			{
				ID:            tenantID,
				Slug:          "vellus",
				OperatorLevel: 1,
				Plan:          "internal",
				Status:        "active",
			},
		},
	}

	mw := NewTenantMiddleware(mockStore)
	jwtMw := NewJWTMiddleware(testSecret)

	claims := auth.TokenClaims{UserID: "u1", Email: "op@vellus.tech", TenantID: tenantID.String(), Role: "admin"}
	token, _ := auth.GenerateAccessToken(claims, testSecret)

	var capturedCtx context.Context
	handler := jwtMw.Wrap(mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest("GET", "/v1/agents", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if capturedCtx == nil {
		t.Fatal("handler was not called — context not captured")
	}
	if !store.IsOperatorMode(capturedCtx) {
		t.Error("IsOperatorMode(ctx) = false after operator tenant HTTP request, want true")
	}
	if !store.IsCrossTenant(capturedCtx) {
		t.Error("IsCrossTenant(ctx) = false after operator tenant HTTP request, want true")
	}
	if got := store.OperatorModeFromContext(capturedCtx); got != tenantID {
		t.Errorf("OperatorModeFromContext = %v, want %v", got, tenantID)
	}
}

func TestTenantMiddleware_NormalTenant_NoOperatorMode(t *testing.T) {
	tenantID := uuid.New()
	mockStore := &mockTenantStoreForOperator{
		tenants: []store.Tenant{
			{
				ID:            tenantID,
				Slug:          "customer-abc",
				OperatorLevel: 0,
				Plan:          "starter",
				Status:        "active",
			},
		},
	}

	mw := NewTenantMiddleware(mockStore)
	jwtMw := NewJWTMiddleware(testSecret)

	claims := auth.TokenClaims{UserID: "u2", Email: "user@customer.com", TenantID: tenantID.String(), Role: "member"}
	token, _ := auth.GenerateAccessToken(claims, testSecret)

	var capturedCtx context.Context
	handler := jwtMw.Wrap(mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest("GET", "/v1/agents", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if capturedCtx == nil {
		t.Fatal("handler was not called — context not captured")
	}
	if store.IsOperatorMode(capturedCtx) {
		t.Error("IsOperatorMode(ctx) = true for normal tenant, want false")
	}
	// Normal tenant should NOT have cross-tenant access
	// Note: the middleware sets WithCrossTenant for the GetByID lookup, but the
	// final context passed to the handler should only have it if operator_level >= 1
	if store.IsCrossTenant(capturedCtx) {
		t.Error("IsCrossTenant(ctx) = true for normal tenant, want false")
	}
}

func TestTenantMiddleware_NilStore_NoOperatorMode(t *testing.T) {
	// When TenantStore is nil (gateway-token-only mode), operator mode is never activated.
	mw := NewTenantMiddleware(nil)
	jwtMw := NewJWTMiddleware(testSecret)
	tenantID := uuid.New()

	claims := auth.TokenClaims{UserID: "u3", Email: "t@t.com", TenantID: tenantID.String(), Role: "admin"}
	token, _ := auth.GenerateAccessToken(claims, testSecret)

	var capturedCtx context.Context
	handler := jwtMw.Wrap(mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest("GET", "/v1/agents", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if capturedCtx == nil {
		t.Fatal("handler was not called")
	}
	if store.IsOperatorMode(capturedCtx) {
		t.Error("IsOperatorMode(ctx) = true when TenantStore is nil, want false")
	}
}

func TestTenantMiddleware_OperatorLevel2_ActivatesOperatorMode(t *testing.T) {
	tenantID := uuid.New()
	mockStore := &mockTenantStoreForOperator{
		tenants: []store.Tenant{
			{
				ID:            tenantID,
				Slug:          "super-admin",
				OperatorLevel: 2,
				Plan:          "internal",
				Status:        "active",
			},
		},
	}

	mw := NewTenantMiddleware(mockStore)
	jwtMw := NewJWTMiddleware(testSecret)

	claims := auth.TokenClaims{UserID: "u4", Email: "super@vellus.tech", TenantID: tenantID.String(), Role: "admin"}
	token, _ := auth.GenerateAccessToken(claims, testSecret)

	var capturedCtx context.Context
	handler := jwtMw.Wrap(mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest("GET", "/v1/agents", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if capturedCtx == nil {
		t.Fatal("handler was not called")
	}
	if !store.IsOperatorMode(capturedCtx) {
		t.Error("IsOperatorMode(ctx) = false for operator_level=2, want true")
	}
	if !store.IsCrossTenant(capturedCtx) {
		t.Error("IsCrossTenant(ctx) = false for operator_level=2, want true")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// PBT: ∀ tenant T: (T.OperatorLevel >= 1) ↔ (IsOperatorMode(ctx) = true)
// after TenantMiddleware HTTP processing.
//
// **Validates: Requirements 2.2, 2.6**
// ─────────────────────────────────────────────────────────────────────────────

func TestTenantMiddleware_PBT_OperatorModeBiconditional(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate a tenant with operator_level in {0, 1, 2}
		operatorLevel := rapid.IntRange(0, 2).Draw(rt, "operatorLevel")
		tenantID := uuid.New()

		mockStore := &mockTenantStoreForOperator{
			tenants: []store.Tenant{
				{
					ID:            tenantID,
					Slug:          "tenant-" + tenantID.String()[:8],
					OperatorLevel: operatorLevel,
					Plan:          "starter",
					Status:        "active",
				},
			},
		}

		mw := NewTenantMiddleware(mockStore)
		jwtMw := NewJWTMiddleware(testSecret)

		claims := auth.TokenClaims{
			UserID:   "pbt-user",
			Email:    "pbt@test.com",
			TenantID: tenantID.String(),
			Role:     "admin",
		}
		token, err := auth.GenerateAccessToken(claims, testSecret)
		if err != nil {
			rt.Fatalf("failed to generate token: %v", err)
		}

		var capturedCtx context.Context
		handler := jwtMw.Wrap(mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedCtx = r.Context()
			w.WriteHeader(http.StatusOK)
		})))

		req := httptest.NewRequest("GET", "/v1/test", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			rt.Fatalf("status = %d, want 200", rec.Code)
		}
		if capturedCtx == nil {
			rt.Fatal("handler was not called — context not captured")
		}

		isOp := store.IsOperatorMode(capturedCtx)
		expectOperator := operatorLevel >= 1

		// Biconditional: (OperatorLevel >= 1) ↔ (IsOperatorMode(ctx) = true)
		if expectOperator && !isOp {
			rt.Fatalf("operator_level=%d: IsOperatorMode=false, want true", operatorLevel)
		}
		if !expectOperator && isOp {
			rt.Fatalf("operator_level=%d: IsOperatorMode=true, want false", operatorLevel)
		}

		// Also verify IsCrossTenant follows the same biconditional
		isCT := store.IsCrossTenant(capturedCtx)
		if expectOperator && !isCT {
			rt.Fatalf("operator_level=%d: IsCrossTenant=false, want true", operatorLevel)
		}
		if !expectOperator && isCT {
			rt.Fatalf("operator_level=%d: IsCrossTenant=true, want false", operatorLevel)
		}

		// Verify operator tenant ID is correctly stored
		if expectOperator {
			if got := store.OperatorModeFromContext(capturedCtx); got != tenantID {
				rt.Fatalf("OperatorModeFromContext = %v, want %v", got, tenantID)
			}
		} else {
			if got := store.OperatorModeFromContext(capturedCtx); got != uuid.Nil {
				rt.Fatalf("OperatorModeFromContext = %v, want uuid.Nil for normal tenant", got)
			}
		}
	})
}

func TestTenantMiddleware_TenantStoreLookupError_ContinuesWithoutOperatorMode(t *testing.T) {
	// When GetByID returns an error, middleware should continue without operator mode
	// (non-fatal: tenant_id is still set, just no operator mode)
	tenantID := uuid.New()
	mockStore := &mockTenantStoreForOperator{
		err: errors.New("db connection refused"),
	}

	mw := NewTenantMiddleware(mockStore)
	jwtMw := NewJWTMiddleware(testSecret)

	claims := auth.TokenClaims{UserID: "u5", Email: "err@test.com", TenantID: tenantID.String(), Role: "admin"}
	token, _ := auth.GenerateAccessToken(claims, testSecret)

	var capturedCtx context.Context
	handler := jwtMw.Wrap(mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest("GET", "/v1/agents", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (non-fatal error)", rec.Code)
	}
	if capturedCtx == nil {
		t.Fatal("handler was not called")
	}
	// Should NOT have operator mode when DB lookup fails
	if store.IsOperatorMode(capturedCtx) {
		t.Error("IsOperatorMode(ctx) = true after DB error, want false")
	}
	// tenant_id should still be set
	if got := store.TenantIDFromContext(capturedCtx); got != tenantID {
		t.Errorf("TenantIDFromContext = %v, want %v", got, tenantID)
	}
}
