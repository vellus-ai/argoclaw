package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/vellus-ai/argoclaw/internal/i18n"
	"github.com/vellus-ai/argoclaw/internal/plugins"
	"github.com/vellus-ai/argoclaw/internal/store"
)

// PluginDataHandler handles plugin KV data store HTTP endpoints.
// Endpoints are scoped per plugin: /v1/plugins/{name}/data/{collection}[/{key}]
// All data access goes through DataProxy which enforces tenant context, length limits,
// and plugin installation checks.
type PluginDataHandler struct {
	proxy    *plugins.DataProxy
	token    string
	tenantMw *TenantMiddleware
}

// NewPluginDataHandler creates a handler for plugin data proxy endpoints.
func NewPluginDataHandler(proxy *plugins.DataProxy, token string, tenantMw *TenantMiddleware) *PluginDataHandler {
	return &PluginDataHandler{proxy: proxy, token: token, tenantMw: tenantMw}
}

// RegisterRoutes registers plugin data proxy routes on the given mux.
// Routes use /v1/plugin-data/ prefix to avoid conflicts with /v1/plugins/installed/{name}/...
func (h *PluginDataHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/plugin-data/{name}/{collection}", h.withTenant(h.auth(h.handleListKeys)))
	mux.HandleFunc("GET /v1/plugin-data/{name}/{collection}/{key}", h.withTenant(h.auth(h.handleGetValue)))
	mux.HandleFunc("PUT /v1/plugin-data/{name}/{collection}/{key}", h.withTenant(h.auth(h.handlePutValue)))
	mux.HandleFunc("DELETE /v1/plugin-data/{name}/{collection}/{key}", h.withTenant(h.auth(h.handleDeleteValue)))
}

func (h *PluginDataHandler) auth(next http.HandlerFunc) http.HandlerFunc {
	return requireAuth(h.token, "", next)
}

func (h *PluginDataHandler) locale(r *http.Request) string {
	return store.LocaleFromContext(r.Context())
}

// withTenant wraps a handler with TenantMiddleware to inject tenant_id from JWT into context.
func (h *PluginDataHandler) withTenant(next http.HandlerFunc) http.HandlerFunc {
	if h.tenantMw == nil {
		return next
	}
	wrapped := h.tenantMw.Wrap(next)
	return wrapped.ServeHTTP
}

// validatePathParams validates plugin name, collection, and optional key from path params.
func validatePathParams(name, collection, key string) string {
	if !plugins.IsValidPluginName(name) {
		return "invalid plugin name"
	}
	if len(collection) == 0 || len(collection) > 100 {
		return "collection must be 1-100 characters"
	}
	if key != "" && len(key) > 500 {
		return "key must be at most 500 characters"
	}
	return ""
}

// handleListKeys lists all keys in a collection.
// GET /v1/plugins/{name}/data/{collection}?prefix=&limit=50&offset=0
func (h *PluginDataHandler) handleListKeys(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	collection := r.PathValue("collection")
	if msg := validatePathParams(name, collection, ""); msg != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}
	prefix := r.URL.Query().Get("prefix")

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil && n >= 0 {
			offset = n
		}
	}

	keys, err := h.proxy.ListKeys(r.Context(), name, collection, prefix, limit, offset)
	if err != nil {
		h.handleProxyError(w, r, "plugins.data.list_keys", name, collection, "", err)
		return
	}
	if keys == nil {
		keys = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"plugin":     name,
		"collection": collection,
		"keys":       keys,
	})
}

// handleGetValue retrieves a single KV value.
// GET /v1/plugins/{name}/data/{collection}/{key}
func (h *PluginDataHandler) handleGetValue(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	collection := r.PathValue("collection")
	key := r.PathValue("key")
	if msg := validatePathParams(name, collection, key); msg != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}

	entry, err := h.proxy.Get(r.Context(), name, collection, key)
	if err != nil {
		h.handleProxyError(w, r, "plugins.data.get", name, collection, key, err)
		return
	}

	writeJSON(w, http.StatusOK, entry)
}

// putValueRequest is the body for upsert operations.
type putValueRequest struct {
	Value json.RawMessage `json:"value"`
}

// handlePutValue upserts a KV value.
// PUT /v1/plugins/{name}/data/{collection}/{key}
func (h *PluginDataHandler) handlePutValue(w http.ResponseWriter, r *http.Request) {
	locale := h.locale(r)
	name := r.PathValue("name")
	collection := r.PathValue("collection")
	key := r.PathValue("key")
	if msg := validatePathParams(name, collection, key); msg != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}

	var req putValueRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgInvalidJSON)})
		return
	}

	value := req.Value
	if len(value) == 0 {
		value = json.RawMessage(`null`)
	}

	if err := h.proxy.Put(r.Context(), name, collection, key, value, nil); err != nil {
		h.handleProxyError(w, r, "plugins.data.put", name, collection, key, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "stored", "key": key})
}

// handleDeleteValue deletes a KV entry.
// DELETE /v1/plugins/{name}/data/{collection}/{key}
func (h *PluginDataHandler) handleDeleteValue(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	collection := r.PathValue("collection")
	key := r.PathValue("key")
	if msg := validatePathParams(name, collection, key); msg != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}

	if err := h.proxy.Delete(r.Context(), name, collection, key); err != nil {
		h.handleProxyError(w, r, "plugins.data.delete", name, collection, key, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleProxyError maps DataProxy errors to HTTP responses.
func (h *PluginDataHandler) handleProxyError(w http.ResponseWriter, r *http.Request, op, name, collection, key string, err error) {
	switch {
	case errors.Is(err, plugins.ErrPluginNotInstalled):
		locale := h.locale(r)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": i18n.T(locale, i18n.MsgNotFound, "plugin", name)})
	case errors.Is(err, plugins.ErrMissingTenantContext):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "tenant context required"})
	case errors.Is(err, plugins.ErrKeyTooLong):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "key exceeds maximum length"})
	case errors.Is(err, plugins.ErrCollectionTooLong):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "collection exceeds maximum length"})
	case errors.Is(err, store.ErrPluginNotFound):
		locale := h.locale(r)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": i18n.T(locale, i18n.MsgNotFound, "key", key)})
	default:
		slog.Error(op, "plugin", name, "collection", collection, "key", key, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
}
