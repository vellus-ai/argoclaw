//go:build integration

package testutil

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
)

// CreateAgent inserts a minimal agent row for testing and returns its UUID.
func CreateAgent(t *testing.T, db *sql.DB, tenantID uuid.UUID, agentKey, displayName string) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO agents (id, agent_key, display_name, agent_type, status, created_at, updated_at, tenant_id)
		 VALUES ($1, $2, $3, 'open', 'active', NOW(), NOW(), $4)`,
		id, agentKey, displayName, tenantID,
	)
	if err != nil {
		t.Fatalf("CreateAgent %q: %v", agentKey, err)
	}
	t.Cleanup(func() {
		db.ExecContext(context.Background(), "DELETE FROM agents WHERE id = $1", id)
	})
	return id
}

// CreateSession inserts a minimal session row for testing and returns its UUID.
func CreateSession(t *testing.T, db *sql.DB, tenantID uuid.UUID, sessionKey string, agentID uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO sessions (id, session_key, messages, agent_id, created_at, updated_at, tenant_id)
		 VALUES ($1, $2, '[]', $3, NOW(), NOW(), $4)`,
		id, sessionKey, agentID, tenantID,
	)
	if err != nil {
		t.Fatalf("CreateSession %q: %v", sessionKey, err)
	}
	t.Cleanup(func() {
		db.ExecContext(context.Background(), "DELETE FROM sessions WHERE id = $1", id)
	})
	return id
}

// CreateCronJob inserts a minimal cron_jobs row and returns its UUID.
func CreateCronJob(t *testing.T, db *sql.DB, tenantID uuid.UUID, agentID uuid.UUID, name string) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO cron_jobs (id, name, agent_id, enabled, schedule_kind, payload, delete_after_run, created_at, updated_at, tenant_id)
		 VALUES ($1, $2, $3, true, 'every', '{"kind":"agent_turn","message":"test","deliver":false}', false, NOW(), NOW(), $4)`,
		id, name, agentID, tenantID,
	)
	if err != nil {
		t.Fatalf("CreateCronJob %q: %v", name, err)
	}
	t.Cleanup(func() {
		db.ExecContext(context.Background(), "DELETE FROM cron_jobs WHERE id = $1", id)
	})
	return id
}

// CreateSkill inserts a minimal skill row and returns its UUID.
func CreateSkill(t *testing.T, db *sql.DB, tenantID uuid.UUID, name, slug string) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO skills (id, name, slug, owner_id, visibility, version, status, enabled, file_path, file_size, created_at, updated_at, tenant_id)
		 VALUES ($1, $2, $3, 'system', 'private', 1, 'active', true, '', 0, NOW(), NOW(), $4)`,
		id, name, slug, tenantID,
	)
	if err != nil {
		t.Fatalf("CreateSkill %q: %v", slug, err)
	}
	t.Cleanup(func() {
		db.ExecContext(context.Background(), "DELETE FROM skills WHERE id = $1", id)
	})
	return id
}
