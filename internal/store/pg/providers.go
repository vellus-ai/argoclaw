package pg

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/vellus-ai/argoclaw/internal/crypto"
	"github.com/vellus-ai/argoclaw/internal/store"
)

// PGProviderStore implements store.ProviderStore backed by Postgres.
type PGProviderStore struct {
	db     *sql.DB
	encKey string // AES-256 encryption key for API keys (empty = plain text)
}

func NewPGProviderStore(db *sql.DB, encryptionKey string) *PGProviderStore {
	if encryptionKey != "" {
		slog.Info("provider store: API key encryption enabled")
	} else {
		slog.Warn("provider store: API key encryption disabled (plain text storage)")
	}
	return &PGProviderStore{db: db, encKey: encryptionKey}
}

func (s *PGProviderStore) CreateProvider(ctx context.Context, p *store.LLMProviderData) error {
	tid, err := requireTenantID(ctx)
	if err != nil {
		return err
	}

	if p.ID == uuid.Nil {
		p.ID = store.GenNewID()
	}

	apiKey := p.APIKey
	if s.encKey != "" && apiKey != "" {
		encrypted, err := crypto.Encrypt(apiKey, s.encKey)
		if err != nil {
			return fmt.Errorf("encrypt api key: %w", err)
		}
		apiKey = encrypted
	}

	settings := p.Settings
	if len(settings) == 0 {
		settings = []byte("{}")
	}

	now := time.Now()
	p.CreatedAt = now
	p.UpdatedAt = now
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO llm_providers (id, tenant_id, name, display_name, provider_type, api_base, api_key, enabled, settings, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		p.ID, nilUUID(&tid), p.Name, p.DisplayName, p.ProviderType, p.APIBase, apiKey, p.Enabled, settings, now, now,
	)
	return err
}

// SeedOnboardProvider implements store.ProviderStore.SeedOnboardProvider.
// Uses ON CONFLICT (name, tenant_id) DO NOTHING so that parallel initContainers
// (replicas >= 2) racing to seed the same placeholder row are both safe: the second
// INSERT is silently discarded and the final DB state is correct. User-modified
// configuration is never overwritten because there is no DO UPDATE clause.
func (s *PGProviderStore) SeedOnboardProvider(ctx context.Context, p *store.LLMProviderData) error {
	tid, err := requireTenantID(ctx)
	if err != nil {
		return err
	}

	if p.ID == uuid.Nil {
		p.ID = store.GenNewID()
	}

	settings := p.Settings
	if len(settings) == 0 {
		settings = []byte("{}")
	}

	now := time.Now()
	_, err = s.db.ExecContext(ctx,
		// ON CONFLICT DO NOTHING: onboarding seed — reference data, immutable after first boot.
		// Race condition tolerated: parallel initContainers may attempt the same INSERT;
		// the second one is a silent no-op. No user data is overwritten.
		`INSERT INTO llm_providers (id, tenant_id, name, display_name, provider_type, api_base, api_key, enabled, settings, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 ON CONFLICT (name, tenant_id) DO NOTHING`,
		p.ID, nilUUID(&tid), p.Name, p.DisplayName, p.ProviderType, p.APIBase, "", p.Enabled, settings, now, now,
	)
	return err
}

func (s *PGProviderStore) GetProvider(ctx context.Context, id uuid.UUID) (*store.LLMProviderData, error) {
	tid, err := requireTenantID(ctx)
	if err != nil {
		return nil, err
	}

	var p store.LLMProviderData
	var apiKey string

	q := `SELECT id, name, display_name, provider_type, api_base, api_key, enabled, settings, created_at, updated_at
		 FROM llm_providers WHERE id = $1`
	args := []any{id}

	if tid != uuid.Nil {
		q += ` AND tenant_id = $2`
		args = append(args, tid)
	}

	err = s.db.QueryRowContext(ctx, q, args...).Scan(
		&p.ID, &p.Name, &p.DisplayName, &p.ProviderType, &p.APIBase, &apiKey, &p.Enabled, &p.Settings, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("provider not found: %s", id)
	}
	p.APIKey = s.decryptKey(apiKey, p.Name)
	return &p, nil
}

func (s *PGProviderStore) GetProviderByName(ctx context.Context, name string) (*store.LLMProviderData, error) {
	tid, err := requireTenantID(ctx)
	if err != nil {
		return nil, err
	}

	var p store.LLMProviderData
	var apiKey string

	q := `SELECT id, name, display_name, provider_type, api_base, api_key, enabled, settings, created_at, updated_at
		 FROM llm_providers WHERE name = $1`
	args := []any{name}

	if tid != uuid.Nil {
		q += ` AND tenant_id = $2`
		args = append(args, tid)
	}

	err = s.db.QueryRowContext(ctx, q, args...).Scan(
		&p.ID, &p.Name, &p.DisplayName, &p.ProviderType, &p.APIBase, &apiKey, &p.Enabled, &p.Settings, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("provider not found: %s", name)
	}
	p.APIKey = s.decryptKey(apiKey, p.Name)
	return &p, nil
}

func (s *PGProviderStore) ListProviders(ctx context.Context) ([]store.LLMProviderData, error) {
	tid, err := requireTenantID(ctx)
	if err != nil {
		return nil, err
	}

	q := `SELECT id, name, display_name, provider_type, api_base, api_key, enabled, settings, created_at, updated_at
		 FROM llm_providers`
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
	defer rows.Close()

	var result []store.LLMProviderData
	for rows.Next() {
		var p store.LLMProviderData
		var apiKey string
		if err := rows.Scan(&p.ID, &p.Name, &p.DisplayName, &p.ProviderType, &p.APIBase, &apiKey, &p.Enabled, &p.Settings, &p.CreatedAt, &p.UpdatedAt); err != nil {
			continue
		}
		p.APIKey = s.decryptKey(apiKey, p.Name)
		result = append(result, p)
	}
	return result, nil
}

func (s *PGProviderStore) UpdateProvider(ctx context.Context, id uuid.UUID, updates map[string]any) error {
	if _, err := requireTenantID(ctx); err != nil {
		return err
	}

	if apiKey, ok := updates["api_key"]; ok && s.encKey != "" {
		if keyStr, ok := apiKey.(string); ok && keyStr != "" {
			encrypted, err := crypto.Encrypt(keyStr, s.encKey)
			if err != nil {
				return fmt.Errorf("encrypt api key: %w", err)
			}
			updates["api_key"] = encrypted
		}
	}
	return execMapUpdateTenant(ctx, s.db, "llm_providers", id, updates)
}

func (s *PGProviderStore) DeleteProvider(ctx context.Context, id uuid.UUID) error {
	tid, err := requireTenantID(ctx)
	if err != nil {
		return err
	}

	q := "DELETE FROM llm_providers WHERE id = $1"
	args := []any{id}

	if tid != uuid.Nil {
		q += " AND tenant_id = $2"
		args = append(args, tid)
	}

	_, err = s.db.ExecContext(ctx, q, args...)
	return err
}

func (s *PGProviderStore) decryptKey(apiKey, providerName string) string {
	if s.encKey != "" && apiKey != "" {
		decrypted, err := crypto.Decrypt(apiKey, s.encKey)
		if err != nil {
			slog.Warn("provider: could not decrypt API key", "provider", providerName, "error", err)
			return apiKey
		}
		return decrypted
	}
	return apiKey
}
