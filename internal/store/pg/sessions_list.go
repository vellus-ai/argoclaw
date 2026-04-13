package pg

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/vellus-ai/argoclaw/internal/providers"
	"github.com/vellus-ai/argoclaw/internal/store"
)

// buildSessionFilter builds a dynamic WHERE clause from SessionListOpts.
// Returns the WHERE string (with leading " WHERE ") and the positional args.
// The tableAlias is prepended to column names (e.g. "s" → "s.session_key").
// When tid != uuid.Nil, a tenant_id filter is added.
func buildSessionFilter(opts store.SessionListOpts, tableAlias string, tid uuid.UUID) (string, []any) {
	prefix := ""
	if tableAlias != "" {
		prefix = tableAlias + "."
	}
	var conditions []string
	var args []any
	idx := 1

	if tid != uuid.Nil {
		conditions = append(conditions, fmt.Sprintf("%stenant_id = $%d", prefix, idx))
		args = append(args, tid)
		idx++
	}
	if opts.AgentID != "" {
		conditions = append(conditions, fmt.Sprintf("%ssession_key LIKE $%d", prefix, idx))
		args = append(args, "agent:"+opts.AgentID+":%")
		idx++
	}
	if opts.Channel != "" {
		// Match canonical format: agent:X:{channel}:...
		conditions = append(conditions, fmt.Sprintf("%ssession_key LIKE $%d", prefix, idx))
		args = append(args, "agent:%:"+opts.Channel+":%")
		idx++
	}
	if opts.UserID != "" {
		conditions = append(conditions, fmt.Sprintf("%suser_id = $%d", prefix, idx))
		args = append(args, opts.UserID)
		idx++
	}

	if len(conditions) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

func (s *PGSessionStore) List(ctx context.Context, agentID string) []store.SessionInfo {
	tid, err := requireTenantID(ctx)
	if err != nil {
		return nil
	}
	var rows *sql.Rows

	q := "SELECT session_key, messages, created_at, updated_at, label, channel, user_id, COALESCE(metadata, '{}') FROM sessions WHERE 1=1"
	var args []any
	idx := 1

	if tid != uuid.Nil {
		q += fmt.Sprintf(" AND tenant_id = $%d", idx)
		args = append(args, tid)
		idx++
	}
	if agentID != "" {
		q += fmt.Sprintf(" AND session_key LIKE $%d", idx)
		args = append(args, "agent:"+agentID+":%")
		idx++
	}
	q += " ORDER BY updated_at DESC"

	rows, err = s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var result []store.SessionInfo
	for rows.Next() {
		var key string
		var msgsJSON []byte
		var createdAt, updatedAt time.Time
		var label, channel, userID *string
		var metaJSON []byte
		if err := rows.Scan(&key, &msgsJSON, &createdAt, &updatedAt, &label, &channel, &userID, &metaJSON); err != nil {
			continue
		}
		var msgs []providers.Message
		json.Unmarshal(msgsJSON, &msgs)
		var meta map[string]string
		if len(metaJSON) > 0 {
			json.Unmarshal(metaJSON, &meta)
		}
		result = append(result, store.SessionInfo{
			Key:          key,
			MessageCount: len(msgs),
			Created:      createdAt,
			Updated:      updatedAt,
			Label:        derefStr(label),
			Channel:      derefStr(channel),
			UserID:       derefStr(userID),
			Metadata:     meta,
		})
	}
	return result
}

func (s *PGSessionStore) ListPaged(ctx context.Context, opts store.SessionListOpts) store.SessionListResult {
	tid, err := requireTenantID(ctx)
	if err != nil {
		return store.SessionListResult{Sessions: []store.SessionInfo{}, Total: 0}
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := max(opts.Offset, 0)

	where, whereArgs := buildSessionFilter(opts, "", tid)

	// Count total
	var total int
	countQ := "SELECT COUNT(*) FROM sessions" + where
	if err := s.db.QueryRowContext(ctx, countQ, whereArgs...).Scan(&total); err != nil {
		return store.SessionListResult{Sessions: []store.SessionInfo{}, Total: 0}
	}

	// Fetch page using jsonb_array_length to avoid loading full messages
	nextIdx := len(whereArgs) + 1
	selectQ := fmt.Sprintf(`SELECT session_key, jsonb_array_length(messages), created_at, updated_at, label, channel, user_id, COALESCE(metadata, '{}')
		FROM sessions%s ORDER BY updated_at DESC LIMIT $%d OFFSET $%d`, where, nextIdx, nextIdx+1)
	selectArgs := append(append([]any{}, whereArgs...), limit, offset)

	rows, err := s.db.QueryContext(ctx, selectQ, selectArgs...)
	if err != nil {
		return store.SessionListResult{Sessions: []store.SessionInfo{}, Total: total}
	}
	defer rows.Close()

	var result []store.SessionInfo
	for rows.Next() {
		var key string
		var msgCount int
		var createdAt, updatedAt time.Time
		var label, channel, userID *string
		var metaJSON []byte
		if err := rows.Scan(&key, &msgCount, &createdAt, &updatedAt, &label, &channel, &userID, &metaJSON); err != nil {
			continue
		}
		var meta map[string]string
		if len(metaJSON) > 0 {
			json.Unmarshal(metaJSON, &meta)
		}
		result = append(result, store.SessionInfo{
			Key:          key,
			MessageCount: msgCount,
			Created:      createdAt,
			Updated:      updatedAt,
			Label:        derefStr(label),
			Channel:      derefStr(channel),
			UserID:       derefStr(userID),
			Metadata:     meta,
		})
	}
	if result == nil {
		result = []store.SessionInfo{}
	}
	return store.SessionListResult{Sessions: result, Total: total}
}

// ListPagedRich returns enriched session info for API responses (includes model, tokens, agent name).
func (s *PGSessionStore) ListPagedRich(ctx context.Context, opts store.SessionListOpts) store.SessionListRichResult {
	tid, err := requireTenantID(ctx)
	if err != nil {
		return store.SessionListRichResult{Sessions: []store.SessionInfoRich{}, Total: 0}
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := max(opts.Offset, 0)

	where, whereArgs := buildSessionFilter(opts, "s", tid)

	// Count total
	var total int
	countQ := "SELECT COUNT(*) FROM sessions s" + where
	if err := s.db.QueryRowContext(ctx, countQ, whereArgs...).Scan(&total); err != nil {
		return store.SessionListRichResult{Sessions: []store.SessionInfoRich{}, Total: 0}
	}

	// Fetch page with agent name via LEFT JOIN
	const richCols = `s.session_key, jsonb_array_length(s.messages), s.created_at, s.updated_at,
		s.label, s.channel, s.user_id, COALESCE(s.metadata, '{}'),
		s.model, s.provider, s.input_tokens, s.output_tokens,
		COALESCE(a.display_name, ''),
		octet_length(s.messages::text) / 4 + 12000,
		COALESCE(a.context_window, 200000), -- config.DefaultContextWindow
		s.compaction_count`

	nextIdx := len(whereArgs) + 1
	selectQ := fmt.Sprintf(`SELECT %s
		FROM sessions s LEFT JOIN agents a ON s.agent_id = a.id
		%s ORDER BY s.updated_at DESC LIMIT $%d OFFSET $%d`, richCols, where, nextIdx, nextIdx+1)
	selectArgs := append(append([]any{}, whereArgs...), limit, offset)

	rows, err := s.db.QueryContext(ctx, selectQ, selectArgs...)
	if err != nil {
		return store.SessionListRichResult{Sessions: []store.SessionInfoRich{}, Total: total}
	}
	defer rows.Close()

	var result []store.SessionInfoRich
	for rows.Next() {
		var key string
		var msgCount int
		var createdAt, updatedAt time.Time
		var label, channel, userID *string
		var metaJSON []byte
		var model, provider *string
		var inputTokens, outputTokens int64
		var agentName string
		var estimatedTokens, contextWindow, compactionCount int
		if err := rows.Scan(&key, &msgCount, &createdAt, &updatedAt, &label, &channel, &userID, &metaJSON,
			&model, &provider, &inputTokens, &outputTokens, &agentName,
			&estimatedTokens, &contextWindow, &compactionCount); err != nil {
			continue
		}
		var meta map[string]string
		if len(metaJSON) > 0 {
			json.Unmarshal(metaJSON, &meta)
		}
		result = append(result, store.SessionInfoRich{
			SessionInfo: store.SessionInfo{
				Key:          key,
				MessageCount: msgCount,
				Created:      createdAt,
				Updated:      updatedAt,
				Label:        derefStr(label),
				Channel:      derefStr(channel),
				UserID:       derefStr(userID),
				Metadata:     meta,
			},
			Model:           derefStr(model),
			Provider:        derefStr(provider),
			InputTokens:     inputTokens,
			OutputTokens:    outputTokens,
			AgentName:       agentName,
			EstimatedTokens: estimatedTokens,
			ContextWindow:   contextWindow,
			CompactionCount: compactionCount,
		})
	}
	if result == nil {
		result = []store.SessionInfoRich{}
	}
	return store.SessionListRichResult{Sessions: result, Total: total}
}

func (s *PGSessionStore) Save(ctx context.Context, key string) error {
	s.mu.RLock()
	data, ok := s.cache[key]
	if !ok {
		s.mu.RUnlock()
		return nil
	}
	// Snapshot
	snapshot := *data
	msgs := make([]providers.Message, len(data.Messages))
	copy(msgs, data.Messages)
	snapshot.Messages = msgs
	s.mu.RUnlock()

	msgsJSON, _ := json.Marshal(snapshot.Messages)
	metaJSON := []byte("{}")
	if len(snapshot.Metadata) > 0 {
		metaJSON, _ = json.Marshal(snapshot.Metadata)
	}

	tid, err := requireTenantID(ctx)
	if err != nil {
		return err
	}
	q := `UPDATE sessions SET
			messages = $1, summary = $2, model = $3, provider = $4, channel = $5,
			input_tokens = $6, output_tokens = $7, compaction_count = $8,
			memory_flush_compaction_count = $9, memory_flush_at = $10,
			label = $11, spawned_by = $12, spawn_depth = $13,
			agent_id = $14, user_id = $15, metadata = $16, updated_at = $17,
			team_id = $18
		 WHERE session_key = $19`
	args := []any{
		msgsJSON, nilStr(snapshot.Summary), nilStr(snapshot.Model), nilStr(snapshot.Provider), nilStr(snapshot.Channel),
		snapshot.InputTokens, snapshot.OutputTokens, snapshot.CompactionCount,
		snapshot.MemoryFlushCompactionCount, snapshot.MemoryFlushAt,
		nilStr(snapshot.Label), nilStr(snapshot.SpawnedBy), snapshot.SpawnDepth,
		nilSessionUUID(snapshot.AgentUUID), nilStr(snapshot.UserID), metaJSON, snapshot.Updated,
		snapshot.TeamID,
		key,
	}
	if tid != uuid.Nil {
		q += " AND tenant_id = $20"
		args = append(args, tid)
	}
	_, err = s.db.ExecContext(ctx, q, args...)
	return err
}

func (s *PGSessionStore) LastUsedChannel(ctx context.Context, agentID string) (string, string) {
	tid, err := requireTenantID(ctx)
	if err != nil {
		return "", ""
	}
	prefix := "agent:" + agentID + ":%"
	q := `SELECT session_key FROM sessions
		 WHERE session_key LIKE $1
		   AND session_key NOT LIKE $2
		   AND session_key NOT LIKE $3`
	args := []any{
		prefix,
		"agent:" + agentID + ":cron:%",
		"agent:" + agentID + ":subagent:%",
	}
	if tid != uuid.Nil {
		q += " AND tenant_id = $4"
		args = append(args, tid)
	}
	q += " ORDER BY updated_at DESC LIMIT 1"

	var sessionKey string
	if err := s.db.QueryRowContext(ctx, q, args...).Scan(&sessionKey); err != nil {
		return "", ""
	}
	parts := strings.SplitN(sessionKey, ":", 5)
	if len(parts) >= 5 {
		return parts[2], parts[4]
	}
	return "", ""
}

