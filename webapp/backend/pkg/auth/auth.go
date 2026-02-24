// Package auth provides authentication utilities for the Scrutiny API.
// It supports two authentication mechanisms:
//   - API tokens: simple bearer tokens for collector and programmatic access
//   - JWT sessions: signed tokens for future web UI authentication
//
// Authentication is opt-in and disabled by default for backward compatibility.
package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"

	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
)

// ExtractBearerToken parses the Authorization header value and returns
// the token string. Expected format: "Bearer <token>".
// Returns empty string if the header is missing, empty, or malformed.
func ExtractBearerToken(authHeader string) string {
	if authHeader == "" {
		return ""
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

// ValidateAPIToken checks if the provided token matches the configured master token.
// Both values are SHA-256 hashed before comparison so that:
//   - The comparison is always constant-time (fixed 32-byte length)
//   - Token length is never leaked via timing side channels
func ValidateAPIToken(providedToken string, configuredToken string) bool {
	if providedToken == "" || configuredToken == "" {
		return false
	}
	h1 := sha256.Sum256([]byte(providedToken))
	h2 := sha256.Sum256([]byte(configuredToken))
	return subtle.ConstantTimeCompare(h1[:], h2[:]) == 1
}

// HashToken creates a SHA-256 hash of a token for safe database storage.
// Tokens should never be stored in plaintext.
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// IsAuthEnabled returns true if authentication is enabled in the application config.
// When false, all endpoints are accessible without authentication (default behavior).
func IsAuthEnabled(appConfig config.Interface) bool {
	return appConfig.GetBool("web.auth.enabled")
}
