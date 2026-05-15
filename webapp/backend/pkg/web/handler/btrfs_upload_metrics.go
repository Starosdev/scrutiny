package handler

import (
	"net/http"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/validation"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func UploadBtrfsMetrics(c *gin.Context) {
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)
	logger := c.MustGet("LOGGER").(*logrus.Entry)

	uuid := c.Param("uuid")
	if err := validation.ValidateUUID(uuid); err != nil {
		logger.Warnf("Invalid Btrfs UUID format: %s", uuid)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	var filesystem models.BtrfsFilesystem
	if err := c.BindJSON(&filesystem); err != nil {
		logger.Errorln("Cannot parse Btrfs metrics", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	filesystem.UUID = uuid
	if err := deviceRepo.RegisterBtrfsFilesystem(c, filesystem); err != nil {
		logger.Errorln("An error occurred while updating Btrfs filesystem", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}
	if err := deviceRepo.SaveBtrfsMetrics(c, filesystem); err != nil {
		logger.Errorln("An error occurred while saving Btrfs metrics", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
