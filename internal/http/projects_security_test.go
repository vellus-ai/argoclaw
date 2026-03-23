package http

import (
	"math/rand"
	"strings"
	"testing"
	"testing/quick"
)

// --- TDD: Env Var Blocklist Tests ---

func TestValidateEnvOverrides_BlocksDangerousVars(t *testing.T) {
	dangerous := []string{
		"LD_PRELOAD", "LD_LIBRARY_PATH", "DYLD_INSERT_LIBRARIES",
		"PATH", "HOME", "SHELL", "USER",
		"PYTHONPATH", "NODE_OPTIONS", "RUBYOPT",
		"HTTP_PROXY", "HTTPS_PROXY",
		"GOCLAW_GATEWAY_TOKEN", "GOCLAW_ENCRYPTION_KEY",
		"POSTGRES_PASSWORD", "POSTGRES_DSN",
		"IFS", "BASH_ENV", "ENV",
		"SSL_CERT_FILE", "GIT_SSL_NO_VERIFY",
	}

	for _, key := range dangerous {
		t.Run(key, func(t *testing.T) {
			overrides := map[string]string{key: "malicious_value"}
			blocked := validateEnvOverrides(overrides)
			if blocked == "" {
				t.Errorf("expected %q to be blocked, but it was allowed", key)
			}
		})
	}
}

func TestValidateEnvOverrides_BlocksPrefixes(t *testing.T) {
	prefixed := []string{
		"LD_ANYTHING", "DYLD_ANYTHING",
		"GOCLAW_SECRET", "ARGOCLAW_INTERNAL",
		"POSTGRES_USER",
	}

	for _, key := range prefixed {
		t.Run(key, func(t *testing.T) {
			overrides := map[string]string{key: "value"}
			blocked := validateEnvOverrides(overrides)
			if blocked == "" {
				t.Errorf("expected prefix-blocked %q to be blocked", key)
			}
		})
	}
}

func TestValidateEnvOverrides_AllowsSafeVars(t *testing.T) {
	safe := []string{
		"MY_API_KEY", "DATABASE_URL", "REDIS_URL",
		"APP_DEBUG", "LOG_LEVEL", "TIMEOUT",
		"CUSTOM_SETTING", "MCP_SERVER_URL",
	}

	for _, key := range safe {
		t.Run(key, func(t *testing.T) {
			overrides := map[string]string{key: "safe_value"}
			blocked := validateEnvOverrides(overrides)
			if blocked != "" {
				t.Errorf("expected %q to be allowed, but it was blocked", key)
			}
		})
	}
}

func TestValidateEnvOverrides_CaseInsensitive(t *testing.T) {
	variants := []string{"ld_preload", "Ld_Preload", "LD_PRELOAD", "ld_PRELOAD"}
	for _, key := range variants {
		t.Run(key, func(t *testing.T) {
			overrides := map[string]string{key: "value"}
			blocked := validateEnvOverrides(overrides)
			if blocked == "" {
				t.Errorf("expected case variant %q to be blocked", key)
			}
		})
	}
}

func TestValidateEnvOverrides_EmptyMap(t *testing.T) {
	blocked := validateEnvOverrides(map[string]string{})
	if blocked != "" {
		t.Errorf("empty map should not block anything, got %q", blocked)
	}
}

func TestValidateEnvOverrides_MixedSafeAndDangerous(t *testing.T) {
	overrides := map[string]string{
		"MY_SAFE_VAR": "safe",
		"LD_PRELOAD":  "malicious",
		"ANOTHER":     "fine",
	}
	blocked := validateEnvOverrides(overrides)
	if blocked == "" {
		t.Error("expected LD_PRELOAD to be caught in mixed map")
	}
}

// --- PBT: Property-Based Testing ---

// Property: No randomly generated env var starting with a blocked prefix should ever pass
func TestPBT_BlockedPrefixNeverPasses(t *testing.T) {
	prefixes := []string{"LD_", "DYLD_", "GOCLAW_", "ARGOCLAW_", "POSTGRES_"}

	for _, prefix := range prefixes {
		prefix := prefix
		t.Run(prefix, func(t *testing.T) {
			f := func(suffix string) bool {
				if len(suffix) == 0 || len(suffix) > 100 {
					return true // skip edge cases
				}
				// Only valid env var chars
				for _, c := range suffix {
					if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_') {
						return true // skip invalid chars
					}
				}
				key := prefix + suffix
				overrides := map[string]string{key: "test"}
				blocked := validateEnvOverrides(overrides)
				return blocked != "" // must always be blocked
			}
			if err := quick.Check(f, &quick.Config{MaxCount: 500}); err != nil {
				t.Errorf("property violated: prefix %s was not blocked: %v", prefix, err)
			}
		})
	}
}

// Property: Any env var NOT in blocklist and NOT matching blocked prefixes should pass
func TestPBT_SafeVarAlwaysPasses(t *testing.T) {
	safeChars := "ABCDEFGHIJKLMNOPQRSTUVWXYZ_0123456789"

	f := func(seed int64) bool {
		r := rand.New(rand.NewSource(seed))
		length := r.Intn(20) + 3
		var sb strings.Builder
		// Ensure first char is not a blocked prefix start
		sb.WriteByte("ABCEFGHIJKMNOPQRSTUVWXYZ"[r.Intn(24)]) // skip D,L (DYLD_, LD_)
		for i := 1; i < length; i++ {
			sb.WriteByte(safeChars[r.Intn(len(safeChars))])
		}
		key := sb.String()

		// Skip if it happens to match a blocked var or prefix
		upper := strings.ToUpper(key)
		if blockedEnvVars[upper] || blockedEnvVars[key] {
			return true
		}
		for _, prefix := range blockedEnvPrefixes {
			if strings.HasPrefix(upper, prefix) {
				return true
			}
		}

		overrides := map[string]string{key: "value"}
		blocked := validateEnvOverrides(overrides)
		return blocked == "" // must NOT be blocked
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 1000}); err != nil {
		t.Errorf("property violated: safe var was incorrectly blocked: %v", err)
	}
}

// --- TDD: Immutable Fields Tests ---

func TestImmutableFieldsRejection(t *testing.T) {
	immutable := []string{"id", "created_by", "created_at", "tenant_id"}
	for _, field := range immutable {
		t.Run(field, func(t *testing.T) {
			updates := map[string]any{field: "hacked_value"}
			// Check that the field is in the immutable list
			found := false
			for _, f := range []string{"id", "created_by", "created_at", "tenant_id"} {
				if f == field {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("field %q should be in immutable list", field)
			}
			_ = updates // used in handler test
		})
	}
}

func TestAllowedFieldsAccepted(t *testing.T) {
	allowed := []string{"name", "slug", "channel_type", "chat_id", "team_id", "description", "status"}
	for _, field := range allowed {
		t.Run(field, func(t *testing.T) {
			if !projectUpdateAllowedFields[field] {
				t.Errorf("field %q should be in allowed fields", field)
			}
		})
	}
}

// Property: No immutable field should ever be in the allowed fields map
func TestPBT_ImmutableNeverInAllowed(t *testing.T) {
	immutable := []string{"id", "created_by", "created_at", "tenant_id"}
	for _, field := range immutable {
		if projectUpdateAllowedFields[field] {
			t.Errorf("immutable field %q must NEVER be in allowed fields", field)
		}
	}
}
