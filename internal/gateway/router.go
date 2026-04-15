package gateway

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/vellus-ai/argoclaw/internal/auth"
	httpapi "github.com/vellus-ai/argoclaw/internal/http"
	"github.com/vellus-ai/argoclaw/internal/i18n"
	"github.com/vellus-ai/argoclaw/internal/permissions"
	"github.com/vellus-ai/argoclaw/internal/store"
	"github.com/vellus-ai/argoclaw/pkg/protocol"
)

// methodsExemptFromAuth lists methods that skip both permission
// and tenant checks (pre-authentication or unauthenticated flows).
var methodsExemptFromAuth = map[string]bool{
	protocol.MethodConnect:              true,
	protocol.MethodHealth:               true,
	protocol.MethodBrowserPairingStatus: true,
}

// MethodHandler processes a single RPC method request.
type MethodHandler func(ctx context.Context, client *Client, req *protocol.RequestFrame)

// MethodRouter maps method names to handlers.
type MethodRouter struct {
	handlers map[string]MethodHandler
	server   *Server
}

func NewMethodRouter(server *Server) *MethodRouter {
	r := &MethodRouter{
		handlers: make(map[string]MethodHandler),
		server:   server,
	}
	r.registerDefaults()
	return r
}

// Register adds a method handler.
func (r *MethodRouter) Register(method string, handler MethodHandler) {
	r.handlers[method] = handler
}

// Handle dispatches a request to the appropriate handler.
func (r *MethodRouter) Handle(ctx context.Context, client *Client, req *protocol.RequestFrame) {
	handler, ok := r.handlers[req.Method]
	if !ok {
		slog.Warn("unknown method", "method", req.Method, "client", client.id)
		locale := i18n.Normalize(client.locale)
		client.SendResponse(protocol.NewErrorResponse(
			req.ID,
			protocol.ErrInvalidRequest,
			i18n.T(locale, i18n.MsgUnknownMethod, req.Method),
		))
		return
	}

	exempt := methodsExemptFromAuth[req.Method]

	// Permission check: skip for exempt methods (pre-auth / unauthenticated)
	if !exempt {
		if pe := r.server.policyEngine; pe != nil {
			if !pe.CanAccess(client.role, req.Method) {
				slog.Warn("permission denied", "method", req.Method, "role", client.role, "client", client.id)
				locale := i18n.Normalize(client.locale)
				client.SendResponse(protocol.NewErrorResponse(
					req.ID,
					protocol.ErrUnauthorized,
					i18n.T(locale, i18n.MsgPermissionDenied, req.Method),
				))
				return
			}
		}
	}

	// Inject locale into context (always)
	ctx = store.WithLocale(ctx, i18n.Normalize(client.locale))

	// Tenant + User isolation: skip for exempt methods
	if !exempt {
		tid, err := uuid.Parse(client.tenantID)
		if err != nil || tid == uuid.Nil {
			if client.tenantID != "" && client.tenantID != uuid.Nil.String() {
				slog.Warn("security.invalid_tenant_id_ws",
					"method", req.Method,
					"client", client.id,
					"tenant_id", client.tenantID,
					"remote_addr", client.remoteAddr,
				)
			}
			locale := i18n.Normalize(client.locale)
			client.SendResponse(protocol.NewErrorResponse(
				req.ID,
				protocol.ErrForbidden,
				i18n.T(locale, i18n.MsgPermissionDenied, req.Method),
			))
			return
		}
		ctx = store.WithTenantID(ctx, tid)
		if client.userID != "" {
			ctx = store.WithUserID(ctx, client.userID)
		}
		// Propagate Operator Mode when the client is an authenticated operator tenant.
		// appsec:cross-tenant-bypass — gated on operatorLevel set during JWT connect
		if client.operatorLevel >= 1 {
			ctx = store.WithCrossTenant(ctx)
			ctx = store.WithOperatorMode(ctx, tid)
		}
	}

	slog.Debug("handling method", "method", req.Method, "client", client.id, "req_id", req.ID)
	handler(ctx, client, req)
}

// registerDefaults registers built-in Phase 1 method handlers.
func (r *MethodRouter) registerDefaults() {
	// System
	r.Register(protocol.MethodConnect, r.handleConnect)
	r.Register(protocol.MethodHealth, r.handleHealth)
	r.Register(protocol.MethodStatus, r.handleStatus)
}

// --- Built-in handlers ---

func (r *MethodRouter) handleConnect(ctx context.Context, client *Client, req *protocol.RequestFrame) {
	// Parse connect params
	var params struct {
		Token    string `json:"token"`
		UserID   string `json:"user_id"`
		SenderID string `json:"sender_id"` // browser pairing: stored sender ID for reconnect
		Locale   string `json:"locale"`    // user's preferred locale (en, vi, zh)
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	// Set locale on client (persists across all requests for this connection)
	client.locale = i18n.Normalize(params.Locale)

	configToken := r.server.cfg.Gateway.Token

	// Path 1: Valid gateway token → admin (constant-time comparison)
	if configToken != "" && subtle.ConstantTimeCompare([]byte(params.Token), []byte(configToken)) == 1 {
		client.role = permissions.RoleAdmin
		client.authenticated = true
		client.userID = params.UserID
		r.sendConnectResponse(client, req.ID)
		return
	}

	// Path 1b: API key → role derived from scopes (uses shared cache)
	if params.Token != "" {
		if keyData, role := httpapi.ResolveAPIKey(ctx, params.Token); keyData != nil {
			scopes := make([]permissions.Scope, len(keyData.Scopes))
			for i, s := range keyData.Scopes {
				scopes[i] = permissions.Scope(s)
			}
			client.role = role
			client.scopes = scopes
			client.authenticated = true
			client.userID = params.UserID
			r.sendConnectResponse(client, req.ID)
			return
		}
	}

	// Path 1c: JWT token → role from claims (email/password auth)
	// JWTs contain dots (header.payload.signature); gateway tokens and API keys don't.
	if params.Token != "" && strings.Contains(params.Token, ".") {
		jwtSecret := r.server.cfg.Gateway.JWTSecret
		if jwtSecret != "" {
			claims, err := auth.ValidateAccessToken(params.Token, jwtSecret)
			if err != nil {
				slog.Warn("security.jwt_ws_auth_failed",
					"client", client.id,
					"error", err,
					"remote_addr", client.remoteAddr)
				// Fail-closed: reject with explicit error, don't fall through to viewer.
				locale := i18n.Normalize(client.locale)
				client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrUnauthorized,
					i18n.T(locale, i18n.MsgUnauthorized)))
				return
			}
			client.role = httpapi.JWTRoleToPermission(claims.Role)
			client.authenticated = true
			client.userID = claims.UserID       // Trust claims over params
			client.tenantID = claims.TenantID   // From JWT only, never params
			client.mustChangePassword = claims.MustChangePassword
			slog.Info("security.jwt_ws_auth_success",
				"client", client.id,
				"user_id", claims.UserID,
				"tenant_id", claims.TenantID,
				"role", claims.Role,
				"remote_addr", client.remoteAddr)

			// Operator Mode: check operator_level from DB — NEVER from JWT claims.
			// appsec: operator_level is write-protected and never exposed in JWT claims.
			if r.server.tenants != nil && claims.TenantID != "" {
				if tid, err := uuid.Parse(claims.TenantID); err == nil {
					// appsec:cross-tenant-bypass — load tenant to check operator_level
					lookupCtx := store.WithCrossTenant(ctx)
					if tenant, err := r.server.tenants.GetByID(lookupCtx, tid); err != nil {
						slog.Warn("security.ws_operator_level_lookup_failed",
							"client", client.id,
							"tenant_id", claims.TenantID,
							"error", err,
						)
						// Non-fatal: continue without operator mode
					} else if tenant != nil && tenant.OperatorLevel >= 1 {
						// appsec:cross-tenant-bypass — tenant with operator_level >= 1 authenticated via JWT WS
						client.operatorLevel = tenant.OperatorLevel
						slog.Info("security.operator_mode_activated_ws",
							"client", client.id,
							"tenant_id", claims.TenantID,
							"operator_level", tenant.OperatorLevel,
						)
					}
				}
			}

			r.sendConnectResponse(client, req.ID)
			return
		}
	}

	// Path 2: No token configured → operator (backward compat)
	// SECURITY: Log warning — production deployments MUST set ARGOCLAW_GATEWAY_TOKEN.
	if configToken == "" {
		slog.Warn("security.no_gateway_token_configured",
			"client", client.id,
			"msg", "gateway running without token — all clients get operator role. Set ARGOCLAW_GATEWAY_TOKEN for production.")
		client.role = permissions.RoleOperator
		client.authenticated = true
		client.userID = params.UserID
		r.sendConnectResponse(client, req.ID)
		return
	}

	// Path 3: Token configured but not provided/wrong → check browser pairing
	ps := r.server.pairingService

	// Path 3a: Reconnecting with a previously-paired sender_id
	if ps != nil && params.SenderID != "" {
		paired, pairErr := ps.IsPaired(params.SenderID, "browser")
		if pairErr != nil {
			slog.Warn("security.pairing_check_failed",
				"sender_id", params.SenderID, "error", pairErr)
			// Fail-closed: deny access on DB error instead of granting operator role.
			locale := i18n.Normalize(client.locale)
			client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal,
				i18n.T(locale, i18n.MsgInternalError, pairErr.Error())))
			return
		}
		if paired {
			client.role = permissions.RoleOperator
			client.authenticated = true
			client.userID = params.UserID
			client.pairedSenderID = params.SenderID
			client.pairedChannel = "browser"
			slog.Info("browser pairing authenticated", "sender_id", params.SenderID, "client", client.id)
			r.sendConnectResponse(client, req.ID)
			return
		}
	}

	// Path 3b: No token, no valid pairing → initiate browser pairing (if service available)
	if ps != nil && params.Token == "" {
		code, err := ps.RequestPairing(client.id, "browser", "", "default", nil)
		if err != nil {
			slog.Warn("browser pairing request failed", "error", err, "client", client.id)
			// Fall through to viewer role
		} else {
			client.pairingCode = code
			client.pairingPending = true
			// Not authenticated — can only call browser.pairing.status
			client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
				"protocol":     protocol.ProtocolVersion,
				"status":       "pending_pairing",
				"pairing_code": code,
				"sender_id":    client.id,
				"server": map[string]any{
					"name":    "argoclaw",
					"version": "0.2.0",
				},
			}))
			return
		}
	}

	// Path 4: Fallback → viewer (wrong token or pairing not available)
	// SECURITY: Log all viewer fallbacks — helps detect brute-force or misconfiguration.
	slog.Warn("security.auth_fallback_to_viewer",
		"client", client.id,
		"token_provided", params.Token != "",
		"sender_id", params.SenderID)
	client.role = permissions.RoleViewer
	client.authenticated = true
	client.userID = params.UserID
	r.sendConnectResponse(client, req.ID)
}

func (r *MethodRouter) sendConnectResponse(client *Client, reqID string) {
	payload := map[string]any{
		"protocol": protocol.ProtocolVersion,
		"role":     string(client.role),
		"user_id":  client.userID,
		"server": map[string]any{
			"name":    "argoclaw",
			"version": "0.2.0",
		},
	}
	if client.tenantID != "" {
		payload["tenant_id"] = client.tenantID
	}
	if client.mustChangePassword {
		payload["must_change_password"] = true
	}
	client.SendResponse(protocol.NewOKResponse(reqID, payload))
}

func (r *MethodRouter) handleHealth(ctx context.Context, client *Client, req *protocol.RequestFrame) {
	s := r.server
	uptimeMs := time.Since(s.startedAt).Milliseconds()

	mode := "managed"

	// Database status (real ping)
	dbStatus := "n/a"
	if s.db != nil {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if err := s.db.PingContext(pingCtx); err != nil {
			dbStatus = "error"
		} else {
			dbStatus = "ok"
		}
	}

	// Connected clients list
	type clientInfo struct {
		ID          string `json:"id"`
		RemoteAddr  string `json:"remoteAddr"`
		UserID      string `json:"userId"`
		Role        string `json:"role"`
		ConnectedAt string `json:"connectedAt"`
	}
	clients := s.ClientList()
	clientList := make([]clientInfo, 0, len(clients))
	for _, c := range clients {
		clientList = append(clientList, clientInfo{
			ID:          c.ID(),
			RemoteAddr:  c.RemoteAddr(),
			UserID:      c.UserID(),
			Role:        string(c.Role()),
			ConnectedAt: c.ConnectedAt().UTC().Format(time.RFC3339),
		})
	}

	// Tool count
	toolCount := 0
	if s.tools != nil {
		toolCount = s.tools.Count()
	}

	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
		"status":    "ok",
		"version":   s.version,
		"uptime":    uptimeMs,
		"mode":      mode,
		"database":  dbStatus,
		"tools":     toolCount,
		"clients":   clientList,
		"currentId": client.ID(),
	}))
}

func (r *MethodRouter) handleStatus(ctx context.Context, client *Client, req *protocol.RequestFrame) {
	agents := r.server.agents.ListInfo()

	sessionCount := 0
	if r.server.sessions != nil {
		sessionCount = len(r.server.sessions.List(ctx, ""))
	}

	// Agents are lazily resolved — router only has loaded agents.
	// Query the DB store for the real total count.
	agentTotal := len(agents)
	if r.server.agentStore != nil {
		if dbAgents, err := r.server.agentStore.List(ctx, ""); err == nil && len(dbAgents) > agentTotal {
			agentTotal = len(dbAgents)
		}
	}

	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
		"agents":     agents,
		"agentTotal": agentTotal,
		"clients":    len(r.server.clients),
		"sessions":   sessionCount,
	}))
}
