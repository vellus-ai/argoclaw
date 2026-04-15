package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/vellus-ai/argoclaw/internal/store"
)

// mockTenantStoreForOperator is a minimal TenantStore for operator handler tests.
type mockTenantStoreForOperator struct {
	tenants []store.Tenant
	err     error
}

func (m *mockTenantStoreForOperator) CreateTenant(_ context.Context, _ *store.Tenant) error {
	return m.err
}
func (m *mockTenantStoreForOperator) GetByID(_ context.Context, id uuid.UUID) (*store.Tenant, error) {
	if m.err != nil {
		return nil, m.err
	}
	for i := range m.tenants {
		if m.tenants[i].ID == id {
			return &m.tenants[i], nil
		}
	}
	return nil, nil
}
func (m *mockTenantStoreForOperator) GetBySlug(_ context.Context, _ string) (*store.Tenant, error) {
	return nil, m.err
}
func (m *mockTenantStoreForOperator) ListTenants(_ context.Context) ([]store.Tenant, error) {
	return m.tenants, m.err
}
func (m *mockTenantStoreForOperator) UpdateTenant(_ context.Context, _ uuid.UUID, _ map[string]any) error {
	return m.err
}
func (m *mockTenantStoreForOperator) AddUser(_ context.Context, _, _ uuid.UUID, _ string) error {
	return m.err
}
func (m *mockTenantStoreForOperator) RemoveUser(_ context.Context, _, _ uuid.UUID) error {
	return m.err
}
func (m *mockTenantStoreForOperator) ListUsers(_ context.Context, _ uuid.UUID) ([]store.TenantUser, error) {
	return nil, m.err
}
func (m *mockTenantStoreForOperator) GetUserTenants(_ context.Context, _ uuid.UUID) ([]store.TenantUser, error) {
	return nil, m.err
}
func (m *mockTenantStoreForOperator) GetBranding(_ context.Context, _ uuid.UUID) (*store.TenantBranding, error) {
	return nil, m.err
}
func (m *mockTenantStoreForOperator) UpsertBranding(_ context.Context, _ *store.TenantBranding) error {
	return m.err
}
func (m *mockTenantStoreForOperator) GetBrandingByDomain(_ context.Context, _ string) (*store.TenantBranding, error) {
	return nil, m.err
}
func (m *mockTenantStoreForOperator) ListAllTenantsForOperator(_ context.Context, limit, offset int) ([]store.Tenant, int, error) {
	if m.err != nil {
		return nil, 0, m.err
	}
	total := len(m.tenants)
	start := offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}
	return m.tenants[start:end], total, nil
}

// Compile-time check: mockTenantStoreForOperator satisfies store.TenantStore.
var _ store.TenantStore = (*mockTenantStoreForOperator)(nil)

// Ensure time is used (via store.Tenant fields in test data).
var _ = time.Now

func operatorCtx(tenantID uuid.UUID, role string) context.Context {
	ctx := context.Background()
	ctx = store.WithCrossTenant(ctx) // appsec:cross-tenant-bypass — test setup
	ctx = store.WithOperatorMode(ctx, tenantID)
	ctx = store.WithTenantID(ctx, tenantID)
	ctx = store.WithUserID(ctx, "test-user")
	ctx = injectClaims(ctx, role)
	return ctx
}

func TestOperatorHandler_ListTenants_RequiresOperatorRole(t *testing.T) {
	tenantID := uuid.New()
	mockStore := &mockTenantStoreForOperator{
		tenants: []store.Tenant{
			{ID: tenantID, Slug: "test", Name: "Test", Plan: "starter", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		},
	}
	h := NewOperatorHandler(mockStore, nil)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux, requireOperatorRole)

	// Without operator mode context — middleware should reject
	req := httptest.NewRequest("GET", "/v1/operator/tenants", nil)
	ctx := context.Background()
	ctx = store.WithTenantID(ctx, tenantID)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 when no operator mode", rec.Code)
	}
}

func TestOperatorHandler_ListTenants_ReturnsPaginatedList(t *testing.T) {
	operatorTenantID := uuid.New()
	mockStore := &mockTenantStoreForOperator{
		tenants: []store.Tenant{
			{ID: uuid.New(), Slug: "t1", Name: "T1", Plan: "starter", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()},
			{ID: uuid.New(), Slug: "t2", Name: "T2", Plan: "pro", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		},
	}
	h := NewOperatorHandler(mockStore, nil)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux, requireOperatorRole)

	req := httptest.NewRequest("GET", "/v1/operator/tenants?limit=10&offset=0", nil)
	req = req.WithContext(operatorCtx(operatorTenantID, "admin"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if _, ok := resp["data"]; !ok {
		t.Error("response missing 'data' key")
	}
	if _, ok := resp["total"]; !ok {
		t.Error("response missing 'total' key")
	}
}

func TestOperatorHandler_ListTenantAgents_InvalidUUID(t *testing.T) {
	operatorTenantID := uuid.New()
	h := NewOperatorHandler(&mockTenantStoreForOperator{}, nil)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux, requireOperatorRole)

	req := httptest.NewRequest("GET", "/v1/operator/tenants/not-a-uuid/agents", nil)
	req = req.WithContext(operatorCtx(operatorTenantID, "admin"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for invalid UUID", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "INVALID_UUID") {
		t.Errorf("body = %s, want INVALID_UUID error code", rec.Body.String())
	}
}

func TestOperatorHandler_ListTenantAgents_TenantNotFound(t *testing.T) {
	operatorTenantID := uuid.New()
	mockStore := &mockTenantStoreForOperator{tenants: []store.Tenant{}}
	h := NewOperatorHandler(mockStore, nil)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux, requireOperatorRole)

	req := httptest.NewRequest("GET", "/v1/operator/tenants/"+uuid.New().String()+"/agents", nil)
	req = req.WithContext(operatorCtx(operatorTenantID, "admin"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for non-existent tenant", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "TENANT_NOT_FOUND") {
		t.Errorf("body = %s, want TENANT_NOT_FOUND error code", rec.Body.String())
	}
}

func TestOperatorHandler_GetTenantUsage_ValidPeriods(t *testing.T) {
	operatorTenantID := uuid.New()
	targetTenantID := uuid.New()
	mockStore := &mockTenantStoreForOperator{
		tenants: []store.Tenant{
			{ID: targetTenantID, Slug: "t1", Name: "T1", Plan: "starter", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		},
	}
	h := NewOperatorHandler(mockStore, nil)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux, requireOperatorRole)

	periods := []string{"7d", "30d", "90d", ""}
	for _, period := range periods {
		url := "/v1/operator/tenants/" + targetTenantID.String() + "/usage"
		if period != "" {
			url += "?period=" + period
		}
		req := httptest.NewRequest("GET", url, nil)
		req = req.WithContext(operatorCtx(operatorTenantID, "admin"))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("period=%q: status = %d, want 200; body: %s", period, rec.Code, rec.Body.String())
		}
	}
}

func TestOperatorHandler_GetTenantUsage_InvalidPeriod(t *testing.T) {
	operatorTenantID := uuid.New()
	targetTenantID := uuid.New()
	mockStore := &mockTenantStoreForOperator{
		tenants: []store.Tenant{
			{ID: targetTenantID, Slug: "t1", Name: "T1", Plan: "starter", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		},
	}
	h := NewOperatorHandler(mockStore, nil)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux, requireOperatorRole)

	req := httptest.NewRequest("GET", "/v1/operator/tenants/"+targetTenantID.String()+"/usage?period=invalid", nil)
	req = req.WithContext(operatorCtx(operatorTenantID, "admin"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for invalid period", rec.Code)
	}
}

func TestOperatorHandler_ListTenantSessions_InvalidUUID(t *testing.T) {
	operatorTenantID := uuid.New()
	h := NewOperatorHandler(&mockTenantStoreForOperator{}, nil)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux, requireOperatorRole)

	req := httptest.NewRequest("GET", "/v1/operator/tenants/bad-id/sessions", nil)
	req = req.WithContext(operatorCtx(operatorTenantID, "admin"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for invalid UUID", rec.Code)
	}
}

func TestOperatorHandler_ListTenants_StoreError(t *testing.T) {
	operatorTenantID := uuid.New()
	mockStore := &mockTenantStoreForOperator{err: errors.New("db connection failed")}
	h := NewOperatorHandler(mockStore, nil)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux, requireOperatorRole)

	req := httptest.NewRequest("GET", "/v1/operator/tenants", nil)
	req = req.WithContext(operatorCtx(operatorTenantID, "admin"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 on store error", rec.Code)
	}
}

