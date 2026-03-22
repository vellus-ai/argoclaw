package pg

import (
	"testing"
	"testing/quick"
)

// --- TDD: Fix #1 — Table name whitelist ---

func TestValidTable_AllowsKnownTables(t *testing.T) {
	for table := range tablesWithUpdatedAt {
		if !validTableName(table) {
			t.Errorf("validTableName(%q) = false, want true", table)
		}
	}
	// Tables without updated_at that are also valid
	extras := []string{"api_keys", "paired_devices", "team_messages", "delegation_history"}
	for _, table := range extras {
		if !validTableName(table) {
			t.Errorf("validTableName(%q) = false, want true", table)
		}
	}
}

func TestValidTable_RejectsInjection(t *testing.T) {
	attacks := []string{
		"agents; DROP TABLE agents--",
		"agents UNION SELECT * FROM users",
		"1=1; --",
		"agents\x00",
		"",
		" ",
		"../../../etc/passwd",
		"agents OR 1=1",
	}
	for _, attack := range attacks {
		if validTableName(attack) {
			t.Errorf("validTableName(%q) = true, want false (injection)", attack)
		}
	}
}

// PBT: any random string that is NOT in the whitelist must be rejected
func TestValidTable_PBT_RejectsArbitraryStrings(t *testing.T) {
	f := func(s string) bool {
		if allowedTables[s] {
			return true // skip whitelisted — those are allowed
		}
		return !validTableName(s)
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 10000}); err != nil {
		t.Error(err)
	}
}
