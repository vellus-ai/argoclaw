package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/vellus-ai/argoclaw/internal/auth"
	"github.com/vellus-ai/argoclaw/internal/bus"
	"github.com/vellus-ai/argoclaw/internal/permissions"
	"github.com/vellus-ai/argoclaw/internal/store"

	"github.com/google/uuid"
)

// setupTestCache initializes the package-level cache for testing.
// Returns a cleanup function to restore state.
func setupTestCache(t *testing.T, keys map[string]*store.APIKeyData) *mockAPIKeyStore {
	t.Helper()
	ms := newMockAPIKeyStore()
	for hash, key := range keys {
		ms.keys[hash] = key
	}
	pkgAPIKeyCache = newAPIKeyCache(ms, 5*time.Minute)
	t.Cleanup(func() { pkgAPIKeyCache = nil })
	return ms
}

func TestResolveAuth_GatewayToken(t *testing.T) {
	setupTestCache(t, nil)

	r := httptest.NewRequest("GET", "/v1/agents", nil)
	r.Header.Set("Authorization", "Bearer my-gateway-token")

	auth := resolveAuth(r, "my-gateway-token")
	if !auth.Authenticated {
		t.Fatal("expected authenticated")
	}
	if auth.Role != permissions.RoleAdmin {
		t.Errorf("role = %v, want admin", auth.Role)
	}
}

func TestResolveAuth_WrongToken(t *testing.T) {
	setupTestCache(t, nil)

	r := httptest.NewRequest("GET", "/v1/agents", nil)
	r.Header.Set("Authorization", "Bearer wrong-token")

	auth := resolveAuth(r, "correct-token")
	if auth.Authenticated {
		t.Fatal("expected unauthenticated for wrong token")
	}
}

func TestResolveAuth_NoGatewayTokenConfigured_DeniesAccess(t *testing.T) {
	setupTestCache(t, nil)

	r := httptest.NewRequest("GET", "/v1/agents", nil)

	auth := resolveAuth(r, "") // no gateway token configured
	if auth.Authenticated {
		t.Fatal("expected unauthenticated when no token configured")
	}
}

func TestResolveAuth_APIKeyReadScope(t *testing.T) {
	// We need to hash the token the same way crypto.HashAPIKey does
	// For testing, we'll inject directly into the cache
	keyID := uuid.New()
	ms := newMockAPIKeyStore()
	ms.keys["test-hash"] = &store.APIKeyData{
		ID:     keyID,
		Scopes: []string{"operator.read"},
	}
	pkgAPIKeyCache = newAPIKeyCache(ms, 5*time.Minute)
	defer func() { pkgAPIKeyCache = nil }()

	// Pre-populate cache directly for the hash
	pkgAPIKeyCache.getOrFetch(nil, "test-hash")

	// Now test via resolveAuthBearer with the hash lookup
	r := httptest.NewRequest("GET", "/v1/agents", nil)
	// Directly test with the resolved key
	key, role := pkgAPIKeyCache.getOrFetch(nil, "test-hash")
	if key == nil {
		t.Fatal("expected key from cache")
	}
	_ = r
	if role != permissions.RoleViewer {
		t.Errorf("role = %v, want viewer for read scope", role)
	}
}

func TestResolveAuth_APIKeyAdminScope(t *testing.T) {
	ms := newMockAPIKeyStore()
	ms.keys["admin-hash"] = &store.APIKeyData{
		ID:     uuid.New(),
		Scopes: []string{"operator.admin"},
	}
	pkgAPIKeyCache = newAPIKeyCache(ms, 5*time.Minute)
	defer func() { pkgAPIKeyCache = nil }()

	key, role := pkgAPIKeyCache.getOrFetch(nil, "admin-hash")
	if key == nil {
		t.Fatal("expected key from cache")
	}
	if role != permissions.RoleAdmin {
		t.Errorf("role = %v, want admin", role)
	}
}

func TestResolveAuth_APIKeyWriteScope(t *testing.T) {
	ms := newMockAPIKeyStore()
	ms.keys["write-hash"] = &store.APIKeyData{
		ID:     uuid.New(),
		Scopes: []string{"operator.write"},
	}
	pkgAPIKeyCache = newAPIKeyCache(ms, 5*time.Minute)
	defer func() { pkgAPIKeyCache = nil }()

	key, role := pkgAPIKeyCache.getOrFetch(nil, "write-hash")
	if key == nil {
		t.Fatal("expected key from cache")
	}
	if role != permissions.RoleOperator {
		t.Errorf("role = %v, want operator for write scope", role)
	}
}

func TestResolveAuth_JWTRoleMapping(t *testing.T) {
	tests := []struct {
		name     string
		userID   string
		role     string
		wantRole permissions.Role
	}{
		{"owner maps to admin", "user-123", "owner", permissions.RoleAdmin},
		{"admin maps to admin", "user-456", "admin", permissions.RoleAdmin},
		{"member maps to operator", "user-789", "member", permissions.RoleOperator},
		{"operator maps to operator", "user-op", "operator", permissions.RoleOperator},
		{"unknown role maps to viewer", "user-unknown", "guest", permissions.RoleViewer},
		{"empty role maps to viewer", "user-empty", "", permissions.RoleViewer},
		{"empty userID still authenticates", "", "member", permissions.RoleOperator},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupTestCache(t, nil)
			r := httptest.NewRequest("GET", "/v1/agents", nil)
			ctx := context.WithValue(r.Context(), ctxKeyUserClaims, &auth.TokenClaims{
				UserID: tt.userID,
				Role:   tt.role,
			})
			r = r.WithContext(ctx)
			result := resolveAuth(r, "gateway-token")
			if !result.Authenticated {
				t.Fatal("expected authenticated")
			}
			if result.Role != tt.wantRole {
				t.Errorf("role = %v, want %v for JWT role %q", result.Role, tt.wantRole, tt.role)
			}
		})
	}
}

func TestResolveAuth_GatewayTokenTakesPriorityOverJWT(t *testing.T) {
	setupTestCache(t, nil)

	r := httptest.NewRequest("GET", "/v1/agents", nil)
	// Set Bearer header to match the gateway token
	r.Header.Set("Authorization", "Bearer gateway-secret")
	// Also inject JWT claims with a different (lower) role
	ctx := context.WithValue(r.Context(), ctxKeyUserClaims, &auth.TokenClaims{
		UserID:   "user-jwt",
		TenantID: uuid.New().String(),
		Role:     "member", // would map to RoleOperator if JWT were used
	})
	r = r.WithContext(ctx)

	result := resolveAuth(r, "gateway-secret") // gateway token is 1st priority, JWT is 4th
	if !result.Authenticated {
		t.Fatal("expected authenticated")
	}
	if result.Role != permissions.RoleAdmin {
		t.Errorf("role = %v, want admin (gateway token should take priority over JWT member)", result.Role)
	}
}

func TestRequireAuth_JWTPasses(t *testing.T) {
	setupTestCache(t, nil)

	handler := requireAuth("gateway-secret", "", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	r := httptest.NewRequest("GET", "/v1/providers", nil)
	ctx := context.WithValue(r.Context(), ctxKeyUserClaims, &auth.TokenClaims{
		UserID:   "user-123",
		TenantID: uuid.New().String(),
		Role:     "owner",
	})
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()
	handler(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 for JWT auth", w.Code)
	}
}

func TestRequireAuth_JWTInjectsTenantID(t *testing.T) {
	setupTestCache(t, nil)

	tenantID := uuid.New()
	var gotTenantID uuid.UUID
	handler := requireAuth("gateway-secret", "", func(w http.ResponseWriter, r *http.Request) {
		gotTenantID = store.TenantIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	r := httptest.NewRequest("GET", "/v1/providers", nil)
	ctx := context.WithValue(r.Context(), ctxKeyUserClaims, &auth.TokenClaims{
		UserID:   "user-123",
		TenantID: tenantID.String(),
		Role:     "owner",
	})
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()
	handler(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if gotTenantID != tenantID {
		t.Errorf("tenantID = %v, want %v", gotTenantID, tenantID)
	}
}

func TestHttpMinRole(t *testing.T) {
	tests := []struct {
		method string
		want   permissions.Role
	}{
		{http.MethodGet, permissions.RoleViewer},
		{http.MethodHead, permissions.RoleViewer},
		{http.MethodOptions, permissions.RoleViewer},
		{http.MethodPost, permissions.RoleOperator},
		{http.MethodPut, permissions.RoleOperator},
		{http.MethodPatch, permissions.RoleOperator},
		{http.MethodDelete, permissions.RoleOperator},
	}

	for _, tt := range tests {
		got := httpMinRole(tt.method)
		if got != tt.want {
			t.Errorf("httpMinRole(%s) = %v, want %v", tt.method, got, tt.want)
		}
	}
}

func TestRequireAuth_Unauthorized(t *testing.T) {
	setupTestCache(t, nil)

	handler := requireAuth("secret", "", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	r := httptest.NewRequest("GET", "/v1/agents", nil)
	w := httptest.NewRecorder()
	handler(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestRequireAuth_GatewayTokenPasses(t *testing.T) {
	setupTestCache(t, nil)

	handler := requireAuth("secret", "", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	r := httptest.NewRequest("GET", "/v1/agents", nil)
	r.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()
	handler(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestRequireAuth_InjectLocaleAndUserID(t *testing.T) {
	setupTestCache(t, nil)

	var gotLocale, gotUserID string
	handler := requireAuth("secret", "", func(w http.ResponseWriter, r *http.Request) {
		gotLocale = store.LocaleFromContext(r.Context())
		gotUserID = store.UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	r := httptest.NewRequest("GET", "/v1/agents", nil)
	r.Header.Set("Authorization", "Bearer secret")
	r.Header.Set("Accept-Language", "vi")
	r.Header.Set("X-ArgoClaw-User-Id", "user123")
	w := httptest.NewRecorder()
	handler(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if gotLocale != "vi" {
		t.Errorf("locale = %q, want 'vi'", gotLocale)
	}
	if gotUserID != "user123" {
		t.Errorf("userID = %q, want 'user123'", gotUserID)
	}
}

func TestRequireAuth_AdminRoleEnforced(t *testing.T) {
	setupTestCache(t, nil)

	handler := requireAuth("", permissions.RoleAdmin, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	r := httptest.NewRequest("POST", "/v1/api-keys", nil)
	w := httptest.NewRecorder()
	handler(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestRequireAuth_AutoDetectRole_GET(t *testing.T) {
	setupTestCache(t, nil)

	handler := requireAuth("", "", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	r := httptest.NewRequest("GET", "/v1/agents", nil)
	w := httptest.NewRecorder()
	handler(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

// TestRequireAuth_JWTInjectsUserID verifies that when JWTMiddleware processes a
// valid JWT, it sets the X-ArgoClaw-User-Id header, and requireAuth then picks
// up the user ID via extractUserID and injects it into the store context.
// This documents the end-to-end flow: JWT claims → header → store context.
func TestRequireAuth_JWTInjectsUserID(t *testing.T) {
	setupTestCache(t, nil)

	jwtSecret := "test-jwt-secret-for-userid-flow!!"
	mw := NewJWTMiddleware(jwtSecret)

	tenantID := uuid.New()
	claims := auth.TokenClaims{
		UserID:   "jwt-user-42",
		Email:    "test@example.com",
		TenantID: tenantID.String(),
		Role:     "admin",
	}
	token, err := auth.GenerateAccessToken(claims, jwtSecret)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	var gotUserID string
	// Chain: JWTMiddleware → requireAuth → inner handler that reads context.
	inner := requireAuth("", "", func(w http.ResponseWriter, r *http.Request) {
		gotUserID = store.UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	// Wrap the requireAuth handler with JWTMiddleware so the full flow executes.
	handler := mw.Wrap(http.HandlerFunc(inner))

	r := httptest.NewRequest("GET", "/v1/agents", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	// Do NOT set X-ArgoClaw-User-Id header — it must come from JWT middleware.
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if gotUserID != "jwt-user-42" {
		t.Errorf("UserIDFromContext = %q, want %q", gotUserID, "jwt-user-42")
	}
}

func TestInitAPIKeyCache_PubsubInvalidation(t *testing.T) {
	mb := bus.New()
	ms := newMockAPIKeyStore()
	ms.keys["pubsub-hash"] = &store.APIKeyData{
		ID:     uuid.New(),
		Scopes: []string{"operator.read"},
	}

	// Save original and restore after test
	origCache := pkgAPIKeyCache
	defer func() { pkgAPIKeyCache = origCache }()

	InitAPIKeyCache(ms, mb)

	// Populate cache
	key, _ := pkgAPIKeyCache.getOrFetch(nil, "pubsub-hash")
	if key == nil {
		t.Fatal("expected key after initial fetch")
	}
	if ms.getCalls() != 1 {
		t.Fatalf("calls = %d, want 1", ms.getCalls())
	}

	// Broadcast cache invalidation
	mb.Broadcast(bus.Event{
		Name:    "cache.invalidate",
		Payload: bus.CacheInvalidatePayload{Kind: bus.CacheKindAPIKeys, Key: "any"},
	})

	// Cache should be cleared, next fetch should hit store
	pkgAPIKeyCache.getOrFetch(nil, "pubsub-hash")
	if ms.getCalls() != 2 {
		t.Errorf("calls after invalidation = %d, want 2", ms.getCalls())
	}
}

func TestInitAPIKeyCache_IgnoresOtherKinds(t *testing.T) {
	mb := bus.New()
	ms := newMockAPIKeyStore()
	ms.keys["other-hash"] = &store.APIKeyData{
		ID:     uuid.New(),
		Scopes: []string{"operator.read"},
	}

	origCache := pkgAPIKeyCache
	defer func() { pkgAPIKeyCache = origCache }()

	InitAPIKeyCache(ms, mb)

	// Populate cache
	pkgAPIKeyCache.getOrFetch(nil, "other-hash")

	// Broadcast a different kind
	mb.Broadcast(bus.Event{
		Name:    "cache.invalidate",
		Payload: bus.CacheInvalidatePayload{Kind: bus.CacheKindAgent, Key: "any"},
	})

	// Cache should NOT be cleared
	pkgAPIKeyCache.getOrFetch(nil, "other-hash")
	if ms.getCalls() != 1 {
		t.Errorf("calls = %d, want 1 (non-api_keys kind should not invalidate)", ms.getCalls())
	}
}
