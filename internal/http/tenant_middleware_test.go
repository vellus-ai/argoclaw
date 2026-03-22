package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/vellus-ai/argoclaw/internal/auth"
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
	ctx := context.WithValue(req.Context(), ctxKeyTenantID, tenantID)
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
