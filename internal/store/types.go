package store

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrTenantRequired is returned when a store operation is called without
// tenant_id in the context and the operation is not marked as cross-tenant.
// This is the fail-closed default: queries never run without tenant scoping.
var ErrTenantRequired = errors.New("tenant_id required in context")

// ErrOperatorLevelForbidden is returned when a client attempts to set
// operator_level > 0 via CreateTenant or UpdateTenant.
// appsec: operator_level is derived exclusively from DB/migration — never from API input.
var ErrOperatorLevelForbidden = errors.New("operator_level cannot be set via API")

// BaseModel provides common fields for all database models.
type BaseModel struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// GenNewID generates a new UUID v7 (time-ordered).
func GenNewID() uuid.UUID {
	return uuid.Must(uuid.NewV7())
}

// StoreConfig configures the store layer.
type StoreConfig struct {
	// PostgresDSN is the Postgres connection string (required).
	PostgresDSN string

	// SkillsStorageDir is the directory for skill file content (default: dataDir/skills-store/).
	SkillsStorageDir string

	// Workspace is the default agent workspace path.
	Workspace string

	// GlobalSkillsDir is the global skills directory (e.g. ~/.argoclaw/skills).
	GlobalSkillsDir string

	// BuiltinSkillsDir is the builtin skills directory (bundled with binary).
	BuiltinSkillsDir string

	// EncryptionKey is the AES-256 key for encrypting sensitive data (API keys).
	// If empty, sensitive data is stored in plain text.
	EncryptionKey string
}
