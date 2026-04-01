package http

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/vellus-ai/argoclaw/internal/auth"
	"github.com/vellus-ai/argoclaw/internal/store"
)

type contextKey string

const (
	ctxKeyUserClaims contextKey = "user_claims"
)

// JWTMiddleware validates JWT bearer tokens and injects claims into context.
// Falls through to next handler if no JWT is present (for backward compat with gateway token).
type JWTMiddleware struct {
	jwtSecret string
}

func NewJWTMiddleware(jwtSecret string) *JWTMiddleware {
	return &JWTMiddleware{jwtSecret: jwtSecret}
}

// Wrap returns a middleware that extracts and validates JWT tokens.
// If a valid JWT is found, it sets X-ArgoClaw-User-Id header for downstream compatibility
// and stores claims in context. If no JWT, falls through (gateway token auth still works).
func (m *JWTMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			next.ServeHTTP(w, r)
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")

		// Skip if it looks like a gateway token (not a JWT — no dots)
		if !strings.Contains(token, ".") {
			next.ServeHTTP(w, r)
			return
		}

		claims, err := auth.ValidateAccessToken(token, m.jwtSecret)
		if err != nil {
			slog.Debug("jwt_middleware: invalid token", "error", err)
			writeJSONError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		// Inject user ID for backward compatibility with existing handlers.
		r.Header.Set("X-ArgoClaw-User-Id", claims.UserID)

		// Store claims in context for handlers that need tenant/role info.
		ctx := context.WithValue(r.Context(), ctxKeyUserClaims, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// UserClaimsFromContext extracts JWT claims from request context.
func UserClaimsFromContext(ctx context.Context) *auth.TokenClaims {
	claims, _ := ctx.Value(ctxKeyUserClaims).(*auth.TokenClaims)
	return claims
}

// RequireAuth returns a middleware that rejects requests without a valid JWT.
// Use this for endpoints that MUST have user auth (not gateway token).
func RequireUserAuth(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := UserClaimsFromContext(r.Context())
			if claims == nil {
				writeJSONError(w, http.StatusUnauthorized, "authentication required")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// --- Helper to extract tenant context ---

// TenantIDFromContext returns the tenant ID from JWT claims, if available.
func TenantIDFromContext(ctx context.Context) string {
	claims := UserClaimsFromContext(ctx)
	if claims == nil {
		return ""
	}
	return claims.TenantID
}

// WithTenantID adds tenant isolation to store context.
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	if tenantID == "" {
		return ctx
	}
	id, err := uuid.Parse(tenantID)
	if err != nil {
		slog.Warn("security.invalid_tenant_id", "tenant_id", tenantID, "error", err)
		return ctx
	}
	return store.WithTenantID(ctx, id)
}
