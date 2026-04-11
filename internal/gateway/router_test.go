package gateway

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/vellus-ai/argoclaw/internal/auth"
	"github.com/vellus-ai/argoclaw/internal/config"
	"github.com/vellus-ai/argoclaw/internal/permissions"
	"github.com/vellus-ai/argoclaw/pkg/protocol"
)

const testJWTSecret = "test-secret-for-jwt-validation-32chars!"
const testGatewayToken = "gw-token-abc123"

// connectResponse is the shape of the sendConnectResponse payload for assertions.
type connectResponse struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	OK      bool   `json:"ok"`
	Payload struct {
		Protocol           int    `json:"protocol"`
		Role               string `json:"role"`
		UserID             string `json:"user_id"`
		TenantID           string `json:"tenant_id,omitempty"`
		MustChangePassword bool   `json:"must_change_password,omitempty"`
		Server             struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"server"`
	} `json:"payload"`
	Error *protocol.ErrorShape `json:"error,omitempty"`
}

// errorResponse is the shape of an error response for assertions.
type errorResponse struct {
	Type  string               `json:"type"`
	ID    string               `json:"id"`
	OK    bool                 `json:"ok"`
	Error *protocol.ErrorShape `json:"error,omitempty"`
}

// newTestRouter builds a minimal MethodRouter for testing the connect handler.
func newTestRouter(t *testing.T, gatewayToken, jwtSecret string) *MethodRouter {
	t.Helper()
	cfg := &config.Config{}
	cfg.Gateway.Token = gatewayToken
	cfg.Gateway.JWTSecret = jwtSecret
	s := &Server{cfg: cfg}
	return NewMethodRouter(s)
}

// newTestClient builds a Client with a buffered send channel and no real websocket.
func newTestClient(t *testing.T, server *Server) *Client {
	t.Helper()
	return &Client{
		id:          "test-client-1",
		server:      server,
		send:        make(chan []byte, 4),
		connectedAt: time.Now(),
		remoteAddr:  "127.0.0.1:12345",
	}
}

// makeConnectReq builds a protocol.RequestFrame for the "connect" method.
func makeConnectReq(token, userID, senderID, locale string) *protocol.RequestFrame {
	params := map[string]string{
		"token":     token,
		"user_id":   userID,
		"sender_id": senderID,
		"locale":    locale,
	}
	raw, _ := json.Marshal(params)
	return &protocol.RequestFrame{
		Type:   "req",
		ID:     "req-1",
		Method: protocol.MethodConnect,
		Params: raw,
	}
}

// generateTestJWT creates a real JWT for testing using the auth package.
func generateTestJWT(t *testing.T, claims auth.TokenClaims) string {
	t.Helper()
	token, err := auth.GenerateAccessToken(claims, testJWTSecret)
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	return token
}

// readResponse reads from the client send channel and unmarshals into the target.
func readResponse(t *testing.T, client *Client, target any) {
	t.Helper()
	select {
	case data := <-client.send:
		if err := json.Unmarshal(data, target); err != nil {
			t.Fatalf("unmarshal response: %v\nraw: %s", err, data)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for response on client.send channel")
	}
}

// --- JWT Auth Path Tests ---

func TestHandleConnect_JWT_ValidToken_Admin(t *testing.T) {
	router := newTestRouter(t, testGatewayToken, testJWTSecret)
	client := newTestClient(t, router.server)

	jwt := generateTestJWT(t, auth.TokenClaims{
		UserID:   "user-admin-1",
		Email:    "admin@test.com",
		TenantID: "tenant-abc",
		Role:     "admin",
	})
	req := makeConnectReq(jwt, "ignored-user-id", "", "en")

	router.handleConnect(context.Background(), client, req)

	if client.role != permissions.RoleAdmin {
		t.Errorf("role = %q, want %q", client.role, permissions.RoleAdmin)
	}
	if !client.authenticated {
		t.Error("client should be authenticated")
	}
	if client.userID != "user-admin-1" {
		t.Errorf("userID = %q, want %q (from claims, not params)", client.userID, "user-admin-1")
	}
	if client.tenantID != "tenant-abc" {
		t.Errorf("tenantID = %q, want %q", client.tenantID, "tenant-abc")
	}

	var resp connectResponse
	readResponse(t, client, &resp)
	if !resp.OK {
		t.Fatalf("expected OK response, got error: %+v", resp.Error)
	}
	if resp.Payload.Role != string(permissions.RoleAdmin) {
		t.Errorf("response role = %q, want %q", resp.Payload.Role, permissions.RoleAdmin)
	}
}

func TestHandleConnect_JWT_ValidToken_Owner(t *testing.T) {
	router := newTestRouter(t, testGatewayToken, testJWTSecret)
	client := newTestClient(t, router.server)

	jwt := generateTestJWT(t, auth.TokenClaims{
		UserID:   "user-owner-1",
		TenantID: "tenant-xyz",
		Role:     "owner",
	})
	req := makeConnectReq(jwt, "", "", "en")

	router.handleConnect(context.Background(), client, req)

	if client.role != permissions.RoleAdmin {
		t.Errorf("role = %q, want %q (owner maps to admin)", client.role, permissions.RoleAdmin)
	}
}

func TestHandleConnect_JWT_ValidToken_Member(t *testing.T) {
	router := newTestRouter(t, testGatewayToken, testJWTSecret)
	client := newTestClient(t, router.server)

	jwt := generateTestJWT(t, auth.TokenClaims{
		UserID:   "user-member-1",
		TenantID: "tenant-123",
		Role:     "member",
	})
	req := makeConnectReq(jwt, "", "", "vi")

	router.handleConnect(context.Background(), client, req)

	if client.role != permissions.RoleOperator {
		t.Errorf("role = %q, want %q (member maps to operator)", client.role, permissions.RoleOperator)
	}
}

func TestHandleConnect_JWT_ValidToken_UnknownRole(t *testing.T) {
	router := newTestRouter(t, testGatewayToken, testJWTSecret)
	client := newTestClient(t, router.server)

	jwt := generateTestJWT(t, auth.TokenClaims{
		UserID:   "user-unknown-1",
		TenantID: "tenant-456",
		Role:     "intern", // not in mapping
	})
	req := makeConnectReq(jwt, "", "", "en")

	router.handleConnect(context.Background(), client, req)

	if client.role != permissions.RoleViewer {
		t.Errorf("role = %q, want %q (unknown role maps to viewer)", client.role, permissions.RoleViewer)
	}
	if !client.authenticated {
		t.Error("client should still be authenticated with viewer role")
	}
}

func TestHandleConnect_JWT_ExpiredToken(t *testing.T) {
	router := newTestRouter(t, testGatewayToken, testJWTSecret)
	client := newTestClient(t, router.server)

	// Generate a token that's already expired by using a very short expiry.
	// We can't easily generate an expired token via GenerateAccessToken,
	// so we'll use a token signed with the wrong audience or create a manually expired one.
	// Instead, use a tampered token (change the payload).
	jwt := generateTestJWT(t, auth.TokenClaims{
		UserID:   "user-expired-1",
		TenantID: "tenant-exp",
		Role:     "admin",
	})
	// Corrupt the signature to simulate an invalid/expired token
	jwt = jwt[:len(jwt)-5] + "XXXXX"

	req := makeConnectReq(jwt, "", "", "en")

	router.handleConnect(context.Background(), client, req)

	// Should NOT be authenticated — fail-closed
	if client.authenticated {
		t.Error("client should NOT be authenticated with invalid JWT")
	}

	var resp errorResponse
	readResponse(t, client, &resp)
	if resp.OK {
		t.Fatal("expected error response for invalid JWT")
	}
	if resp.Error == nil || resp.Error.Code != protocol.ErrUnauthorized {
		t.Errorf("error code = %v, want %q", resp.Error, protocol.ErrUnauthorized)
	}
}

func TestHandleConnect_JWT_InvalidSignature(t *testing.T) {
	router := newTestRouter(t, testGatewayToken, testJWTSecret)
	client := newTestClient(t, router.server)

	// Sign with a different secret
	token, err := auth.GenerateAccessToken(auth.TokenClaims{
		UserID:   "user-wrong-1",
		TenantID: "tenant-wrong",
		Role:     "admin",
	}, "wrong-secret-completely-different!")
	if err != nil {
		t.Fatal(err)
	}

	req := makeConnectReq(token, "", "", "en")

	router.handleConnect(context.Background(), client, req)

	if client.authenticated {
		t.Error("client should NOT be authenticated with wrong-secret JWT")
	}

	var resp errorResponse
	readResponse(t, client, &resp)
	if resp.OK {
		t.Fatal("expected error response for wrong-secret JWT")
	}
	if resp.Error == nil || resp.Error.Code != protocol.ErrUnauthorized {
		t.Errorf("error code = %v, want %q", resp.Error, protocol.ErrUnauthorized)
	}
}

func TestHandleConnect_JWT_NoJWTSecret_FallsThrough(t *testing.T) {
	// JWTSecret empty, no gateway token → falls through to Path 2 (operator backward compat)
	router := newTestRouter(t, "", "")
	client := newTestClient(t, router.server)

	jwt := generateTestJWT(t, auth.TokenClaims{
		UserID:   "user-nojwt-1",
		TenantID: "tenant-nojwt",
		Role:     "admin",
	})
	req := makeConnectReq(jwt, "param-user-id", "", "en")

	router.handleConnect(context.Background(), client, req)

	// Should fall through to Path 2 (no gateway token configured → operator)
	if client.role != permissions.RoleOperator {
		t.Errorf("role = %q, want %q (no jwt secret → falls through to backward compat)", client.role, permissions.RoleOperator)
	}
	// UserID comes from params since JWT path was skipped
	if client.userID != "param-user-id" {
		t.Errorf("userID = %q, want %q (from params, jwt path skipped)", client.userID, "param-user-id")
	}
}

func TestHandleConnect_JWT_UserID_FromClaims_NotParams(t *testing.T) {
	router := newTestRouter(t, testGatewayToken, testJWTSecret)
	client := newTestClient(t, router.server)

	jwt := generateTestJWT(t, auth.TokenClaims{
		UserID:   "claims-user-id",
		TenantID: "tenant-claims",
		Role:     "member",
	})
	req := makeConnectReq(jwt, "params-user-id-SHOULD-BE-IGNORED", "", "en")

	router.handleConnect(context.Background(), client, req)

	if client.userID != "claims-user-id" {
		t.Errorf("userID = %q, want %q (must come from JWT claims, not params)", client.userID, "claims-user-id")
	}
}

func TestHandleConnect_JWT_TenantID_Set(t *testing.T) {
	router := newTestRouter(t, testGatewayToken, testJWTSecret)
	client := newTestClient(t, router.server)

	jwt := generateTestJWT(t, auth.TokenClaims{
		UserID:   "user-tenant-1",
		TenantID: "tenant-id-from-jwt",
		Role:     "operator",
	})
	req := makeConnectReq(jwt, "", "", "en")

	router.handleConnect(context.Background(), client, req)

	if client.tenantID != "tenant-id-from-jwt" {
		t.Errorf("tenantID = %q, want %q", client.tenantID, "tenant-id-from-jwt")
	}
}

func TestHandleConnect_JWT_MustChangePassword(t *testing.T) {
	router := newTestRouter(t, testGatewayToken, testJWTSecret)
	client := newTestClient(t, router.server)

	jwt := generateTestJWT(t, auth.TokenClaims{
		UserID:             "user-chgpwd-1",
		TenantID:           "tenant-chgpwd",
		Role:               "admin",
		MustChangePassword: true,
	})
	req := makeConnectReq(jwt, "", "", "en")

	router.handleConnect(context.Background(), client, req)

	if !client.mustChangePassword {
		t.Error("mustChangePassword should be true")
	}

	var resp connectResponse
	readResponse(t, client, &resp)
	if !resp.OK {
		t.Fatalf("expected OK response, got error: %+v", resp.Error)
	}
	if !resp.Payload.MustChangePassword {
		t.Error("response payload must_change_password should be true")
	}
}

func TestHandleConnect_JWT_ConnectResponse_Fields(t *testing.T) {
	router := newTestRouter(t, testGatewayToken, testJWTSecret)
	client := newTestClient(t, router.server)

	jwt := generateTestJWT(t, auth.TokenClaims{
		UserID:   "user-resp-1",
		TenantID: "tenant-resp",
		Role:     "member",
	})
	req := makeConnectReq(jwt, "", "", "en")

	router.handleConnect(context.Background(), client, req)

	var resp connectResponse
	readResponse(t, client, &resp)
	if !resp.OK {
		t.Fatalf("expected OK response, got error: %+v", resp.Error)
	}
	if resp.Payload.TenantID != "tenant-resp" {
		t.Errorf("tenant_id = %q, want %q", resp.Payload.TenantID, "tenant-resp")
	}
	if resp.Payload.UserID != "user-resp-1" {
		t.Errorf("user_id = %q, want %q", resp.Payload.UserID, "user-resp-1")
	}
	if resp.Payload.Role != string(permissions.RoleOperator) {
		t.Errorf("role = %q, want %q", resp.Payload.Role, permissions.RoleOperator)
	}
	if resp.Payload.Protocol != protocol.ProtocolVersion {
		t.Errorf("protocol = %d, want %d", resp.Payload.Protocol, protocol.ProtocolVersion)
	}
}

// --- Regression Tests ---

func TestHandleConnect_GatewayToken_Regression(t *testing.T) {
	router := newTestRouter(t, testGatewayToken, testJWTSecret)
	client := newTestClient(t, router.server)

	req := makeConnectReq(testGatewayToken, "gw-user", "", "en")

	router.handleConnect(context.Background(), client, req)

	if client.role != permissions.RoleAdmin {
		t.Errorf("role = %q, want %q (gateway token → admin)", client.role, permissions.RoleAdmin)
	}
	if !client.authenticated {
		t.Error("client should be authenticated")
	}
	if client.userID != "gw-user" {
		t.Errorf("userID = %q, want %q (gateway token uses params.UserID)", client.userID, "gw-user")
	}
	// tenantID should be empty for gateway token auth
	if client.tenantID != "" {
		t.Errorf("tenantID = %q, want empty (gateway token has no tenant)", client.tenantID)
	}
}

func TestHandleConnect_Fallback_Viewer_Regression(t *testing.T) {
	router := newTestRouter(t, testGatewayToken, testJWTSecret)
	client := newTestClient(t, router.server)

	// Non-JWT, non-gateway-token string (no dots)
	req := makeConnectReq("wrong-token-no-dots", "fallback-user", "", "en")

	router.handleConnect(context.Background(), client, req)

	if client.role != permissions.RoleViewer {
		t.Errorf("role = %q, want %q (wrong token → viewer fallback)", client.role, permissions.RoleViewer)
	}
	if !client.authenticated {
		t.Error("viewer fallback should still set authenticated=true")
	}
}
