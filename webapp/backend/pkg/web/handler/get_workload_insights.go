package handler

import (
	"net/http"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func GetWorkloadInsights(c *gin.Context) {
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)

	durationKey := c.DefaultQuery("duration_key", "week")

	workload, err := deviceRepo.GetWorkloadInsights(c, durationKey)
	if err != nil {
		logger.Errorln("An error occurred while retrieving workload insights", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": map[string]interface{}{
			"workload": workload,
		},
	})
}
