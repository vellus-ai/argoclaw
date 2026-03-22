package http

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/vellus-ai/argoclaw/internal/store"
)

// BrandingHandler manages tenant white-label branding configuration.
type BrandingHandler struct {
	tenants store.TenantStore
}

func NewBrandingHandler(tenants store.TenantStore) *BrandingHandler {
	return &BrandingHandler{tenants: tenants}
}

func (h *BrandingHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/branding", h.handleGet)
	mux.HandleFunc("PUT /v1/branding", h.handleUpdate)
	mux.HandleFunc("GET /v1/branding/domain/{domain}", h.handleGetByDomain)
}

func (h *BrandingHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	tenantID := TenantIDFromRequest(r.Context())
	if tenantID.String() == "00000000-0000-0000-0000-000000000000" {
		writeJSONError(w, http.StatusForbidden, "tenant context required")
		return
	}

	branding, err := h.tenants.GetBranding(r.Context(), tenantID)
	if err != nil {
		slog.Error("branding: get", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if branding == nil {
		// Return defaults
		branding = &store.TenantBranding{
			TenantID:    tenantID,
			ProductName: "ARGO",
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(branding)
}

type brandingUpdateRequest struct {
	LogoURL      *string `json:"logo_url"`
	FaviconURL   *string `json:"favicon_url"`
	PrimaryColor *string `json:"primary_color"`
	Palette      *string `json:"palette"`
	CustomDomain *string `json:"custom_domain"`
	SenderEmail  *string `json:"sender_email"`
	ProductName  *string `json:"product_name"`
}

func (h *BrandingHandler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	tenantID := TenantIDFromRequest(r.Context())
	if tenantID.String() == "00000000-0000-0000-0000-000000000000" {
		writeJSONError(w, http.StatusForbidden, "tenant context required")
		return
	}

	var req brandingUpdateRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	branding := &store.TenantBranding{TenantID: tenantID, ProductName: "ARGO"}

	// Apply provided fields
	if req.LogoURL != nil {
		branding.LogoURL = *req.LogoURL
	}
	if req.FaviconURL != nil {
		branding.FaviconURL = *req.FaviconURL
	}
	if req.PrimaryColor != nil {
		branding.PrimaryColor = *req.PrimaryColor
	}
	if req.Palette != nil {
		branding.Palette = *req.Palette
	}
	if req.CustomDomain != nil {
		branding.CustomDomain = *req.CustomDomain
	}
	if req.SenderEmail != nil {
		branding.SenderEmail = *req.SenderEmail
	}
	if req.ProductName != nil {
		branding.ProductName = *req.ProductName
	}

	if err := h.tenants.UpsertBranding(r.Context(), branding); err != nil {
		slog.Error("branding: upsert", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(branding)
}

func (h *BrandingHandler) handleGetByDomain(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	if domain == "" {
		writeJSONError(w, http.StatusBadRequest, "domain is required")
		return
	}

	branding, err := h.tenants.GetBrandingByDomain(r.Context(), domain)
	if err != nil {
		slog.Error("branding: get by domain", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if branding == nil {
		writeJSONError(w, http.StatusNotFound, "domain not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(branding)
}
