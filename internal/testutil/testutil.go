//go:build integration

package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/vellus-ai/argoclaw/internal/store"
)

const defaultDSN = "postgres://argoclaw:argoclaw@localhost:5432/argoclaw_test?sslmode=disable"

var (
	setupOnce sync.Once
	sharedDB  *sql.DB
	setupErr  error
)

// TestDSN returns the test database DSN from ARGOCLAW_TEST_DSN or the default.
func TestDSN() string {
	if dsn := os.Getenv("ARGOCLAW_TEST_DSN"); dsn != "" {
		return dsn
	}
	return defaultDSN
}

// SetupDB returns a shared *sql.DB connected to the test database.
// Idempotent: subsequent calls return the same connection.
// Tests that need isolation should use CleanupTenantData after each test.
func SetupDB(t *testing.T) *sql.DB {
	t.Helper()
	setupOnce.Do(func() {
		dsn := TestDSN()
		db, err := sql.Open("pgx", dsn)
		if err != nil {
			setupErr = fmt.Errorf("open test DB: %w", err)
			return
		}
		db.SetMaxOpenConns(10)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			setupErr = fmt.Errorf("ping test DB %q: %w", dsn, err)
			db.Close()
			return
		}
		sharedDB = db
	})
	if setupErr != nil {
		t.Skipf("SKIP: test DB unavailable: %v", setupErr)
	}
	return sharedDB
}

// TenantCtx returns a context with the given tenant UUID injected.
func TenantCtx(tenantID uuid.UUID) context.Context {
	return store.WithTenantID(context.Background(), tenantID)
}

// CleanupTenantData deletes all data for the given tenant from the 9 tenant-scoped tables.
// Safe to call multiple times (idempotent).
func CleanupTenantData(t *testing.T, db *sql.DB, tenantID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	tables := []string{
		"cron_jobs",
		"sessions",
		"custom_tools",
		"mcp_servers",
		"channel_instances",
		"skills",
		"agent_teams",
		"agents",
		"llm_providers",
	}
	for _, table := range tables {
		if _, err := db.ExecContext(ctx,
			fmt.Sprintf("DELETE FROM %s WHERE tenant_id = $1", table), tenantID,
		); err != nil {
			t.Logf("cleanup %s for tenant %s: %v", table, tenantID, err)
		}
	}
}

// CreateTestTenant inserts a minimal tenant row and returns its UUID.
func CreateTestTenant(t *testing.T, db *sql.DB, slug, name string) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO tenants (id, slug, name, plan, status, created_at, updated_at)
		 VALUES ($1, $2, $3, 'starter', 'active', NOW(), NOW())`,
		id, slug, name,
	)
	if err != nil {
		t.Fatalf("CreateTestTenant %q: %v", slug, err)
	}
	t.Cleanup(func() {
		db.ExecContext(context.Background(), "DELETE FROM tenants WHERE id = $1", id)
	})
	return id
}
