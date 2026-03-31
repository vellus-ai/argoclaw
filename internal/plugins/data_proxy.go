package plugins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/vellus-ai/argoclaw/internal/store"
)

const (
	maxKeyLength        = 500
	maxCollectionLength = 100
)

// ErrMissingTenantContext is returned when the context has no tenant_id.
var ErrMissingTenantContext = errors.New("data proxy: tenant context is required")

// ErrKeyTooLong is returned when a key exceeds the maximum allowed length.
var ErrKeyTooLong = fmt.Errorf("data proxy: key exceeds maximum length of %d characters", maxKeyLength)

// ErrCollectionTooLong is returned when a collection name exceeds the maximum allowed length.
var ErrCollectionTooLong = fmt.Errorf("data proxy: collection exceeds maximum length of %d characters", maxCollectionLength)

// ErrPluginNotInstalled is returned when the plugin is not installed for the tenant.
var ErrPluginNotInstalled = errors.New("data proxy: plugin is not installed for this tenant")

// DataProxy validates and proxies plugin data access to the store.
//
// It enforces:
//   - Tenant context is present (uuid.Nil → ErrMissingTenantContext)
//   - Key length ≤ maxKeyLength
//   - Collection length ≤ maxCollectionLength
//   - Plugin is installed for the tenant (GetTenantPlugin succeeds)
//
// All data operations delegate to the underlying store.PluginStore.
// The tenant_id is always derived from the context — never from caller parameters.
type DataProxy struct {
	store store.PluginStore
}

// NewDataProxy creates a new DataProxy backed by the given store.
func NewDataProxy(s store.PluginStore) *DataProxy {
	return &DataProxy{store: s}
}

func (p *DataProxy) validateContext(ctx context.Context) error {
	if store.TenantIDFromContext(ctx) == uuid.Nil {
		return ErrMissingTenantContext
	}
	return nil
}

func (p *DataProxy) validateKey(key string) error {
	if len(key) > maxKeyLength {
		return ErrKeyTooLong
	}
	return nil
}

func (p *DataProxy) validateCollection(collection string) error {
	if len(collection) > maxCollectionLength {
		return ErrCollectionTooLong
	}
	return nil
}

func (p *DataProxy) checkPluginInstalled(ctx context.Context, pluginName string) error {
	_, err := p.store.GetTenantPlugin(ctx, pluginName)
	if err != nil {
		return ErrPluginNotInstalled
	}
	return nil
}

// Put upserts a value in the plugin data store.
// Validates tenant context, collection length, key length, and plugin installation.
func (p *DataProxy) Put(ctx context.Context, pluginName, collection, key string, value json.RawMessage, expiresAt *time.Time) error {
	if err := p.validateContext(ctx); err != nil {
		return err
	}
	if err := p.validateCollection(collection); err != nil {
		return err
	}
	if err := p.validateKey(key); err != nil {
		return err
	}
	if err := p.checkPluginInstalled(ctx, pluginName); err != nil {
		return err
	}
	return p.store.PutData(ctx, pluginName, collection, key, value, expiresAt)
}

// Get retrieves a value from the plugin data store.
// Validates tenant context, collection length, key length, and plugin installation.
func (p *DataProxy) Get(ctx context.Context, pluginName, collection, key string) (*store.PluginDataEntry, error) {
	if err := p.validateContext(ctx); err != nil {
		return nil, err
	}
	if err := p.validateCollection(collection); err != nil {
		return nil, err
	}
	if err := p.validateKey(key); err != nil {
		return nil, err
	}
	if err := p.checkPluginInstalled(ctx, pluginName); err != nil {
		return nil, err
	}
	return p.store.GetData(ctx, pluginName, collection, key)
}

// ListKeys returns all keys in a collection for the current tenant + plugin.
// Validates tenant context and collection length.
func (p *DataProxy) ListKeys(ctx context.Context, pluginName, collection, prefix string, limit, offset int) ([]string, error) {
	if err := p.validateContext(ctx); err != nil {
		return nil, err
	}
	if err := p.validateCollection(collection); err != nil {
		return nil, err
	}
	if err := p.checkPluginInstalled(ctx, pluginName); err != nil {
		return nil, err
	}
	return p.store.ListDataKeys(ctx, pluginName, collection, prefix, limit, offset)
}

// Delete removes a value from the plugin data store.
// Validates tenant context, collection length, key length, and plugin installation.
func (p *DataProxy) Delete(ctx context.Context, pluginName, collection, key string) error {
	if err := p.validateContext(ctx); err != nil {
		return err
	}
	if err := p.validateCollection(collection); err != nil {
		return err
	}
	if err := p.validateKey(key); err != nil {
		return err
	}
	if err := p.checkPluginInstalled(ctx, pluginName); err != nil {
		return err
	}
	return p.store.DeleteData(ctx, pluginName, collection, key)
}
