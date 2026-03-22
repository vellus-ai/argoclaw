package auth

import (
	"testing"
	"testing/quick"
	"time"
)

func TestGenerateAndValidateAccessToken(t *testing.T) {
	secret := "test-secret-key-min-32-chars!!!!!"
	claims := TokenClaims{
		UserID:   "user-123",
		Email:    "test@example.com",
		TenantID: "tenant-456",
		Role:     "admin",
	}

	token, err := GenerateAccessToken(claims, secret)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	parsed, err := ValidateAccessToken(token, secret)
	if err != nil {
		t.Fatalf("ValidateAccessToken failed: %v", err)
	}

	if parsed.UserID != claims.UserID {
		t.Errorf("UserID = %q, want %q", parsed.UserID, claims.UserID)
	}
	if parsed.Email != claims.Email {
		t.Errorf("Email = %q, want %q", parsed.Email, claims.Email)
	}
	if parsed.TenantID != claims.TenantID {
		t.Errorf("TenantID = %q, want %q", parsed.TenantID, claims.TenantID)
	}
	if parsed.Role != claims.Role {
		t.Errorf("Role = %q, want %q", parsed.Role, claims.Role)
	}
}

func TestValidateAccessToken_Expired(t *testing.T) {
	secret := "test-secret-key-min-32-chars!!!!!"
	claims := TokenClaims{UserID: "user-123", Email: "t@t.com", TenantID: "t", Role: "member"}

	token, _ := generateTokenWithExpiry(claims, secret, -1*time.Minute)

	_, err := ValidateAccessToken(token, secret)
	if err == nil {
		t.Error("expected error for expired token")
	}
}

func TestValidateAccessToken_WrongSecret(t *testing.T) {
	secret1 := "test-secret-key-min-32-chars!!!!!"
	secret2 := "different-secret-key-32-chars!!!!"
	claims := TokenClaims{UserID: "user-123", Email: "t@t.com", TenantID: "t", Role: "member"}

	token, _ := GenerateAccessToken(claims, secret1)

	_, err := ValidateAccessToken(token, secret2)
	if err == nil {
		t.Error("expected error for wrong secret")
	}
}

func TestGenerateRefreshToken_Unique(t *testing.T) {
	t1, _ := GenerateRefreshToken()
	t2, _ := GenerateRefreshToken()
	if t1 == t2 {
		t.Error("refresh tokens must be unique")
	}
}

// PBT: all generated tokens must be parseable back.
func TestJWT_PBT_RoundTrip(t *testing.T) {
	secret := "pbt-test-secret-key-32-chars!!!!!"
	f := func(userID, email string) bool {
		if len(userID) == 0 || len(email) == 0 || len(userID) > 255 || len(email) > 320 {
			return true // skip invalid inputs
		}
		claims := TokenClaims{UserID: userID, Email: email, TenantID: "t", Role: "member"}
		token, err := GenerateAccessToken(claims, secret)
		if err != nil {
			return false
		}
		parsed, err := ValidateAccessToken(token, secret)
		if err != nil {
			return false
		}
		return parsed.UserID == userID && parsed.Email == email
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 1000}); err != nil {
		t.Error(err)
	}
}
