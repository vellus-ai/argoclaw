package pg

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// PGOnboardingStore implements tools.OnboardingStore backed by PostgreSQL.
// Uses tenants.settings (JSONB), tenant_branding, and setup_progress tables.
type PGOnboardingStore struct {
	db *sql.DB
}

// NewPGOnboardingStore creates a new onboarding store.
func NewPGOnboardingStore(db *sql.DB) *PGOnboardingStore {
	return &PGOnboardingStore{db: db}
}

// UpdateTenantSettings sets a key in tenants.settings JSONB and syncs
// structured fields (account_type, industry, team_size) to setup_progress.
func (s *PGOnboardingStore) UpdateTenantSettings(ctx context.Context, tenantID string, key string, value any) error {
	jsonValue, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal value: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Update JSONB settings on tenants table
	_, err = tx.ExecContext(ctx, `
		UPDATE tenants
		SET settings = jsonb_set(COALESCE(settings, '{}'), ARRAY[$1], $2::jsonb),
		    updated_at = NOW()
		WHERE id = $3`,
		key, string(jsonValue), tenantID)
	if err != nil {
		return fmt.Errorf("update tenant settings: %w", err)
	}

	// Sync structured fields to setup_progress (on-demand upsert)
	switch key {
	case "account_type":
		_, err = tx.ExecContext(ctx, `
			INSERT INTO setup_progress (tenant_id, account_type, updated_at)
			VALUES ($1, $2, NOW())
			ON CONFLICT (tenant_id) DO UPDATE SET account_type = $2, updated_at = NOW()`,
			tenantID, value)
	case "industry":
		_, err = tx.ExecContext(ctx, `
			INSERT INTO setup_progress (tenant_id, industry, updated_at)
			VALUES ($1, $2, NOW())
			ON CONFLICT (tenant_id) DO UPDATE SET industry = $2, updated_at = NOW()`,
			tenantID, value)
	case "team_size":
		_, err = tx.ExecContext(ctx, `
			INSERT INTO setup_progress (tenant_id, team_size, updated_at)
			VALUES ($1, $2, NOW())
			ON CONFLICT (tenant_id) DO UPDATE SET team_size = $2, updated_at = NOW()`,
			tenantID, value)
	}
	if err != nil {
		return fmt.Errorf("sync setup_progress %s: %w", key, err)
	}

	return tx.Commit()
}

// UpdateTenantBranding performs a partial upsert on tenant_branding.
// Empty strings are ignored — only non-empty fields are updated.
func (s *PGOnboardingStore) UpdateTenantBranding(ctx context.Context, tenantID string, primaryColor, productName string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO tenant_branding (tenant_id, primary_color, product_name, created_at, updated_at)
		VALUES ($1, NULLIF($2, ''), NULLIF($3, ''), NOW(), NOW())
		ON CONFLICT (tenant_id) DO UPDATE SET
			primary_color = COALESCE(NULLIF($2, ''), tenant_branding.primary_color),
			product_name  = COALESCE(NULLIF($3, ''), tenant_branding.product_name),
			updated_at    = NOW()`,
		tenantID, primaryColor, productName)
	if err != nil {
		return fmt.Errorf("upsert tenant branding: %w", err)
	}
	return nil
}

// GetOnboardingStatus returns the current onboarding state for a tenant.
// Joins tenants, setup_progress, and tenant_branding to build the status map.
func (s *PGOnboardingStore) GetOnboardingStatus(ctx context.Context, tenantID string) (map[string]any, error) {
	var (
		settings           sql.NullString
		accountType        sql.NullString
		industry           sql.NullString
		teamSize           sql.NullString
		onbComplete        bool
		completedAt        *time.Time
		primaryColor       sql.NullString
		productName        sql.NullString
		lastCompletedState sql.NullString
	)

	err := s.db.QueryRowContext(ctx, `
		SELECT
			t.settings,
			sp.account_type, sp.industry, sp.team_size,
			COALESCE(sp.onboarding_complete, false),
			sp.completed_at,
			tb.primary_color, tb.product_name,
			sp.last_completed_state
		FROM tenants t
		LEFT JOIN setup_progress sp ON sp.tenant_id = t.id
		LEFT JOIN tenant_branding tb ON tb.tenant_id = t.id
		WHERE t.id = $1`,
		tenantID).Scan(
		&settings,
		&accountType, &industry, &teamSize,
		&onbComplete,
		&completedAt,
		&primaryColor, &productName,
		&lastCompletedState,
	)
	if err == sql.ErrNoRows {
		return map[string]any{
			"onboarding_complete":  false,
			"workspace_configured": false,
			"branding_set":         false,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get onboarding status: %w", err)
	}

	status := map[string]any{
		"onboarding_complete":  onbComplete,
		"workspace_configured": accountType.Valid && accountType.String != "",
		"branding_set":         primaryColor.Valid || productName.Valid,
	}

	if accountType.Valid {
		status["account_type"] = accountType.String
	}
	if industry.Valid {
		status["industry"] = industry.String
	}
	if teamSize.Valid {
		status["team_size"] = teamSize.String
	}
	if primaryColor.Valid {
		status["primary_color"] = primaryColor.String
	}
	if productName.Valid {
		status["product_name"] = productName.String
	}
	if completedAt != nil {
		status["completed_at"] = completedAt.Format(time.RFC3339)
	}
	if lastCompletedState.Valid {
		status["last_completed_state"] = lastCompletedState.String
	}

	return status, nil
}

// UpdateLastCompletedState records the last completed onboarding state for resume support.
// Idempotent — uses UPSERT to create or update the setup_progress row.
func (s *PGOnboardingStore) UpdateLastCompletedState(ctx context.Context, tenantID string, state string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO setup_progress (tenant_id, last_completed_state, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (tenant_id) DO UPDATE SET
			last_completed_state = $2,
			updated_at = NOW()`,
		tenantID, state)
	if err != nil {
		return fmt.Errorf("update last_completed_state: %w", err)
	}
	return nil
}

// CompleteOnboarding marks the tenant's onboarding as complete.
// Idempotent — calling multiple times is safe.
func (s *PGOnboardingStore) CompleteOnboarding(ctx context.Context, tenantID string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO setup_progress (tenant_id, onboarding_complete, completed_at, updated_at)
		VALUES ($1, true, NOW(), NOW())
		ON CONFLICT (tenant_id) DO UPDATE SET
			onboarding_complete = true,
			completed_at = COALESCE(setup_progress.completed_at, NOW()),
			updated_at = NOW()`,
		tenantID)
	if err != nil {
		return fmt.Errorf("complete onboarding: %w", err)
	}
	return nil
}
