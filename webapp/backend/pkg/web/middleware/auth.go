package middleware

import (
	"net/http"
	"strings"

	"github.com/analogj/scrutiny/webapp/backend/pkg/auth"
	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// publicPathSuffixes lists API path suffixes that never require authentication.
// Health checks must remain open for load balancers and monitoring tools.
// Auth endpoints must be open so clients can check auth status and log in.
var publicPathSuffixes = []string{
	"/api/health",
	"/api/auth/status",
	"/api/auth/login",
}

// AuthMiddleware validates authentication tokens on protected routes.
// When auth is disabled (web.auth.enabled=false), this middleware is a no-op
// and all requests pass through -- preserving backward compatibility.
//
// When auth is enabled, it validates Bearer tokens from the Authorization header.
// The token can be either:
//   - The master API token from config (web.auth.token) -- used by collectors
//   - A valid JWT session token -- used by future web UI sessions
//
// Context values set by this middleware:
//   - "AUTH_ENABLED" (bool): whether auth is enabled
//   - "AUTH_TYPE" (string): "api_token" or "jwt" (only set on successful auth)
//   - "AUTH_CLAIMS" (*auth.Claims): JWT claims (only set for JWT auth)
func AuthMiddleware(appConfig config.Interface, logger *logrus.Entry) gin.HandlerFunc {
	authEnabled := auth.IsAuthEnabled(appConfig)
	configuredToken := appConfig.GetString("web.auth.token")
	jwtSecret := appConfig.GetString("web.auth.jwt_secret")

	// If no JWT secret is configured, generate a random one.
	// Tokens signed with this secret will not survive server restarts.
	if authEnabled && jwtSecret == "" {
		generated, err := auth.GenerateRandomSecret()
		if err != nil {
			logger.Warnf("Failed to generate JWT secret, JWT auth will not work: %v", err)
		} else {
			jwtSecret = generated
			logger.Warn("No web.auth.jwt_secret configured -- generated a random secret. JWT tokens will not survive server restarts. Set web.auth.jwt_secret in scrutiny.yaml for persistent sessions.")
		}
	}

	if authEnabled {
		logger.Info("API authentication is enabled")
	} else {
		logger.Info("API authentication is disabled (all endpoints are open)")
	}

	return func(c *gin.Context) {
		c.Set("AUTH_ENABLED", authEnabled)

		if !authEnabled {
			c.Next()
			return
		}

		// Public routes are accessible without authentication.
		// Uses c.Request.URL.Path (matching the pattern in logger.go) with
		// HasSuffix to handle basepath variations (e.g., /scrutiny/api/health).
		requestPath := c.Request.URL.Path
		for _, suffix := range publicPathSuffixes {
			if strings.HasSuffix(requestPath, suffix) {
				c.Next()
				return
			}
		}

		// Extract token from Authorization header
		tokenString := auth.ExtractBearerToken(c.GetHeader("Authorization"))
		if tokenString == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "Authentication required. Provide a Bearer token in the Authorization header.",
			})
			return
		}

		// Try API token validation first (most common path for collectors)
		if auth.ValidateAPIToken(tokenString, configuredToken) {
			c.Set("AUTH_TYPE", "api_token")
			c.Next()
			return
		}

		// Try JWT validation (for web UI sessions)
		if jwtSecret != "" {
			claims, err := auth.ValidateJWT(tokenString, jwtSecret)
			if err == nil {
				c.Set("AUTH_TYPE", "jwt")
				c.Set("AUTH_CLAIMS", claims)
				c.Next()
				return
			}
		}

		// Both validation methods failed
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Invalid or expired token.",
		})
	}
}
