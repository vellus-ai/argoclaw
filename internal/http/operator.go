package http

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/vellus-ai/argoclaw/internal/store"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// OperatorHandler exposes cross-tenant management endpoints for Vellus operators.
// All routes are protected by requireOperatorRole (operator_level >= 1 + role >= Operator).
//
// appsec: every handler emits a structured audit log entry with the operator tenant ID,
// the target tenant ID, the endpoint, and the requesting user ID.
type OperatorHandler struct {
	tenants store.TenantStore
	db      *sql.DB // optional — used for usage queries (may be nil)
}

// NewOperatorHandler creates an OperatorHandler.
// db may be nil — usage endpoint will return empty counts when db is not provided.
func NewOperatorHandler(tenants store.TenantStore, db *sql.DB) *OperatorHandler {
	return &OperatorHandler{tenants: tenants, db: db}
}

// RegisterRoutes registers all operator API endpoints on mux, wrapping each with requireOp.
func (h *OperatorHandler) RegisterRoutes(mux *http.ServeMux, requireOp func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("GET /v1/operator/tenants", requireOp(h.listTenants))
	mux.HandleFunc("GET /v1/operator/tenants/{id}/agents", requireOp(h.listTenantAgents))
	mux.HandleFunc("GET /v1/operator/tenants/{id}/sessions", requireOp(h.listTenantSessions))
	mux.HandleFunc("GET /v1/operator/tenants/{id}/usage", requireOp(h.getTenantUsage))
}

// writeOperatorJSONError writes a structured JSON error with a machine-readable code.
// Follows the project's error response format: {"error": "...", "code": "..."}.
func writeOperatorJSONError(w http.ResponseWriter, status int, message, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
		"error": message,
		"code":  code,
	})
}

// parsePagination extracts and validates limit/offset from query params.
// Defaults: limit=20, offset=0. Limits: 1 <= limit <= 100.
func parsePagination(r *http.Request) (limit, offset int) {
	limit = 20
	offset = 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	return limit, offset
}

// operatorTracer is the OTel tracer for operator endpoints.
var operatorTracer = otel.Tracer("argoclaw/operator")

// auditOperatorAccess emits a structured audit log for every operator API request
// and starts an OTel span with operator.mode = true for tracing.
// This is the primary accountability mechanism for cross-tenant access.
func auditOperatorAccess(r *http.Request, targetTenantID string) (context.Context, trace.Span) {
	ctx := r.Context()
	operatorTenantID := store.OperatorModeFromContext(ctx)

	slog.Info("operator.access",
		"operator_tenant_id", operatorTenantID.String(),
		"target_tenant_id", targetTenantID,
		"endpoint", r.URL.Path,
		"method", r.Method,
		"user_id", store.UserIDFromContext(ctx),
	)

	ctx, span := operatorTracer.Start(ctx, "operator."+r.URL.Path,
		trace.WithSpanKind(trace.SpanKindServer))
	span.SetAttributes(
		attribute.Bool("operator.mode", true),
		attribute.String("operator.tenant_id", operatorTenantID.String()),
		attribute.String("operator.target_tenant_id", targetTenantID),
		attribute.String("operator.endpoint", r.URL.Path),
	)

	return ctx, span
}

// listTenants returns a paginated list of all tenants.
//
// GET /v1/operator/tenants?limit=N&offset=M
// Response: {"data": [...], "total": N, "limit": N, "offset": N}
func (h *OperatorHandler) listTenants(w http.ResponseWriter, r *http.Request) {
	ctx, span := auditOperatorAccess(r, "all")
	defer span.End()

	limit, offset := parsePagination(r)

	// appsec:cross-tenant-bypass — context verified by requireOperatorRole
	opCtx := store.WithCrossTenant(ctx)

	tenants, total, err := h.tenants.ListAllTenantsForOperator(opCtx, limit, offset)
	if err != nil {
		slog.Error("operator.list_tenants failed", "error", err)
		writeOperatorJSONError(w, http.StatusInternalServerError, "internal server error", "INTERNAL_ERROR")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data":   tenants,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// listTenantAgents returns a paginated list of agents for a specific tenant.
//
// GET /v1/operator/tenants/{id}/agents?limit=N&offset=M
func (h *OperatorHandler) listTenantAgents(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.parseTenantID(w, r)
	if !ok {
		return
	}

	_, span := auditOperatorAccess(r, tenantID.String())
	defer span.End()

	if _, ok := h.checkTenantExists(w, r, tenantID); !ok {
		return
	}

	// appsec:cross-tenant-bypass — operator requesting agents for a specific tenant
	opCtx := store.WithCrossTenant(store.WithTenantID(r.Context(), tenantID))
	limit, offset := parsePagination(r)

	agents, total, err := h.queryAgents(opCtx, tenantID, limit, offset)
	if err != nil {
		slog.Error("operator.list_tenant_agents failed", "tenant_id", tenantID, "error", err)
		writeOperatorJSONError(w, http.StatusInternalServerError, "internal server error", "INTERNAL_ERROR")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data":   agents,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// listTenantSessions returns a paginated list of sessions for a specific tenant.
//
// GET /v1/operator/tenants/{id}/sessions?limit=N&offset=M
func (h *OperatorHandler) listTenantSessions(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.parseTenantID(w, r)
	if !ok {
		return
	}

	_, span := auditOperatorAccess(r, tenantID.String())
	defer span.End()

	if _, ok := h.checkTenantExists(w, r, tenantID); !ok {
		return
	}

	// appsec:cross-tenant-bypass — operator requesting sessions for a specific tenant
	opCtx := store.WithCrossTenant(store.WithTenantID(r.Context(), tenantID))
	limit, offset := parsePagination(r)
	status := r.URL.Query().Get("status") // optional status filter

	sessions, total, err := h.querySessions(opCtx, tenantID, limit, offset, status)
	if err != nil {
		slog.Error("operator.list_tenant_sessions failed", "tenant_id", tenantID, "error", err)
		writeOperatorJSONError(w, http.StatusInternalServerError, "internal server error", "INTERNAL_ERROR")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data":   sessions,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// TenantUsage holds usage metrics for a tenant over a period.
// Matches the TenantUsageStatsResponse from the design spec.
type TenantUsage struct {
	TenantID     uuid.UUID `json:"tenant_id"`
	Period       string    `json:"period"`
	MessageCount int64     `json:"message_count"`
	ToolCallCount int64   `json:"tool_call_count"`
	SessionCount int64     `json:"session_count"`
	TokensUsed   int64     `json:"tokens_used"`
}

// getTenantUsage returns usage metrics for a specific tenant.
//
// GET /v1/operator/tenants/{id}/usage?period=7d|30d|90d
func (h *OperatorHandler) getTenantUsage(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.parseTenantID(w, r)
	if !ok {
		return
	}

	_, span := auditOperatorAccess(r, tenantID.String())
	defer span.End()

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "30d"
	}

	var days int
	switch period {
	case "7d":
		days = 7
	case "30d":
		days = 30
	case "90d":
		days = 90
	default:
		writeOperatorJSONError(w, http.StatusBadRequest,
			"invalid period: use 7d, 30d, or 90d", "INVALID_PERIOD")
		return
	}

	if _, ok := h.checkTenantExists(w, r, tenantID); !ok {
		return
	}

	usage := h.queryUsage(r.Context(), tenantID, period, days)
	writeJSON(w, http.StatusOK, usage)
}

// --- helpers ---

// parseTenantID extracts and validates the {id} path parameter as a UUID.
func (h *OperatorHandler) parseTenantID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	raw := r.PathValue("id")
	id, err := uuid.Parse(raw)
	if err != nil {
		writeOperatorJSONError(w, http.StatusBadRequest, "invalid tenant UUID", "INVALID_UUID")
		return uuid.Nil, false
	}
	return id, true
}

// checkTenantExists verifies the tenant exists, writing 404 and returning (nil, false) if not.
func (h *OperatorHandler) checkTenantExists(w http.ResponseWriter, r *http.Request, tenantID uuid.UUID) (*store.Tenant, bool) {
	// appsec:cross-tenant-bypass — operator verifying existence of a target tenant by ID
	lookupCtx := store.WithCrossTenant(r.Context())
	tenant, err := h.tenants.GetByID(lookupCtx, tenantID)
	if err != nil {
		slog.Error("operator.get_tenant failed", "tenant_id", tenantID, "error", err)
		writeOperatorJSONError(w, http.StatusInternalServerError, "internal server error", "INTERNAL_ERROR")
		return nil, false
	}
	if tenant == nil {
		writeOperatorJSONError(w, http.StatusNotFound, "tenant not found", "TENANT_NOT_FOUND")
		return nil, false
	}
	return tenant, true
}

// queryAgents returns agents for a tenant from the database.
// Returns empty results when db is nil (test/lite mode).
func (h *OperatorHandler) queryAgents(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]map[string]any, int, error) {
	if h.db == nil {
		return []map[string]any{}, 0, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	rows, err := h.db.QueryContext(ctx, `
		SELECT id, agent_key, display_name, status, created_at,
		       COUNT(*) OVER() AS total_count
		FROM agents
		WHERE tenant_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`,
		tenantID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := []map[string]any{} // initialize as empty slice (not nil) — nil marshals to JSON null
	var total int
	for rows.Next() {
		var id uuid.UUID
		var agentKey, displayName, status string
		var createdAt time.Time
		if err := rows.Scan(&id, &agentKey, &displayName, &status, &createdAt, &total); err != nil {
			return nil, 0, err
		}
		result = append(result, map[string]any{
			"id":           id,
			"agent_key":    agentKey,
			"display_name": displayName,
			"status":       status,
			"created_at":   createdAt,
		})
	}
	return result, total, rows.Err()
}

// querySessions returns sessions for a tenant from the database.
// Returns empty results when db is nil.
// status is an optional filter — when non-empty, only sessions with matching status are returned.
func (h *OperatorHandler) querySessions(ctx context.Context, tenantID uuid.UUID, limit, offset int, status string) ([]map[string]any, int, error) {
	if h.db == nil {
		return []map[string]any{}, 0, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var rows *sql.Rows
	var err error

	if status != "" {
		rows, err = h.db.QueryContext(ctx, `
			SELECT id, agent_id, user_id, label, channel, created_at,
			       COUNT(*) OVER() AS total_count
			FROM sessions
			WHERE tenant_id = $1 AND status = $2
			ORDER BY created_at DESC
			LIMIT $3 OFFSET $4`,
			tenantID, status, limit, offset)
	} else {
		rows, err = h.db.QueryContext(ctx, `
			SELECT id, agent_id, user_id, label, channel, created_at,
			       COUNT(*) OVER() AS total_count
			FROM sessions
			WHERE tenant_id = $1
			ORDER BY created_at DESC
			LIMIT $2 OFFSET $3`,
			tenantID, limit, offset)
	}
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := []map[string]any{} // initialize as empty slice (not nil) — nil marshals to JSON null
	var total int
	for rows.Next() {
		var id, agentID, userID, label, channel string
		var createdAt time.Time
		if err := rows.Scan(&id, &agentID, &userID, &label, &channel, &createdAt, &total); err != nil {
			return nil, 0, err
		}
		result = append(result, map[string]any{
			"id":         id,
			"agent_id":   agentID,
			"user_id":    userID,
			"label":      label,
			"channel":    channel,
			"created_at": createdAt,
		})
	}
	return result, total, rows.Err()
}

// queryUsage returns aggregated usage metrics for a tenant over a period.
// Returns zeros when db is nil.
func (h *OperatorHandler) queryUsage(ctx context.Context, tenantID uuid.UUID, period string, days int) TenantUsage {
	usage := TenantUsage{TenantID: tenantID, Period: period}
	if h.db == nil {
		return usage
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	since := time.Now().UTC().AddDate(0, 0, -days)

	// Session count within period
	h.db.QueryRowContext(ctx, //nolint:errcheck
		`SELECT COUNT(*) FROM sessions WHERE tenant_id = $1 AND created_at >= $2`,
		tenantID, since).Scan(&usage.SessionCount)

	// Message count within period (from traces table — each root trace is a message)
	h.db.QueryRowContext(ctx, //nolint:errcheck
		`SELECT COUNT(*) FROM traces WHERE tenant_id = $1 AND start_time >= $2 AND parent_trace_id IS NULL`,
		tenantID, since).Scan(&usage.MessageCount)

	// Tool call count within period (traces with tool_name set)
	h.db.QueryRowContext(ctx, //nolint:errcheck
		`SELECT COUNT(*) FROM traces WHERE tenant_id = $1 AND start_time >= $2 AND tool_name IS NOT NULL AND tool_name != ''`,
		tenantID, since).Scan(&usage.ToolCallCount)

	// Tokens used within period (sum of input + output tokens from traces)
	h.db.QueryRowContext(ctx, //nolint:errcheck
		`SELECT COALESCE(SUM(COALESCE(input_tokens, 0) + COALESCE(output_tokens, 0)), 0) FROM traces WHERE tenant_id = $1 AND start_time >= $2`,
		tenantID, since).Scan(&usage.TokensUsed)

	return usage
}
