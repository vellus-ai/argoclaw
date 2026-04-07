//go:build integration

package migrations_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"pgregory.net/rapid"
)

// **Validates: Requirements 21.5**
//
// Property P5: up → down → up produces schema identical to the first application.
//
// This test verifies that migration 000032_plugin_host_v2 is idempotent:
// applying up, then down, then up again yields the same schema state as
// the first application alone.

// ─────────────────────────────────────────────────────────────────────────────
// Schema snapshot types
// ─────────────────────────────────────────────────────────────────────────────

// columnInfo captures the relevant attributes of a table column.
type columnInfo struct {
	TableName     string
	ColumnName    string
	DataType      string
	IsNullable    string
	ColumnDefault sql.NullString
}

// constraintInfo captures a CHECK / UNIQUE / FK constraint.
type constraintInfo struct {
	ConstraintName string
	TableName      string
	ConstraintType string
	Definition     string
}

// triggerInfo captures a trigger definition.
type triggerInfo struct {
	TriggerName     string
	EventObject     string
	ActionTiming    string
	EventManip      string
	ActionStatement string
}

// routineInfo captures a stored function/procedure.
type routineInfo struct {
	RoutineName string
	RoutineType string
	DataType    string
	Definition  string
}

// schemaSnapshot is the full observable state we compare.
type schemaSnapshot struct {
	Columns     []columnInfo
	Constraints []constraintInfo
	Triggers    []triggerInfo
	Routines    []routineInfo
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func testDSN() string {
	if dsn := os.Getenv("ARGOCLAW_TEST_DSN"); dsn != "" {
		return dsn
	}
	return "postgres://argoclaw:argoclaw@localhost:5432/argoclaw_test?sslmode=disable"
}

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", testDSN())
	if err != nil {
		t.Skipf("SKIP: cannot open test DB: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Skipf("SKIP: test DB unavailable: %v", err)
	}
	return db
}

// mustExecSQL reads a .sql file and executes it. Returns error instead of failing.
func mustExecSQL(db *sql.DB, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read SQL file %q: %w", path, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := db.ExecContext(ctx, string(data)); err != nil {
		return fmt.Errorf("exec SQL file %q: %w", path, err)
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Schema introspection
// ─────────────────────────────────────────────────────────────────────────────

// pluginTables are the tables affected by migrations 030 and 032.
var pluginTables = []string{
	"plugin_catalog",
	"tenant_plugins",
	"plugin_data",
	"plugin_audit_log",
}

func captureColumns(db *sql.DB) ([]columnInfo, error) {
	placeholders := make([]string, len(pluginTables))
	args := make([]interface{}, len(pluginTables))
	for i, tbl := range pluginTables {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = tbl
	}
	query := fmt.Sprintf(`
		SELECT table_name, column_name, data_type, is_nullable, column_default
		FROM information_schema.columns
		WHERE table_schema = 'public'
		  AND table_name IN (%s)
		ORDER BY table_name, ordinal_position
	`, strings.Join(placeholders, ","))

	rows, err := db.QueryContext(context.Background(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("captureColumns: %w", err)
	}
	defer rows.Close()

	var cols []columnInfo
	for rows.Next() {
		var c columnInfo
		if err := rows.Scan(&c.TableName, &c.ColumnName, &c.DataType, &c.IsNullable, &c.ColumnDefault); err != nil {
			return nil, fmt.Errorf("captureColumns scan: %w", err)
		}
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

func captureConstraints(db *sql.DB) ([]constraintInfo, error) {
	placeholders := make([]string, len(pluginTables))
	args := make([]interface{}, len(pluginTables))
	for i, tbl := range pluginTables {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = tbl
	}
	query := fmt.Sprintf(`
		SELECT tc.constraint_name, tc.table_name, tc.constraint_type,
		       COALESCE(cc.check_clause, '')
		FROM information_schema.table_constraints tc
		LEFT JOIN information_schema.check_constraints cc
		  ON tc.constraint_name = cc.constraint_name
		  AND tc.constraint_schema = cc.constraint_schema
		WHERE tc.table_schema = 'public'
		  AND tc.table_name IN (%s)
		ORDER BY tc.table_name, tc.constraint_name
	`, strings.Join(placeholders, ","))

	rows, err := db.QueryContext(context.Background(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("captureConstraints: %w", err)
	}
	defer rows.Close()

	var cs []constraintInfo
	for rows.Next() {
		var c constraintInfo
		if err := rows.Scan(&c.ConstraintName, &c.TableName, &c.ConstraintType, &c.Definition); err != nil {
			return nil, fmt.Errorf("captureConstraints scan: %w", err)
		}
		cs = append(cs, c)
	}
	return cs, rows.Err()
}

func captureTriggers(db *sql.DB) ([]triggerInfo, error) {
	placeholders := make([]string, len(pluginTables))
	args := make([]interface{}, len(pluginTables))
	for i, tbl := range pluginTables {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = tbl
	}
	query := fmt.Sprintf(`
		SELECT trigger_name, event_object_table, action_timing,
		       event_manipulation, action_statement
		FROM information_schema.triggers
		WHERE trigger_schema = 'public'
		  AND event_object_table IN (%s)
		ORDER BY event_object_table, trigger_name, event_manipulation
	`, strings.Join(placeholders, ","))

	rows, err := db.QueryContext(context.Background(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("captureTriggers: %w", err)
	}
	defer rows.Close()

	var ts []triggerInfo
	for rows.Next() {
		var tr triggerInfo
		if err := rows.Scan(&tr.TriggerName, &tr.EventObject, &tr.ActionTiming, &tr.EventManip, &tr.ActionStatement); err != nil {
			return nil, fmt.Errorf("captureTriggers scan: %w", err)
		}
		ts = append(ts, tr)
	}
	return ts, rows.Err()
}

func captureRoutines(db *sql.DB) ([]routineInfo, error) {
	rows, err := db.QueryContext(context.Background(), `
		SELECT routine_name, routine_type, data_type,
		       COALESCE(routine_definition, '')
		FROM information_schema.routines
		WHERE routine_schema = 'public'
		  AND routine_name = 'reject_audit_mutation'
		ORDER BY routine_name
	`)
	if err != nil {
		return nil, fmt.Errorf("captureRoutines: %w", err)
	}
	defer rows.Close()

	var rs []routineInfo
	for rows.Next() {
		var r routineInfo
		if err := rows.Scan(&r.RoutineName, &r.RoutineType, &r.DataType, &r.Definition); err != nil {
			return nil, fmt.Errorf("captureRoutines scan: %w", err)
		}
		rs = append(rs, r)
	}
	return rs, rows.Err()
}

func captureSchema(db *sql.DB) (schemaSnapshot, error) {
	cols, err := captureColumns(db)
	if err != nil {
		return schemaSnapshot{}, err
	}
	cons, err := captureConstraints(db)
	if err != nil {
		return schemaSnapshot{}, err
	}
	trigs, err := captureTriggers(db)
	if err != nil {
		return schemaSnapshot{}, err
	}
	routs, err := captureRoutines(db)
	if err != nil {
		return schemaSnapshot{}, err
	}
	return schemaSnapshot{
		Columns:     cols,
		Constraints: cons,
		Triggers:    trigs,
		Routines:    routs,
	}, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Comparison helpers
// ─────────────────────────────────────────────────────────────────────────────

func columnsEqual(a, b []columnInfo) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func constraintsEqual(a, b []constraintInfo) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func triggersEqual(a, b []triggerInfo) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func routinesEqual(a, b []routineInfo) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func snapshotsEqual(a, b schemaSnapshot) bool {
	return columnsEqual(a.Columns, b.Columns) &&
		constraintsEqual(a.Constraints, b.Constraints) &&
		triggersEqual(a.Triggers, b.Triggers) &&
		routinesEqual(a.Routines, b.Routines)
}

func diffSnapshots(t *rapid.T, label string, first, second schemaSnapshot) {
	if !columnsEqual(first.Columns, second.Columns) {
		t.Logf("[%s] columns differ", label)
		diffSlice(t, "columns", fmtColumns(first.Columns), fmtColumns(second.Columns))
	}
	if !constraintsEqual(first.Constraints, second.Constraints) {
		t.Logf("[%s] constraints differ", label)
		diffSlice(t, "constraints", fmtConstraints(first.Constraints), fmtConstraints(second.Constraints))
	}
	if !triggersEqual(first.Triggers, second.Triggers) {
		t.Logf("[%s] triggers differ", label)
		diffSlice(t, "triggers", fmtTriggers(first.Triggers), fmtTriggers(second.Triggers))
	}
	if !routinesEqual(first.Routines, second.Routines) {
		t.Logf("[%s] routines differ", label)
		diffSlice(t, "routines", fmtRoutines(first.Routines), fmtRoutines(second.Routines))
	}
}

func diffSlice(t *rapid.T, kind string, a, b []string) {
	setA := make(map[string]bool, len(a))
	for _, s := range a {
		setA[s] = true
	}
	setB := make(map[string]bool, len(b))
	for _, s := range b {
		setB[s] = true
	}
	for _, s := range a {
		if !setB[s] {
			t.Logf("  %s: only in first:  %s", kind, s)
		}
	}
	for _, s := range b {
		if !setA[s] {
			t.Logf("  %s: only in second: %s", kind, s)
		}
	}
}

func fmtColumns(cs []columnInfo) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = fmt.Sprintf("%s.%s type=%s nullable=%s default=%v",
			c.TableName, c.ColumnName, c.DataType, c.IsNullable, c.ColumnDefault)
	}
	sort.Strings(out)
	return out
}

func fmtConstraints(cs []constraintInfo) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = fmt.Sprintf("%s.%s type=%s def=%s",
			c.TableName, c.ConstraintName, c.ConstraintType, c.Definition)
	}
	sort.Strings(out)
	return out
}

func fmtTriggers(ts []triggerInfo) []string {
	out := make([]string, len(ts))
	for i, tr := range ts {
		out[i] = fmt.Sprintf("%s.%s timing=%s event=%s action=%s",
			tr.EventObject, tr.TriggerName, tr.ActionTiming, tr.EventManip, tr.ActionStatement)
	}
	sort.Strings(out)
	return out
}

func fmtRoutines(rs []routineInfo) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = fmt.Sprintf("%s type=%s returns=%s",
			r.RoutineName, r.RoutineType, r.DataType)
	}
	sort.Strings(out)
	return out
}

// ─────────────────────────────────────────────────────────────────────────────
// Property-based test: P5 — Migration idempotency
// ─────────────────────────────────────────────────────────────────────────────

// TestProperty_P5_MigrationIdempotency verifies that applying migration 032
// up → down → up produces a schema identical to the first up application.
//
// **Validates: Requirements 21.5**
//
// The rapid property test runs multiple iterations. Each iteration:
//  1. Applies migration 032 up (first time)
//  2. Captures schema snapshot A
//  3. Runs N down → up cycles (N drawn from [1,3])
//  4. Captures schema snapshot B
//  5. Asserts A == B
//
// The rapid generator produces a variable number of up/down cycles (1..3)
// to verify idempotency holds across repeated applications.
func TestProperty_P5_MigrationIdempotency(t *testing.T) {
	db := openDB(t)
	defer db.Close()

	// Ensure migration 030 (base plugin host) is applied as prerequisite.
	// Migration 032 modifies tables created by 030.
	if err := mustExecSQL(db, "000030_plugin_host.up.sql"); err != nil {
		t.Fatalf("prerequisite migration 030 up: %v", err)
	}
	t.Cleanup(func() {
		// Best-effort rollback: 032 down then 030 down.
		mustExecSQL(db, "000032_plugin_host_v2.down.sql") //nolint:errcheck
		mustExecSQL(db, "000030_plugin_host.down.sql")    //nolint:errcheck
	})

	rapid.Check(t, func(rt *rapid.T) {
		// Generate 1-3 up/down cycles to stress idempotency.
		cycles := rapid.IntRange(1, 3).Draw(rt, "cycles")

		// Step 1: Apply up for the first time and capture baseline.
		if err := mustExecSQL(db, "000032_plugin_host_v2.up.sql"); err != nil {
			rt.Fatalf("first up: %v", err)
		}
		firstSnapshot, err := captureSchema(db)
		if err != nil {
			rt.Fatalf("capture schema after first up: %v", err)
		}

		// Step 2: Run N cycles of down → up.
		for i := 0; i < cycles; i++ {
			if err := mustExecSQL(db, "000032_plugin_host_v2.down.sql"); err != nil {
				rt.Fatalf("down cycle %d: %v", i+1, err)
			}
			if err := mustExecSQL(db, "000032_plugin_host_v2.up.sql"); err != nil {
				rt.Fatalf("up cycle %d: %v", i+1, err)
			}
		}

		// Step 3: Capture schema after cycles and compare.
		finalSnapshot, err := captureSchema(db)
		if err != nil {
			rt.Fatalf("capture schema after cycles: %v", err)
		}

		if !snapshotsEqual(firstSnapshot, finalSnapshot) {
			diffSnapshots(rt, fmt.Sprintf("after %d cycle(s)", cycles), firstSnapshot, finalSnapshot)
			rt.Fatalf("P5 violated: schema differs after %d up/down cycle(s)", cycles)
		}

		// Cleanup: roll back 032 so the next rapid iteration starts clean.
		if err := mustExecSQL(db, "000032_plugin_host_v2.down.sql"); err != nil {
			rt.Fatalf("cleanup down: %v", err)
		}
	})
}
