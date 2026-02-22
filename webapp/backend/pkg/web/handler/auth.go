package handler

import (
	"net/http"

	"github.com/analogj/scrutiny/webapp/backend/pkg/auth"
	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// AuthStatus returns the current authentication configuration status.
// This endpoint is always public so the frontend (and other clients) can
// determine whether a login form should be displayed.
//
// Response: {"success": true, "auth_enabled": bool}
func AuthStatus(c *gin.Context) {
	appConfig := c.MustGet("CONFIG").(config.Interface)

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"auth_enabled": auth.IsAuthEnabled(appConfig),
	})
}

// Login validates credentials and returns a JWT session token.
// In Phase 1, the "credential" is the master API token itself -- the client
// sends the API token and receives a JWT in return. In a future phase,
// this could accept username/password credentials.
//
// Request:  {"token": "the-master-api-token"}
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
		Token string `json:"token" binding:"required"`
	}

	if err := c.BindJSON(&loginRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Request must include a 'token' field.",
		})
		return
	}

	// Validate the provided token against the configured master token
	configuredToken := appConfig.GetString("web.auth.token")
	if !auth.ValidateAPIToken(loginRequest.Token, configuredToken) {
		logger.Warn("Failed login attempt")
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Invalid token.",
		})
		return
	}

	// Generate a JWT session token for the authenticated client
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
