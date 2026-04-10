package http

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/vellus-ai/argoclaw/internal/bus"
	"github.com/vellus-ai/argoclaw/internal/crypto"
	"github.com/vellus-ai/argoclaw/internal/i18n"
	"github.com/vellus-ai/argoclaw/internal/permissions"
	"github.com/vellus-ai/argoclaw/internal/store"
)

// extractBearerToken extracts a bearer token from the Authorization header.
func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	if !strings.HasPrefix(auth, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(auth, "Bearer ")
}

// tokenMatch performs a constant-time comparison of a provided token against the expected token.
func tokenMatch(provided, expected string) bool {
	return subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}

// extractUserID extracts the external user ID from the request header.
// Returns "" if no user ID is provided (anonymous).
// Rejects IDs exceeding MaxUserIDLength (VARCHAR(255) DB constraint).
func extractUserID(r *http.Request) string {
	id := r.Header.Get("X-ArgoClaw-User-Id")
	if id == "" {
		return ""
	}
	if err := store.ValidateUserID(id); err != nil {
		slog.Warn("security.user_id_too_long", "length", len(id), "max", store.MaxUserIDLength)
		return ""
	}
	return id
}

// extractAgentID determines the target agent from the request.
// Checks model field, headers, and falls back to "default".
func extractAgentID(r *http.Request, model string) string {
	// From model field: "argoclaw:<agentId>" or "agent:<agentId>"
	if after, ok := strings.CutPrefix(model, "argoclaw:"); ok {
		return after
	}
	if after, ok := strings.CutPrefix(model, "agent:"); ok {
		return after
	}

	// From headers
	if id := r.Header.Get("X-ArgoClaw-Agent-Id"); id != "" {
		return id
	}
	if id := r.Header.Get("X-ArgoClaw-Agent"); id != "" {
		return id
	}

	return "default"
}

// --- Package-level API key cache for shared auth ---

var pkgAPIKeyCache *apiKeyCache
var pkgPairingStore store.PairingStore

// InitAPIKeyCache initializes the shared API key cache with TTL and pubsub invalidation.
// Must be called once during server startup before handling requests.
func InitAPIKeyCache(s store.APIKeyStore, mb *bus.MessageBus) {
	pkgAPIKeyCache = newAPIKeyCache(s, 5*time.Minute)
	if mb != nil {
		mb.Subscribe("http-api-key-cache", func(e bus.Event) {
			if p, ok := e.Payload.(bus.CacheInvalidatePayload); ok && p.Kind == bus.CacheKindAPIKeys {
				pkgAPIKeyCache.invalidateAll()
			}
		})
	}
}

// InitPairingAuth sets the pairing store for HTTP auth.
// Allows browser-paired users to access HTTP APIs via X-ArgoClaw-Sender-Id header.
func InitPairingAuth(ps store.PairingStore) {
	pkgPairingStore = ps
}

// ResolveAPIKey checks if the bearer token is a valid API key using the shared cache.
// Returns the key data and derived role, or nil if not found/expired/revoked.
func ResolveAPIKey(ctx context.Context, token string) (*store.APIKeyData, permissions.Role) {
	if pkgAPIKeyCache == nil || token == "" {
		return nil, ""
	}
	hash := crypto.HashAPIKey(token)
	return pkgAPIKeyCache.getOrFetch(ctx, hash)
}

// authResult holds the resolved authentication state for an HTTP request.
type authResult struct {
	Role          permissions.Role
	Authenticated bool
}

// resolveAuth determines the caller's role from the request.
// Priority: gateway token → API key → browser pairing → JWT claims.
func resolveAuth(r *http.Request, gatewayToken string) authResult {
	return resolveAuthBearer(r, gatewayToken, extractBearerToken(r))
}

// resolveAuthBearer is like resolveAuth but accepts a pre-extracted bearer token.
// Useful for handlers that also accept tokens from query params.
func resolveAuthBearer(r *http.Request, gatewayToken, bearer string) authResult {
	// Gateway token → admin
	if gatewayToken != "" && tokenMatch(bearer, gatewayToken) {
		return authResult{Role: permissions.RoleAdmin, Authenticated: true}
	}
	// API key → role from scopes
	if _, role := ResolveAPIKey(r.Context(), bearer); role != "" {
		return authResult{Role: role, Authenticated: true}
	}
	// Browser pairing → operator (via X-ArgoClaw-Sender-Id header)
	if senderID := r.Header.Get("X-ArgoClaw-Sender-Id"); senderID != "" && pkgPairingStore != nil {
		paired, err := pkgPairingStore.IsPaired(senderID, "browser")
		if err == nil && paired {
			return authResult{Role: permissions.RoleOperator, Authenticated: true}
		}
		if err != nil {
			slog.Warn("security.http_pairing_check_failed", "sender_id", senderID, "error", err)
		} else {
			slog.Warn("security.http_pairing_auth_failed", "sender_id", senderID, "ip", r.RemoteAddr)
		}
	}
	// JWT claims → role from token (injected by JWTMiddleware)
	if claims := UserClaimsFromContext(r.Context()); claims != nil {
		return authResult{Role: jwtRoleToPermission(claims.Role), Authenticated: true}
	}
	return authResult{}
}

// jwtRoleToPermission maps JWT user roles to system permission roles.
func jwtRoleToPermission(role string) permissions.Role {
	switch role {
	case "owner", "admin":
		return permissions.RoleAdmin
	case "member", "operator":
		return permissions.RoleOperator
	default:
		return permissions.RoleViewer
	}
}

// httpMinRole returns the minimum role required for an HTTP endpoint based on HTTP method.
func httpMinRole(method string) permissions.Role {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return permissions.RoleViewer
	default: // POST, PUT, PATCH, DELETE
		return permissions.RoleOperator
	}
}

// injectJWTContext enriches the context with tenant_id and user_id from JWT claims
// when present. Used by both requireAuth middleware and handlers that call
// resolveAuth directly.
func injectJWTContext(ctx context.Context, r *http.Request) context.Context {
	claims := UserClaimsFromContext(r.Context())
	if claims == nil {
		return ctx
	}
	if claims.TenantID != "" {
		ctx = WithTenantID(ctx, claims.TenantID)
	}
	if claims.UserID != "" {
		ctx = store.WithUserID(ctx, claims.UserID)
	}
	return ctx
}

// requireAuth is a middleware that checks authentication and minimum role.
// Pass "" for minRole to auto-detect from HTTP method (GET→Viewer, POST→Operator).
// Injects locale and userID into request context.
func requireAuth(token string, minRole permissions.Role, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		locale := extractLocale(r)
		auth := resolveAuth(r, token)

		if !auth.Authenticated {
			writeJSON(w, http.StatusUnauthorized, map[string]string{
				"error": i18n.T(locale, i18n.MsgUnauthorized),
			})
			return
		}

		required := minRole
		if required == "" {
			required = httpMinRole(r.Method)
		}

		if !permissions.HasMinRole(auth.Role, required) {
			writeJSON(w, http.StatusForbidden, map[string]string{
				"error": i18n.T(locale, i18n.MsgPermissionDenied, r.URL.Path),
			})
			return
		}

		ctx := store.WithLocale(r.Context(), locale)
		if userID := extractUserID(r); userID != "" {
			ctx = store.WithUserID(ctx, userID)
		}
		// Inject tenant isolation from JWT claims so stores filter by tenant.
		ctx = injectJWTContext(ctx, r)
		next(w, r.WithContext(ctx))
	}
}

// requireAuthBearer is like requireAuth but accepts a pre-extracted bearer token.
// Used by handlers that accept tokens from query params (files, media).
// On success, enriches r's context with JWT tenant/user context and returns the
// updated request alongside true. Callers MUST use the returned *http.Request.
func requireAuthBearer(token string, minRole permissions.Role, bearer string, w http.ResponseWriter, r *http.Request) (*http.Request, bool) {
	locale := extractLocale(r)
	auth := resolveAuthBearer(r, token, bearer)

	if !auth.Authenticated {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": i18n.T(locale, i18n.MsgUnauthorized),
		})
		return r, false
	}

	required := minRole
	if required == "" {
		required = httpMinRole(r.Method)
	}

	if !permissions.HasMinRole(auth.Role, required) {
		writeJSON(w, http.StatusForbidden, map[string]string{
			"error": i18n.T(locale, i18n.MsgPermissionDenied, r.URL.Path),
		})
		return r, false
	}

	// Inject tenant isolation from JWT claims so stores filter by tenant.
	ctx := injectJWTContext(r.Context(), r)
	return r.WithContext(ctx), true
}

// extractLocale parses the Accept-Language header and returns a supported locale.
// Falls back to "en" if no supported language is found.
func extractLocale(r *http.Request) string {
	accept := r.Header.Get("Accept-Language")
	if accept == "" {
		return i18n.DefaultLocale
	}
	// Simple parser: take the first language tag before comma or semicolon
	for part := range strings.SplitSeq(accept, ",") {
		tag := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
		locale := i18n.Normalize(tag)
		if locale != i18n.DefaultLocale || strings.HasPrefix(tag, "en") {
			return locale
		}
	}
	return i18n.DefaultLocale
}
