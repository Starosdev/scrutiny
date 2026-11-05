package handler

import (
	"net/http"

	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/notify"
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

	// The previous state has to be read before the register call replaces it:
	// per-device error counters live only in the device rows it deletes.
	var previous *models.BtrfsFilesystem
	if stored, err := deviceRepo.GetBtrfsFilesystemDetails(c, uuid); err == nil {
		previous = &stored
	} else {
		logger.Debugf("No previous Btrfs state for filesystem %s: %v", uuid, err)
	}

	filesystem.UUID = uuid
	if err := deviceRepo.RegisterBtrfsFilesystem(c, &filesystem); err != nil {
		logger.Errorln("An error occurred while updating Btrfs filesystem", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}
	if err := deviceRepo.SaveBtrfsMetrics(c, &filesystem); err != nil {
		logger.Errorln("An error occurred while saving Btrfs metrics", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	maybeNotifyBtrfs(c, deviceRepo, logger, previous, &filesystem)

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func maybeNotifyBtrfs(c *gin.Context, deviceRepo database.DeviceRepo, logger *logrus.Entry, previous *models.BtrfsFilesystem, current *models.BtrfsFilesystem) {
	// Mute and archive are written by their own routes and the upload payload
	// never carries them, so the stored row is the only place to read them.
	if previous != nil && (previous.Muted || previous.Archived) {
		logger.Debugf("Skipping notification for Btrfs filesystem %s: muted or archived", current.UUID)
		return
	}
	// A cheap pre-check so a healthy upload never loads settings.
	if len(notify.BtrfsFailureTypes(previous, current, true)) == 0 {
		return
	}

	settings, err := deviceRepo.LoadSettings(c.Request.Context())
	if err != nil {
		logger.Warnf("Failed to load settings for Btrfs notification: %v", err)
	}
	repeat := settings != nil && settings.Metrics.RepeatNotifications
	failureTypes := notify.BtrfsFailureTypes(previous, current, repeat)
	if len(failureTypes) == 0 {
		return
	}

	appConfig := c.MustGet("CONFIG").(config.Interface)
	notification := notify.NewBtrfsNotify(logger, appConfig, current, failureTypes)
	notification.LoadDatabaseUrls(c.Request.Context(), deviceRepo)
	sendNotificationWithGate(c, logger, current.UUID, &notification, settings)
}
