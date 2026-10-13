package middleware

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/analogj/scrutiny/webapp/backend/pkg/auth"
	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
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
	"/api/version",
}

// versionPathSuffix is the public route that authenticates opportunistically.
const versionPathSuffix = "/api/version"

// docsPathSuffixes lists the served documentation routes.
var docsPathSuffixes = []string{
	"/docs/api",
	"/docs/api/",
	"/api/docs/openapi.yaml",
}

// metricsPathSuffix is the path suffix for the Prometheus metrics endpoint,
// which supports an independent authentication token (web.metrics.token).
const metricsPathSuffix = "/api/metrics"

// configKeyJWTSecret is the config key for the JWT signing secret.
const configKeyJWTSecret = "web.auth.jwt_secret" //nolint:gosec

// authScopeKey is the gin context key holding the scope of the authenticated caller.
const authScopeKey = "AUTH_SCOPE"

// apiTokenStore is the slice of the device repository the scoped-token path needs.
type apiTokenStore interface {
	GetActiveAPITokenByHash(ctx context.Context, tokenHash string) (*models.ApiToken, error)
	TouchAPIToken(ctx context.Context, id uint) error
}

// authContext holds pre-computed auth configuration to avoid repeated config lookups.
type authContext struct {
	configuredToken string
	jwtSecret       string
	metricsToken    string
	authEnabled     bool
	docsPublic      bool
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

func isDocsPath(requestPath string) bool {
	for _, suffix := range docsPathSuffixes {
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
		c.Set(authScopeKey, models.APITokenScopeAdmin)
		return true
	}

	if ac.jwtSecret != "" {
		claims, err := auth.ValidateJWT(tokenString, ac.jwtSecret)
		if err == nil {
			c.Set("AUTH_TYPE", "jwt")
			c.Set("AUTH_CLAIMS", claims)
			c.Set(authScopeKey, models.APITokenScopeAdmin)
			return true
		}
	}

	return ac.validateScopedToken(c, tokenString)
}

// validateScopedToken accepts a named token created through /api/settings/tokens. Only the SHA-256
// hash is stored, so the lookup key is the hash of the presented token. The stored hash is compared
// in constant time again after the lookup. A failed lookup is a plain miss, never an error to the
// client, so revoked, expired, and unknown tokens are indistinguishable.
func (ac *authContext) validateScopedToken(c *gin.Context, tokenString string) bool {
	repo, exists := c.Get("DEVICE_REPOSITORY")
	if !exists {
		return false
	}
	store, ok := repo.(apiTokenStore)
	if !ok {
		return false
	}
	hash := auth.HashToken(tokenString)
	token, err := store.GetActiveAPITokenByHash(c, hash)
	if err != nil || token == nil || subtle.ConstantTimeCompare([]byte(token.TokenHash), []byte(hash)) != 1 {
		return false
	}
	if !models.IsValidAPITokenScope(token.Scope) {
		return false
	}
	// last_used_at is an audit aid; failing to write it must not fail the request.
	_ = store.TouchAPIToken(c, token.ID)
	c.Set("AUTH_TYPE", "scoped_token")
	c.Set(authScopeKey, token.Scope)
	c.Set("AUTH_TOKEN_ID", token.ID)
	return true
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
	logger.Warn("No web.auth.jwt_secret configured and none could be persisted -- generated a per-process secret. JWT tokens will not survive server restarts. Set web.auth.jwt_secret in scrutiny.yaml or make web.secrets.dir writable.")
	return generated
}

// logAuthStatus logs the current authentication configuration at startup.
func logAuthStatus(logger *logrus.Entry, ac *authContext) {
	if ac.authEnabled {
		logger.Info("API authentication is enabled")
	} else {
		logger.Warn("API authentication is disabled (all endpoints are open)")
	}
	if ac.metricsToken != "" {
		logger.Info("Metrics endpoint authentication is enabled (web.metrics.token)")
	}
}

// AuthMiddleware validates authentication tokens on protected routes.
// Authentication is enabled by default. When it is explicitly disabled
// (web.auth.enabled=false), this middleware is a no-op and all requests pass through.
//
// When auth is enabled, it validates Bearer tokens from the Authorization header.
// The token can be either:
//   - The master API token from config (web.auth.token) -- used by collectors
//   - A valid JWT session token -- used by future web UI sessions
//   - A named scoped token (read, collector, admin) created via /api/settings/tokens
//   - A dedicated metrics token (web.metrics.token) -- only for /api/metrics
//
// The metrics endpoint supports an independent token (web.metrics.token) that
// works regardless of whether general auth is enabled. This allows securing
// Prometheus scraping without enabling full API authentication.
//
// Context values set by this middleware:
//   - "AUTH_ENABLED" (bool): whether auth is enabled
//   - "AUTH_TYPE" (string): "api_token", "jwt", "scoped_token", or "metrics_token" (only set on successful auth)
//   - "AUTH_SCOPE" (string): "admin" for the master token, JWTs, and auth-disabled mode; the token's
//     scope for scoped tokens; "read" for the metrics token. Route groups enforce it with RequireScope.
//   - "AUTH_CLAIMS" (*auth.Claims): JWT claims (only set for JWT auth)
func AuthMiddleware(appConfig config.Interface, logger *logrus.Entry) gin.HandlerFunc {
	jwtSecret := initJWTSecret(auth.IsAuthEnabled(appConfig), appConfig.GetString(configKeyJWTSecret), logger)
	// Write the effective secret back so the login handler (which reads config directly) uses the same value.
	if jwtSecret != "" && appConfig.GetString(configKeyJWTSecret) == "" {
		appConfig.Set(configKeyJWTSecret, jwtSecret)
	}

	ac := &authContext{
		authEnabled:     auth.IsAuthEnabled(appConfig),
		configuredToken: appConfig.GetString("web.auth.token"),
		jwtSecret:       jwtSecret,
		metricsToken:    appConfig.GetString("web.metrics.token"),
		docsPublic:      appConfig.GetBool("web.docs.public"),
	}

	logAuthStatus(logger, ac)

	return func(c *gin.Context) {
		c.Set("AUTH_ENABLED", ac.authEnabled)
		ac.authorize(c)
	}
}

// authorize applies the request-path-based authentication rules for a single request.
func (ac *authContext) authorize(c *gin.Context) {
	requestPath := c.Request.URL.Path

	if !ac.authEnabled {
		ac.handleAuthDisabled(c, requestPath)
		return
	}

	// Served docs can be public or private independently of the broader auth toggle.
	if isDocsPath(requestPath) {
		ac.handleDocsPath(c)
		return
	}

	// Non-API routes (frontend static files, SPA routes) and public API routes bypass auth.
	// Only authenticated /api/* paths are subject to authentication.
	if !strings.Contains(requestPath, "/api/") || isPublicPath(requestPath) {
		// /api/version shows more to an authenticated caller. Try to authenticate without rejecting:
		// a missing or bad token just leaves AUTH_SCOPE unset.
		if strings.HasSuffix(requestPath, versionPathSuffix) {
			ac.validateGeneralAuth(c)
		}
		c.Next()
		return
	}

	// Dedicated metrics token: accepted as an alternative for /api/metrics.
	if isMetricsPath(requestPath) && ac.validateMetricsToken(c) {
		c.Set("AUTH_TYPE", "metrics_token")
		c.Set(authScopeKey, models.APITokenScopeRead)
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

// handleAuthDisabled handles requests when general auth is disabled: only the metrics endpoint may
// still require its own token.
func (ac *authContext) handleAuthDisabled(c *gin.Context, requestPath string) {
	if ac.metricsToken != "" && isMetricsPath(requestPath) && !ac.validateMetricsToken(c) {
		rejectUnauthorized(c, "Metrics endpoint requires authentication. Provide a Bearer token.")
		return
	}
	// Auth is off: every request is effectively admin, so scope checks pass.
	c.Set(authScopeKey, models.APITokenScopeAdmin)
	c.Next()
}

// handleDocsPath authorizes the served docs path, which may be public or require general auth.
func (ac *authContext) handleDocsPath(c *gin.Context) {
	if ac.docsPublic || ac.validateGeneralAuth(c) {
		c.Next()
		return
	}
	rejectMissingOrInvalid(c)
}
