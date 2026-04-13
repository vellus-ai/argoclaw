package pg

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/vellus-ai/argoclaw/internal/crypto"
	"github.com/vellus-ai/argoclaw/internal/store"
)

// PGChannelInstanceStore implements store.ChannelInstanceStore backed by Postgres.
type PGChannelInstanceStore struct {
	db     *sql.DB
	encKey string
}

func NewPGChannelInstanceStore(db *sql.DB, encryptionKey string) *PGChannelInstanceStore {
	return &PGChannelInstanceStore{db: db, encKey: encryptionKey}
}

const channelInstanceSelectCols = `id, name, display_name, channel_type, agent_id,
 credentials, config, enabled, created_by, created_at, updated_at`

func (s *PGChannelInstanceStore) Create(ctx context.Context, inst *store.ChannelInstanceData) error {
	if err := store.ValidateUserID(inst.CreatedBy); err != nil {
		return err
	}
	if inst.ID == uuid.Nil {
		inst.ID = store.GenNewID()
	}

	// Encrypt credentials if provided
	var credsBytes []byte
	if len(inst.Credentials) > 0 && s.encKey != "" {
		encrypted, err := crypto.Encrypt(string(inst.Credentials), s.encKey)
		if err != nil {
			return fmt.Errorf("encrypt credentials: %w", err)
		}
		credsBytes = []byte(encrypted)
	} else {
		credsBytes = inst.Credentials
	}

	tid, err := requireTenantID(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	inst.CreatedAt = now
	inst.UpdatedAt = now

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO channel_instances (id, tenant_id, name, display_name, channel_type, agent_id,
		 credentials, config, enabled, created_by, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		inst.ID, nilUUID(&tid), inst.Name, inst.DisplayName, inst.ChannelType, inst.AgentID,
		credsBytes, jsonOrEmpty(inst.Config),
		inst.Enabled, inst.CreatedBy, now, now,
	)
	return err
}

func (s *PGChannelInstanceStore) Get(ctx context.Context, id uuid.UUID) (*store.ChannelInstanceData, error) {
	tid, err := requireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	q := `SELECT ` + channelInstanceSelectCols + ` FROM channel_instances WHERE id = $1`
	args := []any{id}

	if tid != uuid.Nil {
		q += ` AND tenant_id = $2`
		args = append(args, tid)
	}

	row := s.db.QueryRowContext(ctx, q, args...)
	return s.scanInstance(row)
}

func (s *PGChannelInstanceStore) GetByName(ctx context.Context, name string) (*store.ChannelInstanceData, error) {
	tid, err := requireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	q := `SELECT ` + channelInstanceSelectCols + ` FROM channel_instances WHERE name = $1`
	args := []any{name}

	if tid != uuid.Nil {
		q += ` AND tenant_id = $2`
		args = append(args, tid)
	}

	row := s.db.QueryRowContext(ctx, q, args...)
	return s.scanInstance(row)
}

func (s *PGChannelInstanceStore) scanInstance(row *sql.Row) (*store.ChannelInstanceData, error) {
	var inst store.ChannelInstanceData
	var displayName *string
	var creds []byte
	var config *[]byte

	err := row.Scan(
		&inst.ID, &inst.Name, &displayName, &inst.ChannelType, &inst.AgentID,
		&creds, &config,
		&inst.Enabled, &inst.CreatedBy, &inst.CreatedAt, &inst.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	inst.DisplayName = derefStr(displayName)
	if config != nil {
		inst.Config = *config
	}

	// Decrypt credentials
	if len(creds) > 0 && s.encKey != "" {
		decrypted, err := crypto.Decrypt(string(creds), s.encKey)
		if err != nil {
			slog.Warn("channel_instances: failed to decrypt credentials", "name", inst.Name, "error", err)
		} else {
			inst.Credentials = []byte(decrypted)
		}
	} else {
		inst.Credentials = creds
	}

	return &inst, nil
}

func (s *PGChannelInstanceStore) scanInstances(rows *sql.Rows) ([]store.ChannelInstanceData, error) {
	defer rows.Close()
	var result []store.ChannelInstanceData
	for rows.Next() {
		var inst store.ChannelInstanceData
		var displayName *string
		var creds []byte
		var config *[]byte

		if err := rows.Scan(
			&inst.ID, &inst.Name, &displayName, &inst.ChannelType, &inst.AgentID,
			&creds, &config,
			&inst.Enabled, &inst.CreatedBy, &inst.CreatedAt, &inst.UpdatedAt,
		); err != nil {
			continue
		}

		inst.DisplayName = derefStr(displayName)
		if config != nil {
			inst.Config = *config
		}
		if len(creds) > 0 && s.encKey != "" {
			if decrypted, err := crypto.Decrypt(string(creds), s.encKey); err == nil {
				inst.Credentials = []byte(decrypted)
			}
		} else {
			inst.Credentials = creds
		}

		result = append(result, inst)
	}
	return result, nil
}

func (s *PGChannelInstanceStore) Update(ctx context.Context, id uuid.UUID, updates map[string]any) error {
	// Merge and encrypt credentials if present
	if credsVal, ok := updates["credentials"]; ok && credsVal != nil {
		var newCreds map[string]any
		switch v := credsVal.(type) {
		case map[string]any:
			newCreds = v
		default:
			var raw []byte
			switch vv := v.(type) {
			case []byte:
				raw = vv
			case string:
				raw = []byte(vv)
			default:
				if b, err := json.Marshal(v); err == nil {
					raw = b
				}
			}
			if len(raw) > 0 {
				if err := json.Unmarshal(raw, &newCreds); err != nil {
					newCreds = nil
				}
			}
		}

		// Merge with existing credentials so partial updates don't wipe other fields
		if len(newCreds) > 0 {
			existing, err := s.loadExistingCreds(ctx, id)
			if err != nil {
				return fmt.Errorf("load existing credentials for merge: %w", err)
			}
			maps.Copy(existing, newCreds)
			newCreds = existing
		}

		var credsBytes []byte
		if len(newCreds) > 0 {
			credsBytes, _ = json.Marshal(newCreds)
		}
		if len(credsBytes) > 0 && s.encKey != "" {
			encrypted, err := crypto.Encrypt(string(credsBytes), s.encKey)
			if err != nil {
				return fmt.Errorf("encrypt credentials: %w", err)
			}
			credsBytes = []byte(encrypted)
		}
		updates["credentials"] = credsBytes
	}
	updates["updated_at"] = time.Now()
	return execMapUpdateTenant(ctx, s.db, "channel_instances", id, updates)
}

// loadExistingCreds reads and decrypts the current credentials for merging.
func (s *PGChannelInstanceStore) loadExistingCreds(ctx context.Context, id uuid.UUID) (map[string]any, error) {
	q := "SELECT credentials FROM channel_instances WHERE id = $1"
	args := []any{id}

	tid, err := requireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	if tid != uuid.Nil {
		q += " AND tenant_id = $2"
		args = append(args, tid)
	}

	var raw []byte
	err = s.db.QueryRowContext(ctx, q, args...).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) || len(raw) == 0 {
		return make(map[string]any), nil
	}
	if err != nil {
		return nil, err
	}
	if s.encKey != "" {
		if dec, err := crypto.Decrypt(string(raw), s.encKey); err == nil {
			raw = []byte(dec)
		}
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return make(map[string]any), nil
	}
	return m, nil
}

func (s *PGChannelInstanceStore) Delete(ctx context.Context, id uuid.UUID) error {
	tid, err := requireTenantID(ctx)
	if err != nil {
		return err
	}
	q := "DELETE FROM channel_instances WHERE id = $1"
	args := []any{id}

	if tid != uuid.Nil {
		q += " AND tenant_id = $2"
		args = append(args, tid)
	}

	_, err = s.db.ExecContext(ctx, q, args...)
	return err
}

func (s *PGChannelInstanceStore) ListEnabled(ctx context.Context) ([]store.ChannelInstanceData, error) {
	tid, err := requireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	q := `SELECT ` + channelInstanceSelectCols + ` FROM channel_instances WHERE enabled = true`
	var args []any

	if tid != uuid.Nil {
		q += ` AND tenant_id = $1`
		args = append(args, tid)
	}
	q += ` ORDER BY name`

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	return s.scanInstances(rows)
}

func (s *PGChannelInstanceStore) ListAll(ctx context.Context) ([]store.ChannelInstanceData, error) {
	tid, err := requireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	q := `SELECT ` + channelInstanceSelectCols + ` FROM channel_instances`
	var args []any

	if tid != uuid.Nil {
		q += ` WHERE tenant_id = $1`
		args = append(args, tid)
	}
	q += ` ORDER BY name`

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	return s.scanInstances(rows)
}

func buildChannelInstanceWhere(ctx context.Context, opts store.ChannelInstanceListOpts) (string, []any, error) {
	var conditions []string
	var args []any
	argIdx := 1

	tid, err := requireTenantID(ctx)
	if err != nil {
		return "", nil, err
	}
	if tid != uuid.Nil {
		conditions = append(conditions, fmt.Sprintf("tenant_id = $%d", argIdx))
		args = append(args, tid)
		argIdx++
	}

	if opts.Search != "" {
		conditions = append(conditions, fmt.Sprintf("(name ILIKE $%d ESCAPE '\\' OR display_name ILIKE $%d ESCAPE '\\' OR channel_type ILIKE $%d ESCAPE '\\')", argIdx, argIdx, argIdx))
		escaped := strings.NewReplacer("%", "\\%", "_", "\\_").Replace(opts.Search)
		args = append(args, "%"+escaped+"%")
	}

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}
	return where, args, nil
}

func (s *PGChannelInstanceStore) ListPaged(ctx context.Context, opts store.ChannelInstanceListOpts) ([]store.ChannelInstanceData, error) {
	where, args, err := buildChannelInstanceWhere(ctx, opts)
	if err != nil {
		return nil, err
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	q := `SELECT ` + channelInstanceSelectCols + ` FROM channel_instances` + where +
		fmt.Sprintf(" ORDER BY name OFFSET %d LIMIT %d", opts.Offset, limit)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	return s.scanInstances(rows)
}

func (s *PGChannelInstanceStore) CountInstances(ctx context.Context, opts store.ChannelInstanceListOpts) (int, error) {
	where, args, err := buildChannelInstanceWhere(ctx, opts)
	if err != nil {
		return 0, err
	}
	var count int
	err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM channel_instances"+where, args...).Scan(&count)
	return count, err
}
