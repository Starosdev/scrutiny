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

// metricsPathSuffix is the path suffix for the Prometheus metrics endpoint,
// which supports an independent authentication token (web.metrics.token).
const metricsPathSuffix = "/api/metrics"

// authContext holds pre-computed auth configuration to avoid repeated config lookups.
type authContext struct {
	authEnabled     bool
	configuredToken string
	jwtSecret       string
	metricsToken    string
}

// isPublicPath returns true if the request path matches a public route suffix.
func isPublicPath(requestPath string) bool {
	for _, suffix := range publicPathSuffixes {
		if strings.HasSuffix(requestPath, suffix) {
			return true
		}
	}
	return false
}

// isMetricsPath returns true if the request path is the Prometheus metrics endpoint.
func isMetricsPath(requestPath string) bool {
	return strings.HasSuffix(requestPath, metricsPathSuffix)
}

// validateMetricsToken checks if the request carries a valid dedicated metrics token.
// Returns true if the metrics token is configured and the request token matches.
func (ac *authContext) validateMetricsToken(c *gin.Context) bool {
	if ac.metricsToken == "" {
		return false
	}
	tokenString := auth.ExtractBearerToken(c.GetHeader("Authorization"))
	return auth.ValidateAPIToken(tokenString, ac.metricsToken)
}

// validateGeneralAuth tries API token and JWT validation in order.
// Returns true and sets context values if authentication succeeds.
func (ac *authContext) validateGeneralAuth(c *gin.Context) bool {
	tokenString := auth.ExtractBearerToken(c.GetHeader("Authorization"))
	if tokenString == "" {
		return false
	}

	if auth.ValidateAPIToken(tokenString, ac.configuredToken) {
		c.Set("AUTH_TYPE", "api_token")
		return true
	}

	if ac.jwtSecret != "" {
		claims, err := auth.ValidateJWT(tokenString, ac.jwtSecret)
		if err == nil {
			c.Set("AUTH_TYPE", "jwt")
			c.Set("AUTH_CLAIMS", claims)
			return true
		}
	}

	return false
}

// rejectUnauthorized sends a 401 response with the given error message.
func rejectUnauthorized(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"success": false,
		"error":   message,
	})
}

// rejectMissingOrInvalid sends a 401 with a message appropriate to whether
// the request carries a Bearer token at all or an invalid one.
func rejectMissingOrInvalid(c *gin.Context) {
	if auth.ExtractBearerToken(c.GetHeader("Authorization")) == "" {
		rejectUnauthorized(c, "Authentication required. Provide a Bearer token in the Authorization header.")
	} else {
		rejectUnauthorized(c, "Invalid or expired token.")
	}
}

// initJWTSecret generates a random JWT secret if auth is enabled but no secret is configured.
func initJWTSecret(authEnabled bool, jwtSecret string, logger *logrus.Entry) string {
	if !authEnabled || jwtSecret != "" {
		return jwtSecret
	}
	generated, err := auth.GenerateRandomSecret()
	if err != nil {
		logger.Warnf("Failed to generate JWT secret, JWT auth will not work: %v", err)
		return ""
	}
	logger.Warn("No web.auth.jwt_secret configured -- generated a random secret. JWT tokens will not survive server restarts. Set web.auth.jwt_secret in scrutiny.yaml for persistent sessions.")
	return generated
}

// logAuthStatus logs the current authentication configuration at startup.
func logAuthStatus(logger *logrus.Entry, ac *authContext) {
	if ac.authEnabled {
		logger.Info("API authentication is enabled")
	} else {
		logger.Info("API authentication is disabled (all endpoints are open)")
	}
	if ac.metricsToken != "" {
		logger.Info("Metrics endpoint authentication is enabled (web.metrics.token)")
	}
}

// AuthMiddleware validates authentication tokens on protected routes.
// When auth is disabled (web.auth.enabled=false), this middleware is a no-op
// and all requests pass through -- preserving backward compatibility.
//
// When auth is enabled, it validates Bearer tokens from the Authorization header.
// The token can be either:
//   - The master API token from config (web.auth.token) -- used by collectors
//   - A valid JWT session token -- used by future web UI sessions
//   - A dedicated metrics token (web.metrics.token) -- only for /api/metrics
//
// The metrics endpoint supports an independent token (web.metrics.token) that
// works regardless of whether general auth is enabled. This allows securing
// Prometheus scraping without enabling full API authentication.
//
// Context values set by this middleware:
//   - "AUTH_ENABLED" (bool): whether auth is enabled
//   - "AUTH_TYPE" (string): "api_token", "jwt", or "metrics_token" (only set on successful auth)
//   - "AUTH_CLAIMS" (*auth.Claims): JWT claims (only set for JWT auth)
func AuthMiddleware(appConfig config.Interface, logger *logrus.Entry) gin.HandlerFunc {
	ac := &authContext{
		authEnabled:     auth.IsAuthEnabled(appConfig),
		configuredToken: appConfig.GetString("web.auth.token"),
		jwtSecret:       initJWTSecret(auth.IsAuthEnabled(appConfig), appConfig.GetString("web.auth.jwt_secret"), logger),
		metricsToken:    appConfig.GetString("web.metrics.token"),
	}

	logAuthStatus(logger, ac)

	return func(c *gin.Context) {
		c.Set("AUTH_ENABLED", ac.authEnabled)
		requestPath := c.Request.URL.Path

		// When general auth is disabled, only metrics may require its own token.
		if !ac.authEnabled {
			if ac.metricsToken != "" && isMetricsPath(requestPath) && !ac.validateMetricsToken(c) {
				rejectUnauthorized(c, "Metrics endpoint requires authentication. Provide a Bearer token.")
				return
			}
			c.Next()
			return
		}

		// Public routes bypass authentication entirely.
		if isPublicPath(requestPath) {
			c.Next()
			return
		}

		// Dedicated metrics token: accepted as an alternative for /api/metrics.
		if isMetricsPath(requestPath) && ac.validateMetricsToken(c) {
			c.Set("AUTH_TYPE", "metrics_token")
			c.Next()
			return
		}

		// General auth: API token or JWT.
		if ac.validateGeneralAuth(c) {
			c.Next()
			return
		}

		rejectMissingOrInvalid(c)
	}
}
