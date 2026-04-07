package plugins

import (
	"errors"
	"net/http"
)

// ─────────────────────────────────────────────────────────────────────────────
// Sentinel Errors
// ─────────────────────────────────────────────────────────────────────────────

var (
	// Lookup / state errors
	ErrPluginNotFound         = errors.New("plugin not found")
	ErrPluginNotInstalled     = errors.New("plugin not installed for this tenant")
	ErrPluginAlreadyInstalled = errors.New("plugin already installed")
	ErrPluginNotEnabled       = errors.New("plugin is not enabled")
	ErrPluginEnabled          = errors.New("plugin must be disabled before uninstall")

	// State machine / eligibility errors
	ErrInvalidState    = errors.New("invalid state transition")
	ErrPlanInsufficient = errors.New("tenant plan does not meet plugin requirements")
	ErrPlatformVersion = errors.New("platform version does not meet plugin requirements")
	ErrDependencyMissing = errors.New("required plugin dependency not installed")
	ErrDependentExists = errors.New("other plugins depend on this plugin")

	// Manifest / sandbox errors
	ErrManifestInvalid  = errors.New("plugin manifest validation failed")
	ErrCommandNotAllowed = errors.New("plugin command not in allowlist")
	ErrPathTraversal    = errors.New("plugin command contains path traversal")
	ErrBlockedEnvVar    = errors.New("plugin env contains blocked variable")

	// Config / migration errors
	ErrConfigInvalid   = errors.New("plugin config does not match schema")
	ErrMigrationFailed = errors.New("plugin migration failed")
	ErrMigrationPrefix = errors.New("migration table name missing required prefix")

	// Runtime errors
	ErrPluginCrashed = errors.New("plugin process crashed")
	ErrPluginTimeout = errors.New("plugin tool call timed out")
	ErrCircuitOpen   = errors.New("plugin circuit breaker is open")

	// Resource / access errors
	ErrDataTooLarge    = errors.New("plugin data value exceeds 1MB limit")
	ErrPermissionDenied = errors.New("insufficient permissions for this operation")
	ErrRateLimited     = errors.New("too many lifecycle operations")
)

// ─────────────────────────────────────────────────────────────────────────────
// HTTP Status → Error Code Mapping
// ─────────────────────────────────────────────────────────────────────────────

// httpMapping holds the HTTP status code and machine-readable error code for
// each sentinel error. Looked up by ErrorToHTTP using errors.Is.
type httpMapping struct {
	status int
	code   string
}

var errHTTPMap = map[error]httpMapping{
	// 400 Bad Request
	ErrManifestInvalid: {http.StatusBadRequest, "manifest_invalid"},
	ErrConfigInvalid:   {http.StatusBadRequest, "config_invalid"},
	ErrMigrationPrefix: {http.StatusBadRequest, "migration_prefix_invalid"},
	ErrDataTooLarge:    {http.StatusBadRequest, "data_too_large"},

	// 403 Forbidden
	ErrPlanInsufficient: {http.StatusForbidden, "plan_insufficient"},
	ErrPermissionDenied: {http.StatusForbidden, "permission_denied"},
	ErrPlatformVersion:  {http.StatusForbidden, "platform_version_mismatch"},

	// 404 Not Found
	ErrPluginNotFound:     {http.StatusNotFound, "plugin_not_found"},
	ErrPluginNotInstalled: {http.StatusNotFound, "plugin_not_installed"},
	ErrPluginNotEnabled:   {http.StatusNotFound, "plugin_not_enabled"},

	// 409 Conflict
	ErrPluginAlreadyInstalled: {http.StatusConflict, "plugin_already_installed"},
	ErrDependentExists:        {http.StatusConflict, "dependent_plugins_exist"},
	ErrPluginEnabled:          {http.StatusConflict, "plugin_enabled"},

	// 422 Unprocessable Entity
	ErrInvalidState:      {http.StatusUnprocessableEntity, "invalid_state_transition"},
	ErrDependencyMissing: {http.StatusUnprocessableEntity, "dependency_missing"},

	// 429 Too Many Requests
	ErrRateLimited: {http.StatusTooManyRequests, "rate_limited"},

	// 502 Bad Gateway
	ErrPluginCrashed:   {http.StatusBadGateway, "plugin_crashed"},
	ErrMigrationFailed: {http.StatusBadGateway, "migration_failed"},

	// 503 Service Unavailable
	ErrCircuitOpen: {http.StatusServiceUnavailable, "circuit_open"},

	// 504 Gateway Timeout
	ErrPluginTimeout: {http.StatusGatewayTimeout, "plugin_timeout"},
}

// ErrorToHTTP maps a domain error to its HTTP status code and machine-readable
// error code. It uses errors.Is for matching, so wrapped errors are handled
// correctly. Unknown errors return 500 / "internal_error".
func ErrorToHTTP(err error) (status int, code string) {
	for sentinel, m := range errHTTPMap {
		if errors.Is(err, sentinel) {
			return m.status, m.code
		}
	}
	return http.StatusInternalServerError, "internal_error"
}
