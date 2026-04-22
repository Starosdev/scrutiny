package handler

import (
	"net/http"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// GetMdadmArrayDetails returns metadata and historical metrics for a specific MDADM array
func GetMdadmArrayDetails(c *gin.Context) {
	dbRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	uuid := c.Param("uuid")
		durationKey := c.DefaultQuery("duration", "week")

		array, err := dbRepo.GetMdadmArrayDetails(c.Request.Context(), uuid)
		if err != nil {
			logger.Errorf("Failed to get MDADM array details for %s: %v", uuid, err)
			c.JSON(http.StatusNotFound, gin.H{"success": false, "errors": []string{"Array not found"}})
			return
		}

		history, err := dbRepo.GetMdadmMetricsHistory(c.Request.Context(), uuid, durationKey)
		if err != nil {
			logger.Errorf("Failed to get MDADM metrics history for %s: %v", uuid, err)
			// Continue with empty history
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"array":   array,
				"history": history,
			},
		})
	}
