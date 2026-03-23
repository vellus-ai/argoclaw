package telegram

import (
	"math/rand"
	"strings"
	"testing"
	"testing/quick"
)

// --- PBT: Property-Based Tests for Telegram Mention Handling ---

// Property: @mentions with underscores are NEVER mangled by markdown conversion
func TestPBT_MentionWithUnderscoresPreserved(t *testing.T) {
	chars := "abcdefghijklmnopqrstuvwxyz0123456789_"

	f := func(seed int64) bool {
		r := rand.New(rand.NewSource(seed))
		// Generate valid Telegram username (5-32 chars)
		length := r.Intn(28) + 5
		var sb strings.Builder
		sb.WriteByte('@')
		for i := 0; i < length; i++ {
			sb.WriteByte(chars[r.Intn(len(chars))])
		}
		mention := sb.String()

		// Input: "Hello @username test"
		input := "Hello " + mention + " test"
		output := markdownToTelegramHTML(input)

		// The @mention must appear in the output unchanged
		if !strings.Contains(output, mention) {
			return false // property violated: mention was mangled
		}
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 500}); err != nil {
		t.Errorf("property violated: mention was mangled by markdown conversion: %v", err)
	}
}

// Property: Multiple valid Telegram @mentions in same message are ALL preserved
func TestPBT_MultipleMentionsAllPreserved(t *testing.T) {
	// Valid Telegram usernames: 5-32 alphanumeric + underscore, no consecutive underscores
	f := func(seed int64) bool {
		r := rand.New(rand.NewSource(seed))
		numMentions := r.Intn(3) + 2
		mentions := make([]string, numMentions)
		var msg strings.Builder

		for i := 0; i < numMentions; i++ {
			length := r.Intn(15) + 5
			var sb strings.Builder
			sb.WriteByte('@')
			lastWasUnderscore := false
			for j := 0; j < length; j++ {
				if r.Float32() < 0.2 && !lastWasUnderscore && j > 0 && j < length-1 {
					sb.WriteByte('_')
					lastWasUnderscore = true
				} else {
					sb.WriteByte('a' + byte(r.Intn(26)))
					lastWasUnderscore = false
				}
			}
			mentions[i] = sb.String()
			if i > 0 {
				msg.WriteString(" and ")
			}
			msg.WriteString("cc ")
			msg.WriteString(mentions[i])
		}

		output := markdownToTelegramHTML(msg.String())

		for _, m := range mentions {
			if !strings.Contains(output, m) {
				return false
			}
		}
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 300}); err != nil {
		t.Errorf("property violated: some mentions were mangled: %v", err)
	}
}

// Property: Text without @mentions is never altered to contain unexpected @ symbols
func TestPBT_NoFalseMentionInjection(t *testing.T) {
	f := func(text string) bool {
		if len(text) > 200 || strings.Contains(text, "@") {
			return true // skip texts with @ or too long
		}
		output := markdownToTelegramHTML(text)
		// Output should not contain @ unless it was in input
		atCount := strings.Count(output, "@")
		return atCount == 0
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 500}); err != nil {
		t.Errorf("property violated: false @ injection detected: %v", err)
	}
}
