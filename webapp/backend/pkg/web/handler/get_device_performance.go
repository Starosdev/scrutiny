package handler

import (
	"net/http"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/validation"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// GetDevicePerformance retrieves historical performance benchmark data for a device
func GetDevicePerformance(c *gin.Context) {
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)
	logger := c.MustGet("LOGGER").(*logrus.Entry)

	wwn := c.Param("wwn")
	if err := validation.ValidateWWN(wwn); err != nil {
		logger.Warnf("Invalid WWN format: %s", wwn)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	durationKey := c.DefaultQuery("duration", "week")

	history, err := deviceRepo.GetPerformanceHistory(c, wwn, durationKey)
	if err != nil {
		logger.Errorln("An error occurred while retrieving performance history", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	baseline, err := deviceRepo.GetPerformanceBaseline(c, wwn, 5)
	if err != nil {
		logger.Warnf("Could not retrieve performance baseline for %s: %v", wwn, err)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"history":  history,
			"baseline": baseline,
		},
	})
}
