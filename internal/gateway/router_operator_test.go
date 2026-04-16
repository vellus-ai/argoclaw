package gateway

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"pgregory.net/rapid"

	"github.com/vellus-ai/argoclaw/internal/auth"
	"github.com/vellus-ai/argoclaw/internal/config"
	"github.com/vellus-ai/argoclaw/internal/store"
	"github.com/vellus-ai/argoclaw/pkg/protocol"
)

// ─────────────────────────────────────────────────────────────────────────────
// Task 3.1 — WebSocket handshake operator mode tests
// **Validates: Requirements 2.1, 2.2, 2.6**
//
// Unit tests:
//   (a) WS connect with tenant operator_level >= 1 → IsOperatorMode = true, IsCrossTenant = true
//   (b) WS connect with tenant operator_level = 0 → normal context, no operator mode
//   (c) Nonexistent tenant in GetByID → ErrUnauthorized
//
// PBT:
//   ∀ tenant T: (T.OperatorLevel >= 1) ↔ (IsOperatorMode(ctx) = true AND IsCrossTenant(ctx) = true)
// ─────────────────────────────────────────────────────────────────────────────

// mockTenantStore is a minimal TenantStore for testing operator_level lookup.
type mockTenantStore struct {
	store.TenantStore // embed to satisfy interface; only GetByID is used
	tenants           map[uuid.UUID]*store.Tenant
	getByIDErr        error
}

func (m *mockTenantStore) GetByID(_ context.Context, id uuid.UUID) (*store.Tenant, error) {
	if m.getByIDErr != nil {
		return nil, m.getByIDErr
	}
	t, ok := m.tenants[id]
	if !ok {
		return nil, errors.New("tenant not found")
	}
	return t, nil
}

// newTestRouterWithTenants builds a MethodRouter with a mock TenantStore injected.
func newTestRouterWithTenants(t *testing.T, tenantStore store.TenantStore) *MethodRouter {
	t.Helper()
	cfg := &config.Config{}
	cfg.Gateway.Token = testGatewayToken
	cfg.Gateway.JWTSecret = testJWTSecret
	s := &Server{cfg: cfg, tenants: tenantStore}
	return NewMethodRouter(s)
}

// --- Unit Tests ---

func TestHandleConnect_JWT_OperatorTenant_ActivatesOperatorMode(t *testing.T) {
	tenantID := uuid.New()
	ts := &mockTenantStore{
		tenants: map[uuid.UUID]*store.Tenant{
			tenantID: {
				ID:            tenantID,
				Slug:          "vellus",
				OperatorLevel: 1,
				Status:        "active",
			},
		},
	}
	router := newTestRouterWithTenants(t, ts)
	client := newTestClient(t, router.server)

	jwt := generateTestJWT(t, auth.TokenClaims{
		UserID:   "user-op-1",
		TenantID: tenantID.String(),
		Role:     "admin",
	})
	req := makeConnectReq(jwt, "", "", "en")

	router.handleConnect(context.Background(), client, req)

	if !client.authenticated {
		t.Fatal("client should be authenticated")
	}
	if client.operatorLevel < 1 {
		t.Errorf("operatorLevel = %d, want >= 1", client.operatorLevel)
	}

	// Verify the connect response is OK
	var resp connectResponse
	readResponse(t, client, &resp)
	if !resp.OK {
		t.Fatalf("expected OK response, got error: %+v", resp.Error)
	}

	// Now verify that Handle() propagates operator mode into context.
	// We do this by calling Handle with a custom method that captures the context.
	var capturedCtx context.Context
	router.Register("test.capture_ctx", func(ctx context.Context, _ *Client, _ *protocol.RequestFrame) {
		capturedCtx = ctx
	})

	captureReq := &protocol.RequestFrame{
		Type:   "req",
		ID:     "req-capture",
		Method: "test.capture_ctx",
	}
	// Need a policy engine that allows access, or nil (which skips check)
	router.Handle(context.Background(), client, captureReq)

	if capturedCtx == nil {
		t.Fatal("context was not captured — handler was not called")
	}
	if !store.IsOperatorMode(capturedCtx) {
		t.Error("IsOperatorMode(ctx) = false after operator tenant connect, want true")
	}
	if !store.IsCrossTenant(capturedCtx) {
		t.Error("IsCrossTenant(ctx) = false after operator tenant connect, want true")
	}
}

func TestHandleConnect_JWT_NormalTenant_NoOperatorMode(t *testing.T) {
	tenantID := uuid.New()
	ts := &mockTenantStore{
		tenants: map[uuid.UUID]*store.Tenant{
			tenantID: {
				ID:            tenantID,
				Slug:          "customer-abc",
				OperatorLevel: 0,
				Status:        "active",
			},
		},
	}
	router := newTestRouterWithTenants(t, ts)
	client := newTestClient(t, router.server)

	jwt := generateTestJWT(t, auth.TokenClaims{
		UserID:   "user-normal-1",
		TenantID: tenantID.String(),
		Role:     "admin",
	})
	req := makeConnectReq(jwt, "", "", "en")

	router.handleConnect(context.Background(), client, req)

	if !client.authenticated {
		t.Fatal("client should be authenticated")
	}
	if client.operatorLevel != 0 {
		t.Errorf("operatorLevel = %d, want 0 for normal tenant", client.operatorLevel)
	}

	var resp connectResponse
	readResponse(t, client, &resp)
	if !resp.OK {
		t.Fatalf("expected OK response, got error: %+v", resp.Error)
	}

	// Verify Handle() does NOT inject operator mode
	var capturedCtx context.Context
	router.Register("test.capture_ctx", func(ctx context.Context, _ *Client, _ *protocol.RequestFrame) {
		capturedCtx = ctx
	})
	captureReq := &protocol.RequestFrame{
		Type:   "req",
		ID:     "req-capture-2",
		Method: "test.capture_ctx",
	}
	router.Handle(context.Background(), client, captureReq)

	if capturedCtx == nil {
		t.Fatal("context was not captured")
	}
	if store.IsOperatorMode(capturedCtx) {
		t.Error("IsOperatorMode(ctx) = true for normal tenant, want false")
	}
	if store.IsCrossTenant(capturedCtx) {
		t.Error("IsCrossTenant(ctx) = true for normal tenant, want false")
	}
}

func TestHandleConnect_JWT_TenantNotFound_ReturnsUnauthorized(t *testing.T) {
	// Empty tenant store — no tenants exist
	ts := &mockTenantStore{
		tenants: map[uuid.UUID]*store.Tenant{},
	}
	router := newTestRouterWithTenants(t, ts)
	client := newTestClient(t, router.server)

	nonexistentTenantID := uuid.New()
	jwt := generateTestJWT(t, auth.TokenClaims{
		UserID:   "user-ghost-1",
		TenantID: nonexistentTenantID.String(),
		Role:     "admin",
	})
	req := makeConnectReq(jwt, "", "", "en")

	router.handleConnect(context.Background(), client, req)

	// Per task spec: tenant not found in GetByID → ErrUnauthorized
	if client.authenticated {
		t.Error("client should NOT be authenticated when tenant not found")
	}

	var resp errorResponse
	readResponse(t, client, &resp)
	if resp.OK {
		t.Fatal("expected error response for nonexistent tenant")
	}
	if resp.Error == nil || resp.Error.Code != protocol.ErrUnauthorized {
		t.Errorf("error code = %v, want %q", resp.Error, protocol.ErrUnauthorized)
	}
}

func TestHandleConnect_JWT_TenantStoreError_ReturnsUnauthorized(t *testing.T) {
	// TenantStore returns a DB error
	ts := &mockTenantStore{
		tenants:    map[uuid.UUID]*store.Tenant{},
		getByIDErr: errors.New("connection refused"),
	}
	router := newTestRouterWithTenants(t, ts)
	client := newTestClient(t, router.server)

	jwt := generateTestJWT(t, auth.TokenClaims{
		UserID:   "user-dberr-1",
		TenantID: uuid.New().String(),
		Role:     "admin",
	})
	req := makeConnectReq(jwt, "", "", "en")

	router.handleConnect(context.Background(), client, req)

	// Per task spec: GetByID error → ErrUnauthorized
	if client.authenticated {
		t.Error("client should NOT be authenticated when TenantStore returns error")
	}

	var resp errorResponse
	readResponse(t, client, &resp)
	if resp.OK {
		t.Fatal("expected error response for TenantStore error")
	}
	if resp.Error == nil || resp.Error.Code != protocol.ErrUnauthorized {
		t.Errorf("error code = %v, want %q", resp.Error, protocol.ErrUnauthorized)
	}
}

func TestHandleConnect_JWT_OperatorLevel2_ActivatesOperatorMode(t *testing.T) {
	tenantID := uuid.New()
	ts := &mockTenantStore{
		tenants: map[uuid.UUID]*store.Tenant{
			tenantID: {
				ID:            tenantID,
				Slug:          "super-admin",
				OperatorLevel: 2,
				Status:        "active",
			},
		},
	}
	router := newTestRouterWithTenants(t, ts)
	client := newTestClient(t, router.server)

	jwt := generateTestJWT(t, auth.TokenClaims{
		UserID:   "user-super-1",
		TenantID: tenantID.String(),
		Role:     "admin",
	})
	req := makeConnectReq(jwt, "", "", "en")

	router.handleConnect(context.Background(), client, req)

	if !client.authenticated {
		t.Fatal("client should be authenticated")
	}
	if client.operatorLevel != 2 {
		t.Errorf("operatorLevel = %d, want 2", client.operatorLevel)
	}
}

func TestHandleConnect_JWT_NoTenantStore_ContinuesNormally(t *testing.T) {
	// When server.tenants is nil, operator mode check is skipped gracefully
	router := newTestRouter(t, testGatewayToken, testJWTSecret)
	client := newTestClient(t, router.server)

	jwt := generateTestJWT(t, auth.TokenClaims{
		UserID:   "user-nostore-1",
		TenantID: uuid.New().String(),
		Role:     "admin",
	})
	req := makeConnectReq(jwt, "", "", "en")

	router.handleConnect(context.Background(), client, req)

	if !client.authenticated {
		t.Fatal("client should be authenticated even without TenantStore")
	}
	if client.operatorLevel != 0 {
		t.Errorf("operatorLevel = %d, want 0 when TenantStore is nil", client.operatorLevel)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// PBT: ∀ tenant T: (T.OperatorLevel >= 1) ↔ (IsOperatorMode(ctx) = true AND IsCrossTenant(ctx) = true)
// **Validates: Requirements 2.1, 2.2, 2.6**
// ─────────────────────────────────────────────────────────────────────────────

func TestHandleConnect_PBT_OperatorModeBiconditional(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate a tenant with operator_level in {0, 1, 2}
		operatorLevel := rapid.IntRange(0, 2).Draw(rt, "operatorLevel")
		tenantID := uuid.New()

		ts := &mockTenantStore{
			tenants: map[uuid.UUID]*store.Tenant{
				tenantID: {
					ID:            tenantID,
					Slug:          "tenant-" + tenantID.String()[:8],
					OperatorLevel: operatorLevel,
					Status:        "active",
				},
			},
		}

		cfg := &config.Config{}
		cfg.Gateway.Token = testGatewayToken
		cfg.Gateway.JWTSecret = testJWTSecret
		srv := &Server{cfg: cfg, tenants: ts}
		router := NewMethodRouter(srv)

		client := &Client{
			id:          "pbt-client-" + tenantID.String()[:8],
			server:      srv,
			send:        make(chan []byte, 4),
			connectedAt: time.Now(),
			remoteAddr:  "127.0.0.1:9999",
		}

		jwtToken, err := auth.GenerateAccessToken(auth.TokenClaims{
			UserID:   "user-pbt-" + tenantID.String()[:8],
			TenantID: tenantID.String(),
			Role:     "admin",
		}, testJWTSecret)
		if err != nil {
			rt.Fatalf("GenerateAccessToken: %v", err)
		}
		req := makeConnectReq(jwtToken, "", "", "en")

		router.handleConnect(context.Background(), client, req)

		if !client.authenticated {
			rt.Fatal("client should be authenticated for existing tenant")
		}

		// Drain the connect response
		select {
		case <-client.send:
		default:
			rt.Fatal("no connect response received")
		}

		// Capture context via Handle
		var capturedCtx context.Context
		router.Register("test.pbt_capture", func(ctx context.Context, _ *Client, _ *protocol.RequestFrame) {
			capturedCtx = ctx
		})
		captureReq := &protocol.RequestFrame{
			Type:   "req",
			ID:     "req-pbt",
			Method: "test.pbt_capture",
		}
		router.Handle(context.Background(), client, captureReq)

		if capturedCtx == nil {
			rt.Fatal("context was not captured")
		}

		isOp := store.IsOperatorMode(capturedCtx)
		isCT := store.IsCrossTenant(capturedCtx)
		expectOperator := operatorLevel >= 1

		// Biconditional: (OperatorLevel >= 1) ↔ (IsOperatorMode AND IsCrossTenant)
		if expectOperator && !isOp {
			rt.Fatalf("operator_level=%d: IsOperatorMode=false, want true", operatorLevel)
		}
		if expectOperator && !isCT {
			rt.Fatalf("operator_level=%d: IsCrossTenant=false, want true", operatorLevel)
		}
		if !expectOperator && isOp {
			rt.Fatalf("operator_level=%d: IsOperatorMode=true, want false", operatorLevel)
		}
		if !expectOperator && isCT {
			rt.Fatalf("operator_level=%d: IsCrossTenant=true, want false", operatorLevel)
		}
	})
}
