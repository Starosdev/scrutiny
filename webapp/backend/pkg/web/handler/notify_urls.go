package handler

import (
	"net/http"
	"strconv"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/notify"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type notifyUrlResponse struct {
	ID     uint   `json:"id,omitempty"`
	URL    string `json:"url"`
	Label  string `json:"label,omitempty"`
	Source string `json:"source"`
}

// GetNotifyUrls returns a merged list of notification URLs from all sources.
// Config/env URLs are read-only (source: "config"), DB URLs are editable (source: "ui").
// All URLs have credentials masked.
func GetNotifyUrls(c *gin.Context) {
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)
	appConfig := c.MustGet("CONFIG").(config.Interface)

	var result []notifyUrlResponse

	// Load config/env URLs from Viper (read-only, not stored in DB)
	configUrls := appConfig.GetStringSlice("notify.urls")
	for _, u := range configUrls {
		result = append(result, notifyUrlResponse{
			URL:    notify.MaskNotifyUrl(u),
			Source: "config",
		})
	}

	// Load UI URLs from database
	dbUrls, err := deviceRepo.GetNotifyUrls(c)
	if err != nil {
		logger.Errorln("Error retrieving notification URLs:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to retrieve notification URLs"})
		return
	}
	for _, u := range dbUrls {
		result = append(result, notifyUrlResponse{
			ID:     u.ID,
			URL:    notify.MaskNotifyUrl(u.URL),
			Label:  u.Label,
			Source: u.Source,
		})
	}

	if result == nil {
		result = []notifyUrlResponse{}
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

// SaveNotifyUrl creates a new UI-sourced notification URL
func SaveNotifyUrl(c *gin.Context) {
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)

	var input struct {
		URL   string `json:"url"`
		Label string `json:"label"`
	}
	if err := c.BindJSON(&input); err != nil {
		logger.Errorln("Cannot parse notification URL:", err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request body"})
		return
	}

	if input.URL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "URL is required"})
		return
	}

	entry := &models.NotifyUrl{
		URL:   input.URL,
		Label: input.Label,
	}

	if err := deviceRepo.SaveNotifyUrl(c, entry); err != nil {
		logger.Errorln("Error saving notification URL:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to save notification URL"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": notifyUrlResponse{
			ID:     entry.ID,
			URL:    notify.MaskNotifyUrl(entry.URL),
			Label:  entry.Label,
			Source: entry.Source,
		},
	})
}

// DeleteNotifyUrl removes a UI-sourced notification URL by ID
func DeleteNotifyUrl(c *gin.Context) {
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid ID format"})
		return
	}

	if err := deviceRepo.DeleteNotifyUrl(c, uint(id)); err != nil {
		logger.Errorln("Error deleting notification URL:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to delete notification URL"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// TestNotifyUrl sends a test notification to a single specific URL by its database ID.
// Uses SendToUrls to bypass the config URL loading and only send to the target URL.
func TestNotifyUrl(c *gin.Context) {
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)
	appConfig := c.MustGet("CONFIG").(config.Interface)

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid ID format"})
		return
	}

	// Fetch the URL from DB
	dbUrls, err := deviceRepo.GetNotifyUrls(c)
	if err != nil {
		logger.Errorln("Error retrieving notification URLs for test:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to retrieve notification URL"})
		return
	}

	var targetUrl string
	for _, u := range dbUrls {
		if u.ID == uint(id) {
			targetUrl = u.URL
			break
		}
	}

	if targetUrl == "" {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Notification URL not found"})
		return
	}

	testNotify := notify.New(
		logger,
		appConfig,
		models.Device{
			SerialNumber: "FAKEWDDJ324KSO",
			DeviceType:   pkg.DeviceProtocolAta,
			DeviceName:   "/dev/sda",
		},
		true,
	)

	if err := testNotify.SendToUrls([]string{targetUrl}); err != nil {
		logger.Errorln("Test notification failed:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "errors": []string{err.Error()}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
