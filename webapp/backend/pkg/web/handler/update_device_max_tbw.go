package handler

import (
	"net/http"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type maxTBWRequest struct {
	MaxTBW float64 `json:"max_tbw"`
}

func UpdateDeviceMaxTBW(c *gin.Context) {
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)

	device, err := ResolveDevice(c, logger, deviceRepo)
	if err != nil {
		return
	}

	var req maxTBWRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Warnf("Invalid request body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid request body"})
		return
	}
	if req.MaxTBW <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "max_tbw must be > 0"})
		return
	}

	if err := deviceRepo.UpdateDeviceMaxTBW(c, device.DeviceID, req.MaxTBW); err != nil {
		logger.Errorln("An error occurred while updating device max TBW", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
