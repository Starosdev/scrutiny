package handler

import (
	"net/http"
	"strconv"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	"github.com/analogj/scrutiny/webapp/backend/pkg/validation"
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
	"acknowledge":  true,
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
	if o.DeviceID != "" && o.WWN != "" {
		return "Choose either device_id or legacy wwn, not both"
	}
	if o.DeviceID != "" && validation.ValidateUUID(o.DeviceID) != nil {
		return "Invalid device_id format"
	}
	if o.WWN != "" && validation.ValidateWWN(o.WWN) != nil {
		return "Invalid WWN format"
	}
	if o.Action != "acknowledge" && o.PinnedValue != nil {
		return "pinned_value is only valid when action is 'acknowledge'"
	}
	switch o.Action {
	case "force_status":
		return validateForceStatus(o)
	case "acknowledge":
		return validateAcknowledge(o)
	case "":
		return validateThresholds(o)
	}
	return ""
}

// validateAcknowledge checks an acknowledge override. Acknowledgement pins a
// status to one device's current value, so a fleet-wide rule cannot express it:
// the same attribute holds a different value on every device.
func validateAcknowledge(o *models.AttributeOverride) string {
	if o.DeviceID == "" && o.WWN == "" {
		return "Acknowledge requires a specific device (device_id or wwn)"
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

	if override.Action == "acknowledge" && override.PinnedValue == nil {
		if errMsg := resolvePinnedValue(c, deviceRepo, &override); errMsg != "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": errMsg})
			return
		}
	}

	if err := deviceRepo.SaveAttributeOverride(c, &override); err != nil {
		logger.Errorln("Error saving attribute override:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to save override"})
		return
	}

	// Recalculate device status for affected devices
	recalculateDeviceStatusForOverride(c, logger, deviceRepo, &override)

	c.JSON(http.StatusOK, gin.H{"success": true, "data": override})
}

// resolvePinnedValue fills in the value an acknowledgement pins to, read from the device's
// latest stored SMART submission. The server resolves it rather than trusting a client-sent
// value because which field an attribute is evaluated against differs by protocol (ATA
// compares the raw value, every other protocol the normalized one) and that rule already
// lives in measurements.AttributeThresholdValue. Restating it in the frontend would let the
// two drift apart silently.
func resolvePinnedValue(c *gin.Context, deviceRepo database.DeviceRepo, override *models.AttributeOverride) string {
	var device models.Device
	var err error
	if override.DeviceID != "" {
		device, err = deviceRepo.GetDeviceByID(c, override.DeviceID)
	} else {
		device, err = deviceRepo.GetDeviceByWWN(c, override.WWN)
	}
	if err != nil {
		return "Device not found for acknowledge override"
	}

	submissions, err := deviceRepo.GetLatestSmartSubmission(c, device.WWN)
	if err != nil || len(submissions) == 0 {
		return "No SMART data available to acknowledge for this device"
	}

	attribute, found := submissions[0].Attributes[override.AttributeId]
	if !found {
		return "Attribute not present in the device's latest SMART data"
	}

	value, ok := measurements.AttributeThresholdValue(attribute)
	if !ok {
		return "Attribute type cannot be acknowledged"
	}
	override.PinnedValue = &value
	return ""
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
		switch {
		case override.DeviceID != "":
			if device.DeviceID != override.DeviceID {
				continue
			}
		case override.WWN != "":
			// Override applies to specific device - match by WWN
			if device.WWN != override.WWN {
				continue
			}
		case device.DeviceProtocol != override.Protocol:
			// Override applies to all devices of this protocol
			continue
		}
		if err := deviceRepo.RecalculateDeviceStatusFromHistory(c, device.DeviceID); err != nil {
			logger.Warnf("Failed to recalculate status for device %s: %v", device.DeviceID, err)
		}
	}
}
