package handler

import (
	"net/http"

	"github.com/analogj/scrutiny/webapp/backend/pkg/auth"
	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

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
		logger.Warn("Failed login attempt (token)")
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Invalid token.",
		})
		return
	}

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
		logger.Warn("Failed login attempt (password)")
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Invalid username or password.",
		})
		return
	}

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
