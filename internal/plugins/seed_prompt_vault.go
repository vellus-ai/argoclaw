package plugins

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// PromptVaultToolCount is the expected number of MCP tools exposed by the
// Prompt Vault plugin. Used as a baseline for E2E validation.
const PromptVaultToolCount = 24

// promptVaultToolNames lists all 24 MCP tools provided by the Prompt Vault plugin.
// These are the raw tool names before the host applies the plugin_ prefix.
var promptVaultToolNames = []string{
	"vault_prompt_create",
	"vault_prompt_get",
	"vault_prompt_list",
	"vault_prompt_update",
	"vault_prompt_delete",
	"vault_prompt_search",
	"vault_version_create",
	"vault_version_get",
	"vault_version_list",
	"vault_version_diff",
	"vault_version_rollback",
	"vault_version_promote",
	"vault_tag_add",
	"vault_tag_remove",
	"vault_tag_list",
	"vault_folder_create",
	"vault_folder_list",
	"vault_folder_move",
	"vault_folder_delete",
	"vault_export",
	"vault_import",
	"vault_stats",
	"vault_duplicate",
	"vault_archive",
}

// SeedPromptVaultCatalog returns a CatalogEntry for the built-in Prompt Vault
// plugin. This is used to populate the catalog at gateway startup and as the
// reference entry for E2E tests.
func SeedPromptVaultCatalog() CatalogEntry {
	manifest := PluginManifest{
		Metadata: ManifestMetadata{
			Name:            "prompt-vault",
			DisplayName:     "Prompt Vault",
			Version:         "1.0.0",
			Description:     "Version-controlled prompt management with folders, tags, diffs, and rollback. Store, organize, and evolve your prompts with full history.",
			Author:          "Vellus AI",
			Icon:            "vault",
			Tags:            []string{"prompts", "versioning", "management"},
			License:         "proprietary",
			ManifestVersion: CurrentManifestVersion,
		},
		Spec: ManifestSpec{
			Type: "tool",
			Requires: ManifestRequires{
				Platform: ">=1.0.0",
				Plan:     "starter",
			},
			Permissions: ManifestPermissions{
				Tools: ToolPermissions{
					Provide: promptVaultToolNames,
				},
				Data: DataPermissions{
					Read:  []string{"plugin:prompt-vault"},
					Write: []string{"plugin:prompt-vault"},
				},
				Events: EventPermissions{
					Publish:   []string{"plugin.prompt-vault.version_created", "plugin.prompt-vault.prompt_created", "plugin.prompt-vault.prompt_deleted"},
					Subscribe: []string{"agent.activity"},
				},
			},
			Runtime: ManifestRuntime{
				Transport: "stdio",
				Command:   "./server",
				Args:      []string{"--mode", "mcp"},
				Resources: RuntimeResources{
					MemoryMB:   128,
					CPUs:       0.5,
					TimeoutSec: 30,
				},
				HealthCheck: HealthCheckConfig{
					Interval: 60,
					Timeout:  5,
				},
			},
			Migrations: &ManifestMigrations{
				Dir: "migrations",
			},
		},
		Name:    "prompt-vault",
		Version: "1.0.0",
	}

	manifestJSON, _ := json.Marshal(manifest)

	return CatalogEntry{
		ID:          uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		Name:        "prompt-vault",
		Version:     "1.0.0",
		DisplayName: "Prompt Vault",
		Description: manifest.Metadata.Description,
		Author:      "Vellus AI",
		Manifest:    manifestJSON,
		Tags:        []string{"prompts", "versioning", "management"},
		MinPlan:     "starter",
		Source:      "builtin",
		Checksum:    "sha256:placeholder-will-be-replaced-on-release",
		TenantID:    nil, // builtin — visible to all tenants
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// PromptVaultToolNames returns the list of raw tool names for the Prompt Vault
// plugin. Useful for test assertions.
func PromptVaultToolNames() []string {
	out := make([]string, len(promptVaultToolNames))
	copy(out, promptVaultToolNames)
	return out
}
