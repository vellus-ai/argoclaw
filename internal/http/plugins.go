package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/vellus-ai/argoclaw/internal/bus"
	"github.com/vellus-ai/argoclaw/internal/i18n"
	"github.com/vellus-ai/argoclaw/internal/permissions"
	"github.com/vellus-ai/argoclaw/internal/plugins"
	"github.com/vellus-ai/argoclaw/internal/store"
)

// PluginHandler handles plugin management HTTP endpoints.
type PluginHandler struct {
	store    store.PluginStore
	token    string
	msgBus   *bus.MessageBus    // optional
	tenantMw *TenantMiddleware  // injects tenant_id from JWT into context
}

// NewPluginHandler creates a handler for plugin management endpoints.
func NewPluginHandler(s store.PluginStore, token string, msgBus *bus.MessageBus, tenantMw *TenantMiddleware) *PluginHandler {
	return &PluginHandler{store: s, token: token, msgBus: msgBus, tenantMw: tenantMw}
}

// RegisterRoutes registers all plugin management routes on the given mux.
// All routes are wrapped with TenantMiddleware to ensure tenant_id is injected for JWT users.
func (h *PluginHandler) RegisterRoutes(mux *http.ServeMux) {
	// Catalog — POST requires admin role
	mux.HandleFunc("GET /v1/plugins/catalog", h.withTenant(h.auth(h.handleListCatalog)))
	mux.HandleFunc("POST /v1/plugins/catalog", h.withTenant(h.authAdmin(h.handleCreateCatalogEntry)))
	mux.HandleFunc("GET /v1/plugins/catalog/{id}", h.withTenant(h.auth(h.handleGetCatalogEntry)))

	// Installed plugins (tenant-scoped)
	mux.HandleFunc("GET /v1/plugins/installed", h.withTenant(h.auth(h.handleListInstalled)))
	mux.HandleFunc("POST /v1/plugins/install", h.withTenant(h.auth(h.handleInstallPlugin)))
	mux.HandleFunc("DELETE /v1/plugins/installed/{name}", h.withTenant(h.auth(h.handleUninstallPlugin)))
	mux.HandleFunc("PUT /v1/plugins/installed/{name}", h.withTenant(h.auth(h.handleUpdatePluginConfig)))
	mux.HandleFunc("POST /v1/plugins/installed/{name}/enable", h.withTenant(h.auth(h.handleEnablePlugin)))
	mux.HandleFunc("POST /v1/plugins/installed/{name}/disable", h.withTenant(h.auth(h.handleDisablePlugin)))
	mux.HandleFunc("GET /v1/plugins/installed/{name}/audit", h.withTenant(h.auth(h.handleAuditLog)))

	// Agent plugin grants
	mux.HandleFunc("POST /v1/plugins/agents/{agentID}/grant", h.withTenant(h.auth(h.handleGrantAgent)))
	mux.HandleFunc("DELETE /v1/plugins/agents/{agentID}/{pluginName}", h.withTenant(h.auth(h.handleRevokeAgent)))
}

func (h *PluginHandler) auth(next http.HandlerFunc) http.HandlerFunc {
	return requireAuth(h.token, "", next)
}

func (h *PluginHandler) authAdmin(next http.HandlerFunc) http.HandlerFunc {
	return requireAuth(h.token, permissions.RoleAdmin, next)
}

// withTenant wraps a handler with TenantMiddleware to inject tenant_id from JWT into context.
func (h *PluginHandler) withTenant(next http.HandlerFunc) http.HandlerFunc {
	if h.tenantMw == nil {
		return next
	}
	wrapped := h.tenantMw.Wrap(next)
	return wrapped.ServeHTTP
}

func (h *PluginHandler) locale(r *http.Request) string {
	return store.LocaleFromContext(r.Context())
}

// ─── Catalog ─────────────────────────────────────────────────────────────────

func (h *PluginHandler) handleListCatalog(w http.ResponseWriter, r *http.Request) {
	entries, err := h.store.ListCatalog(r.Context())
	if err != nil {
		slog.Error("plugins.list_catalog", "error", err)
		locale := h.locale(r)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": i18n.T(locale, i18n.MsgFailedToList, "plugins")})
		return
	}
	if entries == nil {
		entries = []store.PluginCatalogEntry{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"catalog": entries})
}

// createCatalogRequest is the JSON body for registering a catalog entry.
type createCatalogRequest struct {
	Name        string          `json:"name"`
	Version     string          `json:"version"`
	DisplayName string          `json:"display_name"`
	Description string          `json:"description"`
	Author      string          `json:"author"`
	Source      string          `json:"source"`
	MinPlan     string          `json:"min_plan"`
	Tags        []string        `json:"tags"`
	Manifest    json.RawMessage `json:"manifest"`
}

func (h *PluginHandler) handleCreateCatalogEntry(w http.ResponseWriter, r *http.Request) {
	locale := h.locale(r)
	var req createCatalogRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgInvalidJSON)})
		return
	}

	if req.Name == "" || req.Version == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgRequired, "name and version")})
		return
	}

	// G4 blocker: validate permissions declared in the manifest (if provided).
	if len(req.Manifest) > 0 {
		var m plugins.PluginManifest
		if err := json.Unmarshal(req.Manifest, &m); err == nil {
			if err := plugins.ValidatePermissions(plugins.Permissions{
				Tools: m.Spec.Permissions.Tools,
				Data:  m.Spec.Permissions.Data,
			}); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
		}
	}

	entry := &store.PluginCatalogEntry{
		Name:        req.Name,
		Version:     req.Version,
		DisplayName: req.DisplayName,
		Description: req.Description,
		Author:      req.Author,
		Source:      req.Source,
		MinPlan:     req.MinPlan,
		Tags:        req.Tags,
		Manifest:    req.Manifest,
	}

	if err := h.store.UpsertCatalogEntry(r.Context(), entry); err != nil {
		slog.Error("plugins.create_catalog_entry", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, entry)
}

func (h *PluginHandler) handleGetCatalogEntry(w http.ResponseWriter, r *http.Request) {
	locale := h.locale(r)
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgInvalidID, "catalog entry")})
		return
	}
	entry, err := h.store.GetCatalogEntry(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": i18n.T(locale, i18n.MsgNotFound, "catalog entry", id.String())})
		return
	}
	writeJSON(w, http.StatusOK, entry)
}

// ─── Installed plugins ────────────────────────────────────────────────────────

func (h *PluginHandler) handleListInstalled(w http.ResponseWriter, r *http.Request) {
	plugins, err := h.store.ListTenantPlugins(r.Context())
	if err != nil {
		slog.Error("plugins.list_installed", "error", err)
		locale := h.locale(r)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": i18n.T(locale, i18n.MsgFailedToList, "plugins")})
		return
	}
	if plugins == nil {
		plugins = []store.TenantPlugin{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"plugins": plugins})
}

// installRequest is the JSON body for installing a plugin.
type installRequest struct {
	PluginName    string          `json:"plugin_name"`
	PluginVersion string          `json:"plugin_version"`
	Config        json.RawMessage `json:"config"`
}

func (h *PluginHandler) handleInstallPlugin(w http.ResponseWriter, r *http.Request) {
	locale := h.locale(r)
	var req installRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgInvalidJSON)})
		return
	}

	if req.PluginName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgRequired, "plugin_name")})
		return
	}
	if !plugins.IsValidPluginName(req.PluginName) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid plugin name"})
		return
	}

	config := req.Config
	if len(config) == 0 {
		config = json.RawMessage(`{}`)
	}

	tp := &store.TenantPlugin{
		PluginName:    req.PluginName,
		PluginVersion: req.PluginVersion,
		State:         store.PluginStateInstalled,
		Config:        config,
		Permissions:   json.RawMessage(`{}`),
	}

	if err := h.store.InstallPlugin(r.Context(), tp); err != nil {
		if errors.Is(err, store.ErrPluginAlreadyInstalled) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "plugin already installed"})
			return
		}
		slog.Error("plugins.install", "plugin", req.PluginName, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, tp)
}

func (h *PluginHandler) handleUninstallPlugin(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !plugins.IsValidPluginName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid plugin name"})
		return
	}
	if err := h.store.UninstallPlugin(r.Context(), name, nil); err != nil {
		if errors.Is(err, store.ErrPluginNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "plugin not found"})
			return
		}
		slog.Error("plugins.uninstall", "plugin", name, "error", err)
		locale := h.locale(r)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": i18n.T(locale, i18n.MsgFailedToDelete, "plugin", name)})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// updateConfigRequest is the JSON body for updating plugin config.
type updateConfigRequest struct {
	Config json.RawMessage `json:"config"`
}

func (h *PluginHandler) handleUpdatePluginConfig(w http.ResponseWriter, r *http.Request) {
	locale := h.locale(r)
	name := r.PathValue("name")
	if !plugins.IsValidPluginName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid plugin name"})
		return
	}
	var req updateConfigRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgInvalidJSON)})
		return
	}

	if len(req.Config) == 0 {
		req.Config = json.RawMessage(`{}`)
	}

	if err := h.store.UpdatePluginConfig(r.Context(), name, req.Config, nil); err != nil {
		if errors.Is(err, store.ErrPluginNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "plugin not found"})
			return
		}
		slog.Error("plugins.update_config", "plugin", name, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *PluginHandler) handleEnablePlugin(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !plugins.IsValidPluginName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid plugin name"})
		return
	}
	if err := h.store.EnablePlugin(r.Context(), name, nil); err != nil {
		if errors.Is(err, store.ErrPluginNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "plugin not found"})
			return
		}
		slog.Error("plugins.enable", "plugin", name, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "enabled"})
}

func (h *PluginHandler) handleDisablePlugin(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !plugins.IsValidPluginName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid plugin name"})
		return
	}
	if err := h.store.DisablePlugin(r.Context(), name, nil); err != nil {
		if errors.Is(err, store.ErrPluginNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "plugin not found"})
			return
		}
		slog.Error("plugins.disable", "plugin", name, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "disabled"})
}

func (h *PluginHandler) handleAuditLog(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !plugins.IsValidPluginName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid plugin name"})
		return
	}
	entries, err := h.store.ListAuditLog(r.Context(), name, 50)
	if err != nil {
		slog.Error("plugins.audit_log", "plugin", name, "error", err)
		locale := h.locale(r)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": i18n.T(locale, i18n.MsgFailedToList, "audit log")})
		return
	}
	if entries == nil {
		entries = []store.PluginAuditEntry{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

// ─── Agent plugin grants ──────────────────────────────────────────────────────

// agentGrantRequest is the JSON body for granting a plugin to an agent.
type agentGrantRequest struct {
	PluginName string          `json:"plugin_name"`
	Enabled    bool            `json:"enabled"`
	Config     json.RawMessage `json:"config"`
}

func (h *PluginHandler) handleGrantAgent(w http.ResponseWriter, r *http.Request) {
	locale := h.locale(r)
	agentID, err := uuid.Parse(r.PathValue("agentID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgInvalidID, "agent")})
		return
	}

	var req agentGrantRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgInvalidJSON)})
		return
	}
	if req.PluginName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgRequired, "plugin_name")})
		return
	}
	if !plugins.IsValidPluginName(req.PluginName) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid plugin name"})
		return
	}

	cfg := req.Config
	if len(cfg) == 0 {
		cfg = json.RawMessage(`{}`)
	}

	ap := &store.AgentPlugin{
		AgentID:        agentID,
		PluginName:     req.PluginName,
		Enabled:        req.Enabled,
		ConfigOverride: cfg,
	}

	if err := h.store.SetAgentPlugin(r.Context(), ap); err != nil {
		slog.Error("plugins.grant_agent", "agent", agentID, "plugin", req.PluginName, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, ap)
}

func (h *PluginHandler) handleRevokeAgent(w http.ResponseWriter, r *http.Request) {
	locale := h.locale(r)
	agentID, err := uuid.Parse(r.PathValue("agentID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgInvalidID, "agent")})
		return
	}
	pluginName := r.PathValue("pluginName")
	if !plugins.IsValidPluginName(pluginName) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid plugin name"})
		return
	}

	ap := &store.AgentPlugin{
		AgentID:        agentID,
		PluginName:     pluginName,
		Enabled:        false,
		ConfigOverride: json.RawMessage(`{}`),
	}

	if err := h.store.SetAgentPlugin(r.Context(), ap); err != nil {
		slog.Error("plugins.revoke_agent", "agent", agentID, "plugin", pluginName, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
