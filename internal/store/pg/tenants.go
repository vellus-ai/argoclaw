package pg

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/vellus-ai/argoclaw/internal/store"
)

// PGTenantStore implements store.TenantStore backed by PostgreSQL.
type PGTenantStore struct {
	db *sql.DB
}

func NewPGTenantStore(db *sql.DB) *PGTenantStore {
	return &PGTenantStore{db: db}
}

func (s *PGTenantStore) CreateTenant(ctx context.Context, tenant *store.Tenant) error {
	// appsec: operator_level is derived exclusively from DB/migration — reject any attempt to set it via API.
	if tenant.OperatorLevel > 0 {
		return store.ErrOperatorLevelForbidden
	}
	if tenant.ID == uuid.Nil {
		tenant.ID = uuid.New()
	}
	settings := tenant.Settings
	if settings == "" {
		settings = "{}"
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO tenants (id, slug, name, plan, status, trial_ends_at, settings, stripe_customer_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7::JSONB, $8, NOW(), NOW())`,
		tenant.ID, tenant.Slug, tenant.Name, tenant.Plan, tenant.Status,
		tenant.TrialEndsAt, settings, tenant.StripeCustomerID)
	if err != nil {
		return fmt.Errorf("create tenant: %w", err)
	}
	return nil
}

func (s *PGTenantStore) GetByID(ctx context.Context, id uuid.UUID) (*store.Tenant, error) {
	var t store.Tenant
	var settings sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, slug, name, plan, status, trial_ends_at, settings, stripe_customer_id, operator_level, created_at, updated_at
		FROM tenants WHERE id = $1`, id).Scan(
		&t.ID, &t.Slug, &t.Name, &t.Plan, &t.Status,
		&t.TrialEndsAt, &settings, &t.StripeCustomerID, &t.OperatorLevel, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get tenant: %w", err)
	}
	if settings.Valid {
		t.Settings = settings.String
	}
	return &t, nil
}

func (s *PGTenantStore) GetBySlug(ctx context.Context, slug string) (*store.Tenant, error) {
	var t store.Tenant
	var settings sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, slug, name, plan, status, trial_ends_at, settings, stripe_customer_id, operator_level, created_at, updated_at
		FROM tenants WHERE slug = $1`, slug).Scan(
		&t.ID, &t.Slug, &t.Name, &t.Plan, &t.Status,
		&t.TrialEndsAt, &settings, &t.StripeCustomerID, &t.OperatorLevel, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get tenant by slug: %w", err)
	}
	if settings.Valid {
		t.Settings = settings.String
	}
	return &t, nil
}

func (s *PGTenantStore) ListTenants(ctx context.Context) ([]store.Tenant, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, slug, name, plan, status, trial_ends_at, stripe_customer_id, operator_level, created_at, updated_at
		FROM tenants ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []store.Tenant
	for rows.Next() {
		var t store.Tenant
		if err := rows.Scan(&t.ID, &t.Slug, &t.Name, &t.Plan, &t.Status,
			&t.TrialEndsAt, &t.StripeCustomerID, &t.OperatorLevel, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, t)
	}
	return result, rows.Err()
}

// ListAllTenantsForOperator returns all tenants without tenant_id filter, paginated.
// Requires IsCrossTenant(ctx) == true.
// appsec:cross-tenant-bypass — called only via OperatorHandler protected by requireOperatorRole.
func (s *PGTenantStore) ListAllTenantsForOperator(ctx context.Context, limit, offset int) ([]store.Tenant, int, error) {
	if !store.IsCrossTenant(ctx) {
		return nil, 0, store.ErrTenantRequired
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, slug, name, plan, status, trial_ends_at, stripe_customer_id, operator_level,
		       created_at, updated_at, COUNT(*) OVER() AS total_count
		FROM tenants
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list all tenants for operator: %w", err)
	}
	defer rows.Close()

	var result []store.Tenant
	var total int
	for rows.Next() {
		var t store.Tenant
		if err := rows.Scan(&t.ID, &t.Slug, &t.Name, &t.Plan, &t.Status,
			&t.TrialEndsAt, &t.StripeCustomerID, &t.OperatorLevel,
			&t.CreatedAt, &t.UpdatedAt, &total); err != nil {
			return nil, 0, fmt.Errorf("scan tenant row: %w", err)
		}
		result = append(result, t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return result, total, nil
}

func (s *PGTenantStore) UpdateTenant(ctx context.Context, id uuid.UUID, updates map[string]any) error {
	// appsec: operator_level is derived exclusively from DB/migration — reject any attempt to set it via API.
	if v, ok := updates["operator_level"]; ok {
		switch n := v.(type) {
		case int:
			if n > 0 {
				return store.ErrOperatorLevelForbidden
			}
		case float64:
			if n > 0 {
				return store.ErrOperatorLevelForbidden
			}
		case int64:
			if n > 0 {
				return store.ErrOperatorLevelForbidden
			}
		default:
			// Any non-zero-like value for operator_level is forbidden
			return store.ErrOperatorLevelForbidden
		}
	}
	return execMapUpdate(ctx, s.db, "tenants", id, updates)
}

// --- Membership ---

func (s *PGTenantStore) AddUser(ctx context.Context, tenantID, userID uuid.UUID, role string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO tenant_users (tenant_id, user_id, role, joined_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (tenant_id, user_id) DO UPDATE SET role = EXCLUDED.role`,
		tenantID, userID, role)
	return err
}

func (s *PGTenantStore) RemoveUser(ctx context.Context, tenantID, userID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM tenant_users WHERE tenant_id = $1 AND user_id = $2`, tenantID, userID)
	return err
}

func (s *PGTenantStore) ListUsers(ctx context.Context, tenantID uuid.UUID) ([]store.TenantUser, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT tenant_id, user_id, role, joined_at
		FROM tenant_users WHERE tenant_id = $1 ORDER BY joined_at`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []store.TenantUser
	for rows.Next() {
		var tu store.TenantUser
		if err := rows.Scan(&tu.TenantID, &tu.UserID, &tu.Role, &tu.JoinedAt); err != nil {
			return nil, err
		}
		result = append(result, tu)
	}
	return result, rows.Err()
}

func (s *PGTenantStore) GetUserTenants(ctx context.Context, userID uuid.UUID) ([]store.TenantUser, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT tenant_id, user_id, role, joined_at
		FROM tenant_users WHERE user_id = $1 ORDER BY joined_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []store.TenantUser
	for rows.Next() {
		var tu store.TenantUser
		if err := rows.Scan(&tu.TenantID, &tu.UserID, &tu.Role, &tu.JoinedAt); err != nil {
			return nil, err
		}
		result = append(result, tu)
	}
	return result, rows.Err()
}

// --- Branding ---

func (s *PGTenantStore) GetBranding(ctx context.Context, tenantID uuid.UUID) (*store.TenantBranding, error) {
	var b store.TenantBranding
	err := s.db.QueryRowContext(ctx, `
		SELECT tenant_id, logo_url, favicon_url, primary_color, palette, custom_domain,
		       sender_email, product_name, created_at, updated_at
		FROM tenant_branding WHERE tenant_id = $1`, tenantID).Scan(
		&b.TenantID, &b.LogoURL, &b.FaviconURL, &b.PrimaryColor, &b.Palette,
		&b.CustomDomain, &b.SenderEmail, &b.ProductName, &b.CreatedAt, &b.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (s *PGTenantStore) UpsertBranding(ctx context.Context, b *store.TenantBranding) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO tenant_branding (tenant_id, logo_url, favicon_url, primary_color, palette, custom_domain, sender_email, product_name, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5::JSONB, $6, $7, $8, NOW(), NOW())
		ON CONFLICT (tenant_id) DO UPDATE SET
			logo_url = EXCLUDED.logo_url,
			favicon_url = EXCLUDED.favicon_url,
			primary_color = EXCLUDED.primary_color,
			palette = EXCLUDED.palette,
			custom_domain = EXCLUDED.custom_domain,
			sender_email = EXCLUDED.sender_email,
			product_name = EXCLUDED.product_name,
			updated_at = NOW()`,
		b.TenantID, b.LogoURL, b.FaviconURL, b.PrimaryColor, nullIfEmpty(b.Palette),
		nullIfEmpty(b.CustomDomain), nullIfEmpty(b.SenderEmail), b.ProductName)
	return err
}

func (s *PGTenantStore) GetBrandingByDomain(ctx context.Context, domain string) (*store.TenantBranding, error) {
	var b store.TenantBranding
	err := s.db.QueryRowContext(ctx, `
		SELECT tenant_id, logo_url, favicon_url, primary_color, palette, custom_domain,
		       sender_email, product_name, created_at, updated_at
		FROM tenant_branding WHERE custom_domain = $1`, domain).Scan(
		&b.TenantID, &b.LogoURL, &b.FaviconURL, &b.PrimaryColor, &b.Palette,
		&b.CustomDomain, &b.SenderEmail, &b.ProductName, &b.CreatedAt, &b.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}
