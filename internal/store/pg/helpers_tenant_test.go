package pg

import (
	"context"
	"testing"
	"testing/quick"

	"github.com/google/uuid"

	"github.com/vellus-ai/argoclaw/internal/store"
)

// --- Unit tests for requireTenantID (Task 1.1) ---

func TestRequireTenantID_ValidTenant(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	ctx := store.WithTenantID(context.Background(), tenantID)

	got, err := requireTenantID(ctx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != tenantID {
		t.Errorf("expected %s, got %s", tenantID, got)
	}
}

func TestRequireTenantID_EmptyContext_ReturnsErrTenantRequired(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	_, err := requireTenantID(ctx)
	if err == nil {
		t.Fatal("expected ErrTenantRequired, got nil")
	}
	if err != store.ErrTenantRequired {
		t.Errorf("expected ErrTenantRequired, got %v", err)
	}
}

func TestRequireTenantID_NilUUID_ReturnsErrTenantRequired(t *testing.T) {
	t.Parallel()
	ctx := store.WithTenantID(context.Background(), uuid.Nil)

	_, err := requireTenantID(ctx)
	if err == nil {
		t.Fatal("expected ErrTenantRequired, got nil")
	}
	if err != store.ErrTenantRequired {
		t.Errorf("expected ErrTenantRequired, got %v", err)
	}
}

func TestRequireTenantID_CrossTenant_ReturnsNilNoError(t *testing.T) {
	t.Parallel()
	ctx := store.WithCrossTenant(context.Background())

	got, err := requireTenantID(ctx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != uuid.Nil {
		t.Errorf("expected uuid.Nil for cross-tenant, got %s", got)
	}
}

func TestRequireTenantID_CrossTenantWithTenantID_PrioritizesCrossTenant(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	ctx := store.WithTenantID(context.Background(), tenantID)
	ctx = store.WithCrossTenant(ctx)

	got, err := requireTenantID(ctx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	// CrossTenant bypasses tenant check entirely
	if got != uuid.Nil {
		t.Errorf("expected uuid.Nil for cross-tenant bypass, got %s", got)
	}
}

// --- PBT for requireTenantID (Task 1.7) ---

func TestPBT_RequireTenantID_ValidUUID_AlwaysReturned(t *testing.T) {
	t.Parallel()
	f := func(hi, lo uint64) bool {
		// Generate a non-Nil UUID from random bytes
		id, err := uuid.FromBytes([]byte{
			byte(hi >> 56), byte(hi >> 48), byte(hi >> 40), byte(hi >> 32),
			byte(hi >> 24), byte(hi >> 16), byte(hi >> 8), byte(hi),
			byte(lo >> 56), byte(lo >> 48), byte(lo >> 40), byte(lo >> 32),
			byte(lo >> 24), byte(lo >> 16), byte(lo >> 8), byte(lo),
		})
		if err != nil {
			return true // skip invalid
		}
		if id == uuid.Nil {
			return true // skip Nil UUID
		}

		ctx := store.WithTenantID(context.Background(), id)
		got, err := requireTenantID(ctx)
		return err == nil && got == id
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("PBT failed: %v", err)
	}
}

func TestPBT_RequireTenantID_EmptyContext_AlwaysReturnsError(t *testing.T) {
	t.Parallel()
	f := func() bool {
		ctx := context.Background()
		_, err := requireTenantID(ctx)
		return err == store.ErrTenantRequired
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("PBT failed: %v", err)
	}
}
