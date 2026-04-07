package plugins

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ─────────────────────────────────────────────────────────────────────────────
// Manifest validation constants
// ─────────────────────────────────────────────────────────────────────────────

const (
	// MinPluginNameLen is the minimum length for a plugin name (kebab-case).
	MinPluginNameLen = 3

	// MaxPluginNameLen is the maximum length for a plugin name (kebab-case).
	MaxPluginNameLen = 100

	// MaxCommandArgs is the maximum number of arguments allowed in runtime.args.
	MaxCommandArgs = 50

	// CurrentManifestVersion is the only supported manifest schema version.
	CurrentManifestVersion = "1.0"

	// coreResourcePrefix is the protected prefix for core data resources.
	// Plugins cannot declare write access to scopes starting with this prefix.
	coreResourcePrefix = "core:"

	// pluginDataPrefix is the prefix for plugin-scoped data namespaces.
	pluginDataPrefix = "plugin:"

	// pluginEventPrefix is the prefix for plugin-scoped event namespaces.
	pluginEventPrefix = "plugin."
)

// ─────────────────────────────────────────────────────────────────────────────
// Validation patterns
// ─────────────────────────────────────────────────────────────────────────────

// validPluginName matches kebab-case plugin names: must start with [a-z],
// followed by segments of [a-z0-9] separated by single hyphens.
var validPluginName = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// validSemver matches semver MAJOR.MINOR.PATCH with optional pre-release and build metadata.
var validSemver = regexp.MustCompile(`^\d+\.\d+\.\d+(-[a-zA-Z0-9.-]+)?(\+[a-zA-Z0-9.-]+)?$`)

// IsValidPluginName reports whether name is a valid kebab-case plugin name
// within the allowed length range [MinPluginNameLen, MaxPluginNameLen].
func IsValidPluginName(name string) bool {
	return len(name) >= MinPluginNameLen && len(name) <= MaxPluginNameLen && validPluginName.MatchString(name)
}

// validTransports is the set of allowed MCP transports.
var validTransports = map[string]bool{
	"stdio":           true,
	"sse":             true,
	"streamable-http": true,
	"sidecar":         true,
}

// validPluginTypes is the set of allowed plugin types.
var validPluginTypes = map[string]bool{
	"tool":     true,
	"agent":    true,
	"ui":       true,
	"workflow": true,
	"full":     true,
}

// ─────────────────────────────────────────────────────────────────────────────
// New nested PluginManifest (design.md structure)
// ─────────────────────────────────────────────────────────────────────────────

// PluginManifest represents a parsed and validated plugin.yaml manifest.
type PluginManifest struct {
	Metadata ManifestMetadata `yaml:"metadata" json:"metadata"`
	Spec     ManifestSpec     `yaml:"spec" json:"spec"`

	// Backward-compat convenience accessors (not serialized).
	// These are populated after parsing for code that still uses flat fields.
	Name    string `yaml:"-" json:"-"`
	Version string `yaml:"-" json:"-"`
}

type ManifestMetadata struct {
	Name            string   `yaml:"name" json:"name"`
	DisplayName     string   `yaml:"displayName" json:"display_name"`
	Version         string   `yaml:"version" json:"version"`
	Description     string   `yaml:"description" json:"description"`
	Author          string   `yaml:"author" json:"author"`
	Icon            string   `yaml:"icon" json:"icon"`
	Tags            []string `yaml:"tags" json:"tags"`
	License         string   `yaml:"license" json:"license"`
	ManifestVersion string   `yaml:"manifestVersion" json:"manifest_version"`
}

type ManifestSpec struct {
	Type         string              `yaml:"type" json:"type"`
	Requires     ManifestRequires    `yaml:"requires" json:"requires"`
	Permissions  ManifestPermissions `yaml:"permissions" json:"permissions"`
	Runtime      ManifestRuntime     `yaml:"runtime" json:"runtime"`
	Migrations   *ManifestMigrations `yaml:"migrations" json:"migrations"`
	UI           *ManifestUI         `yaml:"ui" json:"ui"`
	ConfigSchema json.RawMessage     `yaml:"configSchema" json:"config_schema"`
}

type ManifestRequires struct {
	Platform string   `yaml:"platform" json:"platform"`
	Plan     string   `yaml:"plan" json:"plan"`
	Features []string `yaml:"features" json:"features"`
	Plugins  []string `yaml:"plugins" json:"plugins"`
}

type ManifestPermissions struct {
	Tools    ToolPermissions  `yaml:"tools" json:"tools"`
	Data     DataPermissions  `yaml:"data" json:"data"`
	Events   EventPermissions `yaml:"events" json:"events"`
	API      APIPermissions   `yaml:"api" json:"api"`
	Cron     []CronJob        `yaml:"cron" json:"cron"`
	Webhooks []WebhookConfig  `yaml:"webhooks" json:"webhooks"`
}

type ToolPermissions struct {
	Provide []string `yaml:"provide" json:"provide"`
	Consume []string `yaml:"consume" json:"consume"`
}

type DataPermissions struct {
	Read  []string `yaml:"read" json:"read"`
	Write []string `yaml:"write" json:"write"`
}

type EventPermissions struct {
	Subscribe []string `yaml:"subscribe" json:"subscribe"`
	Publish   []string `yaml:"publish" json:"publish"`
}


type APIPermissions struct {
	Endpoints []string `yaml:"endpoints" json:"endpoints"`
}

type CronJob struct {
	Name     string `yaml:"name" json:"name"`
	Schedule string `yaml:"schedule" json:"schedule"`
	Tool     string `yaml:"tool" json:"tool"`
}

type WebhookConfig struct {
	Path string `yaml:"path" json:"path"`
	Tool string `yaml:"tool" json:"tool"`
}

type ManifestRuntime struct {
	Transport   string            `yaml:"transport" json:"transport"`
	Command     string            `yaml:"command" json:"command"`
	Args        []string          `yaml:"args" json:"args"`
	Env         map[string]string `yaml:"env" json:"env"`
	URL         string            `yaml:"url" json:"url"`
	Resources   RuntimeResources  `yaml:"resources" json:"resources"`
	HealthCheck HealthCheckConfig `yaml:"healthCheck" json:"health_check"`
}

type RuntimeResources struct {
	MemoryMB   int     `yaml:"memoryMB" json:"memory_mb"`
	CPUs       float64 `yaml:"cpus" json:"cpus"`
	TimeoutSec int     `yaml:"timeoutSec" json:"timeout_sec"`
}

type HealthCheckConfig struct {
	Interval int `yaml:"interval" json:"interval"`
	Timeout  int `yaml:"timeout" json:"timeout"`
}

type ManifestMigrations struct {
	Dir string `yaml:"dir" json:"dir"`
}

type ManifestUI struct {
	Panel string `yaml:"panel" json:"panel"`
}

// ─────────────────────────────────────────────────────────────────────────────
// ManifestParser — validates and parses plugin.yaml
// ─────────────────────────────────────────────────────────────────────────────

// sortedMapKeys returns a sorted, bracket-enclosed list of keys from a map[string]bool.
// Used to build deterministic error messages listing allowed values.
func sortedMapKeys(m map[string]bool) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return "[" + strings.Join(keys, ", ") + "]"
}

// ManifestParser validates and parses plugin.yaml manifests.
// It enforces format rules (kebab-case names, semver versions, known types/transports)
// and security rules (command allowlist, blocked env vars, permission namespacing).
type ManifestParser struct {
	allowedCommands []string
	blockedEnvVars  map[string]bool
}

// NewManifestParser creates a ManifestParser from the given Config.
func NewManifestParser(cfg Config) *ManifestParser {
	return &ManifestParser{
		allowedCommands: cfg.AllowedCommands,
		blockedEnvVars:  cfg.BlockedEnvVars,
	}
}

// Parse validates the manifest against all format and security rules.
// Returns a validated PluginManifest or a descriptive error.
func (p *ManifestParser) Parse(raw []byte) (*PluginManifest, error) {
	var m PluginManifest
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("invalid manifest YAML: %w", err)
	}

	// Validate metadata
	if err := p.validateMetadata(&m.Metadata); err != nil {
		return nil, err
	}

	// Validate spec
	if err := p.validateSpec(&m.Spec); err != nil {
		return nil, err
	}

	// Validate permissions
	if err := p.ValidatePermissions(m.Metadata.Name, m.Spec.Permissions); err != nil {
		return nil, err
	}

	// Set backward-compat fields
	m.Name = m.Metadata.Name
	m.Version = m.Metadata.Version

	return &m, nil
}

func (p *ManifestParser) validateMetadata(md *ManifestMetadata) error {
	if md.Name == "" {
		return fmt.Errorf("%w: metadata.name is required", ErrManifestInvalid)
	}
	if len(md.Name) < MinPluginNameLen {
		return fmt.Errorf("%w: metadata.name must be at least %d characters, got %d",
			ErrManifestInvalid, MinPluginNameLen, len(md.Name))
	}
	if len(md.Name) > MaxPluginNameLen {
		return fmt.Errorf("%w: metadata.name must be at most %d characters, got %d",
			ErrManifestInvalid, MaxPluginNameLen, len(md.Name))
	}
	if !validPluginName.MatchString(md.Name) {
		return fmt.Errorf("%w: metadata.name must be kebab-case (regex: %s), got %q",
			ErrManifestInvalid, validPluginName.String(), md.Name)
	}
	if md.Version == "" {
		return fmt.Errorf("%w: metadata.version is required", ErrManifestInvalid)
	}
	if !validSemver.MatchString(md.Version) {
		return fmt.Errorf("%w: metadata.version must follow semver (MAJOR.MINOR.PATCH), got %q",
			ErrManifestInvalid, md.Version)
	}
	if md.ManifestVersion == "" {
		return fmt.Errorf("%w: metadata.manifestVersion is required", ErrManifestInvalid)
	}
	if md.ManifestVersion != CurrentManifestVersion {
		return fmt.Errorf("%w: metadata.manifestVersion must be %q, got %q",
			ErrManifestInvalid, CurrentManifestVersion, md.ManifestVersion)
	}
	return nil
}

func (p *ManifestParser) validateSpec(spec *ManifestSpec) error {
	if spec.Type == "" {
		return fmt.Errorf("%w: spec.type is required", ErrManifestInvalid)
	}
	if !validPluginTypes[spec.Type] {
		return fmt.Errorf("%w: spec.type must be one of %s, got %q",
			ErrManifestInvalid, sortedMapKeys(validPluginTypes), spec.Type)
	}
	if spec.Runtime.Transport == "" {
		return fmt.Errorf("%w: spec.runtime.transport is required", ErrManifestInvalid)
	}
	if !validTransports[spec.Runtime.Transport] {
		return fmt.Errorf("%w: spec.runtime.transport must be one of %s, got %q",
			ErrManifestInvalid, sortedMapKeys(validTransports), spec.Runtime.Transport)
	}
	if len(spec.Permissions.Tools.Provide) == 0 {
		return fmt.Errorf("%w: permissions.tools.provide must not be empty (plugin must expose at least one tool)",
			ErrManifestInvalid)
	}
	return nil
}

// ValidateCommand verifies that the runtime command is safe for sandbox execution.
// It checks: no path traversal, relative path only, command in allowlist,
// no blocked env vars, and arg count within limits.
func (p *ManifestParser) ValidateCommand(runtime ManifestRuntime) error {
	cmd := runtime.Command

	// Check path traversal first (before allowlist, since traversal is always bad).
	if strings.Contains(cmd, "..") {
		return fmt.Errorf("%w: command contains path traversal sequence: %q", ErrPathTraversal, cmd)
	}

	// Must be relative (not start with /).
	if strings.HasPrefix(cmd, "/") {
		return fmt.Errorf("%w: command must be a relative path, got absolute path: %q", ErrPathTraversal, cmd)
	}

	// Command must be in allowlist.
	allowed := false
	for _, ac := range p.allowedCommands {
		if cmd == ac {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("%w: %q is not in the allowed commands list", ErrCommandNotAllowed, cmd)
	}

	// Check blocked env vars.
	for envKey := range runtime.Env {
		if p.blockedEnvVars[envKey] {
			return fmt.Errorf("%w: environment variable %q cannot be overridden by plugins",
				ErrBlockedEnvVar, envKey)
		}
	}

	// Enforce maximum arg count.
	if len(runtime.Args) > MaxCommandArgs {
		return fmt.Errorf("%w: runtime.args has %d entries, maximum allowed is %d",
			ErrManifestInvalid, len(runtime.Args), MaxCommandArgs)
	}

	return nil
}

// ValidatePermissions verifies security rules for the plugin's declared permissions.
// It ensures plugins cannot write to core resources, read other plugins' data,
// or publish events outside their own namespace.
func (p *ManifestParser) ValidatePermissions(name string, perms ManifestPermissions) error {
	// data.write cannot target core resources.
	for _, scope := range perms.Data.Write {
		if strings.HasPrefix(scope, coreResourcePrefix) {
			return fmt.Errorf("%w: plugins cannot declare write access to core resources: %q",
				ErrManifestInvalid, scope)
		}
	}

	// data.read cannot reference another plugin's namespace.
	ownPrefix := pluginDataPrefix + name
	for _, scope := range perms.Data.Read {
		if strings.HasPrefix(scope, pluginDataPrefix) && !strings.HasPrefix(scope, ownPrefix) {
			return fmt.Errorf("%w: cross-plugin data access not permitted: %q (only %q namespace allowed)",
				ErrManifestInvalid, scope, ownPrefix+"*")
		}
	}

	// events.publish must be within "plugin.{name}.*" namespace.
	requiredPrefix := pluginEventPrefix + name + "."
	for _, event := range perms.Events.Publish {
		if !strings.HasPrefix(event, requiredPrefix) {
			return fmt.Errorf("%w: events.publish must be in namespace %q, got %q",
				ErrManifestInvalid, pluginEventPrefix+name+".*", event)
		}
	}

	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Backward-compatible types and functions
// ─────────────────────────────────────────────────────────────────────────────

// Permissions is a backward-compatible alias for the old flat permissions struct.
// Used by permissions_test.go and old code.
type Permissions struct {
	Tools ToolPermissions `yaml:"tools,omitempty"`
	Data  DataPermissions `yaml:"data,omitempty"`
}

// ValidatePermissions is the backward-compatible standalone validation function.
// It checks that the plugin does not declare write access to protected core resources.
func ValidatePermissions(perms Permissions) error {
	for _, scope := range perms.Data.Write {
		if strings.HasPrefix(scope, "core:") {
			return fmt.Errorf("permission denied: plugins may not declare write access to protected resource %q (only plugin-scoped writes are allowed)", scope)
		}
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Backward-compatible ParseManifest (old flat API)
// ─────────────────────────────────────────────────────────────────────────────

// legacyManifest is the old flat manifest structure used by ParseManifest.
type legacyManifest struct {
	Name        string           `yaml:"name"`
	DisplayName string           `yaml:"display_name"`
	Version     string           `yaml:"version"`
	Description string           `yaml:"description,omitempty"`
	Author      string           `yaml:"author,omitempty"`
	Homepage    string           `yaml:"homepage,omitempty"`
	License     string           `yaml:"license,omitempty"`
	Runtime     legacyRuntime    `yaml:"runtime"`
	Permissions legacyPermissions `yaml:"permissions,omitempty"`
}

type legacyRuntime struct {
	Transport  string            `yaml:"transport"`
	Command    string            `yaml:"command,omitempty"`
	Args       []string          `yaml:"args,omitempty"`
	Env        map[string]string `yaml:"env,omitempty"`
	URL        string            `yaml:"url,omitempty"`
	MemoryMB   int               `yaml:"memory_mb,omitempty"`
	CPUs       float64           `yaml:"cpus,omitempty"`
	TimeoutSec int               `yaml:"timeout_sec,omitempty"`
}

type legacyPermissions struct {
	Tools legacyToolPermissions `yaml:"tools,omitempty"`
	Data  legacyDataPermissions `yaml:"data,omitempty"`
}

type legacyToolPermissions struct {
	Provide []legacyToolDecl `yaml:"provide,omitempty"`
	Consume []string         `yaml:"consume,omitempty"`
}

type legacyToolDecl struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
}

type legacyDataPermissions struct {
	Read  []string `yaml:"read,omitempty"`
	Write []string `yaml:"write,omitempty"`
}

// ParseManifest parses and validates a plugin manifest from YAML bytes.
// This is the backward-compatible API that uses the old flat structure.
// Returns a validated PluginManifest or a descriptive error.
func ParseManifest(data []byte) (*PluginManifest, error) {
	var lm legacyManifest
	if err := yaml.Unmarshal(data, &lm); err != nil {
		return nil, fmt.Errorf("invalid manifest YAML: %w", err)
	}
	if err := validateLegacyManifest(&lm); err != nil {
		return nil, err
	}

	// Convert legacy tool provide to string slice
	toolNames := make([]string, len(lm.Permissions.Tools.Provide))
	for i, t := range lm.Permissions.Tools.Provide {
		toolNames[i] = t.Name
	}

	m := &PluginManifest{
		Metadata: ManifestMetadata{
			Name:        lm.Name,
			DisplayName: lm.DisplayName,
			Version:     lm.Version,
			Description: lm.Description,
			Author:      lm.Author,
			License:     lm.License,
		},
		Spec: ManifestSpec{
			Runtime: ManifestRuntime{
				Transport: lm.Runtime.Transport,
				Command:   lm.Runtime.Command,
				Args:      lm.Runtime.Args,
				Env:       lm.Runtime.Env,
				URL:       lm.Runtime.URL,
				Resources: RuntimeResources{
					MemoryMB:   lm.Runtime.MemoryMB,
					CPUs:       lm.Runtime.CPUs,
					TimeoutSec: lm.Runtime.TimeoutSec,
				},
			},
			Permissions: ManifestPermissions{
				Tools: ToolPermissions{
					Provide: toolNames,
					Consume: lm.Permissions.Tools.Consume,
				},
				Data: DataPermissions{
					Read:  lm.Permissions.Data.Read,
					Write: lm.Permissions.Data.Write,
				},
			},
		},
		Name:    lm.Name,
		Version: lm.Version,
	}
	return m, nil
}

// validateLegacyManifest checks all required fields for the old flat format.
func validateLegacyManifest(m *legacyManifest) error {
	if m.Name == "" {
		return fmt.Errorf("manifest: field 'name' is required")
	}
	if !validPluginName.MatchString(m.Name) {
		return fmt.Errorf("manifest: 'name' must be kebab-case (lowercase alphanumeric and hyphens, min %d chars, start/end with alphanumeric), got %q",
			MinPluginNameLen, m.Name)
	}
	if len(m.Name) < MinPluginNameLen {
		return fmt.Errorf("manifest: 'name' must be at least %d characters, got %q",
			MinPluginNameLen, m.Name)
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
		return fmt.Errorf("manifest: 'runtime.transport' must be one of %s, got %q",
			sortedMapKeys(validTransports), m.Runtime.Transport)
	}

	// Validate permissions — rejects core:* writes.
	for _, scope := range m.Permissions.Data.Write {
		if strings.HasPrefix(scope, coreResourcePrefix) {
			return fmt.Errorf("manifest: permission denied: plugins may not declare write access to protected resource %q (only plugin-scoped writes are allowed)", scope)
		}
	}

	return nil
}
