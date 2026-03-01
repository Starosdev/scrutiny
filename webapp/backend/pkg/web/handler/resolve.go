package handler

import (
	"fmt"
	"net/http"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/validation"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// ResolveDevice accepts either a device_id (UUID format) or a legacy WWN from
// the :id route parameter. It returns the full device record for use by handlers.
// This enables backward compatibility: old collectors and bookmarks using WWN
// continue to work alongside the new device_id-based routes.
func ResolveDevice(c *gin.Context, logger *logrus.Entry, deviceRepo database.DeviceRepo) (models.Device, error) {
	id := c.Param("id")
	if err := validation.ValidateDeviceIdentifier(id); err != nil {
		logger.Warnf("Invalid device identifier format: %s", id)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": fmt.Sprintf("invalid device identifier: %s", err.Error())})
		return models.Device{}, err
	}

	// Try device_id lookup first (primary key)
	device, err := deviceRepo.GetDeviceDetails(c, id)
	if err == nil {
		return device, nil
	}

	// Legacy WWN fallback for backward compatibility with old collectors/bookmarks
	device, wwnErr := deviceRepo.GetDeviceByWWN(c, id)
	if wwnErr == nil {
		logger.Warnf("DEPRECATED: Device lookup by WWN (%s). Use device_id (%s) instead.", id, device.DeviceID)
		return device, nil
	}

	logger.Warnf("Device not found for identifier: %s", id)
	c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "device not found"})
	return models.Device{}, fmt.Errorf("device not found: %s", id)
}
