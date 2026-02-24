package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

// --- ExtractBearerToken tests ---

func TestExtractBearerToken_Valid(t *testing.T) {
	token := ExtractBearerToken("Bearer mytoken123")
	require.Equal(t, "mytoken123", token)
}

func TestExtractBearerToken_CaseInsensitive(t *testing.T) {
	token := ExtractBearerToken("bearer mytoken123")
	require.Equal(t, "mytoken123", token)

	token = ExtractBearerToken("BEARER mytoken123")
	require.Equal(t, "mytoken123", token)
}

func TestExtractBearerToken_Empty(t *testing.T) {
	token := ExtractBearerToken("")
	require.Equal(t, "", token)
}

func TestExtractBearerToken_NoBearer(t *testing.T) {
	// "Basic" auth scheme should not be accepted
	token := ExtractBearerToken("Basic abc123")
	require.Equal(t, "", token)
}

func TestExtractBearerToken_MissingToken(t *testing.T) {
	// "Bearer" with no token value
	token := ExtractBearerToken("Bearer")
	require.Equal(t, "", token)
}

func TestExtractBearerToken_ExtraSpaces(t *testing.T) {
	token := ExtractBearerToken("Bearer  mytoken123 ")
	require.Equal(t, "mytoken123", token)
}

// --- ValidateAPIToken tests ---

func TestValidateAPIToken_Match(t *testing.T) {
	require.True(t, ValidateAPIToken("secret-token-123", "secret-token-123"))
}

func TestValidateAPIToken_Mismatch(t *testing.T) {
	require.False(t, ValidateAPIToken("wrong-token", "secret-token-123"))
}

func TestValidateAPIToken_EmptyProvided(t *testing.T) {
	require.False(t, ValidateAPIToken("", "secret-token-123"))
}

func TestValidateAPIToken_EmptyConfigured(t *testing.T) {
	require.False(t, ValidateAPIToken("secret-token-123", ""))
}

func TestValidateAPIToken_BothEmpty(t *testing.T) {
	require.False(t, ValidateAPIToken("", ""))
}

// --- HashToken tests ---

func TestHashToken_Deterministic(t *testing.T) {
	hash1 := HashToken("my-secret-token")
	hash2 := HashToken("my-secret-token")
	require.Equal(t, hash1, hash2)
}

func TestHashToken_DifferentInputs(t *testing.T) {
	hash1 := HashToken("token-a")
	hash2 := HashToken("token-b")
	require.NotEqual(t, hash1, hash2)
}

func TestHashToken_Length(t *testing.T) {
	// SHA-256 produces 64 hex characters
	hash := HashToken("any-token")
	require.Len(t, hash, 64)
}

// --- GenerateRandomSecret tests ---

func TestGenerateRandomSecret_Length(t *testing.T) {
	secret, err := GenerateRandomSecret()
	require.NoError(t, err)
	// 32 bytes = 64 hex characters
	require.Len(t, secret, 64)
}

func TestGenerateRandomSecret_Uniqueness(t *testing.T) {
	secret1, err := GenerateRandomSecret()
	require.NoError(t, err)
	secret2, err := GenerateRandomSecret()
	require.NoError(t, err)
	require.NotEqual(t, secret1, secret2)
}

// --- JWT round-trip tests ---

func TestGenerateAndValidateJWT_RoundTrip(t *testing.T) {
	secret := "test-secret-key-for-jwt-signing"
	tokenString, expiresAt, err := GenerateJWT(secret, 24)
	require.NoError(t, err)
	require.NotEmpty(t, tokenString)
	require.True(t, expiresAt.After(time.Now()))

	claims, err := ValidateJWT(tokenString, secret)
	require.NoError(t, err)
	require.Equal(t, "session", claims.TokenType)
	require.Equal(t, "scrutiny", claims.Issuer)
}

func TestValidateJWT_WrongSecret(t *testing.T) {
	tokenString, _, err := GenerateJWT("correct-secret", 24)
	require.NoError(t, err)

	_, err = ValidateJWT(tokenString, "wrong-secret")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid token")
}

func TestValidateJWT_ExpiredToken(t *testing.T) {
	// Create a token that expired 1 hour ago
	secret := "test-secret"
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			Issuer:    "scrutiny",
		},
		TokenType: "session",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	require.NoError(t, err)

	_, err = ValidateJWT(tokenString, secret)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid token")
}

func TestValidateJWT_MalformedToken(t *testing.T) {
	_, err := ValidateJWT("not-a-valid-jwt", "secret")
	require.Error(t, err)
}

func TestValidateJWT_EmptyToken(t *testing.T) {
	_, err := ValidateJWT("", "secret")
	require.Error(t, err)
}
