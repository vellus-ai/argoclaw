//go:build !integration

package cmd

import (
	"os"
	"strings"
	"testing"
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
