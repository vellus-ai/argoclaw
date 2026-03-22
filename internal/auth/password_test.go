package auth

import (
	"strings"
	"testing"
	"testing/quick"
	"unicode"
)

// --- TDD: Password Validation (PCI DSS) ---

func TestValidatePassword_MinLength(t *testing.T) {
	short := "Abcdef1!abc" // 11 chars — too short
	if err := ValidatePassword(short, ""); err == nil {
		t.Error("expected error for 11-char password")
	}
	exact := "Abcdef1!abcd" // 12 chars — minimum
	if err := ValidatePassword(exact, ""); err != nil {
		t.Errorf("unexpected error for 12-char password: %v", err)
	}
}

func TestValidatePassword_RequiresUppercase(t *testing.T) {
	pw := "abcdefgh1!ab" // no uppercase
	if err := ValidatePassword(pw, ""); err == nil {
		t.Error("expected error: no uppercase")
	}
}

func TestValidatePassword_RequiresLowercase(t *testing.T) {
	pw := "ABCDEFGH1!AB" // no lowercase
	if err := ValidatePassword(pw, ""); err == nil {
		t.Error("expected error: no lowercase")
	}
}

func TestValidatePassword_RequiresDigit(t *testing.T) {
	pw := "Abcdefgh!abc" // no digit
	if err := ValidatePassword(pw, ""); err == nil {
		t.Error("expected error: no digit")
	}
}

func TestValidatePassword_RequiresSpecial(t *testing.T) {
	pw := "Abcdefgh1abc" // no special
	if err := ValidatePassword(pw, ""); err == nil {
		t.Error("expected error: no special char")
	}
}

func TestValidatePassword_RejectsEmailSubstring(t *testing.T) {
	email := "milton@vellus.tech"
	pw := "Milton1!abcdef" // contains "milton" (case-insensitive)
	if err := ValidatePassword(pw, email); err == nil {
		t.Error("expected error: password contains email local part")
	}
}

func TestValidatePassword_AcceptsStrong(t *testing.T) {
	pw := "C0mpl3x!Pass#"
	if err := ValidatePassword(pw, "user@example.com"); err != nil {
		t.Errorf("unexpected error for strong password: %v", err)
	}
}

// PBT: Any password with >= 12 chars, 1+ upper, 1+ lower, 1+ digit, 1+ special
// should be accepted (when email is unrelated).
func TestValidatePassword_PBT_StrongPasswordsAccepted(t *testing.T) {
	f := func(base string) bool {
		// Build a guaranteed-strong password from random base
		if len(base) < 4 {
			return true // skip trivially short inputs
		}
		// Force compliance: prefix with known-good chars
		pw := "Aa1!" + base
		if len(pw) < 12 {
			pw = pw + strings.Repeat("x", 12-len(pw))
		}
		// This password has uppercase (A), lowercase (a), digit (1), special (!)
		err := ValidatePassword(pw, "unrelated@test.com")
		return err == nil
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 5000}); err != nil {
		t.Error(err)
	}
}

// PBT: Purely alphabetic passwords of any length must be rejected.
func TestValidatePassword_PBT_AlphaOnlyRejected(t *testing.T) {
	f := func(s string) bool {
		// Filter to only alpha chars
		var alpha []rune
		for _, r := range s {
			if unicode.IsLetter(r) {
				alpha = append(alpha, r)
			}
		}
		if len(alpha) < 12 {
			return true // too short to test meaningfully
		}
		pw := string(alpha)
		err := ValidatePassword(pw, "")
		return err != nil // must be rejected (no digit, no special)
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 5000}); err != nil {
		t.Error(err)
	}
}

// --- TDD: Password Hashing ---

func TestHashAndVerify(t *testing.T) {
	pw := "TestP@ssw0rd!"
	hash, err := HashPassword(pw)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if !VerifyPassword(pw, hash) {
		t.Error("VerifyPassword returned false for correct password")
	}
	if VerifyPassword("WrongP@ssw0rd!", hash) {
		t.Error("VerifyPassword returned true for wrong password")
	}
}

func TestHashPassword_ProducesArgon2id(t *testing.T) {
	hash, err := HashPassword("TestP@ssw0rd!")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Errorf("hash should start with $argon2id$, got: %s", hash[:20])
	}
}

func TestHashPassword_UniqueHashes(t *testing.T) {
	pw := "SameP@ssw0rd!"
	h1, _ := HashPassword(pw)
	h2, _ := HashPassword(pw)
	if h1 == h2 {
		t.Error("same password should produce different hashes (unique salt)")
	}
}

// PBT: Every hash must be verifiable and unique.
func TestHashPassword_PBT_AlwaysVerifiable(t *testing.T) {
	f := func(pw string) bool {
		if len(pw) == 0 || len(pw) > 200 {
			return true // skip edge cases
		}
		hash, err := HashPassword(pw)
		if err != nil {
			return false
		}
		return VerifyPassword(pw, hash)
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 200}); err != nil {
		t.Error(err)
	}
}
