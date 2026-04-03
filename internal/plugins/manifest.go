package plugins

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// validPluginName matches kebab-case plugin names: lowercase alphanumeric and hyphens,
// must start and end with alphanumeric, minimum 3 chars.
var validPluginName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,}[a-z0-9]$`)

// IsValidPluginName reports whether name is a valid kebab-case plugin name.
func IsValidPluginName(name string) bool {
	return validPluginName.MatchString(name)
}

// validSemver matches a simple semver pattern MAJOR.MINOR.PATCH with optional pre-release.
var validSemver = regexp.MustCompile(`^\d+\.\d+\.\d+(-[a-zA-Z0-9.-]+)?(\+[a-zA-Z0-9.-]+)?$`)

// validTransports is the set of allowed MCP transports.
var validTransports = map[string]bool{
	"stdio":           true,
	"sse":             true,
	"streamable-http": true,
	"sidecar":         true,
}

// PluginManifest represents a parsed plugin.yaml manifest.
type PluginManifest struct {
	Name        string      `yaml:"name"`
	DisplayName string      `yaml:"display_name"`
	Version     string      `yaml:"version"`
	Description string      `yaml:"description,omitempty"`
	Author      string      `yaml:"author,omitempty"`
	Homepage    string      `yaml:"homepage,omitempty"`
	License     string      `yaml:"license,omitempty"`
	Runtime     Runtime     `yaml:"runtime"`
	Permissions Permissions `yaml:"permissions,omitempty"`
}

// Runtime declares how the plugin process is started.
type Runtime struct {
	Transport  string            `yaml:"transport"`
	Command    string            `yaml:"command,omitempty"`
	Args       []string          `yaml:"args,omitempty"`
	Env        map[string]string `yaml:"env,omitempty"`
	URL        string            `yaml:"url,omitempty"` // for sse/streamable-http
	MemoryMB   int               `yaml:"memory_mb,omitempty"`
	CPUs       float64           `yaml:"cpus,omitempty"`
	TimeoutSec int               `yaml:"timeout_sec,omitempty"`
}

// Permissions declares what the plugin is allowed to access.
type Permissions struct {
	Tools ToolPermissions `yaml:"tools,omitempty"`
	Data  DataPermissions `yaml:"data,omitempty"`
}

// ToolPermissions lists tools the plugin provides and consumes.
type ToolPermissions struct {
	Provide []ToolDecl `yaml:"provide,omitempty"`
	Consume []string   `yaml:"consume,omitempty"`
}

// ToolDecl describes a tool exposed by the plugin.
type ToolDecl struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
}

// DataPermissions lists data scopes the plugin can read/write.
type DataPermissions struct {
	Read  []string `yaml:"read,omitempty"`
	Write []string `yaml:"write,omitempty"`
}

// ParseManifest parses and validates a plugin manifest from YAML bytes.
// Returns a validated PluginManifest or a descriptive error.
func ParseManifest(data []byte) (*PluginManifest, error) {
	var m PluginManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("invalid manifest YAML: %w", err)
	}
	if err := validateManifest(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

// validateManifest checks all required fields and format constraints.
func validateManifest(m *PluginManifest) error {
	if m.Name == "" {
		return fmt.Errorf("manifest: field 'name' is required")
	}
	if !validPluginName.MatchString(m.Name) {
		return fmt.Errorf("manifest: 'name' must be kebab-case (lowercase alphanumeric and hyphens, min 3 chars, start/end with alphanumeric), got %q", m.Name)
	}
	if m.Version == "" {
		return fmt.Errorf("manifest: field 'version' is required")
	}
	if !validSemver.MatchString(m.Version) {
		return fmt.Errorf("manifest: 'version' must follow semver (MAJOR.MINOR.PATCH), got %q", m.Version)
	}
	if m.Runtime.Transport == "" {
		return fmt.Errorf("manifest: field 'runtime.transport' is required")
	}
	if !validTransports[m.Runtime.Transport] {
		return fmt.Errorf("manifest: 'runtime.transport' must be one of [stdio, sse, streamable-http, sidecar], got %q", m.Runtime.Transport)
	}

	// Validate permissions inline — rejects core:* writes (G4).
	if err := ValidatePermissions(m.Permissions); err != nil {
		return fmt.Errorf("manifest: %w", err)
	}

	return nil
}

// ValidatePermissions checks that the plugin does not declare write access
// to protected core resources. Read access to core resources is permitted.
// This enforces blocker G4.
func ValidatePermissions(perms Permissions) error {
	for _, scope := range perms.Data.Write {
		if isCoreResource(scope) {
			return fmt.Errorf("permission denied: plugins may not declare write access to protected resource %q (only plugin-scoped writes are allowed)", scope)
		}
	}
	return nil
}

// isCoreResource returns true if the scope refers to a protected core resource.
// Any string starting with "core:" is considered a core resource.
func isCoreResource(scope string) bool {
	return strings.HasPrefix(scope, "core:")
}
