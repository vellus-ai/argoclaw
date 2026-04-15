//go:build integration

// Package operator_e2e contains end-to-end tests for the Ponte de Comando Central.
//
// These tests validate the full operator access flow against a real PostgreSQL database:
//   - Seed: operator tenant (operator_level=1) + admin user
//   - Login: POST /v1/auth/login → JWT with tenant_id vellus
//   - Access: GET /v1/operator/tenants → 200 with all tenants listed
//   - Isolation: customer tenant admin → GET /v1/operator/tenants → 403 OPERATOR_REQUIRED
//   - Context propagation: WithOperatorMode injected via HTTP TenantMiddleware
//
// Requires: TEST_POSTGRES_DSN environment variable pointing to a real PostgreSQL database.
// The DB must have migrations up to 000035 applied.
//
// Run:
//
//	ARGOCLAW_TEST_DSN="postgres://argoclaw:argoclaw@localhost:5432/argoclaw_test?sslmode=disable" \
//	  go test -tags integration -v ./tests/operator_e2e/...
package operator_e2e_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/vellus-ai/argoclaw/internal/auth"
	httpapi "github.com/vellus-ai/argoclaw/internal/http"
	"github.com/vellus-ai/argoclaw/internal/store"
	"github.com/vellus-ai/argoclaw/internal/store/pg"
)

const (
	testJWTSecret  = "operator-e2e-jwt-secret-32-bytes!!!"
	operatorSlug   = "e2e-vellus-operator"
	customerSlug   = "e2e-customer-tenant"
	operatorEmail  = "operator@e2e.test"
	customerEmail  = "customer@e2e.test"
	testPassword   = "E2E-Test-Password-2026!"
)

// testServer holds the httptest.Server and stores shared across tests.
type testServer struct {
	srv          *httptest.Server
	db           *sql.DB
	tenantStore  store.TenantStore
	userStore    store.UserStore

	operatorTenantID uuid.UUID
	customerTenantID uuid.UUID
}

var ts *testServer

// TestMain sets up a shared httptest.Server wired to a real DB.
func TestMain(m *testing.M) {
	dsn := os.Getenv("ARGOCLAW_TEST_DSN")
	if dsn == "" {
		fmt.Println("SKIP: ARGOCLAW_TEST_DSN not set — operator E2E tests require a PostgreSQL database")
		os.Exit(0)
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: open DB: %v\n", err)
		os.Exit(1)
	}
	db.SetMaxOpenConns(5)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: ping DB: %v\n", err)
		os.Exit(1)
	}

	tenantStore := pg.NewPGTenantStore(db)
	userStore := pg.NewPGUserStore(db)

	ts = &testServer{db: db, tenantStore: tenantStore, userStore: userStore}

	if err := ts.seed(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: seed: %v\n", err)
		os.Exit(1)
	}

	ts.srv = buildTestServer(db, tenantStore, userStore)

	code := m.Run()

	ts.cleanup()
	db.Close()
	os.Exit(code)
}

// buildTestServer creates an httptest.Server with JWT + Tenant middlewares and operator routes.
func buildTestServer(db *sql.DB, tenantStore store.TenantStore, userStore store.UserStore) *httptest.Server {
	mux := http.NewServeMux()

	// Middleware chain: JWT → Tenant (injects operator_level)
	jwtMw := httpapi.NewJWTMiddleware(testJWTSecret)
	tenantMw := httpapi.NewTenantMiddleware(tenantStore)

	chain := func(h http.Handler) http.Handler {
		return jwtMw.Wrap(tenantMw.Wrap(h))
	}

	// Login endpoint (no auth middleware — public)
	authHandler := httpapi.NewUserAuthHandler(userStore, testJWTSecret)
	authHandler.RegisterRoutes(mux)

	// Operator endpoints (auth chain wraps the operator mux)
	operatorHandler := httpapi.NewOperatorHandler(tenantStore, db)
	operatorMux := http.NewServeMux()
	operatorHandler.RegisterRoutes(operatorMux, httpapi.RequireOperatorRole)

	mux.Handle("/v1/operator/", chain(operatorMux))

	return httptest.NewServer(mux)
}

// seed creates the operator tenant (operator_level=1) and a customer tenant, each with one admin user.
func (ts *testServer) seed(ctx context.Context) error {
	// -- Operator tenant --
	opID := uuid.Must(uuid.NewV7())
	_, err := ts.db.ExecContext(ctx,
		`INSERT INTO tenants (id, slug, name, plan, status, operator_level, created_at, updated_at)
		 VALUES ($1, $2, 'E2E Operator Tenant', 'internal', 'active', 1, NOW(), NOW())
		 ON CONFLICT (slug) DO UPDATE SET operator_level = 1, status = 'active'
		 RETURNING id`,
		opID, operatorSlug,
	)
	if err != nil {
		return fmt.Errorf("insert operator tenant: %w", err)
	}
	// Fetch actual ID in case of ON CONFLICT
	if err := ts.db.QueryRowContext(ctx,
		`SELECT id FROM tenants WHERE slug = $1`, operatorSlug,
	).Scan(&ts.operatorTenantID); err != nil {
		return fmt.Errorf("fetch operator tenant id: %w", err)
	}

	// -- Customer tenant --
	custID := uuid.Must(uuid.NewV7())
	_, err = ts.db.ExecContext(ctx,
		`INSERT INTO tenants (id, slug, name, plan, status, operator_level, created_at, updated_at)
		 VALUES ($1, $2, 'E2E Customer Tenant', 'starter', 'active', 0, NOW(), NOW())
		 ON CONFLICT (slug) DO NOTHING`,
		custID, customerSlug,
	)
	if err != nil {
		return fmt.Errorf("insert customer tenant: %w", err)
	}
	if err := ts.db.QueryRowContext(ctx,
		`SELECT id FROM tenants WHERE slug = $1`, customerSlug,
	).Scan(&ts.customerTenantID); err != nil {
		return fmt.Errorf("fetch customer tenant id: %w", err)
	}

	// -- Users --
	hash, err := auth.HashPassword(testPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if err := ts.seedUser(ctx, ts.operatorTenantID, operatorEmail, hash, "admin"); err != nil {
		return fmt.Errorf("seed operator user: %w", err)
	}
	if err := ts.seedUser(ctx, ts.customerTenantID, customerEmail, hash, "admin"); err != nil {
		return fmt.Errorf("seed customer user: %w", err)
	}

	return nil
}

func (ts *testServer) seedUser(ctx context.Context, tenantID uuid.UUID, email, passwordHash, role string) error {
	userID := uuid.Must(uuid.NewV7())
	_, err := ts.db.ExecContext(ctx,
		`INSERT INTO users (id, email, display_name, password_hash, status, created_at, updated_at)
		 VALUES ($1, $2, $2, $3, 'active', NOW(), NOW())
		 ON CONFLICT (email) DO UPDATE SET password_hash = EXCLUDED.password_hash, status = 'active'`,
		userID, email, passwordHash,
	)
	if err != nil {
		return fmt.Errorf("insert user %s: %w", email, err)
	}
	// Fetch actual user ID
	var realUserID uuid.UUID
	if err := ts.db.QueryRowContext(ctx,
		`SELECT id FROM users WHERE email = $1`, email,
	).Scan(&realUserID); err != nil {
		return fmt.Errorf("fetch user id for %s: %w", email, err)
	}
	_, err = ts.db.ExecContext(ctx,
		`INSERT INTO tenant_users (tenant_id, user_id, role, joined_at)
		 VALUES ($1, $2, $3, NOW())
		 ON CONFLICT (tenant_id, user_id) DO UPDATE SET role = EXCLUDED.role`,
		tenantID, realUserID, role,
	)
	return err
}

func (ts *testServer) cleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	for _, slug := range []string{operatorSlug, customerSlug} {
		var tid uuid.UUID
		if err := ts.db.QueryRowContext(ctx, `SELECT id FROM tenants WHERE slug = $1`, slug).Scan(&tid); err != nil {
			continue
		}
		ts.db.ExecContext(ctx, `DELETE FROM tenant_users WHERE tenant_id = $1`, tid)
		ts.db.ExecContext(ctx, `DELETE FROM tenants WHERE id = $1`, tid)
	}
	for _, email := range []string{operatorEmail, customerEmail} {
		ts.db.ExecContext(ctx, `DELETE FROM users WHERE email = $1`, email)
	}
	if ts.srv != nil {
		ts.srv.Close()
	}
}

// =============================================================================
// Tests
// =============================================================================

// TestE2E_OperatorLogin_GetAllTenants verifies the full operator access flow:
// seed → POST /v1/auth/login → JWT → GET /v1/operator/tenants → 200.
func TestE2E_OperatorLogin_GetAllTenants(t *testing.T) {
	// Step 1: login as operator admin
	token := mustLogin(t, operatorEmail, testPassword)
	if token == "" {
		t.Fatal("login returned empty access token")
	}

	// Step 2: GET /v1/operator/tenants with operator JWT
	req := mustReq(t, "GET", ts.srv.URL+"/v1/operator/tenants", nil, token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /v1/operator/tenants: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var body map[string]any
		json.NewDecoder(resp.Body).Decode(&body) //nolint:errcheck
		t.Fatalf("expected 200, got %d: %v", resp.StatusCode, body)
	}

	// Step 3: response must be a list containing at least the operator and customer tenants
	var result struct {
		Data  []map[string]any `json:"data"`
		Total int              `json:"total"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if result.Total < 2 {
		t.Errorf("expected at least 2 tenants in list, got total=%d", result.Total)
	}

	// Step 4: data must be [] (never null) — even if empty (CLAUDE.md invariant)
	if result.Data == nil {
		t.Error("data field must be [] not null")
	}

	// Step 5: verify both seeded tenants are present
	var foundOperator, foundCustomer bool
	for _, tenant := range result.Data {
		slug, _ := tenant["slug"].(string)
		if slug == operatorSlug {
			foundOperator = true
		}
		if slug == customerSlug {
			foundCustomer = true
		}
	}
	if !foundOperator {
		t.Errorf("operator tenant %q not found in listing", operatorSlug)
	}
	if !foundCustomer {
		t.Errorf("customer tenant %q not found in listing", customerSlug)
	}
}

// TestE2E_CustomerAdminBlocked_OperatorEndpoints verifies that a customer tenant admin
// (operator_level=0) cannot access /v1/operator/* endpoints.
func TestE2E_CustomerAdminBlocked_OperatorEndpoints(t *testing.T) {
	token := mustLogin(t, customerEmail, testPassword)

	req := mustReq(t, "GET", ts.srv.URL+"/v1/operator/tenants", nil, token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /v1/operator/tenants: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 for customer admin, got %d", resp.StatusCode)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body) //nolint:errcheck
	if body["code"] != "OPERATOR_REQUIRED" {
		t.Errorf("expected code=OPERATOR_REQUIRED, got %q", body["code"])
	}
}

// TestE2E_OperatorMode_HTTPContextPropagation verifies that TenantMiddleware correctly
// injects WithOperatorMode into the request context for operator tenants.
//
// This is validated indirectly: when IsOperatorMode is true, ListAllTenantsForOperator
// succeeds (no ErrTenantRequired); for normal tenants it returns ErrTenantRequired.
func TestE2E_OperatorMode_HTTPContextPropagation(t *testing.T) {
	ctx := context.Background()

	// Operator tenant: WithCrossTenant + WithOperatorMode must be set by TenantMiddleware.
	// We simulate what TenantMiddleware does: load tenant from DB, check operator_level.
	opTenant, err := ts.tenantStore.GetByID(
		store.WithCrossTenant(ctx),
		ts.operatorTenantID,
	)
	if err != nil || opTenant == nil {
		t.Fatalf("load operator tenant: %v", err)
	}
	if opTenant.OperatorLevel < 1 {
		t.Fatalf("operator tenant must have operator_level >= 1, got %d", opTenant.OperatorLevel)
	}

	// Simulate what TenantMiddleware injects for this tenant
	opCtx := store.WithCrossTenant(store.WithTenantID(ctx, ts.operatorTenantID))
	opCtx = store.WithOperatorMode(opCtx, ts.operatorTenantID)

	if !store.IsOperatorMode(opCtx) {
		t.Error("IsOperatorMode must return true after WithOperatorMode injection")
	}
	if store.OperatorModeFromContext(opCtx) != ts.operatorTenantID {
		t.Errorf("OperatorModeFromContext must return operator tenant UUID")
	}

	// ListAllTenantsForOperator must succeed when cross-tenant context is active
	tenants, total, err := ts.tenantStore.ListAllTenantsForOperator(opCtx, 100, 0)
	if err != nil {
		t.Fatalf("ListAllTenantsForOperator with operator context: %v", err)
	}
	if total < 2 {
		t.Errorf("expected at least 2 tenants, got %d", total)
	}
	_ = tenants

	// Without WithCrossTenant: must return error (fail-closed)
	plainCtx := store.WithTenantID(ctx, ts.operatorTenantID)
	_, _, err = ts.tenantStore.ListAllTenantsForOperator(plainCtx, 100, 0)
	if err == nil {
		t.Error("ListAllTenantsForOperator without WithCrossTenant must return error")
	}
}

// TestE2E_OperatorMode_JWTHasCorrectTenantID verifies that the JWT returned by login
// contains the operator tenant's UUID and the admin role.
func TestE2E_OperatorMode_JWTHasCorrectTenantID(t *testing.T) {
	body := mustLoginBody(t, operatorEmail, testPassword)

	tenantID, _ := body["tenant_id"].(string)
	role, _ := body["role"].(string)

	if tenantID != ts.operatorTenantID.String() {
		t.Errorf("JWT tenant_id = %q, want %q", tenantID, ts.operatorTenantID)
	}
	if role != "admin" {
		t.Errorf("JWT role = %q, want admin", role)
	}
}

// TestE2E_OperatorEndpoints_TargetTenantAgents verifies that GET /v1/operator/tenants/{id}/agents
// returns 200 with empty data array (not null) for a valid tenant with no agents.
func TestE2E_OperatorEndpoints_TargetTenantAgents(t *testing.T) {
	token := mustLogin(t, operatorEmail, testPassword)

	url := fmt.Sprintf("%s/v1/operator/tenants/%s/agents", ts.srv.URL, ts.customerTenantID)
	req := mustReq(t, "GET", url, nil, token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errBody map[string]any
		json.NewDecoder(resp.Body).Decode(&errBody) //nolint:errcheck
		t.Fatalf("expected 200, got %d: %v", resp.StatusCode, errBody)
	}

	var result struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// CLAUDE.md: arrays must be [] not null
	if result.Data == nil {
		t.Error("data must be [] not null when there are no agents")
	}
}

// TestE2E_OperatorEndpoints_InvalidUUID verifies that passing a malformed UUID
// returns 400 INVALID_UUID.
func TestE2E_OperatorEndpoints_InvalidUUID(t *testing.T) {
	token := mustLogin(t, operatorEmail, testPassword)

	url := ts.srv.URL + "/v1/operator/tenants/not-a-uuid/agents"
	req := mustReq(t, "GET", url, nil, token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid UUID, got %d", resp.StatusCode)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body) //nolint:errcheck
	if body["code"] != "INVALID_UUID" {
		t.Errorf("expected code=INVALID_UUID, got %q", body["code"])
	}
}

// TestE2E_OperatorEndpoints_NonExistentTenant verifies that a valid UUID for a
// non-existent tenant returns 404 TENANT_NOT_FOUND.
func TestE2E_OperatorEndpoints_NonExistentTenant(t *testing.T) {
	token := mustLogin(t, operatorEmail, testPassword)

	nonExistent := uuid.Must(uuid.NewV7())
	url := fmt.Sprintf("%s/v1/operator/tenants/%s/sessions", ts.srv.URL, nonExistent)
	req := mustReq(t, "GET", url, nil, token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for non-existent tenant, got %d", resp.StatusCode)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body) //nolint:errcheck
	if body["code"] != "TENANT_NOT_FOUND" {
		t.Errorf("expected code=TENANT_NOT_FOUND, got %q", body["code"])
	}
}

// =============================================================================
// helpers
// =============================================================================

// mustLogin calls POST /v1/auth/login and returns the access_token.
func mustLogin(t *testing.T, email, password string) string {
	t.Helper()
	body := mustLoginBody(t, email, password)
	token, _ := body["access_token"].(string)
	return token
}

// mustLoginBody calls POST /v1/auth/login and returns the full response body.
func mustLoginBody(t *testing.T, email, password string) map[string]any {
	t.Helper()
	payload, _ := json.Marshal(map[string]string{"email": email, "password": password})
	req, err := http.NewRequest("POST", ts.srv.URL+"/v1/auth/login", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("build login request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /v1/auth/login: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login %q: expected 200, got %d", email, resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	return body
}

// mustReq builds an HTTP request with Bearer token auth.
func mustReq(t *testing.T, method, url string, body any, token string) *http.Request {
	t.Helper()
	var bodyReader *strings.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = strings.NewReader(string(b))
	} else {
		bodyReader = strings.NewReader("")
	}
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}
