//go:build integration

package pg_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"pgregory.net/rapid"

	"github.com/vellus-ai/argoclaw/internal/store"
	"github.com/vellus-ai/argoclaw/internal/store/pg"
	"github.com/vellus-ai/argoclaw/internal/testutil"
)

// ─────────────────────────────────────────────────────────────────────────────
// Task 2.3 — ListAllTenantsForOperator property-based tests
// **Validates: Requirements 1.4, 3.1**
//
// Property 1 (Pagination bounds):
//   ∀ (limit, offset) with limit ∈ [1,100], offset ∈ [0, total]:
//     len(result) ≤ limit AND total ≥ len(result)
//
// Property 2 (Security — cross-tenant guard):
//   ∀ ctx without WithCrossTenant:
//     ListAllTenantsForOperator(ctx, _, _) = ErrTenantRequired
// ─────────────────────────────────────────────────────────────────────────────

// TestListAllTenantsForOperator_PBT_PaginationBounds verifies that for any
// valid (limit, offset) pair, the result length never exceeds limit and the
// total count is always >= the number of returned rows.
func TestListAllTenantsForOperator_PBT_PaginationBounds(t *testing.T) {
	db := testutil.SetupDB(t)
	ts := pg.NewPGTenantStore(db)

	// Seed a few tenants to ensure non-empty results
	for i := 0; i < 5; i++ {
		slug := "pbt-page-" + uuid.Must(uuid.NewV7()).String()[:8]
		testutil.CreateTestTenant(t, db, slug, "PBT Pagination")
	}

	// appsec:cross-tenant-bypass — PBT test: verifying pagination invariants
	ctx := store.WithCrossTenant(context.Background())

	rapid.Check(t, func(t *rapid.T) {
		limit := rapid.IntRange(1, 100).Draw(t, "limit")
		offset := rapid.IntRange(0, 200).Draw(t, "offset")

		result, total, err := ts.ListAllTenantsForOperator(ctx, limit, offset)
		if err != nil {
			t.Fatalf("ListAllTenantsForOperator(limit=%d, offset=%d): %v", limit, offset, err)
		}

		// Property: len(result) ≤ limit
		if len(result) > limit {
			t.Fatalf("len(result) = %d > limit = %d", len(result), limit)
		}

		// Property: total ≥ len(result)
		// Note: when offset > total rows, COUNT(*) OVER() returns 0 rows,
		// so total=0 and len(result)=0, which still satisfies total >= len(result).
		if total < len(result) {
			t.Fatalf("total = %d < len(result) = %d", total, len(result))
		}
	})
}

// TestListAllTenantsForOperator_PBT_SecurityGuard verifies that for any
// context WITHOUT WithCrossTenant, the method always returns ErrTenantRequired.
func TestListAllTenantsForOperator_PBT_SecurityGuard(t *testing.T) {
	db := testutil.SetupDB(t)
	ts := pg.NewPGTenantStore(db)

	rapid.Check(t, func(t *rapid.T) {
		limit := rapid.IntRange(1, 100).Draw(t, "limit")
		offset := rapid.IntRange(0, 200).Draw(t, "offset")

		// Build a context WITHOUT WithCrossTenant — may have other values
		ctx := context.Background()
		if rapid.Bool().Draw(t, "withTenantID") {
			ctx = store.WithTenantID(ctx, uuid.New())
		}
		if rapid.Bool().Draw(t, "withUserID") {
			ctx = store.WithUserID(ctx, rapid.String().Draw(t, "userID"))
		}
		if rapid.Bool().Draw(t, "withOperatorMode") {
			ctx = store.WithOperatorMode(ctx, uuid.New())
		}

		_, _, err := ts.ListAllTenantsForOperator(ctx, limit, offset)
		if !errors.Is(err, store.ErrTenantRequired) {
			t.Fatalf("ListAllTenantsForOperator without cross-tenant (limit=%d, offset=%d) = %v, want ErrTenantRequired",
				limit, offset, err)
		}
	})
}
