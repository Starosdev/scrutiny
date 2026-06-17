package handler

import (
	"net/http"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// GetDeviceSelfTests retrieves ATA SMART self-test history for a device.
func GetDeviceSelfTests(c *gin.Context) {
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)
	logger := c.MustGet("LOGGER").(*logrus.Entry)

	device, err := ResolveDevice(c, logger, deviceRepo)
	if err != nil {
		return
	}

	selfTests, err := deviceRepo.GetDeviceSelfTests(c, device.DeviceID)
	if err != nil {
		logger.Errorln("An error occurred while retrieving device self-test history", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"self_tests": selfTests,
		},
	})
}
