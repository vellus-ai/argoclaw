//go:build integration

// Package integration contains cross-tenant isolation and migration seed integration tests.
//
// These tests validate:
//   - ListAllTenantsForOperator cross-tenant behavior with real DB
//   - Customer admin blocked from all operator endpoints (operator_level=0)
//   - Migration 000035 idempotency (seed exactly 1 vellus tenant)
//   - operator_level write protection via POST/PUT /v1/tenants
//
// PBT properties:
//   ∀ N executions of migration (N ∈ [1,5]): COUNT(tenants WHERE slug='vellus') = 1
//   ∀ tenant T with operator_level=0: access to /v1/operator/* = 403 regardless of role
//
// Requires: ARGOCLAW_TEST_DSN environment variable pointing to a real PostgreSQL database.
//
// **Validates: Requirements 1.3, 1.5, 2.4, 2.5, 3.5, 3.6, 9.1**
package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"pgregory.net/rapid"

	"github.com/vellus-ai/argoclaw/internal/auth"
	"github.com/vellus-ai/argoclaw/internal/store"
)

// =============================================================================
// Test: ListAllTenantsForOperator cross-tenant behavior
// Validates: Requirements 2.4, 2.5
// =============================================================================

func TestListAllTenantsForOperator_CrossTenant(t *testing.T) {
	// Create multiple tenants to verify cross-tenant listing
	t1 := tempTenant(t, 0)
	t2 := tempTenant(t, 0)
	t3 := tempTenant(t, 1)

	// With WithCrossTenant: should return all tenants (including the ones we just created)
	ctx := store.WithCrossTenant(context.Background())
	tenants, total, err := env.tenantStore.ListAllTenantsForOperator(ctx, 100, 0)
	if err != nil {
		t.Fatalf("ListAllTenantsForOperator with cross-tenant: %v", err)
	}
	if total < 3 {
		t.Errorf("expected total >= 3 (created 3 tenants), got %d", total)
	}

	// Verify our created tenants are in the results
	found := map[uuid.UUID]bool{}
	for _, tenant := range tenants {
		found[tenant.ID] = true
	}
	for _, id := range []uuid.UUID{t1, t2, t3} {
		if !found[id] {
			t.Errorf("tenant %s not found in cross-tenant listing", id)
		}
	}

	// Without WithCrossTenant: should return ErrTenantRequired
	plainCtx := context.Background()
	_, _, err = env.tenantStore.ListAllTenantsForOperator(plainCtx, 100, 0)
	if err == nil {
		t.Fatal("expected ErrTenantRequired without cross-tenant context, got nil")
	}
	if err.Error() != store.ErrTenantRequired.Error() {
		t.Errorf("expected ErrTenantRequired, got %v", err)
	}

	// With tenant_id but no cross-tenant: should still return ErrTenantRequired
	tenantCtx := store.WithTenantID(context.Background(), t1)
	_, _, err = env.tenantStore.ListAllTenantsForOperator(tenantCtx, 100, 0)
	if err == nil {
		t.Fatal("expected ErrTenantRequired with tenant context but no cross-tenant, got nil")
	}
}

// =============================================================================
// Test: Customer admin blocked from ALL operator endpoints
// Validates: Requirements 3.5, 3.6, 9.1
// =============================================================================

func TestCustomerAdmin_BlockedFromAllOperatorEndpoints(t *testing.T) {
	// Create a customer tenant (operator_level=0) with an admin user
	custTenantID := tempTenant(t, 0)
	_, custAdminToken := tempUser(t, custTenantID, "admin")

	// All four operator endpoints must return 403 OPERATOR_REQUIRED
	endpoints := []string{
		"/v1/operator/tenants",
		"/v1/operator/tenants/" + uuid.New().String() + "/agents",
		"/v1/operator/tenants/" + uuid.New().String() + "/sessions",
		"/v1/operator/tenants/" + uuid.New().String() + "/usage?period=7d",
	}

	for _, ep := range endpoints {
		t.Run("GET "+ep, func(t *testing.T) {
			status, body := doGet(t, ep, custAdminToken)
			if status != http.StatusForbidden {
				t.Errorf("status = %d, want 403", status)
			}
			code, _ := body["code"].(string)
			if code != "OPERATOR_REQUIRED" {
				t.Errorf("code = %q, want OPERATOR_REQUIRED", code)
			}
		})
	}
}

// =============================================================================
// Test: Migration 000035 idempotency — execute twice, verify exactly 1 vellus
// Validates: Requirements 1.3, 1.5
// =============================================================================

func TestMigration035_Idempotency_Integration(t *testing.T) {
	// Read the migration SQL
	migrationSQL, err := os.ReadFile("../../migrations/000035_operator_level.up.sql")
	if err != nil {
		t.Fatalf("read migration file: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Execute migration twice
	for i := 0; i < 2; i++ {
		if _, err := env.db.ExecContext(ctx, string(migrationSQL)); err != nil {
			t.Fatalf("migration execution #%d: %v", i+1, err)
		}
	}

	// Verify exactly 1 vellus tenant
	var count int
	if err := env.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tenants WHERE slug = 'vellus'`).Scan(&count); err != nil {
		t.Fatalf("count query: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 vellus tenant after double execution, got %d", count)
	}

	// Verify correct values
	var operatorLevel int
	var plan, status string
	err = env.db.QueryRowContext(ctx,
		`SELECT operator_level, plan, status FROM tenants WHERE slug = 'vellus'`,
	).Scan(&operatorLevel, &plan, &status)
	if err != nil {
		t.Fatalf("select vellus: %v", err)
	}
	if operatorLevel != 1 {
		t.Errorf("operator_level = %d, want 1", operatorLevel)
	}
	if plan != "internal" {
		t.Errorf("plan = %q, want 'internal'", plan)
	}
	if status != "active" {
		t.Errorf("status = %q, want 'active'", status)
	}
}

// =============================================================================
// Test: operator_level write protection via store (POST/PUT equivalent)
// Validates: Requirements 1.3, 2.5
// =============================================================================

func TestOperatorLevel_WriteProtection_Integration(t *testing.T) {
	// Test CreateTenant rejects operator_level > 0
	t.Run("CreateTenant_RejectsOperatorLevel", func(t *testing.T) {
		tenant := &store.Tenant{
			ID:            uuid.Must(uuid.NewV7()),
			Slug:          fmt.Sprintf("write-prot-%s", uuid.New().String()[:8]),
			Name:          "Write Protection Test",
			Plan:          "starter",
			Status:        "active",
			OperatorLevel: 1,
		}
		err := env.tenantStore.CreateTenant(context.Background(), tenant)
		if err == nil {
			// Cleanup if it was created (shouldn't happen)
			cctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			env.db.ExecContext(cctx, `DELETE FROM tenants WHERE id = $1`, tenant.ID)
			t.Fatal("expected ErrOperatorLevelForbidden, got nil")
		}
		if err.Error() != store.ErrOperatorLevelForbidden.Error() {
			t.Errorf("expected ErrOperatorLevelForbidden, got %v", err)
		}
	})

	// Test UpdateTenant rejects operator_level > 0
	t.Run("UpdateTenant_RejectsOperatorLevel", func(t *testing.T) {
		// Create a normal tenant first
		tenantID := tempTenant(t, 0)

		err := env.tenantStore.UpdateTenant(context.Background(), tenantID, map[string]any{
			"operator_level": 1,
		})
		if err == nil {
			t.Fatal("expected ErrOperatorLevelForbidden, got nil")
		}
		if err.Error() != store.ErrOperatorLevelForbidden.Error() {
			t.Errorf("expected ErrOperatorLevelForbidden, got %v", err)
		}

		// Verify operator_level was NOT changed
		var operatorLevel int
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err = env.db.QueryRowContext(ctx,
			`SELECT operator_level FROM tenants WHERE id = $1`, tenantID,
		).Scan(&operatorLevel)
		if err != nil {
			t.Fatalf("select tenant: %v", err)
		}
		if operatorLevel != 0 {
			t.Errorf("operator_level = %d after rejected update, want 0", operatorLevel)
		}
	})

	// Test UpdateTenant with float64 operator_level > 0 (JSON decode produces float64)
	t.Run("UpdateTenant_RejectsOperatorLevel_Float64", func(t *testing.T) {
		tenantID := tempTenant(t, 0)
		err := env.tenantStore.UpdateTenant(context.Background(), tenantID, map[string]any{
			"operator_level": float64(2),
		})
		if err == nil {
			t.Fatal("expected ErrOperatorLevelForbidden for float64(2), got nil")
		}
	})
}

// =============================================================================
// PBT: Migration idempotency — ∀ N ∈ [1,5]: COUNT(slug='vellus') = 1
// **Validates: Requirements 1.3, 1.5**
// =============================================================================

func TestPBT_Migration035_IdempotentSeed_Integration(t *testing.T) {
	migrationSQL, err := os.ReadFile("../../migrations/000035_operator_level.up.sql")
	if err != nil {
		t.Fatalf("read migration file: %v", err)
	}

	rapid.Check(t, func(rt *rapid.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		n := rapid.IntRange(1, 5).Draw(rt, "executions")

		// Execute migration N times
		for i := 0; i < n; i++ {
			if _, err := env.db.ExecContext(ctx, string(migrationSQL)); err != nil {
				rt.Fatalf("migration execution #%d of %d: %v", i+1, n, err)
			}
		}

		// Invariant: exactly 1 vellus tenant
		var count int
		if err := env.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tenants WHERE slug = 'vellus'`).Scan(&count); err != nil {
			rt.Fatalf("count query: %v", err)
		}
		if count != 1 {
			rt.Fatalf("INVARIANT VIOLATED after %d executions: COUNT(slug='vellus') = %d, expected 1", n, count)
		}

		// Verify operator_level = 1
		var operatorLevel int
		if err := env.db.QueryRowContext(ctx,
			`SELECT operator_level FROM tenants WHERE slug = 'vellus'`,
		).Scan(&operatorLevel); err != nil {
			rt.Fatalf("select vellus: %v", err)
		}
		if operatorLevel != 1 {
			rt.Fatalf("INVARIANT VIOLATED after %d executions: operator_level = %d, expected 1", n, operatorLevel)
		}
	})
}

// =============================================================================
// PBT: Isolation — ∀ tenant T with operator_level=0: access to /v1/operator/* = 403
// regardless of role
// **Validates: Requirements 3.5, 3.6, 9.1**
// =============================================================================

func TestPBT_CustomerTenant_AlwaysBlocked_Integration(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate random role
		roles := []string{"viewer", "operator", "admin"}
		roleIdx := rapid.IntRange(0, 2).Draw(rt, "role_index")
		role := roles[roleIdx]

		// Create customer tenant (operator_level=0) with the drawn role
		ctx := context.Background()
		tenantID := uuid.Must(uuid.NewV7())
		slug := fmt.Sprintf("pbt-iso-%s", tenantID.String()[:8])

		_, err := env.db.ExecContext(ctx,
			`INSERT INTO tenants (id, slug, name, plan, status, settings, operator_level, created_at, updated_at)
			 VALUES ($1, $2, $3, 'starter', 'active', '{}'::JSONB, 0, NOW(), NOW())`,
			tenantID, slug, "PBT Isolation "+slug,
		)
		if err != nil {
			rt.Fatalf("create PBT tenant: %v", err)
		}
		defer func() {
			cctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			env.db.ExecContext(cctx, `DELETE FROM tenant_users WHERE tenant_id = $1`, tenantID)
			env.db.ExecContext(cctx, `DELETE FROM tenants WHERE id = $1`, tenantID)
		}()

		// Create user with the drawn role
		userID := uuid.Must(uuid.NewV7())
		email := fmt.Sprintf("pbt-iso-%s@test.local", userID.String()[:8])

		hash, hashErr := hashTestPassword()
		if hashErr != nil {
			rt.Fatalf("hash password: %v", hashErr)
		}

		_, err = env.db.ExecContext(ctx,
			`INSERT INTO users (id, email, display_name, password_hash, status, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, 'active', NOW(), NOW())
			 ON CONFLICT (email) DO UPDATE SET password_hash = EXCLUDED.password_hash`,
			userID, email, "PBT User", hash,
		)
		if err != nil {
			rt.Fatalf("create PBT user: %v", err)
		}
		defer func() {
			cctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			env.db.ExecContext(cctx, `DELETE FROM tenant_users WHERE user_id = $1`, userID)
			env.db.ExecContext(cctx, `DELETE FROM users WHERE id = $1`, userID)
		}()

		_, err = env.db.ExecContext(ctx,
			`INSERT INTO tenant_users (tenant_id, user_id, role, joined_at)
			 VALUES ($1, $2, $3, NOW())
			 ON CONFLICT (tenant_id, user_id) DO UPDATE SET role = EXCLUDED.role`,
			tenantID, userID, role,
		)
		if err != nil {
			rt.Fatalf("add PBT user to tenant: %v", err)
		}

		// Generate JWT
		token, tokenErr := generateTestToken(userID, email, tenantID, role)
		if tokenErr != nil {
			rt.Fatalf("generate PBT JWT: %v", tokenErr)
		}

		// Test all four operator endpoints — all must return 403
		endpoints := []string{
			"/v1/operator/tenants",
			"/v1/operator/tenants/" + uuid.New().String() + "/agents",
			"/v1/operator/tenants/" + uuid.New().String() + "/sessions",
			"/v1/operator/tenants/" + uuid.New().String() + "/usage?period=7d",
		}

		for _, ep := range endpoints {
			req, reqErr := http.NewRequest("GET", env.srv.URL+ep, nil)
			if reqErr != nil {
				rt.Fatalf("build request: %v", reqErr)
			}
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")

			resp, respErr := http.DefaultClient.Do(req)
			if respErr != nil {
				rt.Fatalf("GET %s: %v", ep, respErr)
			}
			defer resp.Body.Close()

			var body map[string]any
			json.NewDecoder(resp.Body).Decode(&body) //nolint:errcheck

			if resp.StatusCode != http.StatusForbidden {
				rt.Fatalf("ISOLATION VIOLATED: operator_level=0, role=%q, endpoint=%s → status=%d (want 403), body=%v",
					role, ep, resp.StatusCode, body)
			}

			code, _ := body["code"].(string)
			if code != "OPERATOR_REQUIRED" {
				rt.Fatalf("ISOLATION VIOLATED: operator_level=0, role=%q, endpoint=%s → code=%q (want OPERATOR_REQUIRED)",
					role, ep, code)
			}
		}
	})
}

// =============================================================================
// Helpers specific to this test file
// =============================================================================

// cachedIsolationHash caches the Argon2id hash to avoid expensive computation per PBT iteration.
var cachedIsolationHash string

func hashTestPassword() (string, error) {
	if cachedIsolationHash != "" {
		return cachedIsolationHash, nil
	}
	h, err := auth.HashPassword("PBT-Isolation-Test-2026!")
	if err != nil {
		return "", err
	}
	cachedIsolationHash = h
	return h, nil
}

func generateTestToken(userID uuid.UUID, email string, tenantID uuid.UUID, role string) (string, error) {
	return auth.GenerateAccessToken(auth.TokenClaims{
		UserID:   userID.String(),
		Email:    email,
		TenantID: tenantID.String(),
		Role:     role,
	}, testJWTSecret)
}
