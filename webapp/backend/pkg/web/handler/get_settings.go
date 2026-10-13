package handler

import (
	"net/http"
	"os/exec"

	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/version"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// collectorTriggerEnabled reports whether the web server can trigger collectors
// directly (Omnibus mode detection). We look for 'scrutiny-collector-metrics' in
// the PATH. Shared by GetSettings and SaveSettings so both responses carry the
// same server-capability flags.
func collectorTriggerEnabled() bool {
	_, execErr := exec.LookPath("scrutiny-collector-metrics")
	return execErr == nil
}

func zfsPoolModificationsAllowed(c *gin.Context) bool {
	appConfig := c.MustGet("CONFIG").(config.Interface)
	return appConfig.GetBool(config.WebZFSAllowPoolModificationsKey)
}

func GetSettings(c *gin.Context) {
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)

	settings, err := deviceRepo.LoadSettings(c)
	if err != nil {
		logger.Errorln("An error occurred while retrieving settings", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	// Only admin callers may see secret-bearing settings such as the Uptime Kuma push URL.
	view := *settings
	if scope, _ := c.Get("AUTH_SCOPE"); scope != models.APITokenScopeAdmin {
		view = settings.Redacted()
	}

	c.JSON(http.StatusOK, gin.H{
		"success":                        true,
		"settings":                       view,
		"server_version":                 version.VERSION,
		"collector_trigger_enabled":      collectorTriggerEnabled(),
		"zfs_pool_modifications_allowed": zfsPoolModificationsAllowed(c),
	})
}
