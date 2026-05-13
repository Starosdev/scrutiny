package handler

import (
	"net/http"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func UploadFilesystemSummary(c *gin.Context) {
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)
	logger := c.MustGet("LOGGER").(*logrus.Entry)

	var payload models.FilesystemSummaryUpload
	if err := c.BindJSON(&payload); err != nil {
		logger.Errorln("Cannot parse filesystem summary", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	if err := deviceRepo.SaveFilesystemSummary(c, payload); err != nil {
		logger.Errorln("An error occurred while saving filesystem summary", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
