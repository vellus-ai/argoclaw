package pg

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/vellus-ai/argoclaw/internal/store"
)

// PGUserStore implements store.UserStore backed by PostgreSQL.
type PGUserStore struct {
	db *sql.DB
}

func NewPGUserStore(db *sql.DB) *PGUserStore {
	return &PGUserStore{db: db}
}

func (s *PGUserStore) CreateUser(ctx context.Context, user *store.User) error {
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO users (id, tenant_id, email, password_hash, display_name, role, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())`,
		user.ID, user.TenantID, user.Email, user.PasswordHash, user.DisplayName, user.Role, user.Status)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (s *PGUserStore) GetByEmail(ctx context.Context, email string) (*store.User, error) {
	var u store.User
	err := s.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, email, password_hash, display_name, role, status,
		       failed_attempts, locked_until, last_login_at, email_verified, mfa_enabled,
		       created_at, updated_at
		FROM users WHERE email = $1`, email).Scan(
		&u.ID, &u.TenantID, &u.Email, &u.PasswordHash, &u.DisplayName, &u.Role, &u.Status,
		&u.FailedAttempts, &u.LockedUntil, &u.LastLoginAt, &u.EmailVerified, &u.MFAEnabled,
		&u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return &u, nil
}

func (s *PGUserStore) GetByID(ctx context.Context, id uuid.UUID) (*store.User, error) {
	var u store.User
	err := s.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, email, password_hash, display_name, role, status,
		       failed_attempts, locked_until, last_login_at, email_verified, mfa_enabled,
		       created_at, updated_at
		FROM users WHERE id = $1`, id).Scan(
		&u.ID, &u.TenantID, &u.Email, &u.PasswordHash, &u.DisplayName, &u.Role, &u.Status,
		&u.FailedAttempts, &u.LockedUntil, &u.LastLoginAt, &u.EmailVerified, &u.MFAEnabled,
		&u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &u, nil
}

func (s *PGUserStore) UpdatePassword(ctx context.Context, userID uuid.UUID, newHash string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE users SET password_hash = $1, failed_attempts = 0, locked_until = NULL, updated_at = NOW()
		WHERE id = $2`, newHash, userID)
	return err
}

func (s *PGUserStore) IncrementFailedAttempts(ctx context.Context, userID uuid.UUID, lockUntil *time.Time) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		UPDATE users SET failed_attempts = failed_attempts + 1, locked_until = $1, updated_at = NOW()
		WHERE id = $2
		RETURNING failed_attempts`, lockUntil, userID).Scan(&count)
	return count, err
}

func (s *PGUserStore) ResetFailedAttempts(ctx context.Context, userID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE users SET failed_attempts = 0, locked_until = NULL, updated_at = NOW()
		WHERE id = $1`, userID)
	return err
}

func (s *PGUserStore) UpdateLastLogin(ctx context.Context, userID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE users SET last_login_at = NOW(), updated_at = NOW() WHERE id = $1`, userID)
	return err
}

// --- Password History ---

func (s *PGUserStore) AddPasswordHistory(ctx context.Context, userID uuid.UUID, hash string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO password_history (id, user_id, password_hash, created_at)
		VALUES ($1, $2, $3, NOW())`, uuid.New(), userID, hash)
	return err
}

func (s *PGUserStore) GetPasswordHistory(ctx context.Context, userID uuid.UUID, limit int) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT password_hash FROM password_history
		WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var hashes []string
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			return nil, err
		}
		hashes = append(hashes, h)
	}
	return hashes, rows.Err()
}

// --- Sessions ---

func (s *PGUserStore) CreateSession(ctx context.Context, session *store.UserSession) error {
	if session.ID == uuid.Nil {
		session.ID = uuid.New()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO user_sessions (id, user_id, refresh_token, user_agent, ip_address, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5::INET, $6, NOW())`,
		session.ID, session.UserID, session.RefreshToken, session.UserAgent, nullIfEmpty(session.IPAddress), session.ExpiresAt)
	return err
}

func (s *PGUserStore) GetSessionByToken(ctx context.Context, tokenHash string) (*store.UserSession, error) {
	var sess store.UserSession
	var ip sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, refresh_token, user_agent, ip_address::TEXT, expires_at, revoked, created_at
		FROM user_sessions
		WHERE refresh_token = $1 AND NOT revoked AND expires_at > NOW()`, tokenHash).Scan(
		&sess.ID, &sess.UserID, &sess.RefreshToken, &sess.UserAgent, &ip,
		&sess.ExpiresAt, &sess.Revoked, &sess.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if ip.Valid {
		sess.IPAddress = ip.String
	}
	return &sess, nil
}

func (s *PGUserStore) RevokeSession(ctx context.Context, sessionID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, `UPDATE user_sessions SET revoked = true WHERE id = $1`, sessionID)
	return err
}

func (s *PGUserStore) RevokeAllSessions(ctx context.Context, userID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, `UPDATE user_sessions SET revoked = true WHERE user_id = $1`, userID)
	return err
}

func (s *PGUserStore) CleanExpiredSessions(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM user_sessions WHERE revoked = true OR expires_at < NOW()`)
	return err
}

// --- Audit ---

func (s *PGUserStore) LogAudit(ctx context.Context, entry *store.LoginAuditEntry) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO login_audit (id, user_id, email, action, ip_address, user_agent, details, created_at)
		VALUES ($1, $2, $3, $4, $5::INET, $6, $7::JSONB, NOW())`,
		uuid.New(), entry.UserID, entry.Email, entry.Action,
		nullIfEmpty(entry.IPAddress), entry.UserAgent, nullIfEmpty(entry.Details))
	return err
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
