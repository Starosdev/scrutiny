package handler

import (
	"net/http"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type missedPingTimeoutRequest struct {
	MissedPingTimeoutOverride int `json:"missed_ping_timeout_override"`
}

func UpdateDeviceMissedPingTimeout(c *gin.Context) {
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)

	device, err := ResolveDevice(c, logger, deviceRepo)
	if err != nil {
		return
	}

	var req missedPingTimeoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Warnf("Invalid request body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid request body"})
		return
	}

	if req.MissedPingTimeoutOverride < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "timeout must be >= 0"})
		return
	}

	err = deviceRepo.UpdateDeviceMissedPingTimeout(c, device.WWN, req.MissedPingTimeoutOverride)
	if err != nil {
		logger.Errorln("An error occurred while updating device missed ping timeout", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
