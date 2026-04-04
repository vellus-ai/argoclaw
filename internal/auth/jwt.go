package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Token durations.
const (
	AccessTokenDuration  = 15 * time.Minute
	RefreshTokenDuration = 7 * 24 * time.Hour // 7 days
	RefreshTokenBytes    = 32
)

// TokenClaims holds the claims embedded in an access token.
type TokenClaims struct {
	UserID   string `json:"uid"`
	Email    string `json:"email"`
	TenantID string `json:"tid"`
	Role     string `json:"role"`
}

// argoClaims wraps TokenClaims with standard JWT claims.
type argoClaims struct {
	TokenClaims
	jwt.RegisteredClaims
}

// GenerateAccessToken creates a signed JWT access token.
func GenerateAccessToken(claims TokenClaims, secret string) (string, error) {
	return generateTokenWithExpiry(claims, secret, AccessTokenDuration)
}

// generateTokenWithExpiry creates a JWT with a custom expiry duration.
func generateTokenWithExpiry(claims TokenClaims, secret string, expiry time.Duration) (string, error) {
	now := time.Now()
	c := argoClaims{
		TokenClaims: claims,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "argoclaw",
			Subject:   claims.UserID,
			// Audience hardcoded to "argoclaw" — binds tokens to this service.
			// Provisioning API shares the same JWT secret and validates the
			// same audience, preventing cross-service token misuse.
			Audience: jwt.ClaimStrings{"argoclaw"},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return token.SignedString([]byte(secret))
}

// ValidateAccessToken parses and validates a JWT access token.
func ValidateAccessToken(tokenString, secret string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &argoClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	}, jwt.WithAudience("argoclaw"))
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*argoClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return &claims.TokenClaims, nil
}

// GenerateRefreshToken creates a cryptographically random refresh token.
// Returns the raw token (to send to client) and its SHA-256 hash (to store in DB).
func GenerateRefreshToken() (raw string, hash string, err error) {
	b := make([]byte, RefreshTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}
	rawToken := hex.EncodeToString(b)
	h := sha256.Sum256([]byte(rawToken))
	return rawToken, hex.EncodeToString(h[:]), nil
}

// HashRefreshToken returns the SHA-256 hash of a raw refresh token.
func HashRefreshToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}
