package handler

import (
	"net/http"
	"strings"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// maxMdadmUUIDLength bounds the identifier a client may send. It is well above
// any identifier the collector produces and keeps an unbounded string out of
// the query.
const maxMdadmUUIDLength = 256

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
	if err := c.BindJSON(&payload); err != nil {
		logger.Errorln("Cannot parse MDADM label payload", err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false})
		return
	}

	if err := deviceRepo.UpdateMdadmArrayLabel(c, uuid, payload.Label); err != nil {
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

	if err := deviceRepo.DeleteMdadmArray(c, uuid); err != nil {
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

	if err := deviceRepo.UpdateMdadmArrayArchived(c, uuid, archived); err != nil {
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

	if err := deviceRepo.UpdateMdadmArrayMuted(c, uuid, muted); err != nil {
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
// use, because an md array identifier is not one: mdadm's own UUID is four
// colon-separated words, an array with no UUID is keyed by a synthetic
// "synthetic:<sha1>" identifier, and an Unraid array carries neither form.
func mdadmArrayUUID(c *gin.Context, logger *logrus.Entry) (string, bool) {
	uuid := strings.TrimSpace(c.Param("uuid"))
	if uuid == "" || len(uuid) > maxMdadmUUIDLength {
		logger.Warnf("Invalid MDADM array identifier: %q", c.Param("uuid"))
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid MDADM array identifier"})
		return "", false
	}
	return uuid, true
}
