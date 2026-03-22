package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vellus-ai/argoclaw/internal/auth"
)

const testSecret = "test-jwt-secret-key-min-32-chars!!!"

func TestJWTMiddleware_ValidToken(t *testing.T) {
	mw := NewJWTMiddleware(testSecret)
	claims := auth.TokenClaims{UserID: "user-123", Email: "test@example.com", TenantID: "tenant-1", Role: "admin"}
	token, _ := auth.GenerateAccessToken(claims, testSecret)

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := UserClaimsFromContext(r.Context())
		if c == nil {
			t.Fatal("expected claims in context")
		}
		if c.UserID != "user-123" {
			t.Errorf("UserID = %q, want %q", c.UserID, "user-123")
		}
		if c.TenantID != "tenant-1" {
			t.Errorf("TenantID = %q, want %q", c.TenantID, "tenant-1")
		}
		// Check backward compat header
		if got := r.Header.Get("X-ArgoClaw-User-Id"); got != "user-123" {
			t.Errorf("X-ArgoClaw-User-Id = %q, want %q", got, "user-123")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/agents", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestJWTMiddleware_InvalidToken(t *testing.T) {
	mw := NewJWTMiddleware(testSecret)

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called for invalid JWT")
	}))

	req := httptest.NewRequest("GET", "/v1/agents", nil)
	req.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiJ9.invalid.signature")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestJWTMiddleware_NoToken_PassThrough(t *testing.T) {
	mw := NewJWTMiddleware(testSecret)
	called := false

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		// No claims in context — that's expected
		c := UserClaimsFromContext(r.Context())
		if c != nil {
			t.Error("expected nil claims without JWT")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/agents", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("handler should be called (pass-through for no JWT)")
	}
}

func TestJWTMiddleware_GatewayToken_PassThrough(t *testing.T) {
	mw := NewJWTMiddleware(testSecret)
	called := false

	handler := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	// Gateway tokens don't have dots (they're hex strings)
	req := httptest.NewRequest("GET", "/v1/agents", nil)
	req.Header.Set("Authorization", "Bearer abc123def456")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("handler should be called (pass-through for gateway token)")
	}
}
