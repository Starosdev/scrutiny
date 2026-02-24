package handler

import (
	"net/http"
	"sync"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/auth"
	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// loginRateLimiter tracks failed login attempts per IP to prevent brute-force attacks.
// After maxFailures within the window, subsequent attempts are rejected until the window expires.
var loginLimiter = &rateLimiter{
	failures:    make(map[string]*failureRecord),
	maxFailures: 10,
	window:      5 * time.Minute,
}

type failureRecord struct {
	count    int
	windowStart time.Time
}

type rateLimiter struct {
	mu          sync.Mutex
	failures    map[string]*failureRecord
	maxFailures int
	window      time.Duration
}

func (rl *rateLimiter) isBlocked(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rec, ok := rl.failures[ip]
	if !ok {
		return false
	}
	if time.Since(rec.windowStart) > rl.window {
		delete(rl.failures, ip)
		return false
	}
	return rec.count >= rl.maxFailures
}

func (rl *rateLimiter) recordFailure(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rec, ok := rl.failures[ip]
	if !ok || time.Since(rec.windowStart) > rl.window {
		rl.failures[ip] = &failureRecord{count: 1, windowStart: time.Now()}
		return
	}
	rec.count++
}

func (rl *rateLimiter) reset(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.failures, ip)
}

// loginMethods returns which login methods are available based on config.
// "token" is always available when auth is enabled (web.auth.token is required).
// "password" is available when web.auth.admin_password is configured.
func loginMethods(appConfig config.Interface) []string {
	methods := []string{"token"}
	if appConfig.GetString("web.auth.admin_password") != "" {
		methods = append(methods, "password")
	}
	return methods
}

// AuthStatus returns the current authentication configuration status.
// This endpoint is always public so the frontend (and other clients) can
// determine whether a login form should be displayed and which login
// methods are available.
//
// Response: {"success": true, "auth_enabled": bool, "login_methods": ["token", "password"]}
func AuthStatus(c *gin.Context) {
	appConfig := c.MustGet("CONFIG").(config.Interface)

	authEnabled := auth.IsAuthEnabled(appConfig)

	response := gin.H{
		"success":      true,
		"auth_enabled": authEnabled,
	}

	if authEnabled {
		response["login_methods"] = loginMethods(appConfig)
	}

	c.JSON(http.StatusOK, response)
}

// Login validates credentials and returns a JWT session token.
// Supports two login methods:
//   - Token login:    {"token": "the-master-api-token"}
//   - Password login: {"username": "admin", "password": "the-admin-password"}
//
// Response: {"success": true, "token": "<jwt>", "expires_at": "...", "token_type": "Bearer"}
func Login(c *gin.Context) {
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	appConfig := c.MustGet("CONFIG").(config.Interface)

	// If auth is not enabled, inform the caller -- no login needed
	if !auth.IsAuthEnabled(appConfig) {
		c.JSON(http.StatusOK, gin.H{
			"success":      true,
			"auth_enabled": false,
			"message":      "Authentication is not enabled.",
		})
		return
	}

	// Rate limiting: reject if too many recent failed attempts from this IP
	clientIP := c.ClientIP()
	if loginLimiter.isBlocked(clientIP) {
		logger.Warnf("Login rate limit exceeded for %s", clientIP)
		c.JSON(http.StatusTooManyRequests, gin.H{
			"success": false,
			"error":   "Too many failed login attempts. Please try again later.",
		})
		return
	}

	var loginRequest struct {
		Token    string `json:"token"`
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := c.BindJSON(&loginRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request body.",
		})
		return
	}

	// Route to the appropriate login method
	if loginRequest.Token != "" {
		handleTokenLogin(c, logger, appConfig, loginRequest.Token)
		return
	}

	if loginRequest.Username != "" && loginRequest.Password != "" {
		handlePasswordLogin(c, logger, appConfig, loginRequest.Username, loginRequest.Password)
		return
	}

	c.JSON(http.StatusBadRequest, gin.H{
		"success": false,
		"error":   "Provide either 'token' or 'username'+'password' fields.",
	})
}

// handleTokenLogin validates the master API token and issues a JWT.
func handleTokenLogin(c *gin.Context, logger *logrus.Entry, appConfig config.Interface, token string) {
	configuredToken := appConfig.GetString("web.auth.token")
	if !auth.ValidateAPIToken(token, configuredToken) {
		loginLimiter.recordFailure(c.ClientIP())
		logger.Warn("Failed login attempt (token)")
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Invalid token.",
		})
		return
	}

	loginLimiter.reset(c.ClientIP())
	issueJWT(c, logger, appConfig)
}

// handlePasswordLogin validates admin credentials and issues a JWT.
func handlePasswordLogin(c *gin.Context, logger *logrus.Entry, appConfig config.Interface, username, password string) {
	configuredPassword := appConfig.GetString("web.auth.admin_password")
	if configuredPassword == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Password login is not configured. Set web.auth.admin_password in scrutiny.yaml.",
		})
		return
	}

	configuredUsername := appConfig.GetString("web.auth.admin_username")
	if !auth.ValidateAPIToken(username, configuredUsername) || !auth.ValidateAPIToken(password, configuredPassword) {
		loginLimiter.recordFailure(c.ClientIP())
		logger.Warn("Failed login attempt (password)")
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Invalid username or password.",
		})
		return
	}

	loginLimiter.reset(c.ClientIP())
	issueJWT(c, logger, appConfig)
}

// issueJWT generates a JWT session token and sends it in the response.
func issueJWT(c *gin.Context, logger *logrus.Entry, appConfig config.Interface) {
	jwtSecret := appConfig.GetString("web.auth.jwt_secret")
	expiryHours := appConfig.GetInt("web.auth.jwt_expiry_hours")
	if expiryHours <= 0 {
		expiryHours = 24
	}

	jwtToken, expiresAt, err := auth.GenerateJWT(jwtSecret, expiryHours)
	if err != nil {
		logger.Errorln("Failed to generate JWT:", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to generate session token.",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"token":      jwtToken,
		"expires_at": expiresAt,
		"token_type": "Bearer",
	})
}
