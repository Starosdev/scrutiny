package handler

import (
	"net/http"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
)

func RegisterBtrfsFilesystems(c *gin.Context) {
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)
	logger := c.MustGet("LOGGER").(*logrus.Entry)

	var wrapper models.BtrfsFilesystemWrapper
	if err := c.BindJSON(&wrapper); err != nil {
		logger.Errorln("Cannot parse Btrfs filesystems", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	filesystems := lo.Filter(wrapper.Data, func(filesystem models.BtrfsFilesystem, _ int) bool {
		return filesystem.UUID != ""
	})

	errs := []error{}
	for i := range filesystems {
		if err := deviceRepo.RegisterBtrfsFilesystem(c, &filesystems[i]); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		logger.Errorln("An error occurred while registering Btrfs filesystems", errs)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	c.JSON(http.StatusOK, models.BtrfsFilesystemWrapper{
		Success: true,
		Data:    filesystems,
	})
}
