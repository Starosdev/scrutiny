package handler

import (
	"net/http"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/validation"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

const msgInvalidBtrfsUUID = "Invalid Btrfs UUID format: %s"

func ArchiveBtrfsFilesystem(c *gin.Context) {
	updateBtrfsArchived(c, true)
}

func UnarchiveBtrfsFilesystem(c *gin.Context) {
	updateBtrfsArchived(c, false)
}

func MuteBtrfsFilesystem(c *gin.Context) {
	updateBtrfsMuted(c, true)
}

func UnmuteBtrfsFilesystem(c *gin.Context) {
	updateBtrfsMuted(c, false)
}

func UpdateBtrfsFilesystemLabel(c *gin.Context) {
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)
	logger := c.MustGet("LOGGER").(*logrus.Entry)

	uuid := c.Param("uuid")
	if err := validation.ValidateUUID(uuid); err != nil {
		logger.Warnf(msgInvalidBtrfsUUID, uuid)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	var payload struct {
		Label string `json:"label"`
	}
	if err := c.BindJSON(&payload); err != nil {
		logger.Errorln("Cannot parse Btrfs label payload", err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false})
		return
	}

	if err := deviceRepo.UpdateBtrfsFilesystemLabel(c, uuid, payload.Label); err != nil {
		logger.Errorln("An error occurred while updating Btrfs label", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func DeleteBtrfsFilesystem(c *gin.Context) {
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)
	logger := c.MustGet("LOGGER").(*logrus.Entry)

	uuid := c.Param("uuid")
	if err := validation.ValidateUUID(uuid); err != nil {
		logger.Warnf(msgInvalidBtrfsUUID, uuid)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	if err := deviceRepo.DeleteBtrfsFilesystem(c, uuid); err != nil {
		logger.Errorln("An error occurred while deleting Btrfs filesystem", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func updateBtrfsArchived(c *gin.Context, archived bool) {
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	uuid := c.Param("uuid")
	if err := validation.ValidateUUID(uuid); err != nil {
		logger.Warnf(msgInvalidBtrfsUUID, uuid)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if err := deviceRepo.UpdateBtrfsFilesystemArchived(c, uuid, archived); err != nil {
		logger.Errorln("An error occurred while updating Btrfs archive state", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func updateBtrfsMuted(c *gin.Context, muted bool) {
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	uuid := c.Param("uuid")
	if err := validation.ValidateUUID(uuid); err != nil {
		logger.Warnf(msgInvalidBtrfsUUID, uuid)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if err := deviceRepo.UpdateBtrfsFilesystemMuted(c, uuid, muted); err != nil {
		logger.Errorln("An error occurred while updating Btrfs mute state", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
