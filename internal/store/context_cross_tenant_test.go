package store

import (
	"context"
	"testing"
)

func TestIsCrossTenant_DefaultFalse(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	if IsCrossTenant(ctx) {
		t.Error("expected IsCrossTenant to return false for empty context")
	}
}

func TestIsCrossTenant_WithCrossTenant_ReturnsTrue(t *testing.T) {
	t.Parallel()
	ctx := WithCrossTenant(context.Background())
	if !IsCrossTenant(ctx) {
		t.Error("expected IsCrossTenant to return true after WithCrossTenant")
	}
}

func TestWithCrossTenant_DoesNotAffectOtherKeys(t *testing.T) {
	t.Parallel()
	ctx := WithUserID(context.Background(), "user-123")
	ctx = WithCrossTenant(ctx)

	if !IsCrossTenant(ctx) {
		t.Error("expected IsCrossTenant to be true")
	}
	if got := UserIDFromContext(ctx); got != "user-123" {
		t.Errorf("expected user_id 'user-123', got %q", got)
	}
}
