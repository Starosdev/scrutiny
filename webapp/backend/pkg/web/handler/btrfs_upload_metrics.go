package handler

import (
	"errors"
	"net/http"

	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/notify"
	"github.com/analogj/scrutiny/webapp/backend/pkg/validation"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
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
	// A read error other than not-found leaves the previous state unknown, and
	// comparing against an unknown state would report every existing counter
	// as new, so that upload skips the notification check.
	var previous *models.BtrfsFilesystem
	previousKnown := true
	if stored, err := deviceRepo.GetBtrfsFilesystemDetails(c, uuid); err == nil {
		previous = &stored
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		logger.Warnf("Could not read previous Btrfs state for filesystem %s, skipping notification check: %v", uuid, err)
		previousKnown = false
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

	if previousKnown {
		maybeNotifyBtrfs(c, deviceRepo, logger, previous, &filesystem)
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func maybeNotifyBtrfs(c *gin.Context, deviceRepo database.DeviceRepo, logger *logrus.Entry, previous *models.BtrfsFilesystem, current *models.BtrfsFilesystem) {
	notifyStateChange(c, deviceRepo, logger, stateNotification{
		kind:       "Btrfs filesystem",
		identifier: current.UUID,
		silenced:   previous != nil && (previous.Muted || previous.Archived),
		failureTypes: func(repeat bool) []string {
			return notify.BtrfsFailureTypes(previous, current, repeat)
		},
		build: func(appConfig config.Interface, failureTypes []string) notify.Notify {
			return notify.NewBtrfsNotify(logger, appConfig, current, failureTypes)
		},
	})
}
