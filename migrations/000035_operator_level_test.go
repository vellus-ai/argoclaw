//go:build integration

package migrations_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"pgregory.net/rapid"
)

// **Validates: Requirements 1.1, 1.3, 1.5, 7.1, 7.5**
//
// Integration tests for migration 000035_operator_level:
//   - Column operator_level exists on tenants table
//   - Tenant vellus is seeded with operator_level=1, plan=internal, status=active
//   - Re-execution of the migration does not create duplicates (idempotency)
//
// PBT property:
//   ∀ N executions (N ∈ [1,5]): COUNT(tenants WHERE slug='vellus') = 1 AND operator_level = 1

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func testDSN035() string {
	if dsn := os.Getenv("ARGOCLAW_TEST_DSN"); dsn != "" {
		return dsn
	}
	return "postgres://argoclaw:argoclaw@localhost:5432/argoclaw_test?sslmode=disable"
}

func openDB035(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", testDSN035())
	if err != nil {
		t.Skipf("SKIP: cannot open test DB: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Skipf("SKIP: test DB unavailable: %v", err)
	}
	return db
}

func execSQL035(db *sql.DB, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read SQL file %q: %w", path, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := db.ExecContext(ctx, string(data)); err != nil {
		return fmt.Errorf("exec SQL file %q: %w", path, err)
	}
	return nil
}

// ensurePrerequisites applies migrations 027 (tenants table) through 034
// so that migration 035 has the schema it depends on.
// In practice, the test DB should already have all prior migrations applied.
// This helper is a safety net — it only applies 027 if the tenants table
// does not exist yet.
func ensureTenantsTable(db *sql.DB) error {
	var exists bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = 'tenants'
		)
	`).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check tenants table: %w", err)
	}
	if !exists {
		if err := execSQL035(db, "000027_multi_tenancy.up.sql"); err != nil {
			return fmt.Errorf("prerequisite migration 027: %w", err)
		}
	}
	return nil
}

// rollback035 removes migration 035 artifacts so tests start clean.
func rollback035(db *sql.DB) {
	execSQL035(db, "000035_operator_level.down.sql") //nolint:errcheck
}

// ─────────────────────────────────────────────────────────────────────────────
// Test: Column operator_level exists after migration
// ─────────────────────────────────────────────────────────────────────────────

func TestMigration035_ColumnExists(t *testing.T) {
	db := openDB035(t)
	defer db.Close()

	if err := ensureTenantsTable(db); err != nil {
		t.Fatalf("prerequisite: %v", err)
	}
	t.Cleanup(func() { rollback035(db) })

	// Apply migration 035
	if err := execSQL035(db, "000035_operator_level.up.sql"); err != nil {
		t.Fatalf("apply migration 035 up: %v", err)
	}

	// Verify column exists with correct type and default
	var dataType string
	var isNullable string
	var columnDefault sql.NullString
	err := db.QueryRow(`
		SELECT data_type, is_nullable, column_default
		FROM information_schema.columns
		WHERE table_schema = 'public'
		  AND table_name = 'tenants'
		  AND column_name = 'operator_level'
	`).Scan(&dataType, &isNullable, &columnDefault)
	if err != nil {
		t.Fatalf("operator_level column not found: %v", err)
	}

	if dataType != "integer" {
		t.Errorf("expected data_type=integer, got %q", dataType)
	}
	if isNullable != "NO" {
		t.Errorf("expected is_nullable=NO, got %q", isNullable)
	}
	if !columnDefault.Valid || columnDefault.String != "0" {
		t.Errorf("expected column_default=0, got %v", columnDefault)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test: Tenant vellus seeded correctly
// ─────────────────────────────────────────────────────────────────────────────

func TestMigration035_VellusTenantSeeded(t *testing.T) {
	db := openDB035(t)
	defer db.Close()

	if err := ensureTenantsTable(db); err != nil {
		t.Fatalf("prerequisite: %v", err)
	}
	t.Cleanup(func() { rollback035(db) })

	if err := execSQL035(db, "000035_operator_level.up.sql"); err != nil {
		t.Fatalf("apply migration 035 up: %v", err)
	}

	var slug, name, plan, status string
	var operatorLevel int
	err := db.QueryRow(`
		SELECT slug, name, plan, status, operator_level
		FROM tenants WHERE slug = 'vellus'
	`).Scan(&slug, &name, &plan, &status, &operatorLevel)
	if err != nil {
		t.Fatalf("vellus tenant not found: %v", err)
	}

	if slug != "vellus" {
		t.Errorf("expected slug=vellus, got %q", slug)
	}
	if name != "Vellus AI" {
		t.Errorf("expected name='Vellus AI', got %q", name)
	}
	if plan != "internal" {
		t.Errorf("expected plan=internal, got %q", plan)
	}
	if status != "active" {
		t.Errorf("expected status=active, got %q", status)
	}
	if operatorLevel != 1 {
		t.Errorf("expected operator_level=1, got %d", operatorLevel)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test: Idempotency — re-execution does not create duplicates
// ─────────────────────────────────────────────────────────────────────────────

func TestMigration035_Idempotent(t *testing.T) {
	db := openDB035(t)
	defer db.Close()

	if err := ensureTenantsTable(db); err != nil {
		t.Fatalf("prerequisite: %v", err)
	}
	t.Cleanup(func() { rollback035(db) })

	// Apply migration twice
	if err := execSQL035(db, "000035_operator_level.up.sql"); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	if err := execSQL035(db, "000035_operator_level.up.sql"); err != nil {
		t.Fatalf("second apply: %v", err)
	}

	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM tenants WHERE slug = 'vellus'`).Scan(&count)
	if err != nil {
		t.Fatalf("count query: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 vellus tenant after double apply, got %d", count)
	}

	// Verify values are still correct after re-apply
	var operatorLevel int
	var status string
	err = db.QueryRow(`SELECT operator_level, status FROM tenants WHERE slug = 'vellus'`).Scan(&operatorLevel, &status)
	if err != nil {
		t.Fatalf("select vellus: %v", err)
	}
	if operatorLevel != 1 {
		t.Errorf("expected operator_level=1 after re-apply, got %d", operatorLevel)
	}
	if status != "active" {
		t.Errorf("expected status=active after re-apply, got %q", status)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test: Partial index exists
// ─────────────────────────────────────────────────────────────────────────────

func TestMigration035_PartialIndexExists(t *testing.T) {
	db := openDB035(t)
	defer db.Close()

	if err := ensureTenantsTable(db); err != nil {
		t.Fatalf("prerequisite: %v", err)
	}
	t.Cleanup(func() { rollback035(db) })

	if err := execSQL035(db, "000035_operator_level.up.sql"); err != nil {
		t.Fatalf("apply migration 035 up: %v", err)
	}

	var indexName string
	err := db.QueryRow(`
		SELECT indexname FROM pg_indexes
		WHERE schemaname = 'public'
		  AND tablename = 'tenants'
		  AND indexname = 'idx_tenants_operator_level'
	`).Scan(&indexName)
	if err != nil {
		t.Fatalf("partial index idx_tenants_operator_level not found: %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test: Down migration removes column and tenant
// ─────────────────────────────────────────────────────────────────────────────

func TestMigration035_DownRollback(t *testing.T) {
	db := openDB035(t)
	defer db.Close()

	if err := ensureTenantsTable(db); err != nil {
		t.Fatalf("prerequisite: %v", err)
	}

	// Apply up then down
	if err := execSQL035(db, "000035_operator_level.up.sql"); err != nil {
		t.Fatalf("apply up: %v", err)
	}
	if err := execSQL035(db, "000035_operator_level.down.sql"); err != nil {
		t.Fatalf("apply down: %v", err)
	}

	// Column should not exist
	var exists bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'public'
			  AND table_name = 'tenants'
			  AND column_name = 'operator_level'
		)
	`).Scan(&exists)
	if err != nil {
		t.Fatalf("check column: %v", err)
	}
	if exists {
		t.Error("operator_level column should not exist after down migration")
	}

	// Vellus tenant should not exist
	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM tenants WHERE slug = 'vellus'`).Scan(&count)
	if err != nil {
		t.Fatalf("count query: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 vellus tenants after down migration, got %d", count)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// PBT: ∀ N executions (N ∈ [1,5]): COUNT(vellus) = 1 AND operator_level = 1
// ─────────────────────────────────────────────────────────────────────────────

// TestProperty_Migration035_IdempotentSeed verifies the formal property:
//
//	∀ execuções N da migration (N ≥ 1):
//	  COUNT(tenants WHERE slug='vellus') = 1 AND operator_level = 1
//
// The rapid property test generates N ∈ [1,5] and applies the migration
// N times, then asserts the invariant holds.
//
// **Validates: Requirements 1.1, 1.2, 1.3, 1.5, 1.6, 7.1, 7.5**
func TestProperty_Migration035_IdempotentSeed(t *testing.T) {
	db := openDB035(t)
	defer db.Close()

	if err := ensureTenantsTable(db); err != nil {
		t.Fatalf("prerequisite: %v", err)
	}

	rapid.Check(t, func(rt *rapid.T) {
		// Ensure clean state before each property check
		rollback035(db)

		n := rapid.IntRange(1, 5).Draw(rt, "executions")

		// Apply migration N times
		for i := 0; i < n; i++ {
			if err := execSQL035(db, "000035_operator_level.up.sql"); err != nil {
				rt.Fatalf("migration apply #%d: %v", i+1, err)
			}
		}

		// Invariant: exactly 1 vellus tenant
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM tenants WHERE slug = 'vellus'`).Scan(&count); err != nil {
			rt.Fatalf("count query: %v", err)
		}
		if count != 1 {
			rt.Fatalf("invariant violated after %d executions: COUNT(vellus) = %d, expected 1", n, count)
		}

		// Invariant: operator_level = 1
		var operatorLevel int
		var plan, status string
		if err := db.QueryRow(`
			SELECT operator_level, plan, status FROM tenants WHERE slug = 'vellus'
		`).Scan(&operatorLevel, &plan, &status); err != nil {
			rt.Fatalf("select vellus: %v", err)
		}
		if operatorLevel != 1 {
			rt.Fatalf("invariant violated after %d executions: operator_level = %d, expected 1", n, operatorLevel)
		}
		if plan != "internal" {
			rt.Fatalf("invariant violated after %d executions: plan = %q, expected 'internal'", n, plan)
		}
		if status != "active" {
			rt.Fatalf("invariant violated after %d executions: status = %q, expected 'active'", n, status)
		}

		// Cleanup for next iteration
		rollback035(db)
	})
}
