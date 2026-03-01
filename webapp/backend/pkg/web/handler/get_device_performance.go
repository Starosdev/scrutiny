package handler

import (
	"net/http"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// GetDevicePerformance retrieves historical performance benchmark data for a device
func GetDevicePerformance(c *gin.Context) {
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)
	logger := c.MustGet("LOGGER").(*logrus.Entry)

	device, err := ResolveDevice(c, logger, deviceRepo)
	if err != nil {
		return
	}

	durationKey := c.DefaultQuery("duration", "week")

	history, err := deviceRepo.GetPerformanceHistory(c, device.WWN, durationKey)
	if err != nil {
		logger.Errorln("An error occurred while retrieving performance history", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	baseline, err := deviceRepo.GetPerformanceBaseline(c, device.WWN, 5)
	if err != nil {
		logger.Warnf("Could not retrieve performance baseline for %s: %v", device.WWN, err)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"history":  history,
			"baseline": baseline,
		},
	})
}
