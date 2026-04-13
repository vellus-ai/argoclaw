package pg

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/vellus-ai/argoclaw/internal/providers"
	"github.com/vellus-ai/argoclaw/internal/store"
)

// PGSessionStore implements store.SessionStore backed by Postgres.
type PGSessionStore struct {
	db *sql.DB
	mu sync.RWMutex
	// In-memory cache for hot sessions (reduces DB reads during tool loops)
	cache map[string]*store.SessionData
	// OnDelete is called with the session key when a session is deleted.
	// Used for media file cleanup.
	OnDelete func(sessionKey string)
}

func NewPGSessionStore(db *sql.DB) *PGSessionStore {
	s := &PGSessionStore{
		db:    db,
		cache: make(map[string]*store.SessionData),
	}
	s.migrateLegacyWSKeys()
	return s
}

// migrateLegacyWSKeys renames old WS session keys from non-canonical format
// (agent:X:ws-userId-ts) to canonical format (agent:X:ws:direct:ts).
// The last hyphen-delimited segment is the base36 timestamp used as convId.
// Idempotent — no-op if no legacy keys exist.
func (s *PGSessionStore) migrateLegacyWSKeys() {
	res, err := s.db.Exec(`
		UPDATE sessions
		SET session_key = regexp_replace(
			session_key,
			'^(agent:[^:]+):ws-.+-([^-]+)$',
			'\1:ws:direct:\2'
		)
		WHERE session_key ~ '^agent:[^:]+:ws-'
	`)
	if err != nil {
		slog.Warn("sessions.migrate_legacy_ws_keys", "error", err)
		return
	}
	if n, _ := res.RowsAffected(); n > 0 {
		slog.Info("sessions.migrate_legacy_ws_keys", "migrated", n)
	}
}

func (s *PGSessionStore) GetOrCreate(ctx context.Context, key string) *store.SessionData {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cached, ok := s.cache[key]; ok {
		return cached
	}

	tid, err := requireTenantID(ctx)
	if err != nil {
		slog.Warn("sessions.GetOrCreate: tenant_id required", "key", key, "error", err)
		return &store.SessionData{
			Key:      key,
			Messages: []providers.Message{},
			Created:  time.Now(),
			Updated:  time.Now(),
		}
	}
	data := s.loadFromDB(key, tid)
	if data != nil {
		s.cache[key] = data
		return data
	}

	// Create new
	now := time.Now()
	data = &store.SessionData{
		Key:      key,
		Messages: []providers.Message{},
		Created:  now,
		Updated:  now,
	}

	// Extract team_id from team session keys (agent:{agentId}:team:{teamId}:{chatId}).
	var teamID *uuid.UUID
	if parts := strings.SplitN(key, ":", 5); len(parts) >= 4 && parts[2] == "team" {
		if teamUUID, err := uuid.Parse(parts[3]); err == nil {
			teamID = &teamUUID
			data.TeamID = teamID
		}
	}
	s.cache[key] = data

	msgsJSON, _ := json.Marshal([]providers.Message{})
	tenantIDPtr := nilSessionUUID(tid)
	s.db.ExecContext(ctx,
		`INSERT INTO sessions (id, session_key, messages, created_at, updated_at, team_id, tenant_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT (session_key) DO NOTHING`,
		uuid.Must(uuid.NewV7()), key, msgsJSON, now, now, teamID, tenantIDPtr,
	)

	return data
}

func (s *PGSessionStore) AddMessage(key string, msg providers.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data := s.getOrInit(key)
	data.Messages = append(data.Messages, msg)
	data.Updated = time.Now()
}

func (s *PGSessionStore) GetHistory(key string) []providers.Message {
	s.mu.RLock()
	if data, ok := s.cache[key]; ok {
		msgs := make([]providers.Message, len(data.Messages))
		copy(msgs, data.Messages)
		s.mu.RUnlock()
		return msgs
	}
	s.mu.RUnlock()

	// Not in cache — load from DB and cache it
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if data, ok := s.cache[key]; ok {
		msgs := make([]providers.Message, len(data.Messages))
		copy(msgs, data.Messages)
		return msgs
	}

	// GetHistory is called from hot path without ctx; use uuid.Nil (no tenant filter).
	// Session was already loaded via GetOrCreate with proper tenant check.
	data := s.loadFromDB(key, uuid.Nil)
	if data == nil {
		return nil
	}
	s.cache[key] = data
	msgs := make([]providers.Message, len(data.Messages))
	copy(msgs, data.Messages)
	return msgs
}

func (s *PGSessionStore) GetSummary(key string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if data, ok := s.cache[key]; ok {
		return data.Summary
	}
	return ""
}

func (s *PGSessionStore) SetSummary(key, summary string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if data, ok := s.cache[key]; ok {
		data.Summary = summary
		data.Updated = time.Now()
	}
}

func (s *PGSessionStore) GetLabel(key string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if data, ok := s.cache[key]; ok {
		return data.Label
	}
	return ""
}

func (s *PGSessionStore) SetLabel(key, label string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if data, ok := s.cache[key]; ok {
		data.Label = label
		data.Updated = time.Now()
	}
}

func (s *PGSessionStore) GetSessionMetadata(key string) map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if data, ok := s.cache[key]; ok && data.Metadata != nil {
		out := make(map[string]string, len(data.Metadata))
		maps.Copy(out, data.Metadata)
		return out
	}
	return nil
}

func (s *PGSessionStore) SetSessionMetadata(key string, metadata map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data := s.getOrInit(key)
	if data.Metadata == nil {
		data.Metadata = make(map[string]string)
	}
	maps.Copy(data.Metadata, metadata)
	data.Updated = time.Now()
}

func (s *PGSessionStore) SetAgentInfo(key string, agentUUID uuid.UUID, userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data := s.getOrInit(key)
	if agentUUID != uuid.Nil {
		data.AgentUUID = agentUUID
	}
	if userID != "" {
		data.UserID = userID
	}
}

func (s *PGSessionStore) UpdateMetadata(key, model, provider, channel string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if data, ok := s.cache[key]; ok {
		if model != "" {
			data.Model = model
		}
		if provider != "" {
			data.Provider = provider
		}
		if channel != "" {
			data.Channel = channel
		}
	}
}

// tenantFilter returns an additional " AND tenant_id = $N" clause and appended args
// when the tenant ID is not nil. nextIdx should be the next positional parameter index.
func tenantFilter(tid uuid.UUID, nextIdx int, args []any) (string, []any, int) {
	if tid == uuid.Nil {
		return "", args, nextIdx
	}
	clause := fmt.Sprintf(" AND tenant_id = $%d", nextIdx)
	args = append(args, tid)
	return clause, args, nextIdx + 1
}

func (s *PGSessionStore) getOrInit(key string) *store.SessionData {
	if data, ok := s.cache[key]; ok {
		return data
	}

	// Try loading from DB first to avoid overwriting existing messages.
	// getOrInit is called from in-memory-only methods (SetLabel, etc.) which don't have ctx.
	// Use uuid.Nil for tenant — these methods operate on already-cached sessions.
	data := s.loadFromDB(key, uuid.Nil)
	if data != nil {
		s.cache[key] = data
		return data
	}

	// Not in DB — initialize in-memory only (no ctx/tenant_id available here).
	// Sessions must be persisted via GetOrCreate(ctx) to include the correct tenant_id.
	// Inserting without tenant_id would create an orphaned row that bypasses tenant isolation.
	slog.Warn("sessions.getOrInit: session not in DB, creating in-memory only — caller should use GetOrCreate first", "key", key)
	now := time.Now()
	data = &store.SessionData{
		Key:      key,
		Messages: []providers.Message{},
		Created:  now,
		Updated:  now,
	}
	s.cache[key] = data
	return data
}

// loadFromDB loads a session from the database. When tid != uuid.Nil, adds a tenant_id filter.
func (s *PGSessionStore) loadFromDB(key string, tid uuid.UUID) *store.SessionData {
	var sessionKey string
	var msgsJSON []byte
	var summary, model, provider, channel, label, spawnedBy, userID *string
	var agentID, teamID *uuid.UUID
	var inputTokens, outputTokens int64
	var compactionCount, memoryFlushCompactionCount, spawnDepth int
	var memoryFlushAt int64
	var createdAt, updatedAt time.Time
	var metaJSON *[]byte

	q := `SELECT session_key, messages, summary, model, provider, channel,
		 input_tokens, output_tokens, compaction_count,
		 memory_flush_compaction_count, memory_flush_at,
		 label, spawned_by, spawn_depth, agent_id, user_id,
		 COALESCE(metadata, '{}'), created_at, updated_at, team_id
		 FROM sessions WHERE session_key = $1`
	args := []any{key}
	if tid != uuid.Nil {
		q += " AND tenant_id = $2"
		args = append(args, tid)
	}

	err := s.db.QueryRow(q, args...).Scan(&sessionKey, &msgsJSON, &summary, &model, &provider, &channel,
		&inputTokens, &outputTokens, &compactionCount,
		&memoryFlushCompactionCount, &memoryFlushAt,
		&label, &spawnedBy, &spawnDepth, &agentID, &userID,
		&metaJSON, &createdAt, &updatedAt, &teamID)
	if err != nil {
		return nil
	}

	var msgs []providers.Message
	json.Unmarshal(msgsJSON, &msgs)

	var meta map[string]string
	if metaJSON != nil {
		json.Unmarshal(*metaJSON, &meta)
	}

	return &store.SessionData{
		Key:                        sessionKey,
		Messages:                   msgs,
		Summary:                    derefStr(summary),
		Created:                    createdAt,
		Updated:                    updatedAt,
		AgentUUID:                  derefUUID(agentID),
		UserID:                     derefStr(userID),
		TeamID:                     teamID,
		Model:                      derefStr(model),
		Provider:                   derefStr(provider),
		Channel:                    derefStr(channel),
		InputTokens:                inputTokens,
		OutputTokens:               outputTokens,
		CompactionCount:            compactionCount,
		MemoryFlushCompactionCount: memoryFlushCompactionCount,
		MemoryFlushAt:              memoryFlushAt,
		Label:                      derefStr(label),
		SpawnedBy:                  derefStr(spawnedBy),
		SpawnDepth:                 spawnDepth,
		Metadata:                   meta,
	}
}

func nilSessionUUID(u uuid.UUID) *uuid.UUID {
	if u == uuid.Nil {
		return nil
	}
	return &u
}
