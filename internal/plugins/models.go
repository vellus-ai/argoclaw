package plugins

import (
	"encoding/json"
	"os"
	"time"

	"github.com/google/uuid"
)

// ─────────────────────────────────────────────────────────────────────────────
// Persistence Models
// ─────────────────────────────────────────────────────────────────────────────

// CatalogEntry representa um plugin disponível no catálogo.
type CatalogEntry struct {
	ID          uuid.UUID       `json:"id" db:"id"`
	Name        string          `json:"name" db:"name"`
	Version     string          `json:"version" db:"version"`
	DisplayName string          `json:"display_name" db:"display_name"`
	Description string          `json:"description" db:"description"`
	Author      string          `json:"author" db:"author"`
	Manifest    json.RawMessage `json:"manifest" db:"manifest"`    // Source of truth — YAML parseado como JSON
	IconURL     *string         `json:"icon_url" db:"icon_url"`    // nullable
	Tags        []string        `json:"tags" db:"tags"`
	MinPlan     string          `json:"min_plan" db:"min_plan"`    // starter | pro | enterprise
	Source      string          `json:"source" db:"source"`        // builtin | marketplace | custom
	Checksum    string          `json:"checksum" db:"checksum"`    // SHA-256 do binário
	TenantID    *uuid.UUID      `json:"tenant_id" db:"tenant_id"` // NULL para builtin
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at" db:"updated_at"`
}

// TenantPlugin representa a instalação de um plugin em um tenant.
type TenantPlugin struct {
	ID            uuid.UUID       `json:"id" db:"id"`
	TenantID      uuid.UUID       `json:"tenant_id" db:"tenant_id"`
	PluginName    string          `json:"plugin_name" db:"plugin_name"`
	PluginVersion string          `json:"plugin_version" db:"plugin_version"`
	State         PluginState     `json:"state" db:"state"`
	Config        json.RawMessage `json:"config" db:"config"`
	Permissions   json.RawMessage `json:"permissions" db:"permissions"`
	ErrorMessage  *string         `json:"error_message" db:"error_message"`
	InstalledBy   uuid.UUID       `json:"installed_by" db:"installed_by"`
	EnabledAt     *time.Time      `json:"enabled_at" db:"enabled_at"`
	Version       int             `json:"version" db:"version"` // Optimistic locking
	CreatedAt     time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at" db:"updated_at"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Plugin State Machine
// ─────────────────────────────────────────────────────────────────────────────

// PluginState represents the lifecycle state of a tenant plugin.
type PluginState string

const (
	StateInstalled PluginState = "installed"
	StateEnabled   PluginState = "enabled"
	StateDisabled  PluginState = "disabled"
	StateError     PluginState = "error"
)

// ValidTransitions define as transições de estado permitidas.
var ValidTransitions = map[PluginState][]PluginState{
	StateInstalled: {StateEnabled},
	StateEnabled:   {StateDisabled, StateError},
	StateDisabled:  {StateEnabled},
	StateError:     {StateEnabled, StateDisabled},
}

// AgentPlugin representa a associação plugin-agente.
type AgentPlugin struct {
	ID             uuid.UUID       `json:"id" db:"id"`
	TenantID       uuid.UUID       `json:"tenant_id" db:"tenant_id"`
	AgentID        uuid.UUID       `json:"agent_id" db:"agent_id"`
	PluginName     string          `json:"plugin_name" db:"plugin_name"`
	Enabled        bool            `json:"enabled" db:"enabled"`
	ConfigOverride json.RawMessage `json:"config_override" db:"config_override"`
	CreatedAt      time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at" db:"updated_at"`
}

// PluginData representa um registro KV do plugin.
type PluginData struct {
	ID         uuid.UUID       `json:"id" db:"id"`
	TenantID   uuid.UUID       `json:"tenant_id" db:"tenant_id"`
	PluginName string          `json:"plugin_name" db:"plugin_name"`
	Collection string          `json:"collection" db:"collection"`
	Key        string          `json:"key" db:"key"`
	Value      json.RawMessage `json:"value" db:"value"`
	ExpiresAt  *time.Time      `json:"expires_at" db:"expires_at"`
	CreatedAt  time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at" db:"updated_at"`
}

// AuditEntry representa uma entrada no audit log.
type AuditEntry struct {
	ID         uuid.UUID       `json:"id" db:"id"`
	TenantID   uuid.UUID       `json:"tenant_id" db:"tenant_id"`
	PluginName string          `json:"plugin_name" db:"plugin_name"`
	Action     string          `json:"action" db:"action"`
	ActorID    uuid.UUID       `json:"actor_id" db:"actor_id"`
	ActorType  string          `json:"actor_type" db:"actor_type"` // user | system | agent
	Details    json.RawMessage `json:"details" db:"details"`
	CreatedAt  time.Time       `json:"created_at" db:"created_at"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Registry Models (in-memory runtime state)
// ─────────────────────────────────────────────────────────────────────────────

// RegistryStatus represents the runtime status of a plugin in the registry.
type RegistryStatus string

const (
	RegistryActive   RegistryStatus = "active"
	RegistryDisabled RegistryStatus = "disabled"
	RegistryError    RegistryStatus = "error"
)

// RegistryEntry armazena o estado runtime de um plugin ativo.
type RegistryEntry struct {
	Manifest  *PluginManifest `json:"manifest"`
	CatalogID uuid.UUID       `json:"catalog_id"`
	Status    RegistryStatus  `json:"status"`
	ErrorMsg  string          `json:"error_msg"`
	Process   *os.Process     `json:"-"`       // referência ao processo MCP (excluído do JSON)
	Tools     []string        `json:"tools"`   // nomes das tools registradas (com prefixo)
	EnabledAt time.Time       `json:"enabled_at"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Status & Discovery Models
// ─────────────────────────────────────────────────────────────────────────────

// PluginStatus contém informações de status e métricas de um plugin.
type PluginStatus struct {
	State           PluginState `json:"state"`
	Uptime          string      `json:"uptime"`
	ToolCount       int         `json:"tool_count"`
	LastError       string      `json:"last_error"`
	LastHealthCheck *time.Time  `json:"last_health_check"`
	HealthResult    string      `json:"health_result"`
	ProcessPID      int         `json:"process_pid"`
	TotalToolCalls  int64       `json:"total_tool_calls"`
	TotalErrors     int64       `json:"total_errors"`
	AvgLatencyMs    float64     `json:"avg_latency_ms"`
	CircuitState    string      `json:"circuit_state"` // closed | open | half-open
}

// DiscoveredTool representa uma tool descoberta via MCP tools/list.
type DiscoveredTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}
