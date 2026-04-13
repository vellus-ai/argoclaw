package http

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/vellus-ai/argoclaw/internal/tools"
)

// allowedOnboardingTools is the whitelist of tools callable via the onboarding endpoint.
// Any tool not in this list is rejected with 403.
var allowedOnboardingTools = map[string]bool{
	"configure_workspace":    true,
	"set_branding":           true,
	"configure_llm_provider": true,
	"test_llm_connection":    true,
	"configure_channel":      true,
	"complete_onboarding":    true,
	"get_onboarding_status":  true,
}

// OnboardingHandler provides HTTP endpoints for the conversational onboarding flow.
// Uses HTTP (not WebSocket) to avoid dependency on WS connection during initial setup.
type OnboardingHandler struct {
	store     tools.OnboardingStore
	registry  *tools.Registry
	token     string // gateway token for auth fallback
	jwtSecret string // JWT signing key
}

// NewOnboardingHandler creates a new onboarding HTTP handler.
func NewOnboardingHandler(store tools.OnboardingStore, registry *tools.Registry, token, jwtSecret string) *OnboardingHandler {
	return &OnboardingHandler{
		store:     store,
		registry:  registry,
		token:     token,
		jwtSecret: jwtSecret,
	}
}

// RegisterRoutes registers onboarding endpoints on the HTTP mux.
func (h *OnboardingHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/onboarding/status", h.authMiddleware(h.handleGetStatus))
	mux.HandleFunc("POST /v1/onboarding/action", h.authMiddleware(h.handleAction))
}

func (h *OnboardingHandler) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return requireAuth(h.token, "", next)
}

// handleGetStatus returns the onboarding status for the authenticated tenant.
func (h *OnboardingHandler) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	tenantID := TenantIDFromContext(r.Context())
	if tenantID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "tenant context not available"})
		return
	}

	status, err := h.store.GetOnboardingStatus(r.Context(), tenantID)
	if err != nil {
		slog.Error("onboarding.get_status", "error", err, "tenant_id", tenantID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get onboarding status"})
		return
	}

	writeJSON(w, http.StatusOK, status)
}

// onboardingActionRequest is the request body for POST /v1/onboarding/action.
type onboardingActionRequest struct {
	Tool           string         `json:"tool"`
	Args           map[string]any `json:"args"`
	CompletedState string         `json:"completed_state,omitempty"`
}

// handleAction executes an onboarding tool with the provided args.
func (h *OnboardingHandler) handleAction(w http.ResponseWriter, r *http.Request) {
	tenantID := TenantIDFromContext(r.Context())
	if tenantID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "tenant context not available"})
		return
	}

	var req onboardingActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if req.Tool == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "tool is required"})
		return
	}

	// Security: whitelist validation
	if !allowedOnboardingTools[req.Tool] {
		slog.Warn("security.onboarding_tool_blocked", "tool", req.Tool, "tenant_id", tenantID)
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "tool not allowed"})
		return
	}

	// Execute the tool via registry (tools handle their own input validation)
	args := req.Args
	if args == nil {
		args = make(map[string]any)
	}

	result := h.registry.Execute(r.Context(), req.Tool, args)

	// Persist last completed state if provided and tool succeeded
	if req.CompletedState != "" && !result.IsError {
		if err := h.store.UpdateLastCompletedState(r.Context(), tenantID, req.CompletedState); err != nil {
			slog.Error("onboarding.update_state", "error", err, "tenant_id", tenantID, "state", req.CompletedState)
			// Non-fatal — don't fail the tool call
		}
	}

	if result.IsError {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":    false,
			"error": result.ForLLM,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"result": result.ForLLM,
	})
}
