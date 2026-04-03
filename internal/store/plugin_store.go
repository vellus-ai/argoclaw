package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Plugin lifecycle state constants.
const (
	PluginStateInstalled = "installed"
	PluginStateEnabled   = "enabled"
	PluginStateDisabled  = "disabled"
	PluginStateError     = "error"
)

// Plugin audit action constants.
const (
	AuditInstall      = "install"
	AuditEnable       = "enable"
	AuditDisable      = "disable"
	AuditUninstall    = "uninstall"
	AuditConfigChange = "config_change"
	AuditError        = "error"
	AuditToolCall     = "tool_call"
	AuditDataAccess   = "data_access"
	AuditPermDenied   = "permission_denied"
)

// ErrPluginNotFound is returned when a requested plugin record is not found.
var ErrPluginNotFound = errors.New("plugin not found")

// ErrPluginAlreadyInstalled is returned on duplicate install attempts.
var ErrPluginAlreadyInstalled = errors.New("plugin already installed for this tenant")

// ─────────────────────────────────────────────────────────────────────────────
// Model types
// ─────────────────────────────────────────────────────────────────────────────

// PluginCatalogEntry represents a plugin definition in the catalog.
type PluginCatalogEntry struct {
	BaseModel
	// TenantID is nil for built-in/marketplace plugins (visible to all tenants).
	TenantID    *uuid.UUID      `json:"tenant_id,omitempty"`
	Name        string          `json:"name"`
	Version     string          `json:"version"`
	DisplayName string          `json:"display_name"`
	Description string          `json:"description"`
	Author      string          `json:"author"`
	// Manifest is the full plugin.yaml content stored as JSONB.
	Manifest    json.RawMessage `json:"manifest"`
	Source      string          `json:"source"`   // builtin, marketplace, custom
	MinPlan     string          `json:"min_plan"` // starter, pro, enterprise
	Checksum    string          `json:"checksum,omitempty"`
	Tags        []string        `json:"tags"`
}

// TenantPlugin represents a plugin installation for a specific tenant.
type TenantPlugin struct {
	ID            uuid.UUID       `json:"id"`
	TenantID      uuid.UUID       `json:"tenant_id"`
	PluginName    string          `json:"plugin_name"`
	PluginVersion string          `json:"plugin_version"`
	State         string          `json:"state"`
	Config        json.RawMessage `json:"config"`
	Permissions   json.RawMessage `json:"permissions"`
	ErrorMessage  string          `json:"error_message,omitempty"`
	InstalledBy   *uuid.UUID      `json:"installed_by,omitempty"`
	EnabledAt     *time.Time      `json:"enabled_at,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// AgentPlugin represents a per-agent plugin override.
type AgentPlugin struct {
	ID             uuid.UUID       `json:"id"`
	TenantID       uuid.UUID       `json:"tenant_id"`
	AgentID        uuid.UUID       `json:"agent_id"`
	PluginName     string          `json:"plugin_name"`
	Enabled        bool            `json:"enabled"`
	ConfigOverride json.RawMessage `json:"config_override"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// PluginDataEntry represents a KV entry in the plugin data store.
type PluginDataEntry struct {
	ID         uuid.UUID       `json:"id"`
	TenantID   uuid.UUID       `json:"tenant_id"`
	PluginName string          `json:"plugin_name"`
	Collection string          `json:"collection"`
	Key        string          `json:"key"`
	Value      json.RawMessage `json:"value"`
	ExpiresAt  *time.Time      `json:"expires_at,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

// PluginAuditEntry represents a single immutable audit log entry.
type PluginAuditEntry struct {
	ID         uuid.UUID       `json:"id"`
	TenantID   uuid.UUID       `json:"tenant_id"`
	PluginName string          `json:"plugin_name"`
	Action     string          `json:"action"`
	ActorID    *uuid.UUID      `json:"actor_id,omitempty"`
	ActorType  string          `json:"actor_type"` // user, system, agent
	Details    json.RawMessage `json:"details"`
	CreatedAt  time.Time       `json:"created_at"`
}

// ─────────────────────────────────────────────────────────────────────────────
// PluginStore interface
// ─────────────────────────────────────────────────────────────────────────────

// PluginStore defines all persistence operations for the plugin host.
// All methods that read tenant data derive tenant_id from ctx
// (via store.TenantIDFromContext) — never from caller-supplied parameters.
type PluginStore interface {
	// ── Catalog ──────────────────────────────────────────────────────────────

	// UpsertCatalogEntry creates or updates a plugin catalog entry.
	// Uniqueness is enforced on (tenant_id, name, version).
	UpsertCatalogEntry(ctx context.Context, entry *PluginCatalogEntry) error

	// GetCatalogEntry returns a catalog entry by ID.
	// Returns ErrPluginNotFound if the entry does not exist.
	GetCatalogEntry(ctx context.Context, id uuid.UUID) (*PluginCatalogEntry, error)

	// GetCatalogEntryByName returns the latest version of a named plugin
	// visible to the current tenant (built-in or tenant-specific).
	// Returns ErrPluginNotFound if not found.
	GetCatalogEntryByName(ctx context.Context, name string) (*PluginCatalogEntry, error)

	// ListCatalog returns all plugin catalog entries visible to the current tenant,
	// including built-in plugins (tenant_id IS NULL) and tenant-specific entries.
	ListCatalog(ctx context.Context) ([]PluginCatalogEntry, error)

	// ── Tenant plugin management ──────────────────────────────────────────────

	// InstallPlugin records a plugin installation for the current tenant.
	// Atomically inserts into tenant_plugins and writes an audit entry.
	// Returns ErrPluginAlreadyInstalled if the plugin is already installed.
	InstallPlugin(ctx context.Context, tp *TenantPlugin) error

	// EnablePlugin sets the plugin state to 'enabled' for the current tenant.
	// Atomically updates tenant_plugins and writes an audit entry.
	EnablePlugin(ctx context.Context, pluginName string, actorID *uuid.UUID) error

	// DisablePlugin sets the plugin state to 'disabled' for the current tenant.
	// Atomically updates tenant_plugins and writes an audit entry.
	DisablePlugin(ctx context.Context, pluginName string, actorID *uuid.UUID) error

	// UninstallPlugin removes all plugin data for the current tenant:
	// deletes agent_plugins, plugin_data, and tenant_plugins in a single transaction.
	// Writes a final audit entry before deletion.
	UninstallPlugin(ctx context.Context, pluginName string, actorID *uuid.UUID) error

	// GetTenantPlugin returns the installation record for a plugin.
	// Returns ErrPluginNotFound if not installed for the current tenant.
	GetTenantPlugin(ctx context.Context, pluginName string) (*TenantPlugin, error)

	// ListTenantPlugins returns all plugins installed for the current tenant.
	ListTenantPlugins(ctx context.Context) ([]TenantPlugin, error)

	// UpdatePluginConfig updates the per-tenant config for an installed plugin.
	// Atomically updates tenant_plugins and writes an audit entry.
	UpdatePluginConfig(ctx context.Context, pluginName string, config json.RawMessage, actorID *uuid.UUID) error

	// SetPluginError sets the plugin state to 'error' with an error message.
	SetPluginError(ctx context.Context, pluginName, errMsg string) error

	// ── Agent plugin overrides ────────────────────────────────────────────────

	// SetAgentPlugin upserts a per-agent plugin override.
	// Uses INSERT ... ON CONFLICT DO UPDATE.
	SetAgentPlugin(ctx context.Context, ap *AgentPlugin) error

	// GetAgentPlugin returns the agent-level override for a plugin.
	// Returns ErrPluginNotFound if no override exists for this agent+plugin pair.
	GetAgentPlugin(ctx context.Context, agentID uuid.UUID, pluginName string) (*AgentPlugin, error)

	// ListAgentPlugins returns all plugin overrides for a specific agent.
	ListAgentPlugins(ctx context.Context, agentID uuid.UUID) ([]AgentPlugin, error)

	// IsPluginEnabledForAgent returns whether a plugin should be active for an agent.
	// Logic: plugin must be enabled at tenant level AND not explicitly disabled at agent level.
	IsPluginEnabledForAgent(ctx context.Context, agentID uuid.UUID, pluginName string) (bool, error)

	// ── Plugin data (KV store) ────────────────────────────────────────────────

	// PutData upserts a KV entry for the current tenant + plugin.
	// Uses INSERT ... ON CONFLICT (tenant_id, plugin_name, collection, key) DO UPDATE.
	PutData(ctx context.Context, pluginName, collection, key string, value json.RawMessage, expiresAt *time.Time) error

	// GetData retrieves a KV entry for the current tenant + plugin.
	// Returns ErrPluginNotFound if the key does not exist.
	GetData(ctx context.Context, pluginName, collection, key string) (*PluginDataEntry, error)

	// ListDataKeys returns all keys in a collection for the current tenant + plugin.
	// prefix filters key names (empty = all keys). Supports pagination.
	ListDataKeys(ctx context.Context, pluginName, collection, prefix string, limit, offset int) ([]string, error)

	// DeleteData removes a KV entry. Returns nil if the key did not exist.
	DeleteData(ctx context.Context, pluginName, collection, key string) error

	// DeleteCollectionData removes all KV entries for a collection.
	// Used during plugin uninstall to clean up plugin_data.
	DeleteCollectionData(ctx context.Context, pluginName, collection string) error

	// ── Audit log ─────────────────────────────────────────────────────────────

	// LogAudit appends an immutable audit entry.
	LogAudit(ctx context.Context, entry *PluginAuditEntry) error

	// ListAuditLog returns the most recent audit entries for a plugin.
	// Ordered by created_at DESC. limit <= 100.
	ListAuditLog(ctx context.Context, pluginName string, limit int) ([]PluginAuditEntry, error)
}
