package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/auth"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const (
	// apiTokenPrefix makes issued tokens recognizable in config files and secret scanners.
	apiTokenPrefix     = "scr_"
	apiTokenNameMaxLen = 100
)

type createAPITokenRequest struct {
	ExpiresAt *time.Time `json:"expires_at"`
	Name      string     `json:"name"`
	Scope     string     `json:"scope"`
}

// apiTokenResponse is the safe view of a token. It never carries the secret or its hash.
type apiTokenResponse struct {
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	ExpiresAt  *time.Time `json:"expires_at"`
	Name       string     `json:"name"`
	Scope      string     `json:"scope"`
	ID         uint       `json:"id"`
	Revoked    bool       `json:"revoked"`
}

func toAPITokenResponse(t *models.ApiToken) apiTokenResponse {
	return apiTokenResponse{
		CreatedAt:  t.CreatedAt,
		LastUsedAt: t.LastUsedAt,
		ExpiresAt:  t.ExpiresAt,
		Name:       t.Name,
		Scope:      t.Scope,
		ID:         t.ID,
		Revoked:    t.Revoked,
	}
}

// CreateAPIToken issues a named, scoped token. The plaintext is returned in this response only;
// the database keeps the SHA-256 hash.
func CreateAPIToken(c *gin.Context) {
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)

	var req createAPITokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request body"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > apiTokenNameMaxLen {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "name is required and must be at most 100 characters"})
		return
	}
	if !models.IsValidAPITokenScope(req.Scope) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "scope must be one of: read, collector, admin"})
		return
	}
	if req.ExpiresAt != nil && !req.ExpiresAt.After(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "expires_at must be in the future"})
		return
	}

	secret, err := auth.GenerateRandomSecret()
	if err != nil {
		logger.Errorln("Failed to generate API token:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to generate token"})
		return
	}
	plaintext := apiTokenPrefix + secret

	token := &models.ApiToken{
		Name:      req.Name,
		Scope:     req.Scope,
		ExpiresAt: req.ExpiresAt,
		TokenHash: auth.HashToken(plaintext),
	}
	if err := deviceRepo.CreateAPIToken(c, token); err != nil {
		logger.Errorln("Failed to store API token:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to store token"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success":   true,
		"token":     plaintext,
		"api_token": toAPITokenResponse(token),
	})
}

// ListAPITokens returns token metadata. The secret and its hash are never returned.
func ListAPITokens(c *gin.Context) {
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)

	tokens, err := deviceRepo.ListAPITokens(c)
	if err != nil {
		logger.Errorln("Failed to list API tokens:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to retrieve tokens"})
		return
	}
	result := make([]apiTokenResponse, 0, len(tokens))
	for i := range tokens {
		result = append(result, toAPITokenResponse(&tokens[i]))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "tokens": result})
}

// RevokeAPIToken revokes a token by id. The row is kept (revoked=true) so the audit trail survives.
func RevokeAPIToken(c *gin.Context) {
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": errInvalidIDFormat})
		return
	}
	if err := deviceRepo.RevokeAPIToken(c, uint(id)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Token not found"})
			return
		}
		logger.Errorln("Failed to revoke API token:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to revoke token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
