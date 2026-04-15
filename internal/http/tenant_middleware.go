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
// When the tenant has operator_level >= 1, also sets WithCrossTenant and WithOperatorMode
// so that downstream handlers can call operator endpoints.
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

		// Operator Mode: load tenant from DB to check operator_level.
		// Only performed when a TenantStore is configured (not in gateway-token-only mode).
		if m.tenants != nil {
			// appsec:cross-tenant-bypass — tenants.GetByID lookup to check operator_level
			lookupCtx := store.WithCrossTenant(ctx)
			if tenant, err := m.tenants.GetByID(lookupCtx, tenantID); err != nil {
				slog.Warn("security.tenant_lookup_failed",
					"tenant_id", tenantID,
					"error", err,
				)
				// Non-fatal: continue without operator mode; tenant_id is still set
			} else if tenant != nil && tenant.OperatorLevel >= 1 {
				// appsec:cross-tenant-bypass — tenant with operator_level >= 1 authenticated via JWT HTTP
				ctx = store.WithCrossTenant(ctx)
				ctx = store.WithOperatorMode(ctx, tenantID)
				slog.Info("security.operator_mode_activated_http",
					"tenant_id", tenantID,
					"operator_level", tenant.OperatorLevel,
				)
			}
		}

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
