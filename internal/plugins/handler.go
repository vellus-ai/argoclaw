package plugins

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"golang.org/x/time/rate"

	"github.com/vellus-ai/argoclaw/internal/permissions"
	"github.com/vellus-ai/argoclaw/internal/store"
)

// ─────────────────────────────────────────────────────────────────────────────
// Service Interfaces
// ─────────────────────────────────────────────────────────────────────────────

// PluginLifecycleService defines lifecycle operations the HTTP handlers need
// from the PluginHost controller.
type PluginLifecycleService interface {
	InstallPlugin(ctx context.Context, pluginName string) (*TenantPlugin, error)
	EnablePlugin(ctx context.Context, pluginName string) (*TenantPlugin, error)
	DisablePlugin(ctx context.Context, pluginName string) (*TenantPlugin, error)
	UninstallPlugin(ctx context.Context, pluginName string) error
	UpdatePluginConfig(ctx context.Context, pluginName string, config json.RawMessage) (*TenantPlugin, error)
	GetPluginStatus(ctx context.Context, pluginName string) (*PluginStatus, error)
}

// PluginCatalogService defines read operations for plugin catalog and queries.
type PluginCatalogService interface {
	ListCatalog(ctx context.Context, search string, tags []string, limit, offset int) ([]CatalogEntry, int, error)
	ListInstalled(ctx context.Context) ([]TenantPlugin, error)
	GetPlugin(ctx context.Context, name string) (*CatalogEntry, error)
	ListAudit(ctx context.Context, pluginName string, limit, offset int) ([]AuditEntry, int, error)
	GetPluginConfig(ctx context.Context, pluginName string) (*TenantPlugin, error)
	UIManifest(ctx context.Context) (interface{}, error)
}

// PluginAgentLinkService defines agent-plugin association operations.
type PluginAgentLinkService interface {
	EnableForAgent(ctx context.Context, pluginName string, agentID uuid.UUID) (*AgentPlugin, error)
	DisableForAgent(ctx context.Context, pluginName string, agentID uuid.UUID) (*AgentPlugin, error)
}

// ─────────────────────────────────────────────────────────────────────────────
// Auth — pluggable authentication for testability
// ─────────────────────────────────────────────────────────────────────────────

// AuthResult represents the outcome of an authentication check.
type AuthResult struct {
	Authenticated bool
	Role          permissions.Role
	UserID        string
	TenantID      uuid.UUID
}

// Authenticator resolves authentication from an HTTP request.
// Implementations may check bearer tokens, JWTs, API keys, etc.
type Authenticator interface {
	Authenticate(r *http.Request) AuthResult
}

// ─────────────────────────────────────────────────────────────────────────────
// Rate Limiter — per-tenant lifecycle rate limiting
// ─────────────────────────────────────────────────────────────────────────────

// tenantRateLimiter enforces per-tenant rate limits for plugin lifecycle operations.
type tenantRateLimiter struct {
	limiters sync.Map
	r        rate.Limit
	burst    int
}

type tenantLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen atomic.Int64
}

func newTenantRateLimiter(rpm, burst int) *tenantRateLimiter {
	if burst <= 0 {
		burst = 1
	}
	r := rate.Limit(0)
	if rpm > 0 {
		r = rate.Limit(float64(rpm) / 60.0)
	}
	trl := &tenantRateLimiter{r: r, burst: burst}
	go trl.cleanupLoop()
	return trl
}

func (trl *tenantRateLimiter) allow(tenantID string) bool {
	if trl.r == 0 {
		return true
	}
	newEntry := &tenantLimiterEntry{limiter: rate.NewLimiter(trl.r, trl.burst)}
	newEntry.lastSeen.Store(time.Now().UnixNano())

	v, _ := trl.limiters.LoadOrStore(tenantID, newEntry)
	entry := v.(*tenantLimiterEntry)
	entry.lastSeen.Store(time.Now().UnixNano())
	return entry.limiter.Allow()
}

func (trl *tenantRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-10 * time.Minute)
		trl.limiters.Range(func(key, value any) bool {
			if value.(*tenantLimiterEntry).lastSeen.Load() < cutoff.UnixNano() {
				trl.limiters.Delete(key)
			}
			return true
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Handler
// ─────────────────────────────────────────────────────────────────────────────

// PluginHandler handles all plugin management HTTP endpoints.
type PluginHandler struct {
	lifecycle   PluginLifecycleService
	catalog     PluginCatalogService
	agentLinks  PluginAgentLinkService
	dataProxy   *DataProxy
	auth        Authenticator
	rateLimiter *tenantRateLimiter
	logger      *slog.Logger
}

// NewPluginHandler creates a new PluginHandler with dependencies injected.
func NewPluginHandler(
	lifecycle PluginLifecycleService,
	catalog PluginCatalogService,
	agentLinks PluginAgentLinkService,
	dataProxy *DataProxy,
	auth Authenticator,
	logger *slog.Logger,
) *PluginHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &PluginHandler{
		lifecycle:   lifecycle,
		catalog:     catalog,
		agentLinks:  agentLinks,
		dataProxy:   dataProxy,
		auth:        auth,
		rateLimiter: newTenantRateLimiter(10, 3), // 10 ops/min/tenant, burst 3
		logger:      logger,
	}
}

// RegisterRoutes registers all plugin management routes on the given ServeMux.
// All routes are under /v1/plugins/.
func (h *PluginHandler) RegisterRoutes(mux *http.ServeMux) {
	// Catalog — read-only, any authenticated user
	mux.HandleFunc("GET /v1/plugins", h.authenticated(h.HandleListCatalog))
	mux.HandleFunc("GET /v1/plugins/installed", h.authenticated(h.HandleListInstalled))
	mux.HandleFunc("GET /v1/plugins/ui-manifest", h.authenticated(h.HandleUIManifest))
	mux.HandleFunc("GET /v1/plugins/{name}", h.authenticated(h.HandleGetPlugin))

	// Lifecycle — admin only, rate limited
	mux.HandleFunc("POST /v1/plugins/{name}/install", h.adminOnly(h.rateLimited(h.HandleInstall)))
	mux.HandleFunc("POST /v1/plugins/{name}/enable", h.adminOnly(h.rateLimited(h.HandleEnable)))
	mux.HandleFunc("POST /v1/plugins/{name}/disable", h.adminOnly(h.rateLimited(h.HandleDisable)))
	mux.HandleFunc("DELETE /v1/plugins/{name}/uninstall", h.adminOnly(h.rateLimited(h.HandleUninstall)))

	// Config — admin or operator
	mux.HandleFunc("GET /v1/plugins/{name}/config", h.minRole(permissions.RoleOperator, h.HandleGetConfig))
	mux.HandleFunc("PUT /v1/plugins/{name}/config", h.minRole(permissions.RoleOperator, h.rateLimited(h.HandleUpdateConfig)))

	// Status — any authenticated
	mux.HandleFunc("GET /v1/plugins/{name}/status", h.authenticated(h.HandleGetStatus))

	// Audit — admin only
	mux.HandleFunc("GET /v1/plugins/{name}/audit", h.adminOnly(h.HandleListAudit))

	// Agent plugin associations — admin or operator
	mux.HandleFunc("POST /v1/plugins/{name}/agents/{id}/enable", h.minRole(permissions.RoleOperator, h.HandleAgentEnable))
	mux.HandleFunc("POST /v1/plugins/{name}/agents/{id}/disable", h.minRole(permissions.RoleOperator, h.HandleAgentDisable))

	// Data Proxy — any authenticated
	mux.HandleFunc("GET /v1/plugins/{name}/data/{collection}", h.authenticated(h.HandleDataList))
	mux.HandleFunc("GET /v1/plugins/{name}/data/{collection}/{key}", h.authenticated(h.HandleDataGet))
	mux.HandleFunc("PUT /v1/plugins/{name}/data/{collection}/{key}", h.authenticated(h.HandleDataPut))
	mux.HandleFunc("DELETE /v1/plugins/{name}/data/{collection}/{key}", h.authenticated(h.HandleDataDelete))
}

// ─────────────────────────────────────────────────────────────────────────────
// Middleware helpers
// ─────────────────────────────────────────────────────────────────────────────

func (h *PluginHandler) authenticated(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := h.auth.Authenticate(r)
		if !auth.Authenticated {
			writePluginError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
			return
		}
		ctx := store.WithTenantID(r.Context(), auth.TenantID)
		if auth.UserID != "" {
			ctx = store.WithUserID(ctx, auth.UserID)
		}
		next(w, r.WithContext(ctx))
	}
}

func (h *PluginHandler) adminOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := h.auth.Authenticate(r)
		if !auth.Authenticated {
			writePluginError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
			return
		}
		if !permissions.HasMinRole(auth.Role, permissions.RoleAdmin) {
			writePluginError(w, http.StatusForbidden, "insufficient permissions", "permission_denied")
			return
		}
		ctx := store.WithTenantID(r.Context(), auth.TenantID)
		if auth.UserID != "" {
			ctx = store.WithUserID(ctx, auth.UserID)
		}
		next(w, r.WithContext(ctx))
	}
}

func (h *PluginHandler) minRole(minRole permissions.Role, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := h.auth.Authenticate(r)
		if !auth.Authenticated {
			writePluginError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
			return
		}
		if !permissions.HasMinRole(auth.Role, minRole) {
			writePluginError(w, http.StatusForbidden, "insufficient permissions", "permission_denied")
			return
		}
		ctx := store.WithTenantID(r.Context(), auth.TenantID)
		if auth.UserID != "" {
			ctx = store.WithUserID(ctx, auth.UserID)
		}
		next(w, r.WithContext(ctx))
	}
}

func (h *PluginHandler) rateLimited(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := store.TenantIDFromContext(r.Context())
		if !h.rateLimiter.allow(tenantID.String()) {
			w.Header().Set("Retry-After", "60")
			writePluginError(w, http.StatusTooManyRequests, "too many lifecycle operations", "rate_limited")
			return
		}
		next(w, r)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Catalog / Query handlers
// ─────────────────────────────────────────────────────────────────────────────

// HandleListCatalog — GET /v1/plugins?search=&tags=&limit=&offset=
func (h *PluginHandler) HandleListCatalog(w http.ResponseWriter, r *http.Request) {
	search := r.URL.Query().Get("search")
	tags := parseTags(r.URL.Query().Get("tags"))
	limit, offset := parsePagination(r)

	entries, total, err := h.catalog.ListCatalog(r.Context(), search, tags, limit, offset)
	if err != nil {
		h.logger.Error("plugins.list_catalog", "error", err)
		writePluginError(w, http.StatusInternalServerError, "internal server error", "internal_error")
		return
	}
	if entries == nil {
		entries = []CatalogEntry{}
	}
	writePluginJSON(w, http.StatusOK, map[string]any{
		"data":   entries,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// HandleListInstalled — GET /v1/plugins/installed
func (h *PluginHandler) HandleListInstalled(w http.ResponseWriter, r *http.Request) {
	plugins, err := h.catalog.ListInstalled(r.Context())
	if err != nil {
		h.logger.Error("plugins.list_installed", "error", err)
		writePluginError(w, http.StatusInternalServerError, "internal server error", "internal_error")
		return
	}
	if plugins == nil {
		plugins = []TenantPlugin{}
	}
	writePluginJSON(w, http.StatusOK, map[string]any{"data": plugins})
}

// HandleGetPlugin — GET /v1/plugins/{name}
func (h *PluginHandler) HandleGetPlugin(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !IsValidPluginName(name) {
		writePluginError(w, http.StatusBadRequest, "invalid plugin name", "bad_request")
		return
	}

	entry, err := h.catalog.GetPlugin(r.Context(), name)
	if err != nil {
		status, code := ErrorToHTTP(err)
		writePluginError(w, status, err.Error(), code)
		return
	}
	writePluginJSON(w, http.StatusOK, map[string]any{"data": entry})
}

// HandleUIManifest — GET /v1/plugins/ui-manifest
func (h *PluginHandler) HandleUIManifest(w http.ResponseWriter, r *http.Request) {
	manifest, err := h.catalog.UIManifest(r.Context())
	if err != nil {
		h.logger.Error("plugins.ui_manifest", "error", err)
		writePluginError(w, http.StatusInternalServerError, "internal server error", "internal_error")
		return
	}
	writePluginJSON(w, http.StatusOK, map[string]any{"data": manifest})
}

// HandleGetStatus — GET /v1/plugins/{name}/status
func (h *PluginHandler) HandleGetStatus(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !IsValidPluginName(name) {
		writePluginError(w, http.StatusBadRequest, "invalid plugin name", "bad_request")
		return
	}

	status, err := h.lifecycle.GetPluginStatus(r.Context(), name)
	if err != nil {
		httpStatus, code := ErrorToHTTP(err)
		writePluginError(w, httpStatus, err.Error(), code)
		return
	}
	writePluginJSON(w, http.StatusOK, map[string]any{"data": status})
}

// HandleListAudit — GET /v1/plugins/{name}/audit?limit=&offset=
func (h *PluginHandler) HandleListAudit(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !IsValidPluginName(name) {
		writePluginError(w, http.StatusBadRequest, "invalid plugin name", "bad_request")
		return
	}

	limit, offset := parsePagination(r)
	entries, total, err := h.catalog.ListAudit(r.Context(), name, limit, offset)
	if err != nil {
		h.logger.Error("plugins.list_audit", "plugin", name, "error", err)
		writePluginError(w, http.StatusInternalServerError, "internal server error", "internal_error")
		return
	}
	if entries == nil {
		entries = []AuditEntry{}
	}
	writePluginJSON(w, http.StatusOK, map[string]any{
		"data":   entries,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Lifecycle handlers
// ─────────────────────────────────────────────────────────────────────────────

// HandleInstall — POST /v1/plugins/{name}/install
func (h *PluginHandler) HandleInstall(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !IsValidPluginName(name) {
		writePluginError(w, http.StatusBadRequest, "invalid plugin name", "bad_request")
		return
	}

	tp, err := h.lifecycle.InstallPlugin(r.Context(), name)
	if err != nil {
		status, code := ErrorToHTTP(err)
		writePluginError(w, status, err.Error(), code)
		return
	}
	writePluginJSON(w, http.StatusCreated, map[string]any{"data": tp})
}

// HandleEnable — POST /v1/plugins/{name}/enable
func (h *PluginHandler) HandleEnable(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !IsValidPluginName(name) {
		writePluginError(w, http.StatusBadRequest, "invalid plugin name", "bad_request")
		return
	}

	tp, err := h.lifecycle.EnablePlugin(r.Context(), name)
	if err != nil {
		status, code := ErrorToHTTP(err)
		writePluginError(w, status, err.Error(), code)
		return
	}
	writePluginJSON(w, http.StatusOK, map[string]any{"data": tp})
}

// HandleDisable — POST /v1/plugins/{name}/disable
func (h *PluginHandler) HandleDisable(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !IsValidPluginName(name) {
		writePluginError(w, http.StatusBadRequest, "invalid plugin name", "bad_request")
		return
	}

	tp, err := h.lifecycle.DisablePlugin(r.Context(), name)
	if err != nil {
		status, code := ErrorToHTTP(err)
		writePluginError(w, status, err.Error(), code)
		return
	}
	writePluginJSON(w, http.StatusOK, map[string]any{"data": tp})
}

// HandleUninstall — DELETE /v1/plugins/{name}/uninstall
func (h *PluginHandler) HandleUninstall(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !IsValidPluginName(name) {
		writePluginError(w, http.StatusBadRequest, "invalid plugin name", "bad_request")
		return
	}

	if err := h.lifecycle.UninstallPlugin(r.Context(), name); err != nil {
		status, code := ErrorToHTTP(err)
		writePluginError(w, status, err.Error(), code)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─────────────────────────────────────────────────────────────────────────────
// Config handlers
// ─────────────────────────────────────────────────────────────────────────────

// HandleGetConfig — GET /v1/plugins/{name}/config
func (h *PluginHandler) HandleGetConfig(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !IsValidPluginName(name) {
		writePluginError(w, http.StatusBadRequest, "invalid plugin name", "bad_request")
		return
	}

	tp, err := h.catalog.GetPluginConfig(r.Context(), name)
	if err != nil {
		status, code := ErrorToHTTP(err)
		writePluginError(w, status, err.Error(), code)
		return
	}
	writePluginJSON(w, http.StatusOK, map[string]any{"data": tp})
}

// updateConfigBody is the JSON body for updating plugin config.
type updateConfigBody struct {
	Config json.RawMessage `json:"config"`
}

// HandleUpdateConfig — PUT /v1/plugins/{name}/config
func (h *PluginHandler) HandleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !IsValidPluginName(name) {
		writePluginError(w, http.StatusBadRequest, "invalid plugin name", "bad_request")
		return
	}

	var body updateConfigBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		writePluginError(w, http.StatusBadRequest, "invalid JSON body", "bad_request")
		return
	}
	if len(body.Config) == 0 {
		writePluginError(w, http.StatusBadRequest, "config is required", "bad_request")
		return
	}

	tp, err := h.lifecycle.UpdatePluginConfig(r.Context(), name, body.Config)
	if err != nil {
		status, code := ErrorToHTTP(err)
		writePluginError(w, status, err.Error(), code)
		return
	}
	writePluginJSON(w, http.StatusOK, map[string]any{"data": tp})
}

// ─────────────────────────────────────────────────────────────────────────────
// Agent plugin handlers
// ─────────────────────────────────────────────────────────────────────────────

// HandleAgentEnable — POST /v1/plugins/{name}/agents/{id}/enable
func (h *PluginHandler) HandleAgentEnable(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !IsValidPluginName(name) {
		writePluginError(w, http.StatusBadRequest, "invalid plugin name", "bad_request")
		return
	}
	agentID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writePluginError(w, http.StatusBadRequest, "invalid agent ID", "bad_request")
		return
	}

	ap, err := h.agentLinks.EnableForAgent(r.Context(), name, agentID)
	if err != nil {
		status, code := ErrorToHTTP(err)
		writePluginError(w, status, err.Error(), code)
		return
	}
	writePluginJSON(w, http.StatusOK, map[string]any{"data": ap})
}

// HandleAgentDisable — POST /v1/plugins/{name}/agents/{id}/disable
func (h *PluginHandler) HandleAgentDisable(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !IsValidPluginName(name) {
		writePluginError(w, http.StatusBadRequest, "invalid plugin name", "bad_request")
		return
	}
	agentID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writePluginError(w, http.StatusBadRequest, "invalid agent ID", "bad_request")
		return
	}

	ap, err := h.agentLinks.DisableForAgent(r.Context(), name, agentID)
	if err != nil {
		status, code := ErrorToHTTP(err)
		writePluginError(w, status, err.Error(), code)
		return
	}
	writePluginJSON(w, http.StatusOK, map[string]any{"data": ap})
}

// ─────────────────────────────────────────────────────────────────────────────
// Data Proxy handlers
// ─────────────────────────────────────────────────────────────────────────────

// HandleDataList — GET /v1/plugins/{name}/data/{collection}?prefix=&limit=&offset=
func (h *PluginHandler) HandleDataList(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	collection := r.PathValue("collection")
	if !IsValidPluginName(name) {
		writePluginError(w, http.StatusBadRequest, "invalid plugin name", "bad_request")
		return
	}

	prefix := r.URL.Query().Get("prefix")
	limit := parseIntParam(r, "limit", 50, 1, 200)
	offset := parseIntParam(r, "offset", 0, 0, 100000)

	keys, err := h.dataProxy.ListKeys(r.Context(), name, collection, prefix, limit, offset)
	if err != nil {
		h.handleDataProxyError(w, "plugins.data.list_keys", name, err)
		return
	}
	if keys == nil {
		keys = []string{}
	}
	writePluginJSON(w, http.StatusOK, map[string]any{
		"data":       keys,
		"plugin":     name,
		"collection": collection,
	})
}

// HandleDataGet — GET /v1/plugins/{name}/data/{collection}/{key}
func (h *PluginHandler) HandleDataGet(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	collection := r.PathValue("collection")
	key := r.PathValue("key")
	if !IsValidPluginName(name) {
		writePluginError(w, http.StatusBadRequest, "invalid plugin name", "bad_request")
		return
	}

	entry, err := h.dataProxy.Get(r.Context(), name, collection, key)
	if err != nil {
		h.handleDataProxyError(w, "plugins.data.get", name, err)
		return
	}
	writePluginJSON(w, http.StatusOK, map[string]any{"data": entry})
}

// dataProxyPutBody is the JSON body for data put operations.
type dataProxyPutBody struct {
	Value json.RawMessage `json:"value"`
}

// HandleDataPut — PUT /v1/plugins/{name}/data/{collection}/{key}
func (h *PluginHandler) HandleDataPut(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	collection := r.PathValue("collection")
	key := r.PathValue("key")
	if !IsValidPluginName(name) {
		writePluginError(w, http.StatusBadRequest, "invalid plugin name", "bad_request")
		return
	}

	var body dataProxyPutBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		writePluginError(w, http.StatusBadRequest, "invalid JSON body", "bad_request")
		return
	}
	if len(body.Value) == 0 {
		body.Value = json.RawMessage(`null`)
	}

	if err := h.dataProxy.Put(r.Context(), name, collection, key, body.Value, nil); err != nil {
		h.handleDataProxyError(w, "plugins.data.put", name, err)
		return
	}
	writePluginJSON(w, http.StatusOK, map[string]any{
		"data": map[string]string{"status": "stored", "key": key},
	})
}

// HandleDataDelete — DELETE /v1/plugins/{name}/data/{collection}/{key}
func (h *PluginHandler) HandleDataDelete(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	collection := r.PathValue("collection")
	key := r.PathValue("key")
	if !IsValidPluginName(name) {
		writePluginError(w, http.StatusBadRequest, "invalid plugin name", "bad_request")
		return
	}

	if err := h.dataProxy.Delete(r.Context(), name, collection, key); err != nil {
		h.handleDataProxyError(w, "plugins.data.delete", name, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *PluginHandler) handleDataProxyError(w http.ResponseWriter, op, name string, err error) {
	// Check data proxy specific errors first
	switch {
	case errors.Is(err, ErrMissingTenantContext):
		writePluginError(w, http.StatusForbidden, "tenant context required", "permission_denied")
		return
	case errors.Is(err, ErrKeyTooLong):
		writePluginError(w, http.StatusBadRequest, "key exceeds maximum length", "bad_request")
		return
	case errors.Is(err, ErrCollectionTooLong):
		writePluginError(w, http.StatusBadRequest, "collection exceeds maximum length", "bad_request")
		return
	case errors.Is(err, store.ErrPluginNotFound):
		writePluginError(w, http.StatusNotFound, "not found", "plugin_not_found")
		return
	case errors.Is(err, ErrPluginNotInstalled):
		writePluginError(w, http.StatusNotFound, "plugin not installed", "plugin_not_installed")
		return
	}

	// Fall through to generic error mapping
	status, code := ErrorToHTTP(err)
	if status == http.StatusInternalServerError {
		h.logger.Error(op, "plugin", name, "error", err)
	}
	writePluginError(w, status, err.Error(), code)
}

// ─────────────────────────────────────────────────────────────────────────────
// JSON helpers
// ─────────────────────────────────────────────────────────────────────────────

func writePluginJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writePluginError(w http.ResponseWriter, status int, message, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
		"code":  code,
	})
}

func parsePagination(r *http.Request) (limit, offset int) {
	limit = parseIntParam(r, "limit", 50, 1, 100)
	offset = parseIntParam(r, "offset", 0, 0, 100000)
	return
}

func parseIntParam(r *http.Request, key string, defaultVal, min, max int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < min {
		return defaultVal
	}
	if n > max {
		return max
	}
	return n
}

func parseTags(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	tags := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			tags = append(tags, t)
		}
	}
	if len(tags) == 0 {
		return nil
	}
	return tags
}
