package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/vellus-ai/argoclaw/internal/i18n"
	"github.com/vellus-ai/argoclaw/internal/store"
)

// PluginDataHandler handles plugin KV data store HTTP endpoints.
// Endpoints are scoped per plugin: /v1/plugins/{name}/data/{collection}[/{key}]
type PluginDataHandler struct {
	store store.PluginStore
	token string
}

// NewPluginDataHandler creates a handler for plugin data proxy endpoints.
func NewPluginDataHandler(s store.PluginStore, token string) *PluginDataHandler {
	return &PluginDataHandler{store: s, token: token}
}

// RegisterRoutes registers plugin data proxy routes on the given mux.
func (h *PluginDataHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/plugins/{name}/data/{collection}", h.auth(h.handleListKeys))
	mux.HandleFunc("GET /v1/plugins/{name}/data/{collection}/{key}", h.auth(h.handleGetValue))
	mux.HandleFunc("PUT /v1/plugins/{name}/data/{collection}/{key}", h.auth(h.handlePutValue))
	mux.HandleFunc("DELETE /v1/plugins/{name}/data/{collection}/{key}", h.auth(h.handleDeleteValue))
}

func (h *PluginDataHandler) auth(next http.HandlerFunc) http.HandlerFunc {
	return requireAuth(h.token, "", next)
}

func (h *PluginDataHandler) locale(r *http.Request) string {
	return store.LocaleFromContext(r.Context())
}

// handleListKeys lists all keys in a collection.
// GET /v1/plugins/{name}/data/{collection}?prefix=&limit=50&offset=0
func (h *PluginDataHandler) handleListKeys(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	collection := r.PathValue("collection")
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

	keys, err := h.store.ListDataKeys(r.Context(), name, collection, prefix, limit, offset)
	if err != nil {
		slog.Error("plugins.data.list_keys", "plugin", name, "collection", collection, "error", err)
		locale := h.locale(r)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": i18n.T(locale, i18n.MsgFailedToList, "keys")})
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

	entry, err := h.store.GetData(r.Context(), name, collection, key)
	if err != nil {
		if errors.Is(err, store.ErrPluginNotFound) {
			locale := h.locale(r)
			writeJSON(w, http.StatusNotFound, map[string]string{"error": i18n.T(locale, i18n.MsgNotFound, "key", key)})
			return
		}
		slog.Error("plugins.data.get", "plugin", name, "collection", collection, "key", key, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
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

	var req putValueRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgInvalidJSON)})
		return
	}

	value := req.Value
	if len(value) == 0 {
		value = json.RawMessage(`null`)
	}

	if err := h.store.PutData(r.Context(), name, collection, key, value, nil); err != nil {
		slog.Error("plugins.data.put", "plugin", name, "collection", collection, "key", key, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
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

	if err := h.store.DeleteData(r.Context(), name, collection, key); err != nil {
		slog.Error("plugins.data.delete", "plugin", name, "collection", collection, "key", key, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
