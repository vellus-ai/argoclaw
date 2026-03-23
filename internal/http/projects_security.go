package http

import "strings"

// blockedEnvVars contains environment variable names that are blocked from
// MCP override configuration. These variables could be used for code injection,
// privilege escalation, or path manipulation attacks.
var blockedEnvVars = map[string]bool{
	// Process execution hijacking
	"LD_PRELOAD":        true,
	"LD_LIBRARY_PATH":   true,
	"DYLD_INSERT_LIBRARIES": true,
	"DYLD_LIBRARY_PATH": true,

	// Path manipulation
	"PATH":              true,
	"HOME":              true,
	"SHELL":             true,
	"TERM":              true,
	"USER":              true,
	"LOGNAME":           true,

	// System configuration
	"IFS":               true,
	"ENV":               true,
	"BASH_ENV":          true,
	"CDPATH":            true,
	"GLOBIGNORE":        true,
	"SHELLOPTS":         true,
	"BASHOPTS":          true,

	// Proxy/Network hijacking
	"HTTP_PROXY":        true,
	"HTTPS_PROXY":       true,
	"ALL_PROXY":         true,
	"NO_PROXY":          true,
	"http_proxy":        true,
	"https_proxy":       true,

	// Language-specific code injection
	"PYTHONPATH":        true,
	"PYTHONSTARTUP":     true,
	"RUBYOPT":           true,
	"NODE_OPTIONS":      true,
	"NODE_PATH":         true,
	"PERL5OPT":          true,
	"PERL5LIB":          true,
	"GOPATH":            true,
	"GOROOT":            true,

	// Security-sensitive
	"SSL_CERT_FILE":     true,
	"SSL_CERT_DIR":      true,
	"CURL_CA_BUNDLE":    true,
	"REQUESTS_CA_BUNDLE": true,
	"GIT_SSL_NO_VERIFY": true,

	// ArgoClaw/GoClaw internal
	"GOCLAW_GATEWAY_TOKEN": true,
	"GOCLAW_ENCRYPTION_KEY": true,
	"GOCLAW_POSTGRES_DSN": true,
	"POSTGRES_PASSWORD":    true,
	"POSTGRES_DSN":         true,
	"RESEND_API_KEY":       true,
	"ADMIN_API_KEY":        true,
}

// blockedEnvPrefixes contains prefixes that are always blocked.
var blockedEnvPrefixes = []string{
	"LD_",
	"DYLD_",
	"GOCLAW_",
	"ARGOCLAW_",
	"POSTGRES_",
}

// validateEnvOverrides checks if any environment variable in the overrides map
// is blocked. Returns the first blocked key found, or empty string if all are allowed.
func validateEnvOverrides(overrides map[string]string) string {
	for key := range overrides {
		upper := strings.ToUpper(key)
		if blockedEnvVars[upper] || blockedEnvVars[key] {
			return key
		}
		for _, prefix := range blockedEnvPrefixes {
			if strings.HasPrefix(upper, prefix) {
				return key
			}
		}
	}
	return ""
}

// projectUpdateAllowedFields defines the fields that can be modified via UpdateProject.
var projectUpdateAllowedFields = map[string]bool{
	"name":         true,
	"slug":         true,
	"channel_type": true,
	"chat_id":      true,
	"team_id":      true,
	"description":  true,
	"status":       true,
}
