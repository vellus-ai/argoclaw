package plugins

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ─────────────────────────────────────────────────────────────────────────────
// MigrationRunner — executes plugin SQL migrations within a transaction
// Validates: Requirements 13.1–13.6
// ─────────────────────────────────────────────────────────────────────────────

// TxExecutor abstracts the transaction interface needed by MigrationRunner.
// This allows testing without a real database connection.
type TxExecutor interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

// MigrationRunner executes plugin SQL migrations during installation.
// Migrations run within the caller's transaction for atomicity with Install.
type MigrationRunner struct{}

// NewMigrationRunner creates a new MigrationRunner.
func NewMigrationRunner() *MigrationRunner {
	return &MigrationRunner{}
}

// createTablePattern matches CREATE TABLE statements (case-insensitive),
// capturing the table name. Handles optional IF NOT EXISTS.
var createTablePattern = regexp.MustCompile(
	`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(\S+)`,
)

// RunMigrations lists .up.sql files from the manifest's migrations directory,
// validates prefix on all CREATE TABLE statements, and executes them in
// alphabetical order within the provided transaction.
//
// If the manifest does not declare spec.migrations.dir, this is a no-op (Req 13.6).
// All migrations run inside the caller's TX for atomicity with Install (Req 4.1, 4.8).
func (mr *MigrationRunner) RunMigrations(ctx context.Context, tx TxExecutor, manifest *PluginManifest) error {
	// Skip when migrations not declared (Req 13.6)
	if manifest.Spec.Migrations == nil || manifest.Spec.Migrations.Dir == "" {
		return nil
	}

	dir := manifest.Spec.Migrations.Dir
	pluginName := manifest.Metadata.Name

	// List .up.sql files in alphabetical order (Req 13.1)
	files, err := mr.listMigrationFiles(dir)
	if err != nil {
		return fmt.Errorf("%w: failed to list migration files in %q: %v", ErrMigrationFailed, dir, err)
	}

	// No migration files is valid — just return
	if len(files) == 0 {
		return nil
	}

	// Read and validate all files before executing any (fail-fast on prefix errors)
	type migrationFile struct {
		path    string
		content string
	}
	migrations := make([]migrationFile, 0, len(files))

	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("%w: failed to read migration file %q: %v", ErrMigrationFailed, f, err)
		}

		// Validate prefix on all CREATE TABLE statements (Req 13.2)
		if err := mr.validatePrefix(pluginName, string(content)); err != nil {
			return err
		}

		migrations = append(migrations, migrationFile{path: f, content: string(content)})
	}

	// Execute each migration within the provided TX (Req 13.1)
	for _, m := range migrations {
		// Check context cancellation before each migration
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("%w: context cancelled during migration %q: %v",
				ErrMigrationFailed, filepath.Base(m.path), err)
		}

		// Execute the migration SQL (Req 13.5 — no string concatenation, direct exec)
		if _, err := tx.ExecContext(ctx, m.content); err != nil {
			return fmt.Errorf("%w: migration %q failed: %v",
				ErrMigrationFailed, filepath.Base(m.path), err)
		}
	}

	return nil
}

// listMigrationFiles returns sorted paths of .up.sql files in the given directory.
func (mr *MigrationRunner) listMigrationFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".up.sql") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}

	sort.Strings(files)
	return files, nil
}

// validatePrefix checks that all CREATE TABLE statements in the SQL content
// use the required prefix: plugin_{name_with_underscores}_
//
// Plugin names are kebab-case (e.g., "prompt-vault"), but table prefixes use
// underscores (e.g., "plugin_prompt_vault_").
func (mr *MigrationRunner) validatePrefix(pluginName, sqlContent string) error {
	prefix := pluginNameToPrefix(pluginName)

	matches := createTablePattern.FindAllStringSubmatch(sqlContent, -1)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		tableName := strings.ToLower(match[1])
		if !strings.HasPrefix(tableName, prefix) {
			return fmt.Errorf("%w: table %q must use prefix %q",
				ErrMigrationPrefix, match[1], prefix)
		}
	}

	return nil
}

// pluginNameToPrefix converts a kebab-case plugin name to the required
// table prefix: "my-plugin" → "plugin_my_plugin_"
func pluginNameToPrefix(name string) string {
	underscoredName := strings.ReplaceAll(name, "-", "_")
	return "plugin_" + underscoredName + "_"
}
