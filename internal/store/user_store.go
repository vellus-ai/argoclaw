package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// User represents a platform user with PCI DSS auth.
type User struct {
	ID             uuid.UUID  `json:"id"`
	TenantID       *uuid.UUID `json:"tenant_id,omitempty"`
	Email          string     `json:"email"`
	PasswordHash   string     `json:"-"` // Never serialized
	DisplayName    string     `json:"display_name,omitempty"`
	Role           string     `json:"role"`   // owner, admin, member
	Status         string     `json:"status"` // active, locked, suspended, pending
	FailedAttempts int        `json:"-"`
	LockedUntil    *time.Time `json:"-"`
	LastLoginAt    *time.Time `json:"last_login_at,omitempty"`
	EmailVerified  bool       `json:"email_verified"`
	MFAEnabled     bool       `json:"mfa_enabled"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// UserSession represents an active refresh token session.
type UserSession struct {
	ID           uuid.UUID `json:"id"`
	UserID       uuid.UUID `json:"user_id"`
	RefreshToken string    `json:"-"` // SHA-256 hash, never serialized
	UserAgent    string    `json:"user_agent,omitempty"`
	IPAddress    string    `json:"ip_address,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	Revoked      bool      `json:"revoked"`
	CreatedAt    time.Time `json:"created_at"`
}

// LoginAuditEntry records an authentication event.
type LoginAuditEntry struct {
	UserID    *uuid.UUID `json:"user_id,omitempty"`
	Email     string     `json:"email"`
	Action    string     `json:"action"` // login_success, login_failed, lockout, password_change, logout
	IPAddress string     `json:"ip_address,omitempty"`
	UserAgent string     `json:"user_agent,omitempty"`
	Details   string     `json:"details,omitempty"` // JSON
}

// UserStore manages user authentication data.
type UserStore interface {
	// CreateUser inserts a new user.
	CreateUser(ctx context.Context, user *User) error

	// GetByEmail looks up a user by email address.
	GetByEmail(ctx context.Context, email string) (*User, error)

	// GetByID looks up a user by UUID.
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)

	// UpdatePassword changes the user's password hash and resets failed attempts.
	UpdatePassword(ctx context.Context, userID uuid.UUID, newHash string) error

	// IncrementFailedAttempts increments the failed login counter.
	// Returns the new count. If count >= MaxFailedAttempts, sets locked_until.
	IncrementFailedAttempts(ctx context.Context, userID uuid.UUID, lockUntil *time.Time) (int, error)

	// ResetFailedAttempts clears the counter and locked_until after successful login.
	ResetFailedAttempts(ctx context.Context, userID uuid.UUID) error

	// UpdateLastLogin sets last_login_at to now.
	UpdateLastLogin(ctx context.Context, userID uuid.UUID) error

	// --- Password History ---

	// AddPasswordHistory stores a previous password hash.
	AddPasswordHistory(ctx context.Context, userID uuid.UUID, hash string) error

	// GetPasswordHistory returns the last N password hashes for reuse check.
	GetPasswordHistory(ctx context.Context, userID uuid.UUID, limit int) ([]string, error)

	// --- Sessions ---

	// CreateSession inserts a new refresh token session.
	CreateSession(ctx context.Context, session *UserSession) error

	// GetSessionByToken looks up a session by refresh token hash.
	GetSessionByToken(ctx context.Context, tokenHash string) (*UserSession, error)

	// RevokeSession marks a session as revoked.
	RevokeSession(ctx context.Context, sessionID uuid.UUID) error

	// RevokeAllSessions revokes all sessions for a user.
	RevokeAllSessions(ctx context.Context, userID uuid.UUID) error

	// CleanExpiredSessions removes expired/revoked sessions older than cutoff.
	CleanExpiredSessions(ctx context.Context) error

	// --- Audit ---

	// LogAudit records an authentication event.
	LogAudit(ctx context.Context, entry *LoginAuditEntry) error
}
