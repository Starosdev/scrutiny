package handler

import (
	"net/http"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type mergeDeviceIntoRequest struct {
	NewDeviceID string `json:"new_device_id"`
}

func MergeDeviceInto(c *gin.Context) {
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)

	sourceDevice, err := ResolveDevice(c, logger, deviceRepo)
	if err != nil {
		return
	}

	var request mergeDeviceIntoRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		logger.Warnf("Invalid merge request body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid request body"})
		return
	}

	if request.NewDeviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "new_device_id is required"})
		return
	}

	if err := deviceRepo.MergeDevices(c, sourceDevice.DeviceID, request.NewDeviceID); err != nil {
		logger.Errorln("An error occurred while merging devices", err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
