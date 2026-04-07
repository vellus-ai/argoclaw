package plugins_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/vellus-ai/argoclaw/internal/plugins"
)

// ─────────────────────────────────────────────────────────────────────────────
// Helper: build a valid manifest YAML for the new nested Metadata/Spec structure.
// Tests override specific fields to trigger validation errors.
// ─────────────────────────────────────────────────────────────────────────────

func validManifestYAML() string {
	return `
metadata:
  name: my-plugin
  displayName: "My Plugin"
  version: "1.0.0"
  description: "A test plugin"
  author: "Test Author"
  manifestVersion: "1.0"
spec:
  type: tool
  runtime:
    transport: stdio
    command: "./server"
    args: ["--port", "8080"]
    env:
      PLUGIN_MODE: "production"
  permissions:
    tools:
      provide:
        - vault-prompt-create
        - vault-prompt-list
    data:
      write:
        - "plugin:my-plugin-*"
      read:
        - "plugin:my-plugin-*"
    events:
      publish:
        - "plugin.my-plugin.created"
      subscribe:
        - "core.agent.started"
`
}

// ─────────────────────────────────────────────────────────────────────────────
// TestManifestParser_Parse — table-driven tests for Parse method
// Validates: Requirements 2.1–2.6, 2.9, 2.11
// ─────────────────────────────────────────────────────────────────────────────

func TestManifestParser_Parse(t *testing.T) {
	cfg := plugins.Config{
		AllowedCommands: []string{"./server", "./plugin", "./bin/server"},
		BlockedEnvVars:  map[string]bool{"PATH": true, "LD_PRELOAD": true, "HOME": true},
	}
	parser := plugins.NewManifestParser(cfg)

	tests := []struct {
		name      string
		yaml      string
		wantErr   bool
		errSubstr string // substring expected in error message
	}{
		{
			name:      "name missing",
			yaml:      manifestWithMetadata("", "1.0.0", "1.0", "tool", "stdio"),
			wantErr:   true,
			errSubstr: "name",
		},
		{
			name:      "name not kebab-case (uppercase)",
			yaml:      manifestWithMetadata("MyPlugin", "1.0.0", "1.0", "tool", "stdio"),
			wantErr:   true,
			errSubstr: "kebab",
		},
		{
			name:      "name too short (<3 chars)",
			yaml:      manifestWithMetadata("ab", "1.0.0", "1.0", "tool", "stdio"),
			wantErr:   true,
			errSubstr: "name",
		},
		{
			name:      "name too long (>100 chars)",
			yaml:      manifestWithMetadata(strings.Repeat("a", 101), "1.0.0", "1.0", "tool", "stdio"),
			wantErr:   true,
			errSubstr: "name",
		},
		{
			name:      "version missing",
			yaml:      manifestWithMetadata("my-plugin", "", "1.0", "tool", "stdio"),
			wantErr:   true,
			errSubstr: "version",
		},
		{
			name:      "version not semver",
			yaml:      manifestWithMetadata("my-plugin", "not-semver", "1.0", "tool", "stdio"),
			wantErr:   true,
			errSubstr: "semver",
		},
		{
			name:      "manifestVersion missing",
			yaml:      manifestWithMetadata("my-plugin", "1.0.0", "", "tool", "stdio"),
			wantErr:   true,
			errSubstr: "manifestVersion",
		},
		{
			name:      "manifestVersion not 1.0",
			yaml:      manifestWithMetadata("my-plugin", "1.0.0", "2.0", "tool", "stdio"),
			wantErr:   true,
			errSubstr: "manifestVersion",
		},
		{
			name:      "type invalid",
			yaml:      manifestWithMetadata("my-plugin", "1.0.0", "1.0", "invalid-type", "stdio"),
			wantErr:   true,
			errSubstr: "type",
		},
		{
			name:      "transport invalid",
			yaml:      manifestWithMetadata("my-plugin", "1.0.0", "1.0", "tool", "grpc"),
			wantErr:   true,
			errSubstr: "transport",
		},
		{
			name: "tools.provide empty",
			yaml: `
metadata:
  name: my-plugin
  displayName: "My Plugin"
  version: "1.0.0"
  manifestVersion: "1.0"
spec:
  type: tool
  runtime:
    transport: stdio
    command: "./server"
  permissions:
    tools:
      provide: []
`,
			wantErr:   true,
			errSubstr: "provide",
		},
		{
			name:    "valid manifest",
			yaml:    validManifestYAML(),
			wantErr: false,
		},
		{
			name:    "valid manifest with all types",
			yaml:    manifestWithMetadata("my-plugin", "1.0.0", "1.0", "agent", "stdio"),
			wantErr: false,
		},
		{
			name:    "valid manifest with sse transport",
			yaml:    manifestWithMetadata("my-plugin", "1.0.0", "1.0", "tool", "sse"),
			wantErr: false,
		},
		{
			name:    "valid manifest with streamable-http transport",
			yaml:    manifestWithMetadata("my-plugin", "1.0.0", "1.0", "tool", "streamable-http"),
			wantErr: false,
		},
		{
			name:    "valid manifest with sidecar transport",
			yaml:    manifestWithMetadata("my-plugin", "1.0.0", "1.0", "tool", "sidecar"),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parser.Parse([]byte(tt.yaml))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errSubstr)
				}
				if tt.errSubstr != "" && !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.errSubstr)) {
					t.Errorf("expected error containing %q, got: %v", tt.errSubstr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			}
		})
	}
}


// ─────────────────────────────────────────────────────────────────────────────
// TestManifestParser_ValidateCommand — table-driven tests for ValidateCommand
// Validates: Requirements 2.7 (sandbox), ErrCommandNotAllowed, ErrPathTraversal,
//            ErrBlockedEnvVar
// ─────────────────────────────────────────────────────────────────────────────

func TestManifestParser_ValidateCommand(t *testing.T) {
	cfg := plugins.Config{
		AllowedCommands: []string{"./server", "./plugin", "./bin/server"},
		BlockedEnvVars: map[string]bool{
			"PATH":       true,
			"LD_PRELOAD": true,
			"HOME":       true,
			"USER":       true,
			"SHELL":      true,
		},
	}
	parser := plugins.NewManifestParser(cfg)

	tests := []struct {
		name    string
		runtime plugins.ManifestRuntime
		wantErr error // sentinel error to check with errors.Is
	}{
		{
			name: "command not in allowlist",
			runtime: plugins.ManifestRuntime{
				Transport: "stdio",
				Command:   "./malicious-binary",
			},
			wantErr: plugins.ErrCommandNotAllowed,
		},
		{
			name: "path traversal with ../",
			runtime: plugins.ManifestRuntime{
				Transport: "stdio",
				Command:   "../../../etc/passwd",
			},
			wantErr: plugins.ErrPathTraversal,
		},
		{
			name: "path traversal embedded in path",
			runtime: plugins.ManifestRuntime{
				Transport: "stdio",
				Command:   "./plugins/../../../etc/shadow",
			},
			wantErr: plugins.ErrPathTraversal,
		},
		{
			name: "absolute path /",
			runtime: plugins.ManifestRuntime{
				Transport: "stdio",
				Command:   "/usr/bin/python3",
			},
			wantErr: plugins.ErrPathTraversal,
		},
		{
			name: "blocked env var PATH",
			runtime: plugins.ManifestRuntime{
				Transport: "stdio",
				Command:   "./server",
				Env:       map[string]string{"PATH": "/tmp/evil"},
			},
			wantErr: plugins.ErrBlockedEnvVar,
		},
		{
			name: "blocked env var LD_PRELOAD",
			runtime: plugins.ManifestRuntime{
				Transport: "stdio",
				Command:   "./server",
				Env:       map[string]string{"LD_PRELOAD": "/tmp/evil.so"},
			},
			wantErr: plugins.ErrBlockedEnvVar,
		},
		{
			name: "blocked env var HOME",
			runtime: plugins.ManifestRuntime{
				Transport: "stdio",
				Command:   "./server",
				Env:       map[string]string{"HOME": "/tmp"},
			},
			wantErr: plugins.ErrBlockedEnvVar,
		},
		{
			name: "too many args (>50)",
			runtime: plugins.ManifestRuntime{
				Transport: "stdio",
				Command:   "./server",
				Args:      make([]string, 51),
			},
			wantErr: plugins.ErrManifestInvalid,
		},
		{
			name: "valid command",
			runtime: plugins.ManifestRuntime{
				Transport: "stdio",
				Command:   "./server",
				Args:      []string{"--port", "8080"},
				Env:       map[string]string{"PLUGIN_MODE": "production"},
			},
			wantErr: nil,
		},
		{
			name: "valid command ./plugin",
			runtime: plugins.ManifestRuntime{
				Transport: "stdio",
				Command:   "./plugin",
			},
			wantErr: nil,
		},
		{
			name: "valid command ./bin/server",
			runtime: plugins.ManifestRuntime{
				Transport: "stdio",
				Command:   "./bin/server",
			},
			wantErr: nil,
		},
		{
			name: "valid command with safe env vars",
			runtime: plugins.ManifestRuntime{
				Transport: "stdio",
				Command:   "./server",
				Env: map[string]string{
					"PLUGIN_MODE":  "production",
					"LOG_LEVEL":    "debug",
					"DATABASE_URL": "postgres://localhost/db",
				},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parser.ValidateCommand(tt.runtime)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("expected error wrapping %v, got: %v", tt.wantErr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestManifestParser_ValidatePermissions — table-driven tests for ValidatePermissions
// Validates: Requirements 2.7, 2.8, 23.5
// ─────────────────────────────────────────────────────────────────────────────

func TestManifestParser_ValidatePermissions(t *testing.T) {
	cfg := plugins.DefaultConfig()
	parser := plugins.NewManifestParser(cfg)

	tests := []struct {
		name    string
		plugin  string // plugin name for namespace validation
		perms   plugins.ManifestPermissions
		wantErr bool
		errSubstr string
	}{
		{
			name:   "data.write with core: prefix",
			plugin: "my-plugin",
			perms: plugins.ManifestPermissions{
				Tools: plugins.ToolPermissions{Provide: []string{"tool-a"}},
				Data: plugins.DataPermissions{
					Write: []string{"core:agents"},
				},
			},
			wantErr:   true,
			errSubstr: "core",
		},
		{
			name:   "data.write with core: prefix (core:sessions)",
			plugin: "my-plugin",
			perms: plugins.ManifestPermissions{
				Tools: plugins.ToolPermissions{Provide: []string{"tool-a"}},
				Data: plugins.DataPermissions{
					Write: []string{"core:sessions"},
				},
			},
			wantErr:   true,
			errSubstr: "core",
		},
		{
			name:   "data.read with other plugin namespace",
			plugin: "my-plugin",
			perms: plugins.ManifestPermissions{
				Tools: plugins.ToolPermissions{Provide: []string{"tool-a"}},
				Data: plugins.DataPermissions{
					Read: []string{"plugin:other-plugin-*"},
				},
			},
			wantErr:   true,
			errSubstr: "cross-plugin",
		},
		{
			name:   "data.read with plugin:OTHER_* namespace",
			plugin: "my-plugin",
			perms: plugins.ManifestPermissions{
				Tools: plugins.ToolPermissions{Provide: []string{"tool-a"}},
				Data: plugins.DataPermissions{
					Read: []string{"plugin:OTHER_PLUGIN_*"},
				},
			},
			wantErr:   true,
			errSubstr: "cross-plugin",
		},
		{
			name:   "events.publish outside plugin.{name}.* namespace",
			plugin: "my-plugin",
			perms: plugins.ManifestPermissions{
				Tools: plugins.ToolPermissions{Provide: []string{"tool-a"}},
				Events: plugins.EventPermissions{
					Publish: []string{"core.agent.started"},
				},
			},
			wantErr:   true,
			errSubstr: "namespace",
		},
		{
			name:   "events.publish with wrong plugin name in namespace",
			plugin: "my-plugin",
			perms: plugins.ManifestPermissions{
				Tools: plugins.ToolPermissions{Provide: []string{"tool-a"}},
				Events: plugins.EventPermissions{
					Publish: []string{"plugin.other-plugin.event"},
				},
			},
			wantErr:   true,
			errSubstr: "namespace",
		},
		{
			name:   "valid permissions — own plugin namespace",
			plugin: "my-plugin",
			perms: plugins.ManifestPermissions{
				Tools: plugins.ToolPermissions{Provide: []string{"tool-a", "tool-b"}},
				Data: plugins.DataPermissions{
					Write: []string{"plugin:my-plugin-*"},
					Read:  []string{"plugin:my-plugin-*", "agents"},
				},
				Events: plugins.EventPermissions{
					Publish:   []string{"plugin.my-plugin.created", "plugin.my-plugin.updated"},
					Subscribe: []string{"core.agent.started"},
				},
			},
			wantErr: false,
		},
		{
			name:   "valid permissions — data.read core entities allowed",
			plugin: "my-plugin",
			perms: plugins.ManifestPermissions{
				Tools: plugins.ToolPermissions{Provide: []string{"tool-a"}},
				Data: plugins.DataPermissions{
					Read: []string{"agents", "sessions"},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parser.ValidatePermissions(tt.plugin, tt.perms)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errSubstr)
				}
				if tt.errSubstr != "" && !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.errSubstr)) {
					t.Errorf("expected error containing %q, got: %v", tt.errSubstr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Helper: generate manifest YAML with specific metadata/spec fields
// ─────────────────────────────────────────────────────────────────────────────

func manifestWithMetadata(name, version, manifestVersion, specType, transport string) string {
	var b strings.Builder
	b.WriteString("metadata:\n")
	if name != "" {
		b.WriteString("  name: " + name + "\n")
	}
	b.WriteString("  displayName: \"Test Plugin\"\n")
	if version != "" {
		b.WriteString("  version: \"" + version + "\"\n")
	}
	b.WriteString("  description: \"A test plugin\"\n")
	b.WriteString("  author: \"Test\"\n")
	if manifestVersion != "" {
		b.WriteString("  manifestVersion: \"" + manifestVersion + "\"\n")
	}
	b.WriteString("spec:\n")
	if specType != "" {
		b.WriteString("  type: " + specType + "\n")
	}
	b.WriteString("  runtime:\n")
	if transport != "" {
		b.WriteString("    transport: " + transport + "\n")
	}
	b.WriteString("    command: \"./server\"\n")
	b.WriteString("  permissions:\n")
	b.WriteString("    tools:\n")
	b.WriteString("      provide:\n")
	b.WriteString("        - tool-a\n")
	return b.String()
}
