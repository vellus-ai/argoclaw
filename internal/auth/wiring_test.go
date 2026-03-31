package auth_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/vellus-ai/argoclaw/internal/auth"
	"github.com/vellus-ai/argoclaw/internal/config"
)

// TestJWTSecretLoadedFromEnv verifies that ARGOCLAW_JWT_SECRET env var is read
// into GatewayConfig.JWTSecret during config load (1.A).
func TestJWTSecretLoadedFromEnv(t *testing.T) {
	t.Setenv("ARGOCLAW_JWT_SECRET", "test-secret-key-at-least-32-bytes-long!!")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	if cfg.Gateway.JWTSecret != "test-secret-key-at-least-32-bytes-long!!" {
		t.Errorf("JWTSecret = %q, want test secret", cfg.Gateway.JWTSecret)
	}
}

// TestJWTSecretEmptyByDefault verifies that JWTSecret is empty when env var is not set.
func TestJWTSecretEmptyByDefault(t *testing.T) {
	t.Setenv("ARGOCLAW_JWT_SECRET", "")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	if cfg.Gateway.JWTSecret != "" {
		t.Errorf("JWTSecret = %q, want empty", cfg.Gateway.JWTSecret)
	}
}

// TestGenerateAccessTokenRoundtrip verifies JWT generation and validation (1.A).
func TestGenerateAccessTokenRoundtrip(t *testing.T) {
	t.Parallel()
	secret := "test-secret-key-for-jwt-signing-hmac256"
	claims := auth.TokenClaims{
		UserID:   "user-123",
		Email:    "test@example.com",
		TenantID: "tenant-456",
		Role:     "admin",
	}

	token, err := auth.GenerateAccessToken(claims, secret)
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	got, err := auth.ValidateAccessToken(token, secret)
	if err != nil {
		t.Fatalf("ValidateAccessToken: %v", err)
	}

	if got.UserID != claims.UserID {
		t.Errorf("UserID = %q, want %q", got.UserID, claims.UserID)
	}
	if got.Email != claims.Email {
		t.Errorf("Email = %q, want %q", got.Email, claims.Email)
	}
	if got.TenantID != claims.TenantID {
		t.Errorf("TenantID = %q, want %q", got.TenantID, claims.TenantID)
	}
	if got.Role != claims.Role {
		t.Errorf("Role = %q, want %q", got.Role, claims.Role)
	}
}

// TestValidateAccessToken_WrongSecret verifies that a JWT signed with a different
// secret is rejected (1.D).
func TestValidateAccessToken_WrongSecret(t *testing.T) {
	t.Parallel()
	claims := auth.TokenClaims{UserID: "user-1", Email: "a@b.com", TenantID: "t-1", Role: "member"}

	token, err := auth.GenerateAccessToken(claims, "correct-secret-key-for-signing")
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	_, err = auth.ValidateAccessToken(token, "wrong-secret-key-for-validation")
	if err == nil {
		t.Error("expected error for wrong secret, got nil")
	}
}

// TestPasswordValidation verifies PCI DSS password rules (1.E).
func TestPasswordValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		password string
		email    string
		wantErr  bool
	}{
		{"valid", "Str0ng!Pass#99", "user@example.com", false},
		{"too_short", "Sh0rt!1", "user@example.com", true},
		{"no_uppercase", "str0ng!pass#99", "user@example.com", true},
		{"no_digit", "StrongPass!Hash", "user@example.com", true},
		{"no_special", "Str0ngPassHash9", "user@example.com", true},
		{"contains_email_local", "user@exAmplE1!X", "user@example.com", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := auth.ValidatePassword(tt.password, tt.email)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePassword(%q, %q) err = %v, wantErr = %v", tt.password, tt.email, err, tt.wantErr)
			}
		})
	}
}

// TestPasswordHashAndVerify verifies Argon2id hashing (1.E).
func TestPasswordHashAndVerify(t *testing.T) {
	t.Parallel()
	password := "Str0ng!Pass#99"

	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	if !auth.VerifyPassword(password, hash) {
		t.Error("VerifyPassword returned false for correct password")
	}

	if auth.VerifyPassword("wrong-password", hash) {
		t.Error("VerifyPassword returned true for wrong password")
	}
}

// TestHashRefreshToken verifies SHA-256 hashing of refresh tokens.
func TestHashRefreshToken(t *testing.T) {
	t.Parallel()
	raw := "abc123"
	hash := auth.HashRefreshToken(raw)

	if hash == "" {
		t.Error("HashRefreshToken returned empty string")
	}
	if hash == raw {
		t.Error("HashRefreshToken returned raw token (not hashed)")
	}
	// Deterministic
	if auth.HashRefreshToken(raw) != hash {
		t.Error("HashRefreshToken not deterministic")
	}
	// Different input → different hash
	if auth.HashRefreshToken("xyz789") == hash {
		t.Error("different inputs produced same hash")
	}
}

// TestGenerateRefreshToken verifies that generated tokens are unique.
func TestGenerateRefreshToken(t *testing.T) {
	t.Parallel()
	raw1, hash1, err := auth.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	raw2, hash2, err := auth.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken 2: %v", err)
	}
	if raw1 == raw2 {
		t.Error("two calls produced same raw token")
	}
	if hash1 == hash2 {
		t.Error("two calls produced same hash")
	}
	// Hash should match raw
	if auth.HashRefreshToken(raw1) != hash1 {
		t.Error("hash1 does not match HashRefreshToken(raw1)")
	}
}

// TestVerifyPassword_MalformedHash verifies that a malformed hash returns false.
func TestVerifyPassword_MalformedHash(t *testing.T) {
	t.Parallel()
	if auth.VerifyPassword("anything", "not-a-valid-argon2-hash") {
		t.Error("VerifyPassword should return false for malformed hash")
	}
	if auth.VerifyPassword("anything", "") {
		t.Error("VerifyPassword should return false for empty hash")
	}
}

// --- PBT (Property-Based Tests) ---

// TestPBT_ValidatePassword_AcceptedPasswordsHaveRequiredProperties ensures that any
// password accepted by ValidatePassword satisfies all PCI DSS rules.
func TestPBT_ValidatePassword_AcceptedPasswordsHaveRequiredProperties(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		pw := rapid.StringMatching(`[A-Za-z0-9!@#$%^&*]{12,64}`).Draw(t, "password")
		email := "test@example.com"

		err := auth.ValidatePassword(pw, email)
		if err != nil {
			return // rejected passwords are fine — we're testing accepted ones
		}

		// If accepted, must satisfy all rules:
		if len(pw) < auth.MinPasswordLength {
			t.Fatalf("accepted password %q has len %d < %d", pw, len(pw), auth.MinPasswordLength)
		}
		hasUpper, hasDigit, hasSpecial := false, false, false
		for _, c := range pw {
			if c >= 'A' && c <= 'Z' {
				hasUpper = true
			}
			if c >= '0' && c <= '9' {
				hasDigit = true
			}
			if (c >= '!' && c <= '/') || (c >= ':' && c <= '@') || (c >= '[' && c <= '`') || (c >= '{' && c <= '~') {
				hasSpecial = true
			}
		}
		if !hasUpper {
			t.Fatalf("accepted password %q has no uppercase", pw)
		}
		if !hasDigit {
			t.Fatalf("accepted password %q has no digit", pw)
		}
		if !hasSpecial {
			t.Fatalf("accepted password %q has no special char", pw)
		}
	})
}

// TestPBT_JWTRoundtrip ensures Generate → Validate returns the original claims.
func TestPBT_JWTRoundtrip(t *testing.T) {
	t.Parallel()
	secret := "test-pbt-secret-key-for-jwt-signing-hmac256"
	rapid.Check(t, func(t *rapid.T) {
		claims := auth.TokenClaims{
			UserID:   rapid.StringMatching(`[a-f0-9-]{36}`).Draw(t, "userID"),
			Email:    rapid.StringMatching(`[a-z]{3,10}@[a-z]{3,8}\.com`).Draw(t, "email"),
			TenantID: rapid.StringMatching(`[a-f0-9-]{36}`).Draw(t, "tenantID"),
			Role:     rapid.SampledFrom([]string{"admin", "member", "operator"}).Draw(t, "role"),
		}

		token, err := auth.GenerateAccessToken(claims, secret)
		if err != nil {
			t.Fatalf("GenerateAccessToken: %v", err)
		}

		got, err := auth.ValidateAccessToken(token, secret)
		if err != nil {
			t.Fatalf("ValidateAccessToken: %v", err)
		}

		if got.UserID != claims.UserID {
			t.Fatalf("UserID: got %q, want %q", got.UserID, claims.UserID)
		}
		if got.Email != claims.Email {
			t.Fatalf("Email: got %q, want %q", got.Email, claims.Email)
		}
		if got.TenantID != claims.TenantID {
			t.Fatalf("TenantID: got %q, want %q", got.TenantID, claims.TenantID)
		}
		if got.Role != claims.Role {
			t.Fatalf("Role: got %q, want %q", got.Role, claims.Role)
		}
	})
}

// TestPBT_HashRefreshToken_Deterministic ensures Hash(x) == Hash(x) for any input.
func TestPBT_HashRefreshToken_Deterministic(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		raw := rapid.String().Draw(t, "raw")
		h1 := auth.HashRefreshToken(raw)
		h2 := auth.HashRefreshToken(raw)
		if h1 != h2 {
			t.Fatalf("HashRefreshToken not deterministic for %q: %q != %q", raw, h1, h2)
		}
		if h1 == raw && len(raw) > 0 {
			t.Fatalf("HashRefreshToken returned raw input for %q", raw)
		}
	})
}

// TestCheckPasswordHistory verifies that recent passwords are rejected (1.E, G2).
func TestCheckPasswordHistory(t *testing.T) {
	t.Parallel()
	// Generate hashes for 4 previous passwords
	var hashes []string
	passwords := []string{"Old!Pass#001X", "Old!Pass#002X", "Old!Pass#003X", "Old!Pass#004X"}
	for _, p := range passwords {
		h, err := auth.HashPassword(p)
		if err != nil {
			t.Fatalf("HashPassword(%q): %v", p, err)
		}
		hashes = append(hashes, h)
	}

	// Reusing any of the 4 should return false (password was used before)
	for i, p := range passwords {
		if auth.CheckPasswordHistory(p, hashes) {
			t.Errorf("CheckPasswordHistory(%q) = true, want false (reused password[%d])", p, i)
		}
	}

	// A new password should return true (password is new)
	if !auth.CheckPasswordHistory("Brand!New#Pass1", hashes) {
		t.Error("CheckPasswordHistory(new) = false, want true (new password)")
	}
}
