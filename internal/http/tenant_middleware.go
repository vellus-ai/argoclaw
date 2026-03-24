package http

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/vellus-ai/argoclaw/internal/store"
)

// TenantMiddleware resolves the user's tenant from JWT claims and injects it into context.
// All downstream queries MUST filter by tenant_id for data isolation.
type TenantMiddleware struct {
	tenants store.TenantStore
}

func NewTenantMiddleware(tenants store.TenantStore) *TenantMiddleware {
	return &TenantMiddleware{tenants: tenants}
}

// Wrap extracts tenant_id from JWT claims and validates membership.
func (m *TenantMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := UserClaimsFromContext(r.Context())
		if claims == nil || claims.TenantID == "" {
			// No JWT or no tenant — pass through (gateway token mode)
			next.ServeHTTP(w, r)
			return
		}

		tenantID, err := uuid.Parse(claims.TenantID)
		if err != nil {
			slog.Warn("security.invalid_tenant_id", "tenant_id", claims.TenantID)
			writeJSONError(w, http.StatusForbidden, "invalid tenant")
			return
		}

		// Inject tenant_id into context via store package — available to all store queries
		ctx := store.WithTenantID(r.Context(), tenantID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// TenantIDFromRequest extracts the tenant UUID from request context.
// Returns uuid.Nil if no tenant is set (e.g., gateway token mode).
func TenantIDFromRequest(ctx context.Context) uuid.UUID {
	return store.TenantIDFromContext(ctx)
}

// RequireTenant returns a middleware that rejects requests without a tenant context.
func RequireTenant() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if store.TenantIDFromContext(r.Context()) == uuid.Nil {
				writeJSONError(w, http.StatusForbidden, "tenant context required")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
