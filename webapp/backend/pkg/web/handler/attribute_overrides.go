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

	// Recalculate device status for affected devices
	recalculateDeviceStatusForOverride(c, logger, deviceRepo, &override)

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

	// Fetch override before deletion to know which devices to recalculate
	override, err := deviceRepo.GetAttributeOverrideByID(c, uint(id))
	if err != nil {
		logger.Warnf("Could not fetch override before deletion: %v", err)
		// Continue with deletion even if we can't fetch it
	}

	if err := deviceRepo.DeleteAttributeOverride(c, uint(id)); err != nil {
		logger.Errorln("Error deleting attribute override:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to delete override"})
		return
	}

	// Recalculate device status for affected devices (if we were able to fetch the override)
	if override != nil {
		recalculateDeviceStatusForOverride(c, logger, deviceRepo, override)
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// recalculateDeviceStatusForOverride triggers device status recalculation for devices
// affected by an attribute override change.
func recalculateDeviceStatusForOverride(c *gin.Context, logger *logrus.Entry, deviceRepo database.DeviceRepo, override *models.AttributeOverride) {
	if override.WWN != "" {
		// Override applies to specific device
		if err := deviceRepo.RecalculateDeviceStatusFromHistory(c, override.WWN); err != nil {
			logger.Warnf("Failed to recalculate status for device %s: %v", override.WWN, err)
		}
	} else {
		// Override applies to all devices of this protocol - recalculate all
		devices, err := deviceRepo.GetDevices(c)
		if err != nil {
			logger.Warnf("Failed to get devices for status recalculation: %v", err)
			return
		}
		for _, device := range devices {
			if device.DeviceProtocol == override.Protocol {
				if err := deviceRepo.RecalculateDeviceStatusFromHistory(c, device.WWN); err != nil {
					logger.Warnf("Failed to recalculate status for device %s: %v", device.WWN, err)
				}
			}
		}
	}
}
