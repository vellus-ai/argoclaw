package gateway

import (
	"context"
	"encoding/json"
	"testing"
	"testing/quick"

	"github.com/google/uuid"

	"github.com/vellus-ai/argoclaw/internal/store"
	"github.com/vellus-ai/argoclaw/pkg/protocol"
)

// testClient creates a minimal Client for testing Handle().
func testClient(tenantID, userID, locale string) *Client {
	return &Client{
		id:       "test-client",
		tenantID: tenantID,
		userID:   userID,
		locale:   locale,
		send:     make(chan []byte, 16),
	}
}

// drainResponse reads the last response from the client's send channel.
func drainResponse(t *testing.T, c *Client) *protocol.ResponseFrame {
	t.Helper()
	select {
	case data := <-c.send:
		var resp protocol.ResponseFrame
		if err := json.Unmarshal(data, &resp); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		return &resp
	default:
		return nil
	}
}

func TestRouter_Handle_InjectsTenantID(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	client := testClient(tenantID.String(), "user-42", "vi")

	var capturedCtx context.Context
	router := &MethodRouter{
		handlers: map[string]MethodHandler{
			"test.method": func(ctx context.Context, _ *Client, _ *protocol.RequestFrame) {
				capturedCtx = ctx
			},
		},
		server: &Server{},
	}

	req := &protocol.RequestFrame{ID: "r1", Method: "test.method"}
	router.Handle(context.Background(), client, req)

	if capturedCtx == nil {
		t.Fatal("handler was not called")
	}

	got := store.TenantIDFromContext(capturedCtx)
	if got != tenantID {
		t.Errorf("tenant_id: got %s, want %s", got, tenantID)
	}
}

func TestRouter_Handle_InjectsUserIDAndLocale(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	client := testClient(tenantID.String(), "user-42", "vi")

	var capturedCtx context.Context
	router := &MethodRouter{
		handlers: map[string]MethodHandler{
			"test.method": func(ctx context.Context, _ *Client, _ *protocol.RequestFrame) {
				capturedCtx = ctx
			},
		},
		server: &Server{},
	}

	req := &protocol.RequestFrame{ID: "r1", Method: "test.method"}
	router.Handle(context.Background(), client, req)

	if capturedCtx == nil {
		t.Fatal("handler was not called")
	}

	if uid := store.UserIDFromContext(capturedCtx); uid != "user-42" {
		t.Errorf("user_id: got %q, want %q", uid, "user-42")
	}
	if loc := store.LocaleFromContext(capturedCtx); loc != "vi" {
		t.Errorf("locale: got %q, want %q", loc, "vi")
	}
}

func TestRouter_Handle_RejectsMissingTenant(t *testing.T) {
	t.Parallel()
	client := testClient("", "user-42", "en")

	handlerCalled := false
	router := &MethodRouter{
		handlers: map[string]MethodHandler{
			"test.method": func(ctx context.Context, _ *Client, _ *protocol.RequestFrame) {
				handlerCalled = true
			},
		},
		server: &Server{},
	}

	req := &protocol.RequestFrame{ID: "r1", Method: "test.method"}
	router.Handle(context.Background(), client, req)

	if handlerCalled {
		t.Error("handler should NOT have been called for missing tenant_id")
	}

	resp := drainResponse(t, client)
	if resp == nil {
		t.Fatal("expected error response")
	}
	if resp.OK {
		t.Error("expected OK=false")
	}
	if resp.Error == nil || resp.Error.Code != protocol.ErrForbidden {
		t.Errorf("expected ErrForbidden, got %+v", resp.Error)
	}
}

func TestRouter_Handle_RejectsInvalidTenantUUID(t *testing.T) {
	t.Parallel()
	client := testClient("not-a-valid-uuid", "user-42", "en")

	handlerCalled := false
	router := &MethodRouter{
		handlers: map[string]MethodHandler{
			"test.method": func(ctx context.Context, _ *Client, _ *protocol.RequestFrame) {
				handlerCalled = true
			},
		},
		server: &Server{},
	}

	req := &protocol.RequestFrame{ID: "r1", Method: "test.method"}
	router.Handle(context.Background(), client, req)

	if handlerCalled {
		t.Error("handler should NOT have been called for invalid tenant UUID")
	}

	resp := drainResponse(t, client)
	if resp == nil {
		t.Fatal("expected error response")
	}
	if resp.Error == nil || resp.Error.Code != protocol.ErrForbidden {
		t.Errorf("expected ErrForbidden, got %+v", resp.Error)
	}
}

func TestRouter_Handle_WhitelistBypassesTenantCheck(t *testing.T) {
	t.Parallel()
	// Client with NO tenant — exempt methods should still work
	client := testClient("", "", "en")

	for _, method := range []string{protocol.MethodConnect, protocol.MethodHealth, protocol.MethodBrowserPairingStatus} {
		t.Run(method, func(t *testing.T) {
			handlerCalled := false
			router := &MethodRouter{
				handlers: map[string]MethodHandler{
					method: func(ctx context.Context, _ *Client, _ *protocol.RequestFrame) {
						handlerCalled = true
					},
				},
				server: &Server{},
			}

			req := &protocol.RequestFrame{ID: "r1", Method: method}
			router.Handle(context.Background(), client, req)

			if !handlerCalled {
				t.Errorf("handler for exempt method %q should have been called", method)
			}
		})
	}
}

func TestRouter_Handle_RejectsNilUUIDTenant(t *testing.T) {
	t.Parallel()
	client := testClient(uuid.Nil.String(), "user-42", "en")

	handlerCalled := false
	router := &MethodRouter{
		handlers: map[string]MethodHandler{
			"test.method": func(ctx context.Context, _ *Client, _ *protocol.RequestFrame) {
				handlerCalled = true
			},
		},
		server: &Server{},
	}

	req := &protocol.RequestFrame{ID: "r1", Method: "test.method"}
	router.Handle(context.Background(), client, req)

	if handlerCalled {
		t.Error("handler should NOT have been called for Nil UUID tenant")
	}

	resp := drainResponse(t, client)
	if resp == nil {
		t.Fatal("expected error response")
	}
	if resp.Error == nil || resp.Error.Code != protocol.ErrForbidden {
		t.Errorf("expected ErrForbidden, got %+v", resp.Error)
	}
}

// --- PBT: whitelist consistency (Task 10.4) ---

func TestPBT_WhitelistMethods_NeverRejectForMissingTenant(t *testing.T) {
	t.Parallel()
	exemptMethods := []string{protocol.MethodConnect, protocol.MethodHealth, protocol.MethodBrowserPairingStatus}

	f := func(idx uint8) bool {
		method := exemptMethods[int(idx)%len(exemptMethods)]
		client := testClient("", "", "en") // no tenant

		handlerCalled := false
		router := &MethodRouter{
			handlers: map[string]MethodHandler{
				method: func(_ context.Context, _ *Client, _ *protocol.RequestFrame) {
					handlerCalled = true
				},
			},
			server: &Server{},
		}

		req := &protocol.RequestFrame{ID: "r1", Method: method}
		router.Handle(context.Background(), client, req)
		return handlerCalled // exempt methods must always reach the handler
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 50}); err != nil {
		t.Errorf("PBT failed: exempt method rejected for missing tenant: %v", err)
	}
}

func TestPBT_NonWhitelistMethods_AlwaysRejectEmptyTenant(t *testing.T) {
	t.Parallel()
	f := func(seed uint16) bool {
		methods := []string{"agents.list", "chat.send", "sessions.list", "cron.list",
			"skills.update", "teams.list", "config.get", "providers.list"}
		method := methods[int(seed)%len(methods)]

		client := testClient("", "user-1", "en") // no tenant

		handlerCalled := false
		router := &MethodRouter{
			handlers: map[string]MethodHandler{
				method: func(_ context.Context, _ *Client, _ *protocol.RequestFrame) {
					handlerCalled = true
				},
			},
			server: &Server{},
		}

		req := &protocol.RequestFrame{ID: "r1", Method: method}
		router.Handle(context.Background(), client, req)

		if handlerCalled {
			return false // handler should NOT have been called
		}
		resp := <-client.send
		var frame protocol.ResponseFrame
		if err := json.Unmarshal(resp, &frame); err != nil {
			return false
		}
		return frame.Error != nil && frame.Error.Code == protocol.ErrForbidden
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 50}); err != nil {
		t.Errorf("PBT failed: non-exempt method with empty tenant was not rejected: %v", err)
	}
}
