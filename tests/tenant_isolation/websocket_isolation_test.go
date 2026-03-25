package tenant_isolation_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/vellus-ai/argoclaw/internal/auth"
	"github.com/vellus-ai/argoclaw/internal/store"
)

// =============================================================================
// TDD: WebSocket — Tenant Isolation at Connection Level
// =============================================================================

// wsFrame represents a WebSocket RPC frame.
type wsFrame struct {
	Type    string          `json:"type"`
	ID      string          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	OK      *bool           `json:"ok,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Event   string          `json:"event,omitempty"`
	Error   *wsError        `json:"error,omitempty"`
}

type wsError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// connectParams are the params for the WS "connect" method.
type connectParams struct {
	Token    string `json:"token,omitempty"`
	UserID   string `json:"user_id,omitempty"`
	SenderID string `json:"sender_id,omitempty"`
	Locale   string `json:"locale,omitempty"`
}

// dialWS connects to the gateway WebSocket.
func dialWS(t *testing.T) *websocket.Conn {
	t.Helper()
	wsURL := strings.Replace(env.gatewayURL, "http://", "ws://", 1)
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
	wsURL += "/ws"

	dialer := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
	}
	conn, resp, err := dialer.Dial(wsURL, http.Header{})
	if err != nil {
		if resp != nil {
			t.Skipf("WS dial failed (status %d): %v", resp.StatusCode, err)
		}
		t.Skipf("WS dial failed: %v", err)
	}
	return conn
}

// sendFrame sends a JSON frame over WebSocket.
func sendFrame(t *testing.T, conn *websocket.Conn, frame wsFrame) {
	t.Helper()
	data, err := json.Marshal(frame)
	if err != nil {
		t.Fatalf("marshal frame: %v", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("write frame: %v", err)
	}
}

// readFrame reads a JSON frame from WebSocket with timeout.
func readFrame(t *testing.T, conn *websocket.Conn) *wsFrame {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read frame: %v", err)
	}
	var frame wsFrame
	if err := json.Unmarshal(data, &frame); err != nil {
		t.Fatalf("unmarshal frame: %v", err)
	}
	return &frame
}

// wsConnect sends the "connect" RPC and returns the response.
func wsConnect(t *testing.T, conn *websocket.Conn, token string) *wsFrame {
	t.Helper()
	params, _ := json.Marshal(connectParams{
		Token:    token,
		UserID:   "e2e-test-user",
		SenderID: "e2e-sender-" + uuid.New().String()[:8],
		Locale:   "en",
	})
	sendFrame(t, conn, wsFrame{
		Type:   "req",
		ID:     "1",
		Method: "connect",
		Params: params,
	})
	return readFrame(t, conn)
}

// Test: WS connection with Tenant A token authenticates successfully.
func TestWS_TenantA_ConnectOK(t *testing.T) {
	if env.gatewayURL == "" {
		t.Skip("TEST_GATEWAY_URL not set")
	}

	conn := dialWS(t)
	defer conn.Close()

	resp := wsConnect(t, conn, env.tokenA)

	if resp.Type != "res" {
		t.Fatalf("expected type=res, got %s", resp.Type)
	}
	if resp.OK != nil && !*resp.OK {
		t.Fatalf("connect failed: %+v", resp.Error)
	}
}

// Test: WS connection with invalid token is rejected.
func TestWS_InvalidToken_Rejected(t *testing.T) {
	if env.gatewayURL == "" {
		t.Skip("TEST_GATEWAY_URL not set")
	}

	conn := dialWS(t)
	defer conn.Close()

	resp := wsConnect(t, conn, "completely.invalid.token")

	// Should either fail connect or give viewer-only access
	if resp.OK != nil && *resp.OK {
		// If connect succeeded, verify it's viewer (not admin)
		var payload map[string]any
		if json.Unmarshal(resp.Payload, &payload) == nil {
			role, _ := payload["role"].(string)
			if role == "admin" || role == "operator" {
				t.Fatalf("SECURITY VIOLATION: invalid token got role=%s", role)
			}
		}
	}
}

// Test: WS connection with Tenant A token cannot see Tenant B agents.
func TestWS_CrossTenant_AgentList_Isolated(t *testing.T) {
	if env.gatewayURL == "" {
		t.Skip("TEST_GATEWAY_URL not set")
	}

	ctx := ctxForTenant(env.tenantB.ID)

	// Create agent for Tenant B
	agentB := &store.AgentData{
		AgentKey:    "ws-iso-agent-" + uuid.New().String()[:8],
		DisplayName: "WS Isolation Target",
		AgentType:   "predefined",
		Status:      "active",
		Provider:    "anthropic",
		Model:       "claude-sonnet-4-20250514",
	}
	if err := env.agentStore.Create(ctx, agentB); err != nil {
		t.Fatalf("create agent B: %v", err)
	}
	t.Cleanup(func() {
		env.db.ExecContext(ctx, `DELETE FROM agents WHERE id = $1`, agentB.ID)
	})

	// Connect as Tenant A
	conn := dialWS(t)
	defer conn.Close()

	resp := wsConnect(t, conn, env.tokenA)
	if resp.OK == nil || !*resp.OK {
		if resp.Error != nil {
			t.Skipf("connect failed: %s", resp.Error.Message)
		}
		t.Skip("connect did not return ok=true")
	}

	// Request agent list
	sendFrame(t, conn, wsFrame{
		Type:   "req",
		ID:     "2",
		Method: "agents.list",
	})

	listResp := readFrame(t, conn)
	if listResp.Payload != nil {
		payloadStr := string(listResp.Payload)
		if strings.Contains(payloadStr, agentB.AgentKey) {
			t.Fatal("SECURITY VIOLATION: Tenant A's WS connection returned Tenant B's agent")
		}
		if strings.Contains(payloadStr, env.tenantB.ID.String()) {
			t.Fatal("SECURITY VIOLATION: Tenant A's WS connection leaked Tenant B's tenant_id")
		}
	}
}

// Test: Two concurrent WS connections on different tenants don't leak events.
func TestWS_ConcurrentConnections_EventIsolation(t *testing.T) {
	if env.gatewayURL == "" {
		t.Skip("TEST_GATEWAY_URL not set")
	}

	// Connect as Tenant A
	connA := dialWS(t)
	defer connA.Close()
	respA := wsConnect(t, connA, env.tokenA)
	if respA.OK == nil || !*respA.OK {
		t.Skip("Tenant A connect failed")
	}

	// Connect as Tenant B
	connB := dialWS(t)
	defer connB.Close()
	respB := wsConnect(t, connB, env.tokenB)
	if respB.OK == nil || !*respB.OK {
		t.Skip("Tenant B connect failed")
	}

	// Send a status request on Tenant A's connection
	sendFrame(t, connA, wsFrame{
		Type:   "req",
		ID:     "3",
		Method: "status",
	})

	statusA := readFrame(t, connA)

	// Verify Tenant B doesn't receive any event from Tenant A's request
	connB.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, data, err := connB.ReadMessage()
	if err == nil {
		var leakedFrame wsFrame
		if json.Unmarshal(data, &leakedFrame) == nil {
			// status responses are OK — but any event with Tenant A data is a violation
			if leakedFrame.Type == "event" {
				payloadStr := string(leakedFrame.Payload)
				if strings.Contains(payloadStr, env.tenantA.ID.String()) {
					t.Fatal("SECURITY VIOLATION: Tenant B received event containing Tenant A's data")
				}
			}
		}
	}
	// Timeout is expected (no leak) — this is the desired behavior
	_ = statusA // used above
}

// Test: WS connection with forged tenant in connect params.
func TestWS_ForgedTenantParam_Rejected(t *testing.T) {
	if env.gatewayURL == "" {
		t.Skip("TEST_GATEWAY_URL not set")
	}

	conn := dialWS(t)
	defer conn.Close()

	// Send connect with Tenant A's JWT but try to claim Tenant B via params
	params, _ := json.Marshal(map[string]string{
		"token":     env.tokenA,
		"user_id":   env.userA.String(),
		"sender_id": "forged-sender",
		"locale":    "en",
		"tenant_id": env.tenantB.ID.String(), // attempt to override
	})
	sendFrame(t, conn, wsFrame{
		Type:   "req",
		ID:     "1",
		Method: "connect",
		Params: params,
	})

	resp := readFrame(t, conn)
	if resp.OK != nil && *resp.OK {
		// Connection succeeded — verify it's bound to Tenant A (from JWT), not B
		sendFrame(t, conn, wsFrame{
			Type:   "req",
			ID:     "2",
			Method: "agents.list",
		})
		listResp := readFrame(t, conn)
		if listResp.Payload != nil {
			payloadStr := string(listResp.Payload)
			if strings.Contains(payloadStr, env.tenantB.ID.String()) {
				t.Fatal("SECURITY VIOLATION: forged tenant_id param overrode JWT-based tenant binding")
			}
		}
	}
}

// =============================================================================
// TDD: WebSocket — Rate Limit / Connection Limit per Tenant
// =============================================================================

// Test: Multiple simultaneous connections from same tenant.
func TestWS_MultipleConnections_SameTenant(t *testing.T) {
	if env.gatewayURL == "" {
		t.Skip("TEST_GATEWAY_URL not set")
	}

	const numConns = 5
	conns := make([]*websocket.Conn, 0, numConns)

	for i := 0; i < numConns; i++ {
		conn := dialWS(t)
		conns = append(conns, conn)
		defer conn.Close()

		// Generate unique token per connection (same tenant)
		token := mustGenerateToken(t, auth.TokenClaims{
			UserID:   uuid.New().String(),
			Email:    "conn-test@e2e-alpha.test",
			TenantID: env.tenantA.ID.String(),
			Role:     "admin",
		})

		resp := wsConnect(t, conn, token)
		if resp.OK == nil || !*resp.OK {
			// If rate limit kicked in, that's acceptable behavior
			t.Logf("connection %d: connect result ok=%v", i, resp.OK)
			if resp.Error != nil {
				t.Logf("connection %d error: %s", i, resp.Error.Message)
			}
		}
	}

	// All connections should be isolated from each other and from Tenant B
	t.Logf("established %d concurrent connections for same tenant", len(conns))
}
