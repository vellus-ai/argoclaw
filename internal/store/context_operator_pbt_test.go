package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"pgregory.net/rapid"

	"github.com/vellus-ai/argoclaw/internal/store"
)

// ─────────────────────────────────────────────────────────────────────────────
// Task 2.2 — OperatorMode context helpers property-based tests
// **Validates: Requirements 2.1, 2.3, 2.4**
//
// Property 1: ∀ uuid U (U ≠ uuid.Nil):
//   WithOperatorMode(ctx, U) → OperatorModeFromContext(ctx) = U
//   AND IsOperatorMode(ctx) = true
//
// Property 2: ∀ ctx sem WithOperatorMode:
//   OperatorModeFromContext(ctx) = uuid.Nil
//   AND IsOperatorMode(ctx) = false
// ─────────────────────────────────────────────────────────────────────────────

// drawNonNilUUID generates a random UUID that is guaranteed to not be uuid.Nil.
func drawNonNilUUID(t *rapid.T, label string) uuid.UUID {
	var b [16]byte
	for i := range b {
		b[i] = rapid.Byte().Draw(t, label)
	}
	id := uuid.UUID(b)
	// Ensure non-nil by setting at least one byte if all zeros
	if id == uuid.Nil {
		id[0] = 1
	}
	return id
}

// TestOperatorMode_P1_RoundTrip verifies that for any non-nil UUID,
// WithOperatorMode followed by OperatorModeFromContext returns the same UUID,
// and IsOperatorMode returns true.
func TestOperatorMode_P1_RoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		operatorID := drawNonNilUUID(t, "operatorTenantID")
		ctx := store.WithOperatorMode(context.Background(), operatorID)

		got := store.OperatorModeFromContext(ctx)
		if got != operatorID {
			t.Fatalf("round-trip failed: WithOperatorMode(%v) → OperatorModeFromContext = %v", operatorID, got)
		}
		if !store.IsOperatorMode(ctx) {
			t.Fatalf("IsOperatorMode = false after WithOperatorMode(%v), want true", operatorID)
		}
	})
}

// TestOperatorMode_P2_CleanContext verifies that a context without
// WithOperatorMode always returns uuid.Nil and IsOperatorMode = false.
func TestOperatorMode_P2_CleanContext(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Build a context with random other values but no operator mode
		ctx := context.Background()

		// Optionally add cross-tenant and tenant ID to ensure no interference
		if rapid.Bool().Draw(t, "withCrossTenant") {
			ctx = store.WithCrossTenant(ctx)
		}
		if rapid.Bool().Draw(t, "withTenantID") {
			tenantID := drawNonNilUUID(t, "tenantID")
			ctx = store.WithTenantID(ctx, tenantID)
		}
		if rapid.Bool().Draw(t, "withUserID") {
			ctx = store.WithUserID(ctx, rapid.String().Draw(t, "userID"))
		}

		got := store.OperatorModeFromContext(ctx)
		if got != uuid.Nil {
			t.Fatalf("OperatorModeFromContext = %v on clean context, want uuid.Nil", got)
		}
		if store.IsOperatorMode(ctx) {
			t.Fatal("IsOperatorMode = true on clean context, want false")
		}
	})
}

// TestOperatorMode_P3_CompositionWithCrossTenant verifies that WithOperatorMode
// never interferes with WithCrossTenant — both values are independently recoverable.
func TestOperatorMode_P3_CompositionWithCrossTenant(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		operatorID := drawNonNilUUID(t, "operatorTenantID")
		tenantID := drawNonNilUUID(t, "tenantID")

		ctx := context.Background()
		ctx = store.WithCrossTenant(ctx)
		ctx = store.WithTenantID(ctx, tenantID)
		ctx = store.WithOperatorMode(ctx, operatorID)

		// CrossTenant must be preserved
		if !store.IsCrossTenant(ctx) {
			t.Fatal("IsCrossTenant = false after WithOperatorMode, must be preserved")
		}
		// TenantID must be preserved
		if store.TenantIDFromContext(ctx) != tenantID {
			t.Fatalf("TenantIDFromContext = %v, want %v", store.TenantIDFromContext(ctx), tenantID)
		}
		// OperatorMode must be active with correct ID
		if store.OperatorModeFromContext(ctx) != operatorID {
			t.Fatalf("OperatorModeFromContext = %v, want %v", store.OperatorModeFromContext(ctx), operatorID)
		}
		if !store.IsOperatorMode(ctx) {
			t.Fatal("IsOperatorMode = false, want true")
		}
	})
}
