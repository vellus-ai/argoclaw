package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Tenant represents a company/client on the platform.
type Tenant struct {
	ID               uuid.UUID  `json:"id"`
	Slug             string     `json:"slug"`
	Name             string     `json:"name"`
	Plan             string     `json:"plan"`   // trial, starter, pro, enterprise
	Status           string     `json:"status"` // active, suspended, cancelled
	TrialEndsAt      *time.Time `json:"trial_ends_at,omitempty"`
	Settings         string     `json:"settings,omitempty"` // JSON
	StripeCustomerID *string    `json:"stripe_customer_id,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// TenantUser links a user to a tenant with a role.
type TenantUser struct {
	TenantID uuid.UUID `json:"tenant_id"`
	UserID   uuid.UUID `json:"user_id"`
	Role     string    `json:"role"` // owner, admin, member
	JoinedAt time.Time `json:"joined_at"`
}

// TenantBranding holds white-label customization for a tenant.
type TenantBranding struct {
	TenantID     uuid.UUID `json:"tenant_id"`
	LogoURL      string    `json:"logo_url,omitempty"`
	FaviconURL   string    `json:"favicon_url,omitempty"`
	PrimaryColor string    `json:"primary_color,omitempty"` // e.g. "#1E40AF"
	Palette      string    `json:"palette,omitempty"`       // JSON with WCAG AA colors
	CustomDomain string    `json:"custom_domain,omitempty"`
	SenderEmail  string    `json:"sender_email,omitempty"`
	ProductName  string    `json:"product_name,omitempty"` // Default: "ARGO"
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TenantStore manages tenant data.
type TenantStore interface {
	// CreateTenant inserts a new tenant.
	CreateTenant(ctx context.Context, tenant *Tenant) error

	// GetByID looks up a tenant by UUID.
	GetByID(ctx context.Context, id uuid.UUID) (*Tenant, error)

	// GetBySlug looks up a tenant by slug.
	GetBySlug(ctx context.Context, slug string) (*Tenant, error)

	// ListTenants returns all tenants (admin only).
	ListTenants(ctx context.Context) ([]Tenant, error)

	// UpdateTenant updates tenant fields.
	UpdateTenant(ctx context.Context, id uuid.UUID, updates map[string]any) error

	// --- Membership ---

	// AddUser adds a user to a tenant with a role.
	AddUser(ctx context.Context, tenantID, userID uuid.UUID, role string) error

	// RemoveUser removes a user from a tenant.
	RemoveUser(ctx context.Context, tenantID, userID uuid.UUID) error

	// ListUsers returns all users in a tenant.
	ListUsers(ctx context.Context, tenantID uuid.UUID) ([]TenantUser, error)

	// GetUserTenants returns all tenants a user belongs to.
	GetUserTenants(ctx context.Context, userID uuid.UUID) ([]TenantUser, error)

	// --- Branding ---

	// GetBranding returns branding for a tenant.
	GetBranding(ctx context.Context, tenantID uuid.UUID) (*TenantBranding, error)

	// UpsertBranding creates or updates branding.
	UpsertBranding(ctx context.Context, branding *TenantBranding) error

	// GetBrandingByDomain looks up branding by custom domain.
	GetBrandingByDomain(ctx context.Context, domain string) (*TenantBranding, error)
}
