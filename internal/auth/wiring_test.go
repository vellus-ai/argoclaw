package auth_test

import (
	"os"
	"testing"

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
	os.Unsetenv("ARGOCLAW_JWT_SECRET")

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
	if auth.VerifyPassword("anything", "not-a-valid-argon2-hash") {
		t.Error("VerifyPassword should return false for malformed hash")
	}
	if auth.VerifyPassword("anything", "") {
		t.Error("VerifyPassword should return false for empty hash")
	}
}

// TestCheckPasswordHistory verifies that recent passwords are rejected (1.E, G2).
func TestCheckPasswordHistory(t *testing.T) {
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
