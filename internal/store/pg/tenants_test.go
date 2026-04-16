package pg

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/vellus-ai/argoclaw/internal/store"
)

// TestCreateTenant_RejectsOperatorLevelAboveZero verifies that CreateTenant
// returns ErrOperatorLevelForbidden when OperatorLevel > 0.
// appsec: operator_level must never be settable via API — only via migration/admin.
func TestCreateTenant_RejectsOperatorLevelAboveZero(t *testing.T) {
	t.Parallel()
	// No real DB needed — validation happens before any SQL.
	s := &PGTenantStore{db: nil}

	tests := []struct {
		name  string
		level int
		want  error
	}{
		{"operator_level=1", 1, store.ErrOperatorLevelForbidden},
		{"operator_level=2", 2, store.ErrOperatorLevelForbidden},
		{"operator_level=99", 99, store.ErrOperatorLevelForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tenant := &store.Tenant{
				Slug:          "test-" + uuid.New().String()[:8],
				Name:          "Test",
				Plan:          "starter",
				Status:        "active",
				OperatorLevel: tt.level,
			}
			err := s.CreateTenant(context.Background(), tenant)
			if !errors.Is(err, tt.want) {
				t.Errorf("CreateTenant(operator_level=%d) = %v, want %v", tt.level, err, tt.want)
			}
		})
	}
}

// TestCreateTenant_AllowsOperatorLevelZero verifies that OperatorLevel=0 does NOT
// trigger the validation error. We recover from the nil-DB panic to confirm
// the validation layer was passed.
func TestCreateTenant_AllowsOperatorLevelZero(t *testing.T) {
	t.Parallel()
	s := &PGTenantStore{db: nil}

	tenant := &store.Tenant{
		Slug:          "test-zero",
		Name:          "Test Zero",
		Plan:          "starter",
		Status:        "active",
		OperatorLevel: 0,
	}

	func() {
		defer func() {
			if r := recover(); r != nil {
				// Expected: nil DB panic means validation passed
			}
		}()
		err := s.CreateTenant(context.Background(), tenant)
		if errors.Is(err, store.ErrOperatorLevelForbidden) {
			t.Errorf("CreateTenant(operator_level=0) should not return ErrOperatorLevelForbidden")
		}
	}()
}

// TestUpdateTenant_RejectsOperatorLevelAboveZero verifies that UpdateTenant
// returns ErrOperatorLevelForbidden when operator_level > 0 is in the updates map.
func TestUpdateTenant_RejectsOperatorLevelAboveZero(t *testing.T) {
	t.Parallel()
	s := &PGTenantStore{db: nil}

	tests := []struct {
		name    string
		updates map[string]any
		want    error
	}{
		{"int_1", map[string]any{"operator_level": 1}, store.ErrOperatorLevelForbidden},
		{"int_2", map[string]any{"operator_level": 2}, store.ErrOperatorLevelForbidden},
		{"float64_1", map[string]any{"operator_level": float64(1)}, store.ErrOperatorLevelForbidden},
		{"int64_1", map[string]any{"operator_level": int64(1)}, store.ErrOperatorLevelForbidden},
		{"string_value", map[string]any{"operator_level": "1"}, store.ErrOperatorLevelForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := s.UpdateTenant(context.Background(), uuid.New(), tt.updates)
			if !errors.Is(err, tt.want) {
				t.Errorf("UpdateTenant(%v) = %v, want %v", tt.updates, err, tt.want)
			}
		})
	}
}

// TestUpdateTenant_AllowsOperatorLevelZero verifies that operator_level=0 in updates
// does NOT trigger the validation error. We recover from nil-DB panics.
func TestUpdateTenant_AllowsOperatorLevelZero(t *testing.T) {
	t.Parallel()
	s := &PGTenantStore{db: nil}

	tests := []struct {
		name    string
		updates map[string]any
	}{
		{"int_0", map[string]any{"operator_level": 0}},
		{"float64_0", map[string]any{"operator_level": float64(0)}},
		{"int64_0", map[string]any{"operator_level": int64(0)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			func() {
				defer func() {
					if r := recover(); r != nil {
						// Expected: nil DB panic means validation passed
					}
				}()
				err := s.UpdateTenant(context.Background(), uuid.New(), tt.updates)
				if errors.Is(err, store.ErrOperatorLevelForbidden) {
					t.Errorf("UpdateTenant(%v) should not return ErrOperatorLevelForbidden", tt.updates)
				}
			}()
		})
	}
}

// TestUpdateTenant_AllowsOtherFieldsWithoutOperatorLevel verifies that updates
// without operator_level pass validation. We recover from nil-DB panics.
func TestUpdateTenant_AllowsOtherFieldsWithoutOperatorLevel(t *testing.T) {
	t.Parallel()
	s := &PGTenantStore{db: nil}

	func() {
		defer func() {
			if r := recover(); r != nil {
				// Expected: nil DB panic means validation passed
			}
		}()
		err := s.UpdateTenant(context.Background(), uuid.New(), map[string]any{
			"name": "Updated Name",
			"plan": "pro",
		})
		if errors.Is(err, store.ErrOperatorLevelForbidden) {
			t.Error("UpdateTenant without operator_level should not return ErrOperatorLevelForbidden")
		}
	}()
}
