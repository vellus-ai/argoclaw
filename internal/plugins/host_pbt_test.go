package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"pgregory.net/rapid"

	"github.com/vellus-ai/argoclaw/internal/store"
)

// ─────────────────────────────────────────────────────────────────────────────
// P2: State transitions — random operation sequences, only valid transitions accepted
// ─────────────────────────────────────────────────────────────────────────────

// TestPBT_StateTransitions_OnlyValidTransitionsAccepted exercises the PluginHost
// state machine with random sequences of enable/disable operations.
// Property: a plugin always reaches a valid state, and invalid transitions are rejected.
func TestPBT_StateTransitions_OnlyValidTransitionsAccepted(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := newMockPluginStoreHost()
		tenantID := uuid.New()
		pluginName := "pbt-plugin"

		s.addCatalogEntry(newCatalogEntry(pluginName, "1.0.0", validManifestYAML(pluginName, "1.0.0")))

		host, _, _, _ := newTestPluginHost(s)
		ctx := tenantCtx(tenantID)

		// Install the plugin first (always valid as starting point).
		_, err := host.Install(ctx, pluginName)
		if err != nil {
			t.Fatalf("install failed: %v", err)
		}

		currentState := StateInstalled

		// Generate a random sequence of operations (enable=0, disable=1).
		numOps := rapid.IntRange(1, 20).Draw(t, "numOps")

		for i := 0; i < numOps; i++ {
			op := rapid.IntRange(0, 1).Draw(t, fmt.Sprintf("op_%d", i))

			switch op {
			case 0: // Enable
				_, err := host.Enable(ctx, pluginName)
				if isValidTransition(currentState, StateEnabled) {
					if err != nil {
						t.Fatalf("enable should succeed from %s, got: %v", currentState, err)
					}
					currentState = StateEnabled
				} else {
					if err == nil {
						t.Fatalf("enable should fail from %s, got nil error", currentState)
					}
				}

			case 1: // Disable
				_, err := host.Disable(ctx, pluginName)
				if isValidTransition(currentState, StateDisabled) {
					if err != nil {
						t.Fatalf("disable should succeed from %s, got: %v", currentState, err)
					}
					currentState = StateDisabled
				} else {
					if err == nil {
						t.Fatalf("disable should fail from %s, got nil error", currentState)
					}
				}
			}

			// Invariant: current state must be one of the known states.
			switch currentState {
			case StateInstalled, StateEnabled, StateDisabled, StateError:
				// OK
			default:
				t.Fatalf("invalid state reached: %s", currentState)
			}
		}
	})
}

// TestPBT_StateTransitions_Reflexive verifies no state can transition to itself.
func TestPBT_StateTransitions_Reflexive(t *testing.T) {
	allStates := []PluginState{StateInstalled, StateEnabled, StateDisabled, StateError}
	for _, state := range allStates {
		if isValidTransition(state, state) {
			t.Errorf("self-transition should not be valid for state %s", state)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// P4: Tool prefix — all registered tools have format plugin_{name}__{tool}
// ─────────────────────────────────────────────────────────────────────────────

// pluginNameGen generates valid kebab-case plugin names.
func pluginNameGen() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		letters := "abcdefghijklmnopqrstuvwxyz"
		// Start with a letter.
		firstIdx := rapid.IntRange(0, len(letters)-1).Draw(t, "first")
		first := letters[firstIdx]

		// Generate 0-3 additional segments.
		numSegments := rapid.IntRange(0, 3).Draw(t, "numSegments")
		var segments []string
		segments = append(segments, string(first))

		chars := "abcdefghijklmnopqrstuvwxyz0123456789"
		for i := 0; i < numSegments; i++ {
			segLen := rapid.IntRange(1, 6).Draw(t, fmt.Sprintf("segLen_%d", i))
			seg := make([]byte, segLen)
			for j := 0; j < segLen; j++ {
				idx := rapid.IntRange(0, len(chars)-1).Draw(t, fmt.Sprintf("char_%d_%d", i, j))
				seg[j] = chars[idx]
			}
			segments = append(segments, string(seg))
		}

		return strings.Join(segments, "-")
	})
}

// toolNameGen generates simple tool names.
func toolNameGen() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		chars := "abcdefghijklmnopqrstuvwxyz_"
		length := rapid.IntRange(2, 20).Draw(t, "toolLen")
		name := make([]byte, length)
		for i := 0; i < length; i++ {
			idx := rapid.IntRange(0, len(chars)-1).Draw(t, fmt.Sprintf("tc_%d", i))
			name[i] = chars[idx]
		}
		return string(name)
	})
}

// TestPBT_ToolPrefix_FormatCorrect verifies that toolPrefix always produces
// the correct format: plugin_{underscored_name}__{tool_name}.
func TestPBT_ToolPrefix_FormatCorrect(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		pluginName := pluginNameGen().Draw(t, "pluginName")
		toolName := toolNameGen().Draw(t, "toolName")

		prefixed := toolPrefix(pluginName, toolName)

		// Must start with "plugin_".
		if !strings.HasPrefix(prefixed, "plugin_") {
			t.Fatalf("prefixed tool %q does not start with 'plugin_'", prefixed)
		}

		// Must contain "__" separator.
		if !strings.Contains(prefixed, "__") {
			t.Fatalf("prefixed tool %q does not contain '__' separator", prefixed)
		}

		// Must end with the tool name.
		if !strings.HasSuffix(prefixed, "__"+toolName) {
			t.Fatalf("prefixed tool %q does not end with '__%s'", prefixed, toolName)
		}

		// No hyphens in the plugin part (they should be replaced with underscores).
		parts := strings.SplitN(prefixed, "__", 2)
		pluginPart := strings.TrimPrefix(parts[0], "plugin_")
		if strings.Contains(pluginPart, "-") {
			t.Fatalf("plugin part %q should not contain hyphens", pluginPart)
		}

		// Reconstruct: replacing underscores back to hyphens in plugin part
		// should match original name (only for single-underscore, not double).
		underscored := strings.ReplaceAll(pluginName, "-", "_")
		expected := fmt.Sprintf("plugin_%s__%s", underscored, toolName)
		if prefixed != expected {
			t.Fatalf("expected %q, got %q", expected, prefixed)
		}
	})
}

// TestPBT_ToolPrefix_EnableRegistersWithCorrectPrefix verifies that when a
// plugin is enabled, all tools registered in the ToolRegistry have the correct prefix.
func TestPBT_ToolPrefix_EnableRegistersWithCorrectPrefix(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := newMockPluginStoreHost()
		tenantID := uuid.New()
		pluginName := "prefix-test"

		s.addCatalogEntry(newCatalogEntry(pluginName, "1.0.0", validManifestYAML(pluginName, "1.0.0")))

		host, toolReg, _, _ := newTestPluginHost(s)
		ctx := tenantCtx(tenantID)

		_, _ = host.Install(ctx, pluginName)
		_, err := host.Enable(ctx, pluginName)
		if err != nil {
			t.Fatalf("enable failed: %v", err)
		}

		// Verify all registered tools have the correct prefix.
		expectedPrefix := "plugin_" + strings.ReplaceAll(pluginName, "-", "_") + "__"
		for _, name := range toolReg.registered() {
			if !strings.HasPrefix(name, expectedPrefix) {
				t.Fatalf("tool %q does not have expected prefix %q", name, expectedPrefix)
			}
		}

		// Verify at least one tool was registered.
		if len(toolReg.registered()) == 0 {
			t.Fatal("expected at least one tool to be registered")
		}
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// P9: Config validation — valid config passes, invalid rejected
// ─────────────────────────────────────────────────────────────────────────────

// TestPBT_ConfigValidation_RequiredFieldPresent verifies that a config with
// all required fields passes validation.
func TestPBT_ConfigValidation_RequiredFieldPresent(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate 1-5 required field names.
		numFields := rapid.IntRange(1, 5).Draw(t, "numFields")
		required := make([]string, numFields)
		config := make(map[string]interface{})

		for i := 0; i < numFields; i++ {
			field := fmt.Sprintf("field%d", i)
			required[i] = field
			// Generate a random string value for each required field.
			valLen := rapid.IntRange(1, 50).Draw(t, fmt.Sprintf("valLen_%d", i))
			config[field] = strings.Repeat("x", valLen)
		}

		schema := map[string]interface{}{
			"type":     "object",
			"required": required,
		}

		schemaJSON, _ := json.Marshal(schema)
		configJSON, _ := json.Marshal(config)

		err := validateJSONSchema(schemaJSON, configJSON)
		if err != nil {
			t.Fatalf("valid config should pass validation, got: %v", err)
		}
	})
}

// TestPBT_ConfigValidation_MissingRequiredFieldRejected verifies that a config
// missing any required field is rejected.
func TestPBT_ConfigValidation_MissingRequiredFieldRejected(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate 2-5 required fields.
		numFields := rapid.IntRange(2, 5).Draw(t, "numFields")
		required := make([]string, numFields)
		config := make(map[string]interface{})

		for i := 0; i < numFields; i++ {
			field := fmt.Sprintf("field%d", i)
			required[i] = field
			config[field] = "value"
		}

		// Remove one random required field from config.
		removeIdx := rapid.IntRange(0, numFields-1).Draw(t, "removeIdx")
		removedField := required[removeIdx]
		delete(config, removedField)

		schema := map[string]interface{}{
			"type":     "object",
			"required": required,
		}

		schemaJSON, _ := json.Marshal(schema)
		configJSON, _ := json.Marshal(config)

		err := validateJSONSchema(schemaJSON, configJSON)
		if err == nil {
			t.Fatalf("config missing required field %q should be rejected", removedField)
		}
		if !strings.Contains(err.Error(), removedField) {
			t.Fatalf("error should mention missing field %q, got: %v", removedField, err)
		}
	})
}

// TestPBT_ConfigValidation_EmptySchemaAcceptsAny verifies that an empty schema
// (no required fields) accepts any valid JSON object.
func TestPBT_ConfigValidation_EmptySchemaAcceptsAny(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random config with 0-5 fields.
		numFields := rapid.IntRange(0, 5).Draw(t, "numFields")
		config := make(map[string]interface{})
		for i := 0; i < numFields; i++ {
			config[fmt.Sprintf("key%d", i)] = "val"
		}

		schema := json.RawMessage(`{"type": "object"}`)
		configJSON, _ := json.Marshal(config)

		err := validateJSONSchema(schema, configJSON)
		if err != nil {
			t.Fatalf("empty schema should accept any config, got: %v", err)
		}
	})
}

// TestPBT_ConfigValidation_NonObjectConfigRejected verifies that non-object
// configs are always rejected.
func TestPBT_ConfigValidation_NonObjectConfigRejected(t *testing.T) {
	nonObjects := []string{
		`"string"`,
		`42`,
		`true`,
		`null`,
		`[1, 2, 3]`,
	}

	schema := json.RawMessage(`{"type": "object", "required": ["key"]}`)

	for _, input := range nonObjects {
		err := validateJSONSchema(schema, json.RawMessage(input))
		if err == nil {
			t.Errorf("non-object config %s should be rejected", input)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// P2 additional: Install + Uninstall idempotency properties
// ─────────────────────────────────────────────────────────────────────────────

// TestPBT_InstallUninstall_TenantIsolation verifies that installing a plugin
// for one tenant does not affect another tenant.
func TestPBT_InstallUninstall_TenantIsolation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := newMockPluginStoreHost()
		pluginName := "iso-pbt"

		s.addCatalogEntry(newCatalogEntry(pluginName, "1.0.0", validManifestYAML(pluginName, "1.0.0")))

		host, _, _, _ := newTestPluginHost(s)

		// Generate 2-5 tenant IDs.
		numTenants := rapid.IntRange(2, 5).Draw(t, "numTenants")
		tenantIDs := make([]uuid.UUID, numTenants)
		for i := range tenantIDs {
			tenantIDs[i] = uuid.New()
		}

		// Install for each tenant.
		for _, tid := range tenantIDs {
			ctx := tenantCtx(tid)
			_, err := host.Install(ctx, pluginName)
			if err != nil {
				t.Fatalf("install for tenant %s failed: %v", tid, err)
			}
		}

		// Uninstall for the first tenant.
		err := host.Uninstall(tenantCtx(tenantIDs[0]), pluginName)
		if err != nil {
			t.Fatalf("uninstall failed: %v", err)
		}

		// Verify other tenants still have the plugin.
		for _, tid := range tenantIDs[1:] {
			ctx := tenantCtx(tid)
			tp, err := s.GetTenantPlugin(ctx, pluginName)
			if err != nil {
				t.Fatalf("tenant %s should still have plugin, got error: %v", tid, err)
			}
			if tp == nil {
				t.Fatalf("tenant %s should still have plugin", tid)
			}
		}
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Semver comparison property
// ─────────────────────────────────────────────────────────────────────────────

// TestPBT_SemverSatisfies_Reflexive verifies that any version satisfies itself.
func TestPBT_SemverSatisfies_Reflexive(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		major := rapid.IntRange(0, 99).Draw(t, "major")
		minor := rapid.IntRange(0, 99).Draw(t, "minor")
		patch := rapid.IntRange(0, 99).Draw(t, "patch")

		v := fmt.Sprintf("%d.%d.%d", major, minor, patch)
		if !semverSatisfies(v, v) {
			t.Fatalf("version %s should satisfy itself", v)
		}
	})
}

// TestPBT_SemverSatisfies_HigherSatisfiesLower verifies that a higher version
// always satisfies a lower version requirement.
func TestPBT_SemverSatisfies_HigherSatisfiesLower(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		major := rapid.IntRange(0, 50).Draw(t, "major")
		minor := rapid.IntRange(0, 50).Draw(t, "minor")
		patch := rapid.IntRange(0, 50).Draw(t, "patch")

		// Increase at least one component.
		component := rapid.IntRange(0, 2).Draw(t, "component")
		bump := rapid.IntRange(1, 10).Draw(t, "bump")

		hMajor, hMinor, hPatch := major, minor, patch
		switch component {
		case 0:
			hMajor += bump
		case 1:
			hMinor += bump
		case 2:
			hPatch += bump
		}

		lower := fmt.Sprintf("%d.%d.%d", major, minor, patch)
		higher := fmt.Sprintf("%d.%d.%d", hMajor, hMinor, hPatch)

		if !semverSatisfies(higher, lower) {
			t.Fatalf("higher version %s should satisfy lower requirement %s", higher, lower)
		}
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Plugin name validation property
// ─────────────────────────────────────────────────────────────────────────────

// TestPBT_PluginName_KebabCaseOnly verifies that IsValidPluginName rejects
// any name with uppercase, underscores, or spaces.
func TestPBT_PluginName_KebabCaseOnly(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a string with at least one invalid character.
		invalidChars := "ABCDEFGHIJKLMNOPQRSTUVWXYZ_ !@#$%^&*()"
		invalidIdx := rapid.IntRange(0, len(invalidChars)-1).Draw(t, "invalidIdx")
		invalidChar := string(invalidChars[invalidIdx])

		// Insert invalid char into an otherwise valid name.
		prefix := "valid"
		name := prefix + invalidChar + "name"

		if IsValidPluginName(name) {
			t.Fatalf("name %q with invalid char %q should be rejected", name, invalidChar)
		}
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Helper: GetTenantPlugin for mock (needed by tenant isolation PBT)
// ─────────────────────────────────────────────────────────────────────────────

func init() {
	// Ensure the mock store method GetTenantPlugin uses context for tenant scoping.
	_ = (*mockPluginStoreHost)(nil)
}

// tenantCtxForPBT creates a context with both tenant ID and plan.
func tenantCtxForPBT(tenantID uuid.UUID) context.Context {
	return store.WithTenantID(context.Background(), tenantID)
}
