package handler

import (
	"net/http"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func ArchiveDevice(c *gin.Context) {
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)

	device, err := ResolveDevice(c, logger, deviceRepo)
	if err != nil {
		return
	}

	err = deviceRepo.UpdateDeviceArchived(c, device.DeviceID, true)
	if err != nil {
		logger.Errorln("An error occurred while archiving device", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	// Remove device from Home Assistant when archived
	removeMqttDevice(c, &device)

	c.JSON(http.StatusOK, gin.H{"success": true})
}
