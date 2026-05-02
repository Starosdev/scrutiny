package handler

import (
	"net/http"
	"os/exec"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/version"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func GetSettings(c *gin.Context) {
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)

	settings, err := deviceRepo.LoadSettings(c)
	if err != nil {
		logger.Errorln("An error occurred while retrieving settings", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	// Check if we can trigger collectors (Omnibus mode detection)
	// We look for 'scrutiny-collector-metrics' in the PATH
	_, execErr := exec.LookPath("scrutiny-collector-metrics")
	collectorTriggerEnabled := execErr == nil

	c.JSON(http.StatusOK, gin.H{
		"success":                   true,
		"settings":                  settings,
		"server_version":            version.VERSION,
		"collector_trigger_enabled": collectorTriggerEnabled,
	})
}
