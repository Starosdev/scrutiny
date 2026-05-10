package handler

import (
	"net/http"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func GetFilesystemSummary(c *gin.Context) {
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)
	logger := c.MustGet("LOGGER").(*logrus.Entry)

	filesystems, hosts, err := deviceRepo.GetFilesystemSummary(c)
	if err != nil {
		logger.Errorln("An error occurred while getting filesystem summary", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"filesystems": filesystems,
			"hosts":       hosts,
		},
	})
}
