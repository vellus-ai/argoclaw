package pg

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/vellus-ai/argoclaw/internal/store"
)

// validColumnName matches safe SQL identifiers (letters, digits, underscores).
// Defense-in-depth: prevents column name injection in execMapUpdate.
var validColumnName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// --- Nullable helpers ---

func nilStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func nilInt(v int) *int {
	if v == 0 {
		return nil
	}
	return &v
}

func nilUUID(u *uuid.UUID) *uuid.UUID {
	if u == nil || *u == uuid.Nil {
		return nil
	}
	return u
}

func nilTime(t *time.Time) *time.Time {
	if t == nil || t.IsZero() {
		return nil
	}
	return t
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefUUID(u *uuid.UUID) uuid.UUID {
	if u == nil {
		return uuid.Nil
	}
	return *u
}

func derefBytes(b *[]byte) []byte {
	if b == nil {
		return nil
	}
	return *b
}

// --- JSON helpers ---

func jsonOrEmpty(data []byte) []byte {
	if data == nil {
		return []byte("{}")
	}
	return data
}

func jsonOrEmptyArray(data []byte) []byte {
	if data == nil {
		return []byte("[]")
	}
	return data
}

func jsonOrNull(data json.RawMessage) any {
	if data == nil {
		return nil
	}
	return []byte(data)
}

// --- PostgreSQL array helpers ---

// pqStringArray converts a Go string slice to a PostgreSQL text[] literal.
// Each element is double-quoted and escaped to prevent array literal injection.
func pqStringArray(arr []string) any {
	if arr == nil {
		return nil
	}
	quoted := make([]string, len(arr))
	for i, s := range arr {
		escaped := strings.ReplaceAll(s, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		quoted[i] = `"` + escaped + `"`
	}
	return "{" + strings.Join(quoted, ",") + "}"
}

// scanStringArray parses a PostgreSQL text[] column (scanned as []byte) into a Go string slice.
// Handles both quoted and unquoted elements in PostgreSQL array literal format.
func scanStringArray(data []byte, dest *[]string) {
	if data == nil || len(data) == 0 {
		return
	}
	s := string(data)
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	if s == "" {
		return
	}

	// Parse PostgreSQL array format: {val1,"quoted,val",val3}
	var result []string
	i := 0
	for i < len(s) {
		if s[i] == '"' {
			// Quoted element: find closing quote (handle escaped quotes)
			i++ // skip opening quote
			var elem strings.Builder
			for i < len(s) {
				if s[i] == '\\' && i+1 < len(s) {
					elem.WriteByte(s[i+1])
					i += 2
				} else if s[i] == '"' {
					i++ // skip closing quote
					break
				} else {
					elem.WriteByte(s[i])
					i++
				}
			}
			result = append(result, elem.String())
		} else {
			// Unquoted element: read until comma
			j := strings.IndexByte(s[i:], ',')
			if j < 0 {
				result = append(result, s[i:])
				break
			}
			result = append(result, s[i:i+j])
			i += j
		}
		// Skip comma separator
		if i < len(s) && s[i] == ',' {
			i++
		}
	}
	*dest = result
}

// allowedTables is a strict whitelist of tables that execMapUpdate may target.
// Any table not in this set is rejected to prevent SQL injection via table name.
var allowedTables = map[string]bool{
	"agents": true, "llm_providers": true, "sessions": true,
	"channel_instances": true, "cron_jobs": true, "custom_tools": true,
	"skills": true, "mcp_servers": true, "agent_links": true,
	"agent_teams": true, "team_tasks": true, "builtin_tools": true,
	"agent_context_files": true, "user_context_files": true,
	"user_agent_overrides": true, "config_secrets": true,
	"memory_documents": true, "memory_chunks": true, "embedding_cache": true,
	"secure_cli_binaries": true,
	"api_keys": true, "paired_devices": true, "team_messages": true,
	"delegation_history": true, "agent_shares": true, "user_agent_profiles": true,
	"team_tasks_comments": true, "team_task_events": true, "team_task_attachments": true,
	"tenants": true, "tenant_users": true, "tenant_branding": true,
	"users": true, "user_sessions": true, "password_history": true, "login_audit": true,
	// Plugin host tables (migration 000030)
	"plugin_catalog": true, "tenant_plugins": true, "agent_plugins": true,
	"plugin_data": true, "plugin_audit_log": true,
}

// validTableName returns true only if the table is in the strict whitelist.
func validTableName(table string) bool {
	return allowedTables[table]
}

// --- Dynamic UPDATE helper ---

// execMapUpdate builds and runs a dynamic UPDATE from a column→value map.
// Column names are validated against a strict identifier regex to prevent SQL injection.
// Table names are validated against a strict whitelist.
func execMapUpdate(ctx context.Context, db *sql.DB, table string, id uuid.UUID, updates map[string]any) error {
	if !validTableName(table) {
		slog.Warn("security.invalid_table_name", "table", table)
		return fmt.Errorf("invalid table name: %q", table)
	}
	if len(updates) == 0 {
		return nil
	}
	var setClauses []string
	var args []any
	i := 1
	for col, val := range updates {
		if !validColumnName.MatchString(col) {
			slog.Warn("security.invalid_column_name", "table", table, "column", col)
			return fmt.Errorf("invalid column name: %q", col)
		}
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, i))
		args = append(args, val)
		i++
	}
	// Auto-set updated_at for tables that have the column, unless caller already included it.
	if _, ok := updates["updated_at"]; !ok && tableHasUpdatedAt(table) {
		setClauses = append(setClauses, fmt.Sprintf("updated_at = $%d", i))
		args = append(args, time.Now().UTC())
		i++
	}
	args = append(args, id)
	q := fmt.Sprintf("UPDATE %s SET %s WHERE id = $%d", table, strings.Join(setClauses, ", "), i)
	_, err := db.ExecContext(ctx, q, args...)
	return err
}

// tablesWithUpdatedAt lists tables that have an updated_at column.
var tablesWithUpdatedAt = map[string]bool{
	"agents": true, "llm_providers": true, "sessions": true,
	"channel_instances": true, "cron_jobs": true, "custom_tools": true,
	"skills": true, "mcp_servers": true, "agent_links": true,
	"agent_teams": true, "team_tasks": true, "builtin_tools": true,
	"agent_context_files": true, "user_context_files": true,
	"user_agent_overrides": true, "config_secrets": true,
	"memory_documents": true, "memory_chunks": true, "embedding_cache": true,
	"secure_cli_binaries": true,
	"tenants": true, "tenant_branding": true, "users": true,
	// Plugin host tables
	"plugin_catalog": true, "tenant_plugins": true, "agent_plugins": true, "plugin_data": true,
}

func tableHasUpdatedAt(table string) bool {
	return tablesWithUpdatedAt[table]
}

// --- Multi-tenancy helpers ---

// tenantIDFromCtx extracts the tenant UUID from context.
// Returns uuid.Nil if no tenant is set (single-tenant / gateway token mode).
func tenantIDFromCtx(ctx context.Context) uuid.UUID {
	return store.TenantIDFromContext(ctx)
}

// requireTenantID enforces tenant_id presence in context (fail-closed).
// Returns (uuid.Nil, nil) if WithCrossTenant is set (intentional bypass).
// Returns (uuid.Nil, ErrTenantRequired) if tenant_id is missing or Nil.
// Returns (tenantID, nil) otherwise.
func requireTenantID(ctx context.Context) (uuid.UUID, error) {
	if store.IsCrossTenant(ctx) {
		return uuid.Nil, nil
	}
	tid := tenantIDFromCtx(ctx)
	if tid == uuid.Nil {
		return uuid.Nil, store.ErrTenantRequired
	}
	return tid, nil
}

// execMapUpdateTenant is like execMapUpdate but adds AND tenant_id = $N to the WHERE clause
// when a tenant_id is present in context. This prevents cross-tenant data modification.
func execMapUpdateTenant(ctx context.Context, db *sql.DB, table string, id uuid.UUID, updates map[string]any) error {
	if !validTableName(table) {
		slog.Warn("security.invalid_table_name", "table", table)
		return fmt.Errorf("invalid table name: %q", table)
	}
	if len(updates) == 0 {
		return nil
	}
	var setClauses []string
	var args []any
	i := 1
	for col, val := range updates {
		if !validColumnName.MatchString(col) {
			slog.Warn("security.invalid_column_name", "table", table, "column", col)
			return fmt.Errorf("invalid column name: %q", col)
		}
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, i))
		args = append(args, val)
		i++
	}
	if _, ok := updates["updated_at"]; !ok && tableHasUpdatedAt(table) {
		setClauses = append(setClauses, fmt.Sprintf("updated_at = $%d", i))
		args = append(args, time.Now().UTC())
		i++
	}
	args = append(args, id)
	where := fmt.Sprintf("id = $%d", i)
	i++

	tid, tidErr := requireTenantID(ctx)
	if tidErr != nil {
		return tidErr
	}
	if tid != uuid.Nil {
		args = append(args, tid)
		where += fmt.Sprintf(" AND tenant_id = $%d", i)
	}

	q := fmt.Sprintf("UPDATE %s SET %s WHERE %s", table, strings.Join(setClauses, ", "), where)
	result, err := db.ExecContext(ctx, q, args...)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 && tid != uuid.Nil {
		return fmt.Errorf("not found or tenant mismatch: %s/%s", table, id)
	}
	return nil
}
