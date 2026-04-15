package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/vellus-ai/argoclaw/internal/store"
)

func TestWithOperatorMode_SetsValue(t *testing.T) {
	ctx := context.Background()
	operatorTenantID := uuid.New()

	ctx = store.WithOperatorMode(ctx, operatorTenantID)

	got := store.OperatorModeFromContext(ctx)
	if got != operatorTenantID {
		t.Errorf("OperatorModeFromContext = %v, want %v", got, operatorTenantID)
	}
}

func TestOperatorModeFromContext_ReturnsNilWhenNotSet(t *testing.T) {
	ctx := context.Background()

	got := store.OperatorModeFromContext(ctx)
	if got != uuid.Nil {
		t.Errorf("OperatorModeFromContext = %v, want uuid.Nil", got)
	}
}

func TestIsOperatorMode_FalseWhenNotSet(t *testing.T) {
	ctx := context.Background()

	if store.IsOperatorMode(ctx) {
		t.Error("IsOperatorMode = true, want false when not set")
	}
}

func TestIsOperatorMode_TrueWhenSet(t *testing.T) {
	ctx := context.Background()
	operatorTenantID := uuid.New()

	ctx = store.WithOperatorMode(ctx, operatorTenantID)

	if !store.IsOperatorMode(ctx) {
		t.Error("IsOperatorMode = false, want true when set")
	}
}

func TestWithOperatorMode_DoesNotOverrideCrossTenant(t *testing.T) {
	ctx := context.Background()
	operatorTenantID := uuid.New()
	tenantID := uuid.New()

	// Set cross-tenant first (existing pattern), then operator mode
	ctx = store.WithCrossTenant(ctx)
	ctx = store.WithTenantID(ctx, tenantID)
	ctx = store.WithOperatorMode(ctx, operatorTenantID)

	// CrossTenant must still be true
	if !store.IsCrossTenant(ctx) {
		t.Error("IsCrossTenant = false after WithOperatorMode, cross-tenant context must be preserved")
	}

	// TenantID must still be set
	if store.TenantIDFromContext(ctx) != tenantID {
		t.Errorf("TenantIDFromContext = %v, want %v", store.TenantIDFromContext(ctx), tenantID)
	}

	// OperatorMode must be active
	if !store.IsOperatorMode(ctx) {
		t.Error("IsOperatorMode = false after WithOperatorMode")
	}

	// Operator tenant ID must match
	if got := store.OperatorModeFromContext(ctx); got != operatorTenantID {
		t.Errorf("OperatorModeFromContext = %v, want %v", got, operatorTenantID)
	}
}

func TestWithOperatorMode_NilUUIDNotConsideredActive(t *testing.T) {
	ctx := context.Background()

	// Explicitly set uuid.Nil — should not be considered active
	ctx = store.WithOperatorMode(ctx, uuid.Nil)

	if store.IsOperatorMode(ctx) {
		t.Error("IsOperatorMode = true when set to uuid.Nil, want false")
	}
}
