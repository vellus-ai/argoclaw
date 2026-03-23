package providers

import (
	"math/rand"
	"strings"
	"testing"
	"testing/quick"
)

// --- PBT: Property-Based Tests for Anthropic Token Validation ---

// Property: Any string with prefix "sk-ant-oat01-" and length >= 80 is a valid setup token
func TestPBT_ValidSetupTokenAlwaysAccepted(t *testing.T) {
	const prefix = "sk-ant-oat01-"
	const minLen = 80
	chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_"

	f := func(seed int64) bool {
		r := rand.New(rand.NewSource(seed))
		suffixLen := minLen - len(prefix) + r.Intn(50)
		var sb strings.Builder
		sb.WriteString(prefix)
		for i := 0; i < suffixLen; i++ {
			sb.WriteByte(chars[r.Intn(len(chars))])
		}
		token := sb.String()

		if !IsAnthropicSetupToken(token) {
			return false // property violated: valid token rejected
		}
		if err := ValidateAnthropicCredential(token); err != nil {
			return false // property violated: valid token failed validation
		}
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 500}); err != nil {
		t.Errorf("property violated: valid setup token rejected: %v", err)
	}
}

// Property: Any string with prefix "sk-ant-api" is a valid API key
func TestPBT_ValidAPIKeyAlwaysAccepted(t *testing.T) {
	const prefix = "sk-ant-api"
	chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_"

	f := func(seed int64) bool {
		r := rand.New(rand.NewSource(seed))
		suffixLen := r.Intn(100) + 10
		var sb strings.Builder
		sb.WriteString(prefix)
		for i := 0; i < suffixLen; i++ {
			sb.WriteByte(chars[r.Intn(len(chars))])
		}
		key := sb.String()

		if !IsAnthropicAPIKey(key) {
			return false
		}
		if err := ValidateAnthropicCredential(key); err != nil {
			return false
		}
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 500}); err != nil {
		t.Errorf("property violated: valid API key rejected: %v", err)
	}
}

// Property: Random strings without valid prefixes are ALWAYS rejected
func TestPBT_RandomStringAlwaysRejected(t *testing.T) {
	chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_!@#$%"

	f := func(seed int64) bool {
		r := rand.New(rand.NewSource(seed))
		length := r.Intn(200) + 1
		var sb strings.Builder
		for i := 0; i < length; i++ {
			sb.WriteByte(chars[r.Intn(len(chars))])
		}
		key := sb.String()

		// Skip if it accidentally has a valid prefix
		if strings.HasPrefix(key, "sk-ant-api") || strings.HasPrefix(key, "sk-ant-oat01-") {
			return true
		}

		err := ValidateAnthropicCredential(key)
		return err != nil // must ALWAYS fail
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 1000}); err != nil {
		t.Errorf("property violated: random string was accepted: %v", err)
	}
}

// Property: Setup token with length < 80 is ALWAYS rejected as setup token
func TestPBT_ShortSetupTokenRejected(t *testing.T) {
	const prefix = "sk-ant-oat01-"

	f := func(seed int64) bool {
		r := rand.New(rand.NewSource(seed))
		// Generate token shorter than 80 chars
		maxSuffix := 80 - len(prefix) - 1
		if maxSuffix <= 0 {
			return true
		}
		suffixLen := r.Intn(maxSuffix)
		var sb strings.Builder
		sb.WriteString(prefix)
		for i := 0; i < suffixLen; i++ {
			sb.WriteByte('a' + byte(r.Intn(26)))
		}
		token := sb.String()

		if len(token) >= 80 {
			return true // skip if accidentally long enough
		}

		// Must NOT be identified as setup token
		if IsAnthropicSetupToken(token) {
			return false // property violated
		}
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 500}); err != nil {
		t.Errorf("property violated: short setup token accepted: %v", err)
	}
}

// Property: Empty/whitespace credentials ALWAYS rejected
func TestPBT_EmptyCredentialRejected(t *testing.T) {
	empties := []string{"", " ", "\t", "\n", "  \t  \n  "}
	for _, empty := range empties {
		if err := ValidateAnthropicCredential(empty); err == nil {
			t.Errorf("empty/whitespace credential %q was accepted", empty)
		}
	}
}
