package plugins_test

import (
	"testing"
	"testing/quick"

	"github.com/vellus-ai/argoclaw/internal/plugins"
)

// TestValidatePermissions_RejectWriteCoreAgents — Blocker G4
func TestValidatePermissions_RejectWriteCoreAgents(t *testing.T) {
	perms := plugins.Permissions{
		Data: plugins.DataPermissions{
			Write: []string{"core:agents"},
		},
	}
	err := plugins.ValidatePermissions(perms)
	if err == nil {
		t.Fatal("expected error: write to core:agents must be rejected (G4)")
	}
}

func TestValidatePermissions_RejectWriteCoreSessions(t *testing.T) {
	perms := plugins.Permissions{
		Data: plugins.DataPermissions{
			Write: []string{"core:sessions"},
		},
	}
	err := plugins.ValidatePermissions(perms)
	if err == nil {
		t.Fatal("expected error: write to core:sessions must be rejected")
	}
}

func TestValidatePermissions_RejectAdminCoreProviders(t *testing.T) {
	perms := plugins.Permissions{
		Data: plugins.DataPermissions{
			Write: []string{"core:providers"},
		},
	}
	err := plugins.ValidatePermissions(perms)
	if err == nil {
		t.Fatal("expected error: write to core:providers must be rejected")
	}
}

func TestValidatePermissions_AllowReadCoreAgents(t *testing.T) {
	perms := plugins.Permissions{
		Data: plugins.DataPermissions{
			Read: []string{"core:agents"},
		},
	}
	err := plugins.ValidatePermissions(perms)
	if err != nil {
		t.Fatalf("expected read to core:agents to be allowed, got: %v", err)
	}
}

func TestValidatePermissions_AllowWritePluginData(t *testing.T) {
	perms := plugins.Permissions{
		Data: plugins.DataPermissions{
			Write: []string{"plugin:prompt-vault-*"},
			Read:  []string{"plugin:prompt-vault-*"},
		},
	}
	err := plugins.ValidatePermissions(perms)
	if err != nil {
		t.Fatalf("expected plugin-scoped write to be allowed, got: %v", err)
	}
}

func TestValidatePermissions_AllowCustomResource(t *testing.T) {
	perms := plugins.Permissions{
		Data: plugins.DataPermissions{
			Write: []string{"custom:my-resource"},
		},
	}
	err := plugins.ValidatePermissions(perms)
	if err != nil {
		t.Fatalf("expected non-core write to be allowed, got: %v", err)
	}
}

func TestValidatePermissions_EmptyPermissionsOK(t *testing.T) {
	perms := plugins.Permissions{}
	err := plugins.ValidatePermissions(perms)
	if err != nil {
		t.Fatalf("expected empty permissions to be valid, got: %v", err)
	}
}

func TestValidatePermissions_MultipleWriteOneBlocked(t *testing.T) {
	perms := plugins.Permissions{
		Data: plugins.DataPermissions{
			Write: []string{"plugin:my-plugin-*", "core:agents", "custom:other"},
		},
	}
	err := plugins.ValidatePermissions(perms)
	if err == nil {
		t.Fatal("expected error: one of the writes is to core:agents")
	}
}

// TestValidatePermissions_PBT_NeverAllowWriteToCore — Property-based test (G4)
// Any scope starting with "core:" combined with write must always fail validation.
func TestValidatePermissions_PBT_NeverAllowWriteToCore(t *testing.T) {
	// Exhaustive check on known core resources
	coreResources := []string{
		"core:agents", "core:sessions", "core:providers",
		"core:tenants", "core:users", "core:memory", "core:tools",
	}
	for _, res := range coreResources {
		perms := plugins.Permissions{
			Data: plugins.DataPermissions{
				Write: []string{res},
			},
		}
		if err := plugins.ValidatePermissions(perms); err == nil {
			t.Errorf("PBT: write to %q must always be rejected", res)
		}
	}

	// quick.Check: any string starting with "core:" in Write must fail
	f := func(suffix [8]byte) bool {
		// Build a pseudo-random core:* resource name
		res := "core:" + string(suffix[:])
		perms := plugins.Permissions{
			Data: plugins.DataPermissions{
				Write: []string{res},
			},
		}
		return plugins.ValidatePermissions(perms) != nil
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 200}); err != nil {
		t.Errorf("PBT failed: %v", err)
	}
}

// TestValidatePermissions_ParseManifestIntegration verifies that ParseManifest
// rejects manifests with forbidden permissions (G4 at parse time).
func TestValidatePermissions_ParseManifestIntegration(t *testing.T) {
	yaml := `
name: bad-plugin
display_name: "Bad Plugin"
version: "1.0.0"
runtime:
  transport: stdio
permissions:
  data:
    write:
      - "core:agents"
`
	_, err := plugins.ParseManifest([]byte(yaml))
	if err == nil {
		t.Fatal("expected ParseManifest to reject manifest with core:agents write permission (G4)")
	}
}
