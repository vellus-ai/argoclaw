-- Migration 026: User authentication with PCI DSS password standards.
-- Adds users, password_history, and user_sessions tables for email+password auth.

-- Users table (per-tenant)
CREATE TABLE IF NOT EXISTS users (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    tenant_id       UUID,                              -- NULL = system/platform user
    email           VARCHAR(320) NOT NULL,              -- RFC 5321 max
    password_hash   TEXT NOT NULL,                      -- Argon2id hash
    display_name    VARCHAR(255),
    role            VARCHAR(20) NOT NULL DEFAULT 'member', -- owner, admin, member
    status          VARCHAR(20) NOT NULL DEFAULT 'active', -- active, locked, suspended, pending
    failed_attempts INT NOT NULL DEFAULT 0,
    locked_until    TIMESTAMPTZ,                        -- Account lockout expiry
    last_login_at   TIMESTAMPTZ,
    email_verified  BOOLEAN NOT NULL DEFAULT false,
    mfa_enabled     BOOLEAN NOT NULL DEFAULT false,
    mfa_secret      TEXT,                               -- Encrypted TOTP secret
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_users_email UNIQUE (email)
);

CREATE INDEX idx_users_tenant ON users(tenant_id) WHERE tenant_id IS NOT NULL;
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_status ON users(status);

-- Password history (PCI DSS: prevent reuse of last 4 passwords)
CREATE TABLE IF NOT EXISTS password_history (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    password_hash   TEXT NOT NULL,                      -- Argon2id hash of previous password
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_password_history_user ON password_history(user_id, created_at DESC);

-- User sessions (JWT refresh tokens)
CREATE TABLE IF NOT EXISTS user_sessions (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token   TEXT NOT NULL,                      -- SHA-256 hash of refresh token
    user_agent      TEXT,
    ip_address      INET,
    expires_at      TIMESTAMPTZ NOT NULL,
    revoked         BOOLEAN NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_user_sessions_user ON user_sessions(user_id);
CREATE INDEX idx_user_sessions_token ON user_sessions(refresh_token);
CREATE INDEX idx_user_sessions_expires ON user_sessions(expires_at) WHERE NOT revoked;

-- Login audit log
CREATE TABLE IF NOT EXISTS login_audit (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_id         UUID REFERENCES users(id) ON DELETE SET NULL,
    email           VARCHAR(320) NOT NULL,
    action          VARCHAR(30) NOT NULL,               -- login_success, login_failed, lockout, password_change, logout
    ip_address      INET,
    user_agent      TEXT,
    details         JSONB,                              -- Extra context (failure reason, etc.)
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_login_audit_user ON login_audit(user_id, created_at DESC);
CREATE INDEX idx_login_audit_email ON login_audit(email, created_at DESC);
