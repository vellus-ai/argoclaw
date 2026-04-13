package gateway

import (
	"context"
	"encoding/json"
	"testing"

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
