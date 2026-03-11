package handler

import (
	"net/http"
	"regexp"
	"strconv"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// wwnPattern matches a valid WWN: optional 0x prefix followed by 1-16 hex digits.
var wwnPattern = regexp.MustCompile(`(?i)^(0x)?[0-9a-f]{1,16}$`)

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

// GetAttributeOverrides retrieves all active attribute overrides for display.
// Includes both UI-created overrides (source: "ui") and config file overrides
// (source: "config"), so users can see everything that is currently active.
func GetAttributeOverrides(c *gin.Context) {
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)

	allOverrides, err := deviceRepo.GetAllOverridesForDisplay(c)
	if err != nil {
		logger.Errorln("Error retrieving attribute overrides:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to retrieve overrides"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    allOverrides,
	})
}

// validateAttributeOverride checks all fields of an override and returns a
// human-readable error string, or an empty string if the override is valid.
func validateAttributeOverride(o *models.AttributeOverride) string {
	if o.Protocol == "" {
		return "Protocol is required"
	}
	if o.AttributeId == "" {
		return "AttributeId is required"
	}
	if !validProtocols[o.Protocol] {
		return "Invalid protocol. Must be ATA, NVMe, or SCSI"
	}
	if !validActions[o.Action] {
		return "Invalid action. Must be empty, 'ignore', or 'force_status'"
	}
	if o.WWN != "" && !wwnPattern.MatchString(o.WWN) {
		return "Invalid WWN format. Must be a hex value (e.g. 0x5000cca264eb01d7)"
	}
	if o.Action == "force_status" {
		return validateForceStatus(o)
	}
	if o.Action == "" {
		return validateThresholds(o)
	}
	return ""
}

func validateForceStatus(o *models.AttributeOverride) string {
	if o.Status == "" {
		return "Status is required when action is 'force_status'"
	}
	if !validStatuses[o.Status] {
		return "Invalid status. Must be 'passed', 'warn', or 'failed'"
	}
	return ""
}

func validateThresholds(o *models.AttributeOverride) string {
	if o.WarnAbove == nil && o.FailAbove == nil {
		return "At least one of warn_above or fail_above is required for custom threshold overrides"
	}
	if o.WarnAbove != nil && *o.WarnAbove < 0 {
		return "warn_above must be a non-negative value"
	}
	if o.FailAbove != nil && *o.FailAbove < 0 {
		return "fail_above must be a non-negative value"
	}
	if o.WarnAbove != nil && o.FailAbove != nil && *o.WarnAbove >= *o.FailAbove {
		return "warn_above must be less than fail_above"
	}
	return ""
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

	if errMsg := validateAttributeOverride(&override); errMsg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": errMsg})
		return
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
	// Get all devices to find the ones affected by this override
	devices, err := deviceRepo.GetDevices(c)
	if err != nil {
		logger.Warnf("Failed to get devices for status recalculation: %v", err)
		return
	}
	for i := range devices {
		device := &devices[i]
		if override.WWN != "" {
			// Override applies to specific device - match by WWN
			if device.WWN != override.WWN {
				continue
			}
		} else if device.DeviceProtocol != override.Protocol {
			// Override applies to all devices of this protocol
			continue
		}
		if err := deviceRepo.RecalculateDeviceStatusFromHistory(c, device.DeviceID); err != nil {
			logger.Warnf("Failed to recalculate status for device %s: %v", device.DeviceID, err)
		}
	}
}
