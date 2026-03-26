//go:build !integration

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

func setTestEnv(t *testing.T, key, val string) {
	t.Helper()
	orig, exists := os.LookupEnv(key)
	if err := os.Setenv(key, val); err != nil {
		t.Fatalf("setenv %s: %v", key, err)
	}
	t.Cleanup(func() {
		if exists {
			os.Setenv(key, orig)
		} else {
			os.Unsetenv(key)
		}
	})
}

func unsetTestEnv(t *testing.T, key string) {
	t.Helper()
	orig, exists := os.LookupEnv(key)
	os.Unsetenv(key)
	t.Cleanup(func() {
		if exists {
			os.Setenv(key, orig)
		}
	})
}

func TestNonInteractive_MissingDSN_ReturnsError(t *testing.T) {
	unsetTestEnv(t, "ARGOCLAW_POSTGRES_DSN")

	err := runOnboardNonInteractiveE(nonInteractiveOpts{
		configPath: t.TempDir() + "/config.json",
		envPath:    t.TempDir() + "/.env.local",
		skipDB:     true,
	})
	if err == nil {
		t.Fatal("expected error when ARGOCLAW_POSTGRES_DSN not set, got nil")
	}
	if !strings.Contains(err.Error(), "ARGOCLAW_POSTGRES_DSN") {
		t.Errorf("error should mention ARGOCLAW_POSTGRES_DSN, got: %v", err)
	}
}

func TestNonInteractive_ValidEnv_WritesEnvFile(t *testing.T) {
	dir := t.TempDir()
	setTestEnv(t, "ARGOCLAW_POSTGRES_DSN", "postgres://user:pass@localhost:5432/argoclaw")

	opts := nonInteractiveOpts{
		configPath: dir + "/config.json",
		envPath:    dir + "/.env.local",
		skipDB:     true,
	}
	if err := runOnboardNonInteractiveE(opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(opts.envPath)
	if err != nil {
		t.Fatalf("env file not written: %v", err)
	}
	content := string(data)
	for _, key := range []string{"ARGOCLAW_POSTGRES_DSN", "ARGOCLAW_GATEWAY_TOKEN", "ARGOCLAW_ENCRYPTION_KEY"} {
		if !strings.Contains(content, key) {
			t.Errorf("env file missing %s; content:\n%s", key, content)
		}
	}
}

func TestNonInteractive_GeneratesTokenWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	setTestEnv(t, "ARGOCLAW_POSTGRES_DSN", "postgres://user:pass@localhost:5432/argoclaw")
	unsetTestEnv(t, "ARGOCLAW_GATEWAY_TOKEN")
	unsetTestEnv(t, "ARGOCLAW_ENCRYPTION_KEY")

	opts := nonInteractiveOpts{
		configPath: dir + "/config.json",
		envPath:    dir + "/.env.local",
		skipDB:     true,
	}
	if err := runOnboardNonInteractiveE(opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(opts.envPath)
	content := string(data)
	// Token and key must be present (auto-generated)
	if !strings.Contains(content, "ARGOCLAW_GATEWAY_TOKEN=") {
		t.Error("gateway token should be auto-generated")
	}
	if !strings.Contains(content, "ARGOCLAW_ENCRYPTION_KEY=") {
		t.Error("encryption key should be auto-generated")
	}
}

func TestNonInteractive_PreservesExistingToken(t *testing.T) {
	dir := t.TempDir()
	setTestEnv(t, "ARGOCLAW_POSTGRES_DSN", "postgres://user:pass@localhost:5432/argoclaw")
	setTestEnv(t, "ARGOCLAW_GATEWAY_TOKEN", "mytoken123456789012345678901234")
	setTestEnv(t, "ARGOCLAW_ENCRYPTION_KEY", "mykey1234567890123456789012345678")

	opts := nonInteractiveOpts{
		configPath: dir + "/config.json",
		envPath:    dir + "/.env.local",
		skipDB:     true,
	}
	if err := runOnboardNonInteractiveE(opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(opts.envPath)
	content := string(data)
	if !strings.Contains(content, "mytoken123456789012345678901234") {
		t.Errorf("gateway token not preserved; env file:\n%s", content)
	}
	if !strings.Contains(content, "mykey1234567890123456789012345678") {
		t.Errorf("encryption key not preserved; env file:\n%s", content)
	}
}

func TestPBT_NonInteractive_NeverPanics(t *testing.T) {
	// Use RuneFrom with explicit printable ASCII runes to avoid null bytes rejected by os.Setenv.
	printableRunes := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_=+/.:@!#%^&*()")
	printable := rapid.StringOf(rapid.RuneFrom(printableRunes))
	rapid.Check(t, func(rt *rapid.T) {
		dsn := printable.Draw(rt, "dsn")
		token := printable.Draw(rt, "token")
		key := printable.Draw(rt, "key")

		// rapid.T doesn't have Setenv; set and restore env vars manually.
		setTestEnv(t, "ARGOCLAW_POSTGRES_DSN", dsn)
		setTestEnv(t, "ARGOCLAW_GATEWAY_TOKEN", token)
		setTestEnv(t, "ARGOCLAW_ENCRYPTION_KEY", key)

		dir := t.TempDir()
		opts := nonInteractiveOpts{
			configPath: filepath.Join(dir, "argoclaw.json"),
			envPath:    filepath.Join(dir, ".env.local"),
			skipDB:     true,
		}
		// Verify no panic occurs with any combination of string inputs.
		func() {
			defer func() {
				if r := recover(); r != nil {
					rt.Fatalf("runOnboardNonInteractiveE panicked: %v", r)
				}
			}()
			_ = runOnboardNonInteractiveE(opts)
		}()
	})
}

func TestPBT_OnboardWriteEnvFile_NeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dsn := rapid.String().Draw(rt, "dsn")
		token := rapid.String().Draw(rt, "token")
		key := rapid.String().Draw(rt, "key")

		dir := t.TempDir()
		path := filepath.Join(dir, ".env.local")
		// Verify it never panics with arbitrary string inputs.
		_ = onboardWriteEnvFile(path, dsn, token, key)
	})
}

func TestOnboardWriteEnvFile_SingleQuotesValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env.local")

	dsn := "postgres://user:p@$$w0rd!@localhost/db"
	token := "tok$en"
	key := "key with spaces"

	if err := onboardWriteEnvFile(path, dsn, token, key); err != nil {
		t.Fatalf("onboardWriteEnvFile returned error: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	text := string(content)
	for _, want := range []string{
		"export ARGOCLAW_POSTGRES_DSN='" + dsn + "'",
		"export ARGOCLAW_GATEWAY_TOKEN='" + token + "'",
		"export ARGOCLAW_ENCRYPTION_KEY='" + key + "'",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("env file missing %q\nactual:\n%s", want, text)
		}
	}
}

func TestOnboardGenerateToken_ReturnsHexOfCorrectLength(t *testing.T) {
	for _, length := range []int{1, 8, 16, 32, 64} {
		tok, err := onboardGenerateToken(length)
		if err != nil {
			t.Fatalf("onboardGenerateToken(%d) unexpected error: %v", length, err)
		}
		if got := len(tok); got != length*2 {
			t.Errorf("onboardGenerateToken(%d) len = %d, want %d", length, got, length*2)
		}
	}
}

func TestNonInteractive_NeverReadsStdin(t *testing.T) {
	dir := t.TempDir()
	setTestEnv(t, "ARGOCLAW_POSTGRES_DSN", "postgres://user:pass@localhost:5432/argoclaw")

	// Replace stdin with /dev/null to detect any accidental stdin reads
	origStdin := os.Stdin
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Skip("cannot open /dev/null:", err)
	}
	os.Stdin = devNull
	defer func() {
		os.Stdin = origStdin
		devNull.Close()
	}()

	opts := nonInteractiveOpts{
		configPath: dir + "/config.json",
		envPath:    dir + "/.env.local",
		skipDB:     true,
	}
	// Should NOT block or fail due to closed stdin
	if err := runOnboardNonInteractiveE(opts); err != nil {
		t.Fatalf("non-interactive should not read stdin; err: %v", err)
	}
}
