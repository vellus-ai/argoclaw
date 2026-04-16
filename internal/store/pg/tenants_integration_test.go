//go:build integration

package pg_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/vellus-ai/argoclaw/internal/store"
	"github.com/vellus-ai/argoclaw/internal/store/pg"
	"github.com/vellus-ai/argoclaw/internal/testutil"
)

// TestGetByID_PopulatesOperatorLevel verifies that GetByID returns the correct
// OperatorLevel from the database (default 0 for regular tenants).
func TestGetByID_PopulatesOperatorLevel(t *testing.T) {
	db := testutil.SetupDB(t)
	slug := "ol-byid-" + uuid.Must(uuid.NewV7()).String()[:8]
	tenantID := testutil.CreateTestTenant(t, db, slug, "OperatorLevel GetByID")

	ts := pg.NewPGTenantStore(db)
	// appsec:cross-tenant-bypass — test setup: GetByID needs no tenant filter
	ctx := store.WithCrossTenant(context.Background())

	tenant, err := ts.GetByID(ctx, tenantID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if tenant == nil {
		t.Fatal("GetByID returned nil")
	}
	if tenant.OperatorLevel != 0 {
		t.Errorf("OperatorLevel = %d, want 0 for regular tenant", tenant.OperatorLevel)
	}
}

// TestGetBySlug_PopulatesOperatorLevel verifies that GetBySlug returns the correct
// OperatorLevel from the database.
func TestGetBySlug_PopulatesOperatorLevel(t *testing.T) {
	db := testutil.SetupDB(t)
	slug := "ol-byslug-" + uuid.Must(uuid.NewV7()).String()[:8]
	testutil.CreateTestTenant(t, db, slug, "OperatorLevel GetBySlug")

	ts := pg.NewPGTenantStore(db)
	ctx := store.WithCrossTenant(context.Background())

	tenant, err := ts.GetBySlug(ctx, slug)
	if err != nil {
		t.Fatalf("GetBySlug: %v", err)
	}
	if tenant == nil {
		t.Fatal("GetBySlug returned nil")
	}
	if tenant.OperatorLevel != 0 {
		t.Errorf("OperatorLevel = %d, want 0 for regular tenant", tenant.OperatorLevel)
	}
}

// TestGetByID_OperatorTenant verifies that a tenant with operator_level=1
// (set directly in DB) is correctly returned by GetByID.
func TestGetByID_OperatorTenant(t *testing.T) {
	db := testutil.SetupDB(t)
	slug := "ol-op-" + uuid.Must(uuid.NewV7()).String()[:8]
	tenantID := testutil.CreateTestTenant(t, db, slug, "Operator Tenant")

	// Set operator_level=1 directly in DB (simulating migration seed)
	_, err := db.ExecContext(context.Background(),
		"UPDATE tenants SET operator_level = 1 WHERE id = $1", tenantID)
	if err != nil {
		t.Fatalf("set operator_level: %v", err)
	}

	ts := pg.NewPGTenantStore(db)
	ctx := store.WithCrossTenant(context.Background())

	tenant, err := ts.GetByID(ctx, tenantID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if tenant.OperatorLevel != 1 {
		t.Errorf("OperatorLevel = %d, want 1 for operator tenant", tenant.OperatorLevel)
	}
}

// TestListTenants_PopulatesOperatorLevel verifies that ListTenants includes
// OperatorLevel in the returned tenant structs.
func TestListTenants_PopulatesOperatorLevel(t *testing.T) {
	db := testutil.SetupDB(t)
	slug := "ol-list-" + uuid.Must(uuid.NewV7()).String()[:8]
	tenantID := testutil.CreateTestTenant(t, db, slug, "OperatorLevel List")

	// Set operator_level=1 directly in DB
	_, err := db.ExecContext(context.Background(),
		"UPDATE tenants SET operator_level = 1 WHERE id = $1", tenantID)
	if err != nil {
		t.Fatalf("set operator_level: %v", err)
	}

	ts := pg.NewPGTenantStore(db)
	ctx := store.WithCrossTenant(context.Background())

	tenants, err := ts.ListTenants(ctx)
	if err != nil {
		t.Fatalf("ListTenants: %v", err)
	}

	var found bool
	for _, tenant := range tenants {
		if tenant.ID == tenantID {
			found = true
			if tenant.OperatorLevel != 1 {
				t.Errorf("OperatorLevel = %d, want 1 for operator tenant in list", tenant.OperatorLevel)
			}
			break
		}
	}
	if !found {
		t.Error("operator tenant not found in ListTenants result")
	}
}

// TestCreateTenant_RejectsOperatorLevel_Integration verifies the store-level
// validation against a real DB connection.
func TestCreateTenant_RejectsOperatorLevel_Integration(t *testing.T) {
	db := testutil.SetupDB(t)
	ts := pg.NewPGTenantStore(db)

	tenant := &store.Tenant{
		Slug:          "ol-reject-" + uuid.Must(uuid.NewV7()).String()[:8],
		Name:          "Should Be Rejected",
		Plan:          "starter",
		Status:        "active",
		OperatorLevel: 1,
	}
	err := ts.CreateTenant(context.Background(), tenant)
	if !errors.Is(err, store.ErrOperatorLevelForbidden) {
		t.Errorf("CreateTenant(operator_level=1) = %v, want ErrOperatorLevelForbidden", err)
	}
}

// TestUpdateTenant_RejectsOperatorLevel_Integration verifies the store-level
// validation for UpdateTenant against a real DB connection.
func TestUpdateTenant_RejectsOperatorLevel_Integration(t *testing.T) {
	db := testutil.SetupDB(t)
	slug := "ol-upd-" + uuid.Must(uuid.NewV7()).String()[:8]
	tenantID := testutil.CreateTestTenant(t, db, slug, "Update Reject")

	ts := pg.NewPGTenantStore(db)
	err := ts.UpdateTenant(context.Background(), tenantID, map[string]any{
		"operator_level": 1,
	})
	if !errors.Is(err, store.ErrOperatorLevelForbidden) {
		t.Errorf("UpdateTenant(operator_level=1) = %v, want ErrOperatorLevelForbidden", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Task 2.3 — ListAllTenantsForOperator integration tests
// **Validates: Requirements 1.4, 3.1**
// ─────────────────────────────────────────────────────────────────────────────

// TestListAllTenantsForOperator_WithCrossTenant verifies that the method
// returns all tenants when WithCrossTenant is active in the context.
func TestListAllTenantsForOperator_WithCrossTenant(t *testing.T) {
	db := testutil.SetupDB(t)
	ts := pg.NewPGTenantStore(db)

	// Create 3 test tenants
	slugs := make([]string, 3)
	ids := make([]uuid.UUID, 3)
	for i := 0; i < 3; i++ {
		slugs[i] = "op-list-" + uuid.Must(uuid.NewV7()).String()[:8]
		ids[i] = testutil.CreateTestTenant(t, db, slugs[i], "OpList Test")
	}

	// appsec:cross-tenant-bypass — test: verifying operator listing
	ctx := store.WithCrossTenant(context.Background())
	tenants, total, err := ts.ListAllTenantsForOperator(ctx, 100, 0)
	if err != nil {
		t.Fatalf("ListAllTenantsForOperator: %v", err)
	}
	if total < 3 {
		t.Errorf("total = %d, want >= 3", total)
	}

	// Verify all 3 test tenants are in the result
	found := map[uuid.UUID]bool{}
	for _, tenant := range tenants {
		found[tenant.ID] = true
	}
	for _, id := range ids {
		if !found[id] {
			t.Errorf("tenant %v not found in ListAllTenantsForOperator result", id)
		}
	}
}

// TestListAllTenantsForOperator_WithoutCrossTenant verifies that the method
// returns ErrTenantRequired when WithCrossTenant is NOT in the context.
func TestListAllTenantsForOperator_WithoutCrossTenant(t *testing.T) {
	db := testutil.SetupDB(t)
	ts := pg.NewPGTenantStore(db)

	// No WithCrossTenant — should fail
	ctx := context.Background()
	_, _, err := ts.ListAllTenantsForOperator(ctx, 10, 0)
	if !errors.Is(err, store.ErrTenantRequired) {
		t.Errorf("ListAllTenantsForOperator without cross-tenant = %v, want ErrTenantRequired", err)
	}

	// Also test with a regular tenant context (not cross-tenant)
	ctx = store.WithTenantID(context.Background(), uuid.New())
	_, _, err = ts.ListAllTenantsForOperator(ctx, 10, 0)
	if !errors.Is(err, store.ErrTenantRequired) {
		t.Errorf("ListAllTenantsForOperator with tenant context = %v, want ErrTenantRequired", err)
	}
}

// TestListAllTenantsForOperator_Pagination verifies that limit, offset, and
// total count work correctly for paginated results.
func TestListAllTenantsForOperator_Pagination(t *testing.T) {
	db := testutil.SetupDB(t)
	ts := pg.NewPGTenantStore(db)

	// Create 5 test tenants to have enough for pagination
	for i := 0; i < 5; i++ {
		slug := "op-page-" + uuid.Must(uuid.NewV7()).String()[:8]
		testutil.CreateTestTenant(t, db, slug, "Pagination Test")
	}

	// appsec:cross-tenant-bypass — test: verifying pagination
	ctx := store.WithCrossTenant(context.Background())

	// Get total count first
	_, totalAll, err := ts.ListAllTenantsForOperator(ctx, 1000, 0)
	if err != nil {
		t.Fatalf("ListAllTenantsForOperator (all): %v", err)
	}
	if totalAll < 5 {
		t.Fatalf("total = %d, want >= 5", totalAll)
	}

	// Page 1: limit=2, offset=0
	page1, total1, err := ts.ListAllTenantsForOperator(ctx, 2, 0)
	if err != nil {
		t.Fatalf("page1: %v", err)
	}
	if len(page1) != 2 {
		t.Errorf("page1 len = %d, want 2", len(page1))
	}
	if total1 != totalAll {
		t.Errorf("page1 total = %d, want %d", total1, totalAll)
	}

	// Page 2: limit=2, offset=2
	page2, total2, err := ts.ListAllTenantsForOperator(ctx, 2, 2)
	if err != nil {
		t.Fatalf("page2: %v", err)
	}
	if len(page2) != 2 {
		t.Errorf("page2 len = %d, want 2", len(page2))
	}
	if total2 != totalAll {
		t.Errorf("page2 total = %d, want %d", total2, totalAll)
	}

	// Ensure page1 and page2 have different tenants (no overlap)
	page1IDs := map[uuid.UUID]bool{}
	for _, tenant := range page1 {
		page1IDs[tenant.ID] = true
	}
	for _, tenant := range page2 {
		if page1IDs[tenant.ID] {
			t.Errorf("tenant %v appears in both page1 and page2", tenant.ID)
		}
	}

	// Offset beyond total: should return empty
	beyond, totalBeyond, err := ts.ListAllTenantsForOperator(ctx, 10, totalAll+100)
	if err != nil {
		t.Fatalf("beyond: %v", err)
	}
	if len(beyond) != 0 {
		t.Errorf("beyond len = %d, want 0", len(beyond))
	}
	// When offset is beyond total, COUNT(*) OVER() returns 0 rows so total is 0
	// This is expected behavior of window functions with no matching rows
	_ = totalBeyond
}

// TestListAllTenantsForOperator_OperatorLevelPopulated verifies that the
// OperatorLevel field is correctly populated in the returned tenant structs.
func TestListAllTenantsForOperator_OperatorLevelPopulated(t *testing.T) {
	db := testutil.SetupDB(t)
	ts := pg.NewPGTenantStore(db)

	// Create a regular tenant and an operator tenant
	regularSlug := "op-lvl-reg-" + uuid.Must(uuid.NewV7()).String()[:8]
	regularID := testutil.CreateTestTenant(t, db, regularSlug, "Regular Tenant")

	operatorSlug := "op-lvl-op-" + uuid.Must(uuid.NewV7()).String()[:8]
	operatorID := testutil.CreateTestTenant(t, db, operatorSlug, "Operator Tenant")
	_, err := db.ExecContext(context.Background(),
		"UPDATE tenants SET operator_level = 1 WHERE id = $1", operatorID)
	if err != nil {
		t.Fatalf("set operator_level: %v", err)
	}

	// appsec:cross-tenant-bypass — test: verifying operator_level in results
	ctx := store.WithCrossTenant(context.Background())
	tenants, _, err := ts.ListAllTenantsForOperator(ctx, 1000, 0)
	if err != nil {
		t.Fatalf("ListAllTenantsForOperator: %v", err)
	}

	for _, tenant := range tenants {
		if tenant.ID == regularID && tenant.OperatorLevel != 0 {
			t.Errorf("regular tenant OperatorLevel = %d, want 0", tenant.OperatorLevel)
		}
		if tenant.ID == operatorID && tenant.OperatorLevel != 1 {
			t.Errorf("operator tenant OperatorLevel = %d, want 1", tenant.OperatorLevel)
		}
	}
}
