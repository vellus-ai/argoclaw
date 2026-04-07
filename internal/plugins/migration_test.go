package plugins

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// ─────────────────────────────────────────────────────────────────────────────
// Task 9.9 — MigrationRunner unit tests (TDD RED)
// Validates: Requirements 13.1–13.6
// ─────────────────────────────────────────────────────────────────────────────

// fakeTx implements a minimal sql transaction interface for testing.
// It records all SQL statements executed against it.
type fakeTx struct {
	execCalls []execCall
	failOnSQL string // if non-empty, ExecContext returns error when SQL contains this
}

type execCall struct {
	query string
	args  []interface{}
}

func (f *fakeTx) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	f.execCalls = append(f.execCalls, execCall{query: query, args: args})
	if f.failOnSQL != "" && strings.Contains(query, f.failOnSQL) {
		return nil, fmt.Errorf("simulated SQL error on: %s", f.failOnSQL)
	}
	return fakeResult{}, nil
}

type fakeResult struct{}

func (fakeResult) LastInsertId() (int64, error) { return 0, nil }
func (fakeResult) RowsAffected() (int64, error) { return 1, nil }

// ─────────────────────────────────────────────────────────────────────────────
// Helper: create temp migration directory with SQL files
// ─────────────────────────────────────────────────────────────────────────────

func createMigrationDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", name, err)
		}
	}
	return dir
}

// ─────────────────────────────────────────────────────────────────────────────
// Test: Valid migrations execute in alphabetical order (Req 13.1)
// ─────────────────────────────────────────────────────────────────────────────

func TestMigrationRunner_ExecutesInAlphabeticalOrder(t *testing.T) {
	pluginName := "my-plugin"
	dir := createMigrationDir(t, map[string]string{
		"002_add_index.up.sql":    "CREATE INDEX idx_plugin_my_plugin_name ON plugin_my_plugin_items(name);",
		"001_create_table.up.sql": "CREATE TABLE IF NOT EXISTS plugin_my_plugin_items (id UUID PRIMARY KEY);",
		"003_add_column.up.sql":   "ALTER TABLE plugin_my_plugin_items ADD COLUMN description TEXT;",
	})

	manifest := &PluginManifest{
		Metadata: ManifestMetadata{Name: pluginName},
		Spec: ManifestSpec{
			Migrations: &ManifestMigrations{Dir: dir},
		},
	}

	tx := &fakeTx{}
	runner := NewMigrationRunner()

	err := runner.RunMigrations(context.Background(), tx, manifest)
	if err != nil {
		t.Fatalf("RunMigrations() unexpected error: %v", err)
	}

	if len(tx.execCalls) != 3 {
		t.Fatalf("expected 3 exec calls, got %d", len(tx.execCalls))
	}

	// Verify alphabetical order
	expectedOrder := []string{
		"001_create_table.up.sql",
		"002_add_index.up.sql",
		"003_add_column.up.sql",
	}
	for i, call := range tx.execCalls {
		if !strings.Contains(call.query, expectedContentForFile(expectedOrder[i], dir, t)) {
			t.Errorf("exec call %d: expected content from %s", i, expectedOrder[i])
		}
	}
}

func expectedContentForFile(filename, dir string, t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, filename))
	if err != nil {
		t.Fatalf("failed to read %s: %v", filename, err)
	}
	return string(data)
}

// ─────────────────────────────────────────────────────────────────────────────
// Test: Only .up.sql files are executed (Req 13.1)
// ─────────────────────────────────────────────────────────────────────────────

func TestMigrationRunner_OnlyUpSQLFiles(t *testing.T) {
	pluginName := "test-plugin"
	dir := createMigrationDir(t, map[string]string{
		"001_create.up.sql":   "CREATE TABLE IF NOT EXISTS plugin_test_plugin_items (id UUID PRIMARY KEY);",
		"001_create.down.sql": "DROP TABLE IF EXISTS plugin_test_plugin_items;",
		"README.md":           "# Migrations",
		"002_index.up.sql":    "CREATE INDEX idx_plugin_test_plugin_name ON plugin_test_plugin_items(name);",
	})

	manifest := &PluginManifest{
		Metadata: ManifestMetadata{Name: pluginName},
		Spec: ManifestSpec{
			Migrations: &ManifestMigrations{Dir: dir},
		},
	}

	tx := &fakeTx{}
	runner := NewMigrationRunner()

	err := runner.RunMigrations(context.Background(), tx, manifest)
	if err != nil {
		t.Fatalf("RunMigrations() unexpected error: %v", err)
	}

	// Only 2 .up.sql files should be executed (not .down.sql or .md)
	if len(tx.execCalls) != 2 {
		t.Fatalf("expected 2 exec calls (only .up.sql), got %d", len(tx.execCalls))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test: Prefix validation rejects invalid CREATE TABLE names (Req 13.2)
// ─────────────────────────────────────────────────────────────────────────────

func TestMigrationRunner_ValidatePrefix(t *testing.T) {
	tests := []struct {
		name       string
		pluginName string
		sql        string
		wantErr    bool
		errType    error
	}{
		{
			name:       "valid prefix with underscore name",
			pluginName: "my-plugin",
			sql:        "CREATE TABLE plugin_my_plugin_items (id UUID PRIMARY KEY);",
			wantErr:    false,
		},
		{
			name:       "valid prefix with IF NOT EXISTS",
			pluginName: "prompt-vault",
			sql:        "CREATE TABLE IF NOT EXISTS plugin_prompt_vault_templates (id UUID PRIMARY KEY);",
			wantErr:    false,
		},
		{
			name:       "invalid prefix - missing plugin_ prefix",
			pluginName: "my-plugin",
			sql:        "CREATE TABLE my_items (id UUID PRIMARY KEY);",
			wantErr:    true,
			errType:    ErrMigrationPrefix,
		},
		{
			name:       "invalid prefix - wrong plugin name",
			pluginName: "my-plugin",
			sql:        "CREATE TABLE plugin_other_plugin_items (id UUID PRIMARY KEY);",
			wantErr:    true,
			errType:    ErrMigrationPrefix,
		},
		{
			name:       "no CREATE TABLE - should pass",
			pluginName: "my-plugin",
			sql:        "CREATE INDEX idx_test ON plugin_my_plugin_items(name);",
			wantErr:    false,
		},
		{
			name:       "multiple CREATE TABLE - all valid",
			pluginName: "my-plugin",
			sql: `CREATE TABLE plugin_my_plugin_items (id UUID PRIMARY KEY);
CREATE TABLE plugin_my_plugin_tags (id UUID PRIMARY KEY);`,
			wantErr: false,
		},
		{
			name:       "multiple CREATE TABLE - one invalid",
			pluginName: "my-plugin",
			sql: `CREATE TABLE plugin_my_plugin_items (id UUID PRIMARY KEY);
CREATE TABLE unauthorized_table (id UUID PRIMARY KEY);`,
			wantErr: true,
			errType: ErrMigrationPrefix,
		},
		{
			name:       "case insensitive CREATE TABLE",
			pluginName: "my-plugin",
			sql:        "create table plugin_my_plugin_items (id UUID PRIMARY KEY);",
			wantErr:    false,
		},
		{
			name:       "case insensitive invalid",
			pluginName: "my-plugin",
			sql:        "Create Table bad_table (id UUID PRIMARY KEY);",
			wantErr:    true,
			errType:    ErrMigrationPrefix,
		},
	}

	runner := NewMigrationRunner()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runner.validatePrefix(tt.pluginName, tt.sql)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errType != nil && !errors.Is(err, tt.errType) {
					t.Errorf("expected error wrapping %v, got: %v", tt.errType, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test: Migration failure propagates error (Req 13.4, 4.8)
// ─────────────────────────────────────────────────────────────────────────────

func TestMigrationRunner_FailurePropagatesError(t *testing.T) {
	pluginName := "my-plugin"
	dir := createMigrationDir(t, map[string]string{
		"001_ok.up.sql":   "CREATE TABLE IF NOT EXISTS plugin_my_plugin_items (id UUID PRIMARY KEY);",
		"002_fail.up.sql": "CREATE TABLE IF NOT EXISTS plugin_my_plugin_bad (INVALID_SQL_HERE);",
	})

	manifest := &PluginManifest{
		Metadata: ManifestMetadata{Name: pluginName},
		Spec: ManifestSpec{
			Migrations: &ManifestMigrations{Dir: dir},
		},
	}

	tx := &fakeTx{failOnSQL: "INVALID_SQL_HERE"}
	runner := NewMigrationRunner()

	err := runner.RunMigrations(context.Background(), tx, manifest)
	if err == nil {
		t.Fatal("expected error from failing migration, got nil")
	}
	if !errors.Is(err, ErrMigrationFailed) {
		t.Errorf("expected ErrMigrationFailed, got: %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test: Skip when spec.migrations.dir not declared (Req 13.6)
// ─────────────────────────────────────────────────────────────────────────────

func TestMigrationRunner_SkipWhenNoMigrationsDir(t *testing.T) {
	tests := []struct {
		name     string
		manifest *PluginManifest
	}{
		{
			name: "nil migrations",
			manifest: &PluginManifest{
				Metadata: ManifestMetadata{Name: "test-plugin"},
				Spec:     ManifestSpec{Migrations: nil},
			},
		},
		{
			name: "empty migrations dir",
			manifest: &PluginManifest{
				Metadata: ManifestMetadata{Name: "test-plugin"},
				Spec:     ManifestSpec{Migrations: &ManifestMigrations{Dir: ""}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := &fakeTx{}
			runner := NewMigrationRunner()

			err := runner.RunMigrations(context.Background(), tx, tt.manifest)
			if err != nil {
				t.Fatalf("expected no error for skip, got: %v", err)
			}
			if len(tx.execCalls) != 0 {
				t.Errorf("expected 0 exec calls when skipping, got %d", len(tx.execCalls))
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test: Prefix validation rejects migration with invalid CREATE TABLE (Req 13.2)
// ─────────────────────────────────────────────────────────────────────────────

func TestMigrationRunner_RejectsMigrationWithBadPrefix(t *testing.T) {
	pluginName := "my-plugin"
	dir := createMigrationDir(t, map[string]string{
		"001_bad.up.sql": "CREATE TABLE unauthorized_table (id UUID PRIMARY KEY);",
	})

	manifest := &PluginManifest{
		Metadata: ManifestMetadata{Name: pluginName},
		Spec: ManifestSpec{
			Migrations: &ManifestMigrations{Dir: dir},
		},
	}

	tx := &fakeTx{}
	runner := NewMigrationRunner()

	err := runner.RunMigrations(context.Background(), tx, manifest)
	if err == nil {
		t.Fatal("expected error for migration with bad prefix, got nil")
	}
	if !errors.Is(err, ErrMigrationPrefix) {
		t.Errorf("expected ErrMigrationPrefix, got: %v", err)
	}
	// Should NOT have executed any SQL since prefix validation happens before exec
	if len(tx.execCalls) != 0 {
		t.Errorf("expected 0 exec calls when prefix validation fails, got %d", len(tx.execCalls))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test: Empty migration directory (no .up.sql files) succeeds (Req 13.1)
// ─────────────────────────────────────────────────────────────────────────────

func TestMigrationRunner_EmptyMigrationDir(t *testing.T) {
	dir := t.TempDir() // empty directory

	manifest := &PluginManifest{
		Metadata: ManifestMetadata{Name: "test-plugin"},
		Spec: ManifestSpec{
			Migrations: &ManifestMigrations{Dir: dir},
		},
	}

	tx := &fakeTx{}
	runner := NewMigrationRunner()

	err := runner.RunMigrations(context.Background(), tx, manifest)
	if err != nil {
		t.Fatalf("expected no error for empty dir, got: %v", err)
	}
	if len(tx.execCalls) != 0 {
		t.Errorf("expected 0 exec calls for empty dir, got %d", len(tx.execCalls))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test: Non-existent migration directory returns error
// ─────────────────────────────────────────────────────────────────────────────

func TestMigrationRunner_NonExistentDir(t *testing.T) {
	manifest := &PluginManifest{
		Metadata: ManifestMetadata{Name: "test-plugin"},
		Spec: ManifestSpec{
			Migrations: &ManifestMigrations{Dir: "/nonexistent/path/to/migrations"},
		},
	}

	tx := &fakeTx{}
	runner := NewMigrationRunner()

	err := runner.RunMigrations(context.Background(), tx, manifest)
	if err == nil {
		t.Fatal("expected error for non-existent dir, got nil")
	}
	if !errors.Is(err, ErrMigrationFailed) {
		t.Errorf("expected ErrMigrationFailed, got: %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test: Plugin name with hyphens converts to underscores for prefix (Req 13.2)
// ─────────────────────────────────────────────────────────────────────────────

func TestMigrationRunner_HyphenToUnderscoreConversion(t *testing.T) {
	runner := NewMigrationRunner()

	// "prompt-vault" → prefix "plugin_prompt_vault_"
	err := runner.validatePrefix("prompt-vault",
		"CREATE TABLE plugin_prompt_vault_templates (id UUID PRIMARY KEY);")
	if err != nil {
		t.Errorf("expected valid prefix for prompt-vault, got: %v", err)
	}

	// Using hyphens in table name should fail
	err = runner.validatePrefix("prompt-vault",
		"CREATE TABLE plugin_prompt-vault_templates (id UUID PRIMARY KEY);")
	if err == nil {
		t.Error("expected error for hyphenated table name")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test: Context cancellation is respected
// ─────────────────────────────────────────────────────────────────────────────

func TestMigrationRunner_ContextCancellation(t *testing.T) {
	pluginName := "my-plugin"
	dir := createMigrationDir(t, map[string]string{
		"001_create.up.sql": "CREATE TABLE IF NOT EXISTS plugin_my_plugin_items (id UUID PRIMARY KEY);",
	})

	manifest := &PluginManifest{
		Metadata: ManifestMetadata{Name: pluginName},
		Spec: ManifestSpec{
			Migrations: &ManifestMigrations{Dir: dir},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	tx := &fakeTx{}
	runner := NewMigrationRunner()

	err := runner.RunMigrations(ctx, tx, manifest)
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test: Alphabetical ordering is deterministic
// ─────────────────────────────────────────────────────────────────────────────

func TestMigrationRunner_AlphabeticalOrderDeterministic(t *testing.T) {
	pluginName := "test-plugin"
	files := map[string]string{
		"c_third.up.sql":  "ALTER TABLE plugin_test_plugin_items ADD COLUMN c TEXT;",
		"a_first.up.sql":  "CREATE TABLE IF NOT EXISTS plugin_test_plugin_items (id UUID PRIMARY KEY);",
		"b_second.up.sql": "ALTER TABLE plugin_test_plugin_items ADD COLUMN b TEXT;",
	}
	dir := createMigrationDir(t, files)

	manifest := &PluginManifest{
		Metadata: ManifestMetadata{Name: pluginName},
		Spec: ManifestSpec{
			Migrations: &ManifestMigrations{Dir: dir},
		},
	}

	// Run multiple times to verify determinism
	for i := 0; i < 5; i++ {
		tx := &fakeTx{}
		runner := NewMigrationRunner()

		err := runner.RunMigrations(context.Background(), tx, manifest)
		if err != nil {
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}

		if len(tx.execCalls) != 3 {
			t.Fatalf("iteration %d: expected 3 calls, got %d", i, len(tx.execCalls))
		}

		// Verify sorted order
		expectedFiles := []string{"a_first.up.sql", "b_second.up.sql", "c_third.up.sql"}
		for j, ef := range expectedFiles {
			content, _ := os.ReadFile(filepath.Join(dir, ef))
			if tx.execCalls[j].query != string(content) {
				t.Errorf("iteration %d, call %d: expected content from %s", i, j, ef)
			}
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test: listMigrationFiles returns sorted .up.sql files
// ─────────────────────────────────────────────────────────────────────────────

func TestMigrationRunner_ListMigrationFiles(t *testing.T) {
	dir := createMigrationDir(t, map[string]string{
		"003_third.up.sql":  "SELECT 1;",
		"001_first.up.sql":  "SELECT 1;",
		"002_second.up.sql": "SELECT 1;",
		"001_first.down.sql": "SELECT 1;",
		"notes.txt":         "not a migration",
	})

	runner := NewMigrationRunner()
	files, err := runner.listMigrationFiles(dir)
	if err != nil {
		t.Fatalf("listMigrationFiles() error: %v", err)
	}

	expected := []string{
		filepath.Join(dir, "001_first.up.sql"),
		filepath.Join(dir, "002_second.up.sql"),
		filepath.Join(dir, "003_third.up.sql"),
	}

	if len(files) != len(expected) {
		t.Fatalf("expected %d files, got %d: %v", len(expected), len(files), files)
	}

	for i, f := range files {
		if f != expected[i] {
			t.Errorf("file[%d] = %q, want %q", i, f, expected[i])
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test: pluginNameToPrefix converts kebab-case to underscore prefix
// ─────────────────────────────────────────────────────────────────────────────

func TestPluginNameToPrefix(t *testing.T) {
	tests := []struct {
		name   string
		want   string
	}{
		{"my-plugin", "plugin_my_plugin_"},
		{"prompt-vault", "plugin_prompt_vault_"},
		{"simple", "plugin_simple_"},
		{"a-b-c", "plugin_a_b_c_"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pluginNameToPrefix(tt.name)
			if got != tt.want {
				t.Errorf("pluginNameToPrefix(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

// Ensure sort is used (compile-time check for import usage).
var _ = sort.Strings
