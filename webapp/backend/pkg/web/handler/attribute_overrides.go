package handler

import (
	"net/http"
	"strconv"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// validProtocols defines the allowed protocol values
var validProtocols = map[string]bool{
	"ATA":  true,
	"NVMe": true,
	"SCSI": true,
}

// validActions defines the allowed action values
var validActions = map[string]bool{
	"":             true, // empty means custom thresholds only
	"ignore":       true,
	"force_status": true,
}

// validStatuses defines the allowed status values for force_status action
var validStatuses = map[string]bool{
	"passed": true,
	"warn":   true,
	"failed": true,
}

// GetAttributeOverrides retrieves all attribute overrides from the database
func GetAttributeOverrides(c *gin.Context) {
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)

	overrides, err := deviceRepo.GetAttributeOverrides(c)
	if err != nil {
		logger.Errorln("Error retrieving attribute overrides:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to retrieve overrides"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    overrides,
	})
}

// SaveAttributeOverride creates or updates an attribute override
func SaveAttributeOverride(c *gin.Context) {
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)

	var override models.AttributeOverride
	if err := c.BindJSON(&override); err != nil {
		logger.Errorln("Cannot parse attribute override:", err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid override data"})
		return
	}

	// Validate required fields
	if override.Protocol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Protocol is required"})
		return
	}
	if override.AttributeId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "AttributeId is required"})
		return
	}

	// Validate protocol
	if !validProtocols[override.Protocol] {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid protocol. Must be ATA, NVMe, or SCSI"})
		return
	}

	// Validate action
	if !validActions[override.Action] {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid action. Must be empty, 'ignore', or 'force_status'"})
		return
	}

	// Validate status if force_status action is used
	if override.Action == "force_status" {
		if override.Status == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Status is required when action is 'force_status'"})
			return
		}
		if !validStatuses[override.Status] {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid status. Must be 'passed', 'warn', or 'failed'"})
			return
		}
	}

	// Source is always "ui" for API-created overrides
	override.Source = "ui"

	if err := deviceRepo.SaveAttributeOverride(c, &override); err != nil {
		logger.Errorln("Error saving attribute override:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to save override"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": override})
}

// DeleteAttributeOverride removes an attribute override by ID
func DeleteAttributeOverride(c *gin.Context) {
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid ID format"})
		return
	}

	if err := deviceRepo.DeleteAttributeOverride(c, uint(id)); err != nil {
		logger.Errorln("Error deleting attribute override:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to delete override"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
