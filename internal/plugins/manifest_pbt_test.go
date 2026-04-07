package plugins_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/vellus-ai/argoclaw/internal/plugins"
	"gopkg.in/yaml.v3"
	"pgregory.net/rapid"
)

// ─────────────────────────────────────────────────────────────────────────────
// Property P1: parse(serialize(manifest)) == manifest for any valid manifest.
// Validates: Requirement 2.13
// ─────────────────────────────────────────────────────────────────────────────

// genKebabCase generates a random valid kebab-case name (3–100 chars).
// Format: lowercase alphanumeric segments separated by hyphens, starting and
// ending with an alphanumeric character.
func genKebabCase(t *rapid.T) string {
	// Generate 1–5 segments of 2–20 lowercase alphanumeric chars each.
	numSegments := rapid.IntRange(1, 5).Draw(t, "numSegments")
	segments := make([]string, numSegments)
	for i := range segments {
		segLen := rapid.IntRange(2, 20).Draw(t, fmt.Sprintf("segLen[%d]", i))
		seg := make([]byte, segLen)
		for j := range seg {
			seg[j] = "abcdefghijklmnopqrstuvwxyz0123456789"[rapid.IntRange(0, 35).Draw(t, fmt.Sprintf("char[%d][%d]", i, j))]
		}
		// Ensure first char is a letter (kebab-case must start with [a-z]).
		if i == 0 {
			seg[0] = "abcdefghijklmnopqrstuvwxyz"[rapid.IntRange(0, 25).Draw(t, "firstChar")]
		}
		segments[i] = string(seg)
	}
	name := strings.Join(segments, "-")
	// Clamp to 100 chars max.
	if len(name) > 100 {
		name = name[:100]
		// Trim trailing hyphen if present.
		name = strings.TrimRight(name, "-")
	}
	// Ensure minimum 3 chars.
	if len(name) < 3 {
		name = name + "aa"
	}
	return name
}

// genSemver generates a random valid semver string (MAJOR.MINOR.PATCH).
func genSemver(t *rapid.T) string {
	major := rapid.IntRange(0, 99).Draw(t, "major")
	minor := rapid.IntRange(0, 99).Draw(t, "minor")
	patch := rapid.IntRange(0, 99).Draw(t, "patch")
	return fmt.Sprintf("%d.%d.%d", major, minor, patch)
}

// genPluginType draws a random valid plugin type.
func genPluginType(t *rapid.T) string {
	types := []string{"tool", "agent", "ui", "workflow", "full"}
	return types[rapid.IntRange(0, len(types)-1).Draw(t, "type")]
}

// genTransport draws a random valid MCP transport.
func genTransport(t *rapid.T) string {
	transports := []string{"stdio", "sse", "streamable-http", "sidecar"}
	return transports[rapid.IntRange(0, len(transports)-1).Draw(t, "transport")]
}

// genToolNames generates 1–10 random kebab-case tool names.
func genToolNames(t *rapid.T) []string {
	count := rapid.IntRange(1, 10).Draw(t, "toolCount")
	names := make([]string, count)
	seen := make(map[string]bool)
	for i := range names {
		for {
			name := genKebabCase(t)
			if !seen[name] {
				seen[name] = true
				names[i] = name
				break
			}
		}
	}
	return names
}

// genPermissions generates random valid ManifestPermissions for a given plugin name.
func genPermissions(t *rapid.T, pluginName string) plugins.ManifestPermissions {
	toolNames := genToolNames(t)

	perms := plugins.ManifestPermissions{
		Tools: plugins.ToolPermissions{
			Provide: toolNames,
		},
	}

	// Optionally add data permissions (scoped to own plugin namespace).
	if rapid.Bool().Draw(t, "hasDataPerms") {
		perms.Data = plugins.DataPermissions{
			Write: []string{fmt.Sprintf("plugin:%s-*", pluginName)},
			Read:  []string{fmt.Sprintf("plugin:%s-*", pluginName)},
		}
	}

	// Optionally add event permissions (scoped to own plugin namespace).
	if rapid.Bool().Draw(t, "hasEventPerms") {
		perms.Events = plugins.EventPermissions{
			Publish:   []string{fmt.Sprintf("plugin.%s.created", pluginName)},
			Subscribe: []string{"core.agent.started"},
		}
	}

	return perms
}

// genManifest generates a random valid PluginManifest using rapid.Custom.
var genManifest = rapid.Custom(func(t *rapid.T) plugins.PluginManifest {
	name := genKebabCase(t)
	return plugins.PluginManifest{
		Metadata: plugins.ManifestMetadata{
			Name:            name,
			DisplayName:     rapid.StringMatching(`[A-Za-z ]{1,50}`).Draw(t, "displayName"),
			Version:         genSemver(t),
			Description:     rapid.StringMatching(`[A-Za-z0-9 .,!?]{0,200}`).Draw(t, "description"),
			Author:          rapid.StringMatching(`[A-Za-z ]{1,50}`).Draw(t, "author"),
			ManifestVersion: "1.0",
		},
		Spec: plugins.ManifestSpec{
			Type: genPluginType(t),
			Runtime: plugins.ManifestRuntime{
				Transport: genTransport(t),
				Command:   "./server",
				Args:      []string{"--port", "8080"},
				Env:       map[string]string{"PLUGIN_MODE": "production"},
			},
			Permissions: genPermissions(t, name),
		},
	}
})

// TestManifestRoundTrip_P1 verifies the round-trip property:
//
//	parse(serialize(manifest)) == manifest
//
// For any valid generated manifest, serializing to YAML and parsing back
// must produce an equivalent PluginManifest struct.
//
// **Validates: Requirement 2.13**
func TestManifestRoundTrip_P1(t *testing.T) {
	cfg := plugins.DefaultConfig()
	parser := plugins.NewManifestParser(cfg)

	rapid.Check(t, func(t *rapid.T) {
		original := genManifest.Draw(t, "manifest")

		// 1. Serialize the manifest to YAML.
		yamlBytes, err := yaml.Marshal(original)
		if err != nil {
			t.Fatalf("failed to serialize manifest to YAML: %v", err)
		}

		// 2. Parse it back using the ManifestParser.
		parsed, err := parser.Parse(yamlBytes)
		if err != nil {
			t.Fatalf("failed to parse serialized manifest: %v\nYAML:\n%s", err, string(yamlBytes))
		}

		// 3. Assert the parsed manifest equals the original.
		assertManifestEqual(t, original, *parsed)
	})
}

// assertManifestEqual compares two PluginManifest structs field by field,
// reporting detailed diffs on mismatch.
func assertManifestEqual(t *rapid.T, want, got plugins.PluginManifest) {
	// Metadata
	if want.Metadata.Name != got.Metadata.Name {
		t.Fatalf("Metadata.Name: want %q, got %q", want.Metadata.Name, got.Metadata.Name)
	}
	if want.Metadata.DisplayName != got.Metadata.DisplayName {
		t.Fatalf("Metadata.DisplayName: want %q, got %q", want.Metadata.DisplayName, got.Metadata.DisplayName)
	}
	if want.Metadata.Version != got.Metadata.Version {
		t.Fatalf("Metadata.Version: want %q, got %q", want.Metadata.Version, got.Metadata.Version)
	}
	if want.Metadata.Description != got.Metadata.Description {
		t.Fatalf("Metadata.Description: want %q, got %q", want.Metadata.Description, got.Metadata.Description)
	}
	if want.Metadata.Author != got.Metadata.Author {
		t.Fatalf("Metadata.Author: want %q, got %q", want.Metadata.Author, got.Metadata.Author)
	}
	if want.Metadata.ManifestVersion != got.Metadata.ManifestVersion {
		t.Fatalf("Metadata.ManifestVersion: want %q, got %q", want.Metadata.ManifestVersion, got.Metadata.ManifestVersion)
	}

	// Spec
	if want.Spec.Type != got.Spec.Type {
		t.Fatalf("Spec.Type: want %q, got %q", want.Spec.Type, got.Spec.Type)
	}

	// Runtime
	if want.Spec.Runtime.Transport != got.Spec.Runtime.Transport {
		t.Fatalf("Spec.Runtime.Transport: want %q, got %q", want.Spec.Runtime.Transport, got.Spec.Runtime.Transport)
	}
	if want.Spec.Runtime.Command != got.Spec.Runtime.Command {
		t.Fatalf("Spec.Runtime.Command: want %q, got %q", want.Spec.Runtime.Command, got.Spec.Runtime.Command)
	}

	// Permissions — tools.provide
	if len(want.Spec.Permissions.Tools.Provide) != len(got.Spec.Permissions.Tools.Provide) {
		t.Fatalf("Spec.Permissions.Tools.Provide length: want %d, got %d",
			len(want.Spec.Permissions.Tools.Provide), len(got.Spec.Permissions.Tools.Provide))
	}
	for i, w := range want.Spec.Permissions.Tools.Provide {
		if w != got.Spec.Permissions.Tools.Provide[i] {
			t.Fatalf("Spec.Permissions.Tools.Provide[%d]: want %q, got %q", i, w, got.Spec.Permissions.Tools.Provide[i])
		}
	}

	// Permissions — data
	assertStringSliceEqual(t, "Spec.Permissions.Data.Write", want.Spec.Permissions.Data.Write, got.Spec.Permissions.Data.Write)
	assertStringSliceEqual(t, "Spec.Permissions.Data.Read", want.Spec.Permissions.Data.Read, got.Spec.Permissions.Data.Read)

	// Permissions — events
	assertStringSliceEqual(t, "Spec.Permissions.Events.Publish", want.Spec.Permissions.Events.Publish, got.Spec.Permissions.Events.Publish)
	assertStringSliceEqual(t, "Spec.Permissions.Events.Subscribe", want.Spec.Permissions.Events.Subscribe, got.Spec.Permissions.Events.Subscribe)
}

// assertStringSliceEqual compares two string slices, treating nil and empty as equal.
func assertStringSliceEqual(t *rapid.T, field string, want, got []string) {
	wantLen := len(want)
	gotLen := len(got)
	if wantLen != gotLen {
		t.Fatalf("%s length: want %d, got %d (want=%v, got=%v)", field, wantLen, gotLen, want, got)
	}
	for i := range want {
		if want[i] != got[i] {
			t.Fatalf("%s[%d]: want %q, got %q", field, i, want[i], got[i])
		}
	}
}
