package tenant_isolation_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/vellus-ai/argoclaw/internal/auth"
	"github.com/vellus-ai/argoclaw/internal/store"
	"github.com/vellus-ai/argoclaw/internal/store/pg"
)

// testEnv holds shared resources for all tenant isolation tests.
type testEnv struct {
	db           *sql.DB
	tenantStore  store.TenantStore
	agentStore   *pg.PGAgentStore
	jwtSecret    string
	gatewayURL   string
	gatewayToken string

	// Two tenants for cross-tenant testing
	tenantA *store.Tenant
	tenantB *store.Tenant

	// JWT tokens per tenant
	tokenA string
	tokenB string

	// Users per tenant
	userA uuid.UUID
	userB uuid.UUID
}

var env *testEnv

// TestMain sets up the shared test environment.
func TestMain(m *testing.M) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		fmt.Println("SKIP: TEST_POSTGRES_DSN not set — tenant isolation E2E tests require a PostgreSQL database")
		os.Exit(0)
	}

	jwtSecret := os.Getenv("TEST_JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "test-jwt-secret-for-e2e-tenant-isolation-32bytes!"
	}

	gatewayURL := os.Getenv("TEST_GATEWAY_URL")
	if gatewayURL == "" {
		gatewayURL = "http://localhost:18790"
	}

	gatewayToken := os.Getenv("TEST_GATEWAY_TOKEN")

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: cannot connect to test DB: %v\n", err)
		os.Exit(1)
	}
	db.SetMaxOpenConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: cannot ping test DB: %v\n", err)
		os.Exit(1)
	}

	tenantStore := pg.NewPGTenantStore(db)
	agentStore := pg.NewPGAgentStore(db)

	env = &testEnv{
		db:           db,
		tenantStore:  tenantStore,
		agentStore:   agentStore,
		jwtSecret:    jwtSecret,
		gatewayURL:   gatewayURL,
		gatewayToken: gatewayToken,
	}

	if err := env.setupTenants(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: cannot setup tenants: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()

	env.cleanup()
	db.Close()
	os.Exit(code)
}

// setupTenants creates two isolated tenants with users and JWT tokens.
func (e *testEnv) setupTenants(ctx context.Context) error {
	e.userA = uuid.New()
	e.userB = uuid.New()

	// Ensure test users exist in the users table.
	for _, uid := range []uuid.UUID{e.userA, e.userB} {
		_, err := e.db.ExecContext(ctx,
			`INSERT INTO users (id, email, password_hash, display_name, role, status, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, 'member', 'active', NOW(), NOW())
			 ON CONFLICT (id) DO NOTHING`,
			uid, uid.String()[:8]+"@test.local", "not-a-real-hash", "test-user-"+uid.String()[:8])
		if err != nil {
			return fmt.Errorf("insert user %s: %w", uid, err)
		}
	}

	// Create Tenant A
	e.tenantA = &store.Tenant{
		ID:     uuid.New(),
		Slug:   "e2e-tenant-alpha-" + uuid.New().String()[:8],
		Name:   "E2E Tenant Alpha",
		Plan:   "pro",
		Status: "active",
	}
	if err := e.tenantStore.CreateTenant(ctx, e.tenantA); err != nil {
		return fmt.Errorf("create tenant A: %w", err)
	}

	// Create Tenant B
	e.tenantB = &store.Tenant{
		ID:     uuid.New(),
		Slug:   "e2e-tenant-bravo-" + uuid.New().String()[:8],
		Name:   "E2E Tenant Bravo",
		Plan:   "starter",
		Status: "active",
	}
	if err := e.tenantStore.CreateTenant(ctx, e.tenantB); err != nil {
		return fmt.Errorf("create tenant B: %w", err)
	}

	// Add users to tenants
	if err := e.tenantStore.AddUser(ctx, e.tenantA.ID, e.userA, "owner"); err != nil {
		return fmt.Errorf("add user A: %w", err)
	}
	if err := e.tenantStore.AddUser(ctx, e.tenantB.ID, e.userB, "owner"); err != nil {
		return fmt.Errorf("add user B: %w", err)
	}

	// Generate JWT tokens for each tenant
	tokenA, err := auth.GenerateAccessToken(auth.TokenClaims{
		UserID:   e.userA.String(),
		Email:    "alice@e2e-alpha.test",
		TenantID: e.tenantA.ID.String(),
		Role:     "admin",
	}, e.jwtSecret)
	if err != nil {
		return fmt.Errorf("generate token A: %w", err)
	}
	e.tokenA = tokenA

	tokenB, err := auth.GenerateAccessToken(auth.TokenClaims{
		UserID:   e.userB.String(),
		Email:    "bob@e2e-bravo.test",
		TenantID: e.tenantB.ID.String(),
		Role:     "admin",
	}, e.jwtSecret)
	if err != nil {
		return fmt.Errorf("generate token B: %w", err)
	}
	e.tokenB = tokenB

	return nil
}

// cleanup removes all test data.
func (e *testEnv) cleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, tid := range []uuid.UUID{e.tenantA.ID, e.tenantB.ID} {
		// Cascade deletes handle tenant_users, agents, etc.
		e.db.ExecContext(ctx, `DELETE FROM agents WHERE tenant_id = $1`, tid)
		e.db.ExecContext(ctx, `DELETE FROM sessions WHERE tenant_id = $1`, tid)
		e.db.ExecContext(ctx, `DELETE FROM llm_providers WHERE tenant_id = $1`, tid)
		e.db.ExecContext(ctx, `DELETE FROM tenant_branding WHERE tenant_id = $1`, tid)
		e.db.ExecContext(ctx, `DELETE FROM tenant_users WHERE tenant_id = $1`, tid)
		e.db.ExecContext(ctx, `DELETE FROM tenants WHERE id = $1`, tid)
	}

	for _, uid := range []uuid.UUID{e.userA, e.userB} {
		e.db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, uid)
	}
}

// ctxForTenant returns a context with the given tenant ID injected.
func ctxForTenant(tenantID uuid.UUID) context.Context {
	return store.WithTenantID(context.Background(), tenantID)
}

// httpReqWithToken creates an HTTP request with the given JWT Bearer token.
func httpReqWithToken(method, url, token string) (*http.Request, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

// mustGenerateToken creates a JWT for testing with custom claims.
func mustGenerateToken(t *testing.T, claims auth.TokenClaims) string {
	t.Helper()
	token, err := auth.GenerateAccessToken(claims, env.jwtSecret)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	return token
}

