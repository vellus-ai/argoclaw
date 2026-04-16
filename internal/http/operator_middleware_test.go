package http

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/vellus-ai/argoclaw/internal/auth"
	"github.com/vellus-ai/argoclaw/internal/store"
)

// injectClaims injects auth.TokenClaims into the context (same path as JWTMiddleware).
func injectClaims(ctx context.Context, role string) context.Context {
	claims := &auth.TokenClaims{
		UserID:   "test-user-id",
		Email:    "test@vellus.tech",
		TenantID: uuid.New().String(),
		Role:     role,
	}
	return context.WithValue(ctx, ctxKeyUserClaims, claims)
}

func TestRequireOperatorRole_RejectsWhenNoOperatorMode(t *testing.T) {
	tenantID := uuid.New()
	ctx := context.Background()
	ctx = store.WithTenantID(ctx, tenantID)
	ctx = store.WithUserID(ctx, "u1")
	ctx = injectClaims(ctx, "admin")

	called := false
	handler := requireOperatorRole(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/v1/operator/tenants", nil)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if called {
		t.Error("handler was called, expected rejection")
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "OPERATOR_REQUIRED") {
		t.Errorf("body = %s, want OPERATOR_REQUIRED error code", rec.Body.String())
	}
}

func TestRequireOperatorRole_RejectsViewerInOperatorTenant(t *testing.T) {
	tenantID := uuid.New()
	ctx := context.Background()
	ctx = store.WithCrossTenant(ctx)        // appsec:cross-tenant-bypass — test helper
	ctx = store.WithOperatorMode(ctx, tenantID)
	ctx = store.WithTenantID(ctx, tenantID)
	ctx = store.WithUserID(ctx, "u-viewer")
	ctx = injectClaims(ctx, "viewer")

	called := false
	handler := requireOperatorRole(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/v1/operator/tenants", nil)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if called {
		t.Error("handler was called, expected rejection for viewer")
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "INSUFFICIENT_ROLE") {
		t.Errorf("body = %s, want INSUFFICIENT_ROLE error code", rec.Body.String())
	}
}

func TestRequireOperatorRole_AllowsAdminInOperatorTenant(t *testing.T) {
	tenantID := uuid.New()
	ctx := context.Background()
	ctx = store.WithCrossTenant(ctx)        // appsec:cross-tenant-bypass — test helper
	ctx = store.WithOperatorMode(ctx, tenantID)
	ctx = store.WithTenantID(ctx, tenantID)
	ctx = store.WithUserID(ctx, "u-admin")
	ctx = injectClaims(ctx, "admin")

	called := false
	handler := requireOperatorRole(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/v1/operator/tenants", nil)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if !called {
		t.Error("handler was not called, expected pass-through for admin in operator tenant")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestRequireOperatorRole_AllowsOperatorRoleInOperatorTenant(t *testing.T) {
	tenantID := uuid.New()
	ctx := context.Background()
	ctx = store.WithCrossTenant(ctx)        // appsec:cross-tenant-bypass — test helper
	ctx = store.WithOperatorMode(ctx, tenantID)
	ctx = store.WithTenantID(ctx, tenantID)
	ctx = store.WithUserID(ctx, "u-operator")
	ctx = injectClaims(ctx, "member") // "member" maps to RoleOperator

	called := false
	handler := requireOperatorRole(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/v1/operator/tenants", nil)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if !called {
		t.Error("handler was not called, expected pass-through for operator role")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestRequireOperatorRole_LogsSecurityEvent_NoOperatorMode(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	orig := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(orig)

	tenantID := uuid.New()
	ctx := context.Background()
	ctx = store.WithTenantID(ctx, tenantID)
	ctx = store.WithUserID(ctx, "u1")
	ctx = injectClaims(ctx, "admin")

	handler := requireOperatorRole(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/v1/operator/tenants", nil)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handler(rec, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "security.operator_access_denied") {
		t.Errorf("expected security.operator_access_denied log, got: %s", logOutput)
	}
}

func TestRequireOperatorRole_DoesNotAffectNonOperatorEndpoints(t *testing.T) {
	// A RoleViewer in the vellus tenant (operator_level=1) should still be able
	// to access normal endpoints that don't use requireOperatorRole.
	// This test verifies that the middleware is NOT applied to non-operator routes.
	tenantID := uuid.New()
	ctx := context.Background()
	ctx = store.WithCrossTenant(ctx)        // appsec:cross-tenant-bypass — test helper
	ctx = store.WithOperatorMode(ctx, tenantID)
	ctx = store.WithTenantID(ctx, tenantID)
	ctx = store.WithUserID(ctx, "u-viewer")
	ctx = injectClaims(ctx, "viewer")

	// A normal handler (not wrapped by requireOperatorRole) should work fine
	normalCalled := false
	normalHandler := func(w http.ResponseWriter, r *http.Request) {
		normalCalled = true
		w.WriteHeader(http.StatusOK)
	}

	req := httptest.NewRequest("GET", "/v1/agents", nil)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	http.HandlerFunc(normalHandler).ServeHTTP(rec, req)

	if !normalCalled {
		t.Error("normal handler was not called — RoleViewer should access non-operator endpoints")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 for non-operator endpoint", rec.Code)
	}
}

func TestRequireOperatorRole_DoesNotAlterContext(t *testing.T) {
	tenantID := uuid.New()
	ctx := context.Background()
	ctx = store.WithCrossTenant(ctx)        // appsec:cross-tenant-bypass — test helper
	ctx = store.WithOperatorMode(ctx, tenantID)
	ctx = store.WithTenantID(ctx, tenantID)
	ctx = store.WithUserID(ctx, "u-admin")
	ctx = injectClaims(ctx, "admin")

	handler := requireOperatorRole(func(w http.ResponseWriter, r *http.Request) {
		// Verify context is unchanged after passing through middleware
		innerCtx := r.Context()
		if store.TenantIDFromContext(innerCtx) != tenantID {
			t.Errorf("tenant_id changed: got %v, want %v", store.TenantIDFromContext(innerCtx), tenantID)
		}
		if store.OperatorModeFromContext(innerCtx) != tenantID {
			t.Errorf("operator_mode changed: got %v, want %v", store.OperatorModeFromContext(innerCtx), tenantID)
		}
		if !store.IsCrossTenant(innerCtx) {
			t.Error("cross_tenant flag was removed by middleware")
		}
		if store.UserIDFromContext(innerCtx) != "u-admin" {
			t.Errorf("user_id changed: got %q, want %q", store.UserIDFromContext(innerCtx), "u-admin")
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/v1/operator/tenants", nil)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestRequireOperatorRole_LogsSecurityEvent_InsufficientRole(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	orig := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(orig)

	tenantID := uuid.New()
	ctx := context.Background()
	ctx = store.WithCrossTenant(ctx)        // appsec:cross-tenant-bypass — test helper
	ctx = store.WithOperatorMode(ctx, tenantID)
	ctx = store.WithTenantID(ctx, tenantID)
	ctx = store.WithUserID(ctx, "u-viewer")
	ctx = injectClaims(ctx, "viewer")

	handler := requireOperatorRole(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/v1/operator/tenants", nil)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handler(rec, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "security.operator_access_denied") {
		t.Errorf("expected security.operator_access_denied log for insufficient role, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "INSUFFICIENT_ROLE") {
		t.Errorf("expected INSUFFICIENT_ROLE reason in log, got: %s", logOutput)
	}
}
