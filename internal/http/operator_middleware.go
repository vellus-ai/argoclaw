package http

import (
	"log/slog"
	"net/http"

	"github.com/vellus-ai/argoclaw/internal/permissions"
	"github.com/vellus-ai/argoclaw/internal/store"
)

// RequireOperatorRole is the exported version of requireOperatorRole for use by server.go.
// It verifies dual-check: operator_level >= 1 AND role >= RoleOperator.
var RequireOperatorRole = requireOperatorRole

// requireOperatorRole verifies dual-check: operator_level >= 1 AND role >= RoleOperator.
// Both conditions are mandatory — neither alone is sufficient.
// appsec: see Requirement 9 — cross-tenant access requires BOTH operator tenant context
// AND sufficient role. A high-role user in a regular tenant is NOT an operator.
func requireOperatorRole(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Check 1: Operator Mode (tenant with operator_level >= 1 authenticated via JWT)
		if !store.IsOperatorMode(ctx) {
			slog.Warn("security.operator_access_denied",
				"reason", "OPERATOR_REQUIRED",
				"tenant_id", store.TenantIDFromContext(ctx).String(),
				"user_id", store.UserIDFromContext(ctx),
				"path", r.URL.Path,
			)
			writeOperatorJSONError(w, http.StatusForbidden,
				"tenant does not have operator access", "OPERATOR_REQUIRED")
			return
		}

		// Check 2: User role must be >= Operator within the operator tenant.
		// resolveAuth uses all available auth sources (gateway token, API key, JWT claims).
		auth := resolveAuth(r, "") // no gateway token check — JWT/API key only
		role := auth.Role
		if !permissions.HasMinRole(role, permissions.RoleOperator) {
			slog.Warn("security.operator_access_denied",
				"reason", "INSUFFICIENT_ROLE",
				"tenant_id", store.TenantIDFromContext(ctx).String(),
				"user_id", store.UserIDFromContext(ctx),
				"role", string(role),
				"path", r.URL.Path,
			)
			writeOperatorJSONError(w, http.StatusForbidden,
				"insufficient role for operator access", "INSUFFICIENT_ROLE")
			return
		}

		next(w, r)
	}
}
