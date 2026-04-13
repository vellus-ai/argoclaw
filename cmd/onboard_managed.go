package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/vellus-ai/argoclaw/internal/store"
	"github.com/vellus-ai/argoclaw/internal/store/pg"
)

// testPostgresConnection verifies connectivity to Postgres with a 5s timeout.
func testPostgresConnection(dsn string) error {
	db, err := pg.OpenDB(dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return db.PingContext(ctx)
}

// defaultPlaceholderProviders defines disabled placeholder providers seeded for
// UI discoverability. Users can later enable and configure them via the dashboard.
var defaultPlaceholderProviders = []store.LLMProviderData{
	{Name: "anthropic-oauth", DisplayName: "Anthropic (OAuth Token)", ProviderType: store.ProviderAnthropicOAuth, Enabled: false},
	{Name: "openrouter", DisplayName: "OpenRouter", ProviderType: store.ProviderOpenRouter, APIBase: "https://openrouter.ai/api/v1", Enabled: false},
	{Name: "synthetic", DisplayName: "Synthetic", ProviderType: store.ProviderOpenAICompat, APIBase: "https://api.synthetic.new/openai/v1", Enabled: false},
	{Name: "alicloud-api", DisplayName: "AliCloud API", ProviderType: store.ProviderDashScope, APIBase: "https://dashscope-intl.aliyuncs.com/compatible-mode/v1", Enabled: false},
	{Name: "alicloud-sub", DisplayName: "AliCloud Sub", ProviderType: store.ProviderBailian, APIBase: "https://coding-intl.dashscope.aliyuncs.com/v1", Enabled: false},
	{Name: "zai", DisplayName: "Z.ai API", ProviderType: store.ProviderZai, APIBase: "https://api.z.ai/api/paas/v4", Enabled: false},
	{Name: "zai-coding", DisplayName: "Z.ai Coding Plan", ProviderType: store.ProviderZaiCoding, APIBase: "https://api.z.ai/api/coding/paas/v4", Enabled: false},
	{Name: "gemini", DisplayName: "Google Gemini", ProviderType: "gemini_native", APIBase: "https://generativelanguage.googleapis.com", Enabled: false},
}

// seedOnboardPlaceholders opens a PG store and seeds disabled placeholder providers
// so they appear in the UI for easy configuration.
func seedOnboardPlaceholders(dsn string) error {
	storeCfg := store.StoreConfig{
		PostgresDSN:   dsn,
		EncryptionKey: os.Getenv("ARGOCLAW_ENCRYPTION_KEY"),
	}
	stores, err := pg.NewPGStores(storeCfg)
	if err != nil {
		return fmt.Errorf("open PG stores: %w", err)
	}

	if stores.Providers == nil {
		return nil
	}

	// appsec:cross-tenant-bypass — system bootstrapping during init container, no user context
	return seedPlaceholdersWithStore(store.WithCrossTenant(context.Background()), stores.Providers)
}

// seedPlaceholdersWithStore seeds placeholder providers using the given ProviderStore.
// Extracted for unit testing without a real database connection.
//
// Uses SeedOnboardProvider (ON CONFLICT DO NOTHING) so that parallel initContainers
// (replicas >= 2) racing to insert the same row are both safe: the second INSERT is a
// silent no-op and the final DB state is correct. User-configured values are never
// overwritten because SeedOnboardProvider has no DO UPDATE clause.
func seedPlaceholdersWithStore(ctx context.Context, providers store.ProviderStore) error {
	// Build a set of existing api_base values to avoid overwriting user-configured entries.
	existing, err := providers.ListProviders(ctx)
	if err != nil {
		return fmt.Errorf("list providers: %w", err)
	}
	existingBases := make(map[string]bool, len(existing))
	for _, p := range existing {
		if p.APIBase != "" {
			existingBases[p.APIBase] = true
		}
	}

	seeded := 0
	for _, ph := range defaultPlaceholderProviders {
		if ph.APIBase != "" && existingBases[ph.APIBase] {
			continue
		}
		p := ph // copy
		// SeedOnboardProvider uses ON CONFLICT DO NOTHING, so parallel initContainers
		// racing to insert the same row are both safe — no duplicate-key errors.
		if err := providers.SeedOnboardProvider(ctx, &p); err != nil {
			slog.Warn("onboard: failed to seed placeholder provider", "name", ph.Name, "error", err)
			continue
		}
		seeded++
	}

	if seeded > 0 {
		slog.Info("seeded placeholder providers", "count", seeded)
	}
	return nil
}
