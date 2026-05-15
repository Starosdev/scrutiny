package handler

import (
	"net/http"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/validation"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func GetBtrfsFilesystemDetails(c *gin.Context) {
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)
	logger := c.MustGet("LOGGER").(*logrus.Entry)

	uuid := c.Param("uuid")
	if err := validation.ValidateUUID(uuid); err != nil {
		logger.Warnf("Invalid Btrfs UUID format: %s", uuid)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	filesystem, err := deviceRepo.GetBtrfsFilesystemDetails(c, uuid)
	if err != nil {
		logger.Errorln("An error occurred while getting Btrfs filesystem details", err)
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Filesystem not found"})
		return
	}

	durationKey := c.DefaultQuery("duration_key", "week")
	metricsHistory, err := deviceRepo.GetBtrfsMetricsHistory(c, uuid, durationKey)
	if err != nil {
		logger.Warnln("Could not get Btrfs metrics history", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"filesystem":      filesystem,
			"metrics_history": metricsHistory,
		},
	})
}
