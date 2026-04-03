package pg

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/vellus-ai/argoclaw/internal/store"
)

// PGPluginStore implements store.PluginStore backed by PostgreSQL.
type PGPluginStore struct {
	db *sql.DB
}

// NewPGPluginStore creates a new PGPluginStore using the provided database connection.
func NewPGPluginStore(db *sql.DB) *PGPluginStore {
	return &PGPluginStore{db: db}
}

// ─────────────────────────────────────────────────────────────────────────────
// Catalog
// ─────────────────────────────────────────────────────────────────────────────

func (s *PGPluginStore) UpsertCatalogEntry(ctx context.Context, e *store.PluginCatalogEntry) error {
	if e.ID == uuid.Nil {
		e.ID = store.GenNewID()
	}
	now := time.Now().UTC()
	e.CreatedAt = now
	e.UpdatedAt = now

	manifest := jsonOrEmpty(e.Manifest)
	tags := pqStringArray(e.Tags)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO plugin_catalog
			(id, tenant_id, name, version, display_name, description, author,
			 manifest, source, min_plan, checksum, tags, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		ON CONFLICT (tenant_id, name, version) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			description  = EXCLUDED.description,
			author       = EXCLUDED.author,
			manifest     = EXCLUDED.manifest,
			source       = EXCLUDED.source,
			min_plan     = EXCLUDED.min_plan,
			checksum     = EXCLUDED.checksum,
			tags         = EXCLUDED.tags,
			updated_at   = EXCLUDED.updated_at`,
		e.ID, nilUUID(e.TenantID), e.Name, e.Version, e.DisplayName,
		e.Description, e.Author, manifest, e.Source, e.MinPlan,
		nilStr(e.Checksum), tags, now, now,
	)
	return err
}

func (s *PGPluginStore) GetCatalogEntry(ctx context.Context, id uuid.UUID) (*store.PluginCatalogEntry, error) {
	tid := tenantIDFromCtx(ctx)
	row := s.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, name, version, display_name, description, author,
		       manifest, source, min_plan, checksum, tags, created_at, updated_at
		FROM plugin_catalog
		WHERE id = $1
		  AND (tenant_id IS NULL OR tenant_id = $2)`,
		id, nilUUID(&tid),
	)
	return s.scanCatalogEntry(row)
}

func (s *PGPluginStore) GetCatalogEntryByName(ctx context.Context, name string) (*store.PluginCatalogEntry, error) {
	tid := tenantIDFromCtx(ctx)
	row := s.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, name, version, display_name, description, author,
		       manifest, source, min_plan, checksum, tags, created_at, updated_at
		FROM plugin_catalog
		WHERE name = $1
		  AND (tenant_id IS NULL OR tenant_id = $2)
		ORDER BY created_at DESC
		LIMIT 1`,
		name, nilUUID(&tid),
	)
	return s.scanCatalogEntry(row)
}

func (s *PGPluginStore) ListCatalog(ctx context.Context) ([]store.PluginCatalogEntry, error) {
	tid := tenantIDFromCtx(ctx)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, name, version, display_name, description, author,
		       manifest, source, min_plan, checksum, tags, created_at, updated_at
		FROM plugin_catalog
		WHERE tenant_id IS NULL OR tenant_id = $1
		ORDER BY name, version`,
		nilUUID(&tid),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanCatalogEntries(rows)
}

func (s *PGPluginStore) scanCatalogEntry(row *sql.Row) (*store.PluginCatalogEntry, error) {
	var e store.PluginCatalogEntry
	var tenantID *uuid.UUID
	var manifest []byte
	var checksum *string
	var tagsRaw []byte

	err := row.Scan(
		&e.ID, &tenantID, &e.Name, &e.Version, &e.DisplayName,
		&e.Description, &e.Author, &manifest, &e.Source, &e.MinPlan,
		&checksum, &tagsRaw, &e.CreatedAt, &e.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, store.ErrPluginNotFound
	}
	if err != nil {
		return nil, err
	}
	e.TenantID = tenantID
	e.Manifest = manifest
	e.Checksum = derefStr(checksum)
	scanStringArray(tagsRaw, &e.Tags)
	return &e, nil
}

func (s *PGPluginStore) scanCatalogEntries(rows *sql.Rows) ([]store.PluginCatalogEntry, error) {
	var entries []store.PluginCatalogEntry
	for rows.Next() {
		var e store.PluginCatalogEntry
		var tenantID *uuid.UUID
		var manifest []byte
		var checksum *string
		var tagsRaw []byte

		if err := rows.Scan(
			&e.ID, &tenantID, &e.Name, &e.Version, &e.DisplayName,
			&e.Description, &e.Author, &manifest, &e.Source, &e.MinPlan,
			&checksum, &tagsRaw, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, err
		}
		e.TenantID = tenantID
		e.Manifest = manifest
		e.Checksum = derefStr(checksum)
		scanStringArray(tagsRaw, &e.Tags)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// ─────────────────────────────────────────────────────────────────────────────
// Tenant plugin management
// ─────────────────────────────────────────────────────────────────────────────

func (s *PGPluginStore) InstallPlugin(ctx context.Context, tp *store.TenantPlugin) error {
	if tp.ID == uuid.Nil {
		tp.ID = store.GenNewID()
	}
	tid := tenantIDFromCtx(ctx)
	if tid == uuid.Nil {
		return fmt.Errorf("tenant context required for InstallPlugin")
	}
	tp.TenantID = tid
	now := time.Now().UTC()
	tp.CreatedAt = now
	tp.UpdatedAt = now

	config := jsonOrEmpty(tp.Config)
	perms := jsonOrEmpty(tp.Permissions)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	_, err = tx.ExecContext(ctx, `
		INSERT INTO tenant_plugins
			(id, tenant_id, plugin_name, plugin_version, state,
			 config, permissions, installed_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		tp.ID, tid, tp.PluginName, tp.PluginVersion,
		store.PluginStateInstalled, config, perms,
		nilUUID(tp.InstalledBy), now, now,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return store.ErrPluginAlreadyInstalled
		}
		return fmt.Errorf("insert tenant_plugins: %w", err)
	}

	if err := s.writeAuditTx(ctx, tx, tid, tp.PluginName, store.AuditInstall, tp.InstalledBy, "system", nil); err != nil {
		return fmt.Errorf("audit install: %w", err)
	}

	return tx.Commit()
}

func (s *PGPluginStore) EnablePlugin(ctx context.Context, pluginName string, actorID *uuid.UUID) error {
	tid := tenantIDFromCtx(ctx)
	if tid == uuid.Nil {
		return fmt.Errorf("tenant context required")
	}
	now := time.Now().UTC()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	res, err := tx.ExecContext(ctx, `
		UPDATE tenant_plugins
		SET state = $1, enabled_at = $2, updated_at = $3
		WHERE tenant_id = $4 AND plugin_name = $5`,
		store.PluginStateEnabled, now, now, tid, pluginName,
	)
	if err != nil {
		return fmt.Errorf("enable plugin: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return store.ErrPluginNotFound
	}

	if err := s.writeAuditTx(ctx, tx, tid, pluginName, store.AuditEnable, actorID, "user", nil); err != nil {
		return fmt.Errorf("audit enable: %w", err)
	}
	return tx.Commit()
}

func (s *PGPluginStore) DisablePlugin(ctx context.Context, pluginName string, actorID *uuid.UUID) error {
	tid := tenantIDFromCtx(ctx)
	if tid == uuid.Nil {
		return fmt.Errorf("tenant context required")
	}
	now := time.Now().UTC()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	res, err := tx.ExecContext(ctx, `
		UPDATE tenant_plugins
		SET state = $1, updated_at = $2
		WHERE tenant_id = $3 AND plugin_name = $4`,
		store.PluginStateDisabled, now, tid, pluginName,
	)
	if err != nil {
		return fmt.Errorf("disable plugin: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return store.ErrPluginNotFound
	}

	if err := s.writeAuditTx(ctx, tx, tid, pluginName, store.AuditDisable, actorID, "user", nil); err != nil {
		return fmt.Errorf("audit disable: %w", err)
	}
	return tx.Commit()
}

func (s *PGPluginStore) UninstallPlugin(ctx context.Context, pluginName string, actorID *uuid.UUID) error {
	tid := tenantIDFromCtx(ctx)
	if tid == uuid.Nil {
		return fmt.Errorf("tenant context required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// Write audit BEFORE deleting so the tenant_id FK is still valid.
	if err := s.writeAuditTx(ctx, tx, tid, pluginName, store.AuditUninstall, actorID, "user", nil); err != nil {
		return fmt.Errorf("audit uninstall: %w", err)
	}

	// Delete agent_plugins first, then plugin_data, then tenant_plugins.
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM agent_plugins WHERE tenant_id = $1 AND plugin_name = $2`,
		tid, pluginName,
	); err != nil {
		return fmt.Errorf("delete agent_plugins: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM plugin_data WHERE tenant_id = $1 AND plugin_name = $2`,
		tid, pluginName,
	); err != nil {
		return fmt.Errorf("delete plugin_data: %w", err)
	}

	res, err := tx.ExecContext(ctx,
		`DELETE FROM tenant_plugins WHERE tenant_id = $1 AND plugin_name = $2`,
		tid, pluginName,
	)
	if err != nil {
		return fmt.Errorf("delete tenant_plugins: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return store.ErrPluginNotFound
	}

	return tx.Commit()
}

func (s *PGPluginStore) GetTenantPlugin(ctx context.Context, pluginName string) (*store.TenantPlugin, error) {
	tid := tenantIDFromCtx(ctx)
	row := s.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, plugin_name, plugin_version, state,
		       config, permissions, error_message, installed_by, enabled_at,
		       created_at, updated_at
		FROM tenant_plugins
		WHERE tenant_id = $1 AND plugin_name = $2`,
		tid, pluginName,
	)
	return s.scanTenantPlugin(row)
}

func (s *PGPluginStore) ListTenantPlugins(ctx context.Context) ([]store.TenantPlugin, error) {
	tid := tenantIDFromCtx(ctx)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, plugin_name, plugin_version, state,
		       config, permissions, error_message, installed_by, enabled_at,
		       created_at, updated_at
		FROM tenant_plugins
		WHERE tenant_id = $1
		ORDER BY plugin_name`,
		tid,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var plugins []store.TenantPlugin
	for rows.Next() {
		tp, err := s.scanTenantPlugin(sqlRowFromRows(rows))
		if err != nil {
			return nil, err
		}
		plugins = append(plugins, *tp)
	}
	return plugins, rows.Err()
}

func (s *PGPluginStore) UpdatePluginConfig(ctx context.Context, pluginName string, config json.RawMessage, actorID *uuid.UUID) error {
	tid := tenantIDFromCtx(ctx)
	if tid == uuid.Nil {
		return fmt.Errorf("tenant context required")
	}
	now := time.Now().UTC()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	res, err := tx.ExecContext(ctx, `
		UPDATE tenant_plugins
		SET config = $1, updated_at = $2
		WHERE tenant_id = $3 AND plugin_name = $4`,
		jsonOrEmpty(config), now, tid, pluginName,
	)
	if err != nil {
		return fmt.Errorf("update config: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return store.ErrPluginNotFound
	}

	details, _ := json.Marshal(map[string]any{"config_size": len(config)})
	if err := s.writeAuditTx(ctx, tx, tid, pluginName, store.AuditConfigChange, actorID, "user", details); err != nil {
		return fmt.Errorf("audit config: %w", err)
	}
	return tx.Commit()
}

func (s *PGPluginStore) SetPluginError(ctx context.Context, pluginName, errMsg string) error {
	tid := tenantIDFromCtx(ctx)
	_, err := s.db.ExecContext(ctx, `
		UPDATE tenant_plugins
		SET state = $1, error_message = $2, updated_at = $3
		WHERE tenant_id = $4 AND plugin_name = $5`,
		store.PluginStateError, errMsg, time.Now().UTC(), tid, pluginName,
	)
	return err
}

func (s *PGPluginStore) scanTenantPlugin(row scanner) (*store.TenantPlugin, error) {
	var tp store.TenantPlugin
	var errMsg *string
	var installedBy *uuid.UUID
	var enabledAt *time.Time
	var config, perms []byte

	err := row.Scan(
		&tp.ID, &tp.TenantID, &tp.PluginName, &tp.PluginVersion, &tp.State,
		&config, &perms, &errMsg, &installedBy, &enabledAt,
		&tp.CreatedAt, &tp.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, store.ErrPluginNotFound
	}
	if err != nil {
		return nil, err
	}
	tp.Config = config
	tp.Permissions = perms
	tp.ErrorMessage = derefStr(errMsg)
	tp.InstalledBy = installedBy
	tp.EnabledAt = enabledAt
	return &tp, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Agent plugin overrides
// ─────────────────────────────────────────────────────────────────────────────

func (s *PGPluginStore) SetAgentPlugin(ctx context.Context, ap *store.AgentPlugin) error {
	if ap.ID == uuid.Nil {
		ap.ID = store.GenNewID()
	}
	tid := tenantIDFromCtx(ctx)
	if tid == uuid.Nil {
		return fmt.Errorf("tenant context required")
	}
	ap.TenantID = tid
	now := time.Now().UTC()

	override := jsonOrEmpty(ap.ConfigOverride)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO agent_plugins
			(id, tenant_id, agent_id, plugin_name, enabled, config_override, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (tenant_id, agent_id, plugin_name) DO UPDATE SET
			enabled         = EXCLUDED.enabled,
			config_override = EXCLUDED.config_override,
			updated_at      = EXCLUDED.updated_at`,
		ap.ID, tid, ap.AgentID, ap.PluginName, ap.Enabled, override, now, now,
	)
	return err
}

func (s *PGPluginStore) GetAgentPlugin(ctx context.Context, agentID uuid.UUID, pluginName string) (*store.AgentPlugin, error) {
	tid := tenantIDFromCtx(ctx)
	row := s.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, agent_id, plugin_name, enabled, config_override, created_at, updated_at
		FROM agent_plugins
		WHERE tenant_id = $1 AND agent_id = $2 AND plugin_name = $3`,
		tid, agentID, pluginName,
	)
	return s.scanAgentPlugin(row)
}

func (s *PGPluginStore) ListAgentPlugins(ctx context.Context, agentID uuid.UUID) ([]store.AgentPlugin, error) {
	tid := tenantIDFromCtx(ctx)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, agent_id, plugin_name, enabled, config_override, created_at, updated_at
		FROM agent_plugins
		WHERE tenant_id = $1 AND agent_id = $2
		ORDER BY plugin_name`,
		tid, agentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var aps []store.AgentPlugin
	for rows.Next() {
		ap, err := s.scanAgentPlugin(sqlRowFromRows(rows))
		if err != nil {
			return nil, err
		}
		aps = append(aps, *ap)
	}
	return aps, rows.Err()
}

func (s *PGPluginStore) IsPluginEnabledForAgent(ctx context.Context, agentID uuid.UUID, pluginName string) (bool, error) {
	tid := tenantIDFromCtx(ctx)

	// Check tenant-level first.
	var tenantState string
	err := s.db.QueryRowContext(ctx, `
		SELECT state FROM tenant_plugins
		WHERE tenant_id = $1 AND plugin_name = $2`,
		tid, pluginName,
	).Scan(&tenantState)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if tenantState != store.PluginStateEnabled {
		return false, nil
	}

	// Check agent-level override.
	var agentEnabled bool
	err = s.db.QueryRowContext(ctx, `
		SELECT enabled FROM agent_plugins
		WHERE tenant_id = $1 AND agent_id = $2 AND plugin_name = $3`,
		tid, agentID, pluginName,
	).Scan(&agentEnabled)
	if errors.Is(err, sql.ErrNoRows) {
		// No override → inherit tenant-level (enabled).
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return agentEnabled, nil
}

func (s *PGPluginStore) scanAgentPlugin(row scanner) (*store.AgentPlugin, error) {
	var ap store.AgentPlugin
	var override []byte
	err := row.Scan(
		&ap.ID, &ap.TenantID, &ap.AgentID, &ap.PluginName,
		&ap.Enabled, &override, &ap.CreatedAt, &ap.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, store.ErrPluginNotFound
	}
	if err != nil {
		return nil, err
	}
	ap.ConfigOverride = override
	return &ap, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Plugin data (KV store)
// ─────────────────────────────────────────────────────────────────────────────

func (s *PGPluginStore) PutData(ctx context.Context, pluginName, collection, key string, value json.RawMessage, expiresAt *time.Time) error {
	tid := tenantIDFromCtx(ctx)
	if tid == uuid.Nil {
		return fmt.Errorf("tenant context required")
	}
	now := time.Now().UTC()
	id := store.GenNewID()

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO plugin_data
			(id, tenant_id, plugin_name, collection, key, value, expires_at, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (tenant_id, plugin_name, collection, key) DO UPDATE SET
			value      = EXCLUDED.value,
			expires_at = EXCLUDED.expires_at,
			updated_at = EXCLUDED.updated_at`,
		id, tid, pluginName, collection, key,
		jsonOrEmpty(value), nilTime(expiresAt), now, now,
	)
	return err
}

func (s *PGPluginStore) GetData(ctx context.Context, pluginName, collection, key string) (*store.PluginDataEntry, error) {
	tid := tenantIDFromCtx(ctx)
	var e store.PluginDataEntry
	var value []byte
	var expiresAt *time.Time

	err := s.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, plugin_name, collection, key, value, expires_at, created_at, updated_at
		FROM plugin_data
		WHERE tenant_id = $1 AND plugin_name = $2 AND collection = $3 AND key = $4
		  AND (expires_at IS NULL OR expires_at > NOW())`,
		tid, pluginName, collection, key,
	).Scan(
		&e.ID, &e.TenantID, &e.PluginName, &e.Collection, &e.Key,
		&value, &expiresAt, &e.CreatedAt, &e.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, store.ErrPluginNotFound
	}
	if err != nil {
		return nil, err
	}
	e.Value = value
	e.ExpiresAt = expiresAt
	return &e, nil
}

func (s *PGPluginStore) ListDataKeys(ctx context.Context, pluginName, collection, prefix string, limit, offset int) ([]string, error) {
	tid := tenantIDFromCtx(ctx)
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	var args []any
	q := `SELECT key FROM plugin_data
		WHERE tenant_id = $1 AND plugin_name = $2 AND collection = $3
		  AND (expires_at IS NULL OR expires_at > NOW())`
	args = append(args, tid, pluginName, collection)

	if prefix != "" {
		escaped := strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(prefix)
		args = append(args, escaped+"%")
		q += fmt.Sprintf(` AND key LIKE $%d ESCAPE '\'`, len(args))
	}
	q += fmt.Sprintf(" ORDER BY key LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (s *PGPluginStore) DeleteData(ctx context.Context, pluginName, collection, key string) error {
	tid := tenantIDFromCtx(ctx)
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM plugin_data
		WHERE tenant_id = $1 AND plugin_name = $2 AND collection = $3 AND key = $4`,
		tid, pluginName, collection, key,
	)
	return err
}

func (s *PGPluginStore) DeleteCollectionData(ctx context.Context, pluginName, collection string) error {
	tid := tenantIDFromCtx(ctx)
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM plugin_data
		WHERE tenant_id = $1 AND plugin_name = $2 AND collection = $3`,
		tid, pluginName, collection,
	)
	return err
}

// ─────────────────────────────────────────────────────────────────────────────
// Audit log
// ─────────────────────────────────────────────────────────────────────────────

func (s *PGPluginStore) LogAudit(ctx context.Context, entry *store.PluginAuditEntry) error {
	if entry.ID == uuid.Nil {
		entry.ID = store.GenNewID()
	}
	entry.CreatedAt = time.Now().UTC()
	details := jsonOrEmpty(entry.Details)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO plugin_audit_log
			(id, tenant_id, plugin_name, action, actor_id, actor_type, details, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		entry.ID, entry.TenantID, entry.PluginName, entry.Action,
		nilUUID(entry.ActorID), entry.ActorType, details, entry.CreatedAt,
	)
	return err
}

func (s *PGPluginStore) ListAuditLog(ctx context.Context, pluginName string, limit int) ([]store.PluginAuditEntry, error) {
	tid := tenantIDFromCtx(ctx)
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, plugin_name, action, actor_id, actor_type, details, created_at
		FROM plugin_audit_log
		WHERE tenant_id = $1 AND plugin_name = $2
		ORDER BY created_at DESC
		LIMIT $3`,
		tid, pluginName, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []store.PluginAuditEntry
	for rows.Next() {
		var e store.PluginAuditEntry
		var actorID *uuid.UUID
		var details []byte
		if err := rows.Scan(
			&e.ID, &e.TenantID, &e.PluginName, &e.Action,
			&actorID, &e.ActorType, &details, &e.CreatedAt,
		); err != nil {
			return nil, err
		}
		e.ActorID = actorID
		e.Details = details
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// ─────────────────────────────────────────────────────────────────────────────
// Internal helpers
// ─────────────────────────────────────────────────────────────────────────────

// writeAuditTx appends an audit entry within an existing transaction.
func (s *PGPluginStore) writeAuditTx(ctx context.Context, tx *sql.Tx, tenantID uuid.UUID, pluginName, action string, actorID *uuid.UUID, actorType string, details []byte) error {
	id := store.GenNewID()
	now := time.Now().UTC()
	if details == nil {
		details = []byte("{}")
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO plugin_audit_log
			(id, tenant_id, plugin_name, action, actor_id, actor_type, details, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		id, tenantID, pluginName, action,
		nilUUID(actorID), actorType, details, now,
	)
	return err
}

// isUniqueViolation returns true if err represents a PostgreSQL unique constraint violation (SQLSTATE 23505).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

// scanner is a common interface for *sql.Row and *sql.Rows to allow shared scan helpers.
type scanner interface {
	Scan(dest ...any) error
}

// sqlRowFromRows adapts *sql.Rows to the scanner interface.
type sqlRowsScanner struct{ rows *sql.Rows }

func (r *sqlRowsScanner) Scan(dest ...any) error { return r.rows.Scan(dest...) }

func sqlRowFromRows(rows *sql.Rows) scanner { return &sqlRowsScanner{rows: rows} }
