package handler

import (
	"context"
	"net/http"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// MissedPingMonitor interface to avoid import cycle
type MissedPingMonitor interface {
	GetStatus(ctx context.Context) (*models.MissedPingStatusData, error)
}

// GetMissedPingStatus returns the current status of the missed ping monitor for diagnostics
func GetMissedPingStatus(c *gin.Context) {
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	monitor := c.MustGet("MISSED_PING_MONITOR").(MissedPingMonitor)

	logger.Debugf("Retrieving missed ping monitor status")

	status, err := monitor.GetStatus(c.Request.Context())
	if err != nil {
		logger.Errorf("Failed to get missed ping monitor status: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Determine overall status string
	statusStr := "disabled"
	if status.Enabled {
		if status.MonitorRunning {
			statusStr = "enabled"
		} else {
			statusStr = "error"
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"status":  statusStr,
		"data":    status,
	})
}
