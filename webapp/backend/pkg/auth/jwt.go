package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims represents the JWT claims for a Scrutiny session token.
// TokenType distinguishes between different token purposes (e.g., "session" for web UI).
type Claims struct {
	jwt.RegisteredClaims
	TokenType string `json:"token_type"`
}

// GenerateRandomSecret creates a cryptographically secure 32-byte random secret,
// returned as a 64-character hex string. Used when no JWT secret is configured,
// though tokens signed with a random secret will not survive server restarts.
func GenerateRandomSecret() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random secret: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// GenerateJWT creates a signed JWT token using HS256 with the given secret and expiry.
// Returns the signed token string, the expiration time, and any error.
func GenerateJWT(secret string, expiryHours int) (string, time.Time, error) {
	expiresAt := time.Now().Add(time.Duration(expiryHours) * time.Hour)

	now := time.Now()
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "scrutiny",
		},
		TokenType: "session",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign JWT: %w", err)
	}

	return tokenString, expiresAt, nil
}

// ValidateJWT parses and validates a JWT token string against the given secret.
// Returns the decoded claims if the token is valid, or an error describing why
// validation failed (expired, wrong secret, malformed, etc.).
func ValidateJWT(tokenString string, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Ensure the signing method is HMAC to prevent algorithm-switching attacks
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}
