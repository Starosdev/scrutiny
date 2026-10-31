package handler

import (
	"net/http"
	"strings"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/validation"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func ArchiveMdadmArray(c *gin.Context) {
	updateMdadmArchived(c, true)
}

func UnarchiveMdadmArray(c *gin.Context) {
	updateMdadmArchived(c, false)
}

func MuteMdadmArray(c *gin.Context) {
	updateMdadmMuted(c, true)
}

func UnmuteMdadmArray(c *gin.Context) {
	updateMdadmMuted(c, false)
}

func UpdateMdadmArrayLabel(c *gin.Context) {
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)
	logger := c.MustGet("LOGGER").(*logrus.Entry)

	uuid, ok := mdadmArrayUUID(c, logger)
	if !ok {
		return
	}

	var payload struct {
		Label string `json:"label"`
	}
	// ShouldBindJSON, not BindJSON: BindJSON flushes its own 400 before the
	// handler writes one, so the response loses its content type.
	if err := c.ShouldBindJSON(&payload); err != nil {
		logger.Errorln("Cannot parse MDADM label payload", err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false})
		return
	}

	if err := deviceRepo.UpdateMdadmArrayLabel(c.Request.Context(), uuid, payload.Label); err != nil {
		logger.Errorln("An error occurred while updating MDADM array label", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// DeleteMdadmArray removes an array's metadata and its metrics history. An
// array the collector still reports is registered again on its next run, so
// this is for an array that is gone: a host retired, or a test array left
// behind.
func DeleteMdadmArray(c *gin.Context) {
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)
	logger := c.MustGet("LOGGER").(*logrus.Entry)

	uuid, ok := mdadmArrayUUID(c, logger)
	if !ok {
		return
	}

	if err := deviceRepo.DeleteMdadmArray(c.Request.Context(), uuid); err != nil {
		logger.Errorln("An error occurred while deleting MDADM array", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func updateMdadmArchived(c *gin.Context, archived bool) {
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)
	logger := c.MustGet("LOGGER").(*logrus.Entry)

	uuid, ok := mdadmArrayUUID(c, logger)
	if !ok {
		return
	}

	if err := deviceRepo.UpdateMdadmArrayArchived(c.Request.Context(), uuid, archived); err != nil {
		logger.Errorln("An error occurred while updating MDADM archive state", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func updateMdadmMuted(c *gin.Context, muted bool) {
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)
	logger := c.MustGet("LOGGER").(*logrus.Entry)

	uuid, ok := mdadmArrayUUID(c, logger)
	if !ok {
		return
	}

	if err := deviceRepo.UpdateMdadmArrayMuted(c.Request.Context(), uuid, muted); err != nil {
		logger.Errorln("An error occurred while updating MDADM mute state", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// mdadmArrayUUID reads the array identifier from the route and rejects an
// empty or oversized one.
//
// It is not checked against the RFC 4122 shape the device and Btrfs routes
// use, because an md array identifier is not one. validation.ValidateMdadmArrayID
// holds what the shape is, and the registration route applies the same check,
// so an array cannot be stored under an identifier these routes refuse.
func mdadmArrayUUID(c *gin.Context, logger *logrus.Entry) (string, bool) {
	uuid := strings.TrimSpace(c.Param("uuid"))
	if err := validation.ValidateMdadmArrayID(uuid); err != nil {
		logger.Warnf("Invalid MDADM array identifier %q: %v", c.Param("uuid"), err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return "", false
	}
	return uuid, true
}
