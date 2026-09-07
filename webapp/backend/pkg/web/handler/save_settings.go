package handler

import (
	"fmt"
	"net/http"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/version"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func SaveSettings(c *gin.Context) {
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)

	var settings models.Settings
	// Defaults precede decoding so omitted values differ from explicit zeroes.
	settings.Metrics.TemperatureThresholdCelsius = models.DefaultTemperatureThresholdCelsius
	settings.Metrics.TemperatureDurationMinutes = models.DefaultTemperatureDurationMinutes
	err := c.BindJSON(&settings)
	if err != nil {
		logger.Errorln("Cannot parse updated settings", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}
	if !validTemperatureNotifySettings(&settings) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": fmt.Sprintf("temperature_threshold_celsius must be %d..%d and temperature_duration_minutes must be 0..%d", models.MinTemperatureThresholdCelsius, models.MaxTemperatureThresholdCelsius, models.MaxTemperatureDurationMinutes)})
		return
	}
	settings.ApplyDefaults()
	if !validSummaryPageSize(settings.DashboardPageSize) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "dashboard_page_size must be one of 25, 50, 100, or 250"})
		return
	}
	if !validSummaryHostPageSize(settings.DashboardHostPageSize) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "dashboard_host_page_size must be one of 5, 10, 25, or 50"})
		return
	}

	err = deviceRepo.SaveSettings(c, settings)
	if err != nil {
		logger.Errorln("An error occurred while saving settings", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":                        true,
		"settings":                       settings,
		"server_version":                 version.VERSION,
		"collector_trigger_enabled":      collectorTriggerEnabled(),
		"zfs_pool_modifications_allowed": zfsPoolModificationsAllowed(c),
	})
}

func validTemperatureNotifySettings(settings *models.Settings) bool {
	return !settings.Metrics.NotifyOnTemperature ||
		(settings.Metrics.TemperatureThresholdCelsius >= models.MinTemperatureThresholdCelsius &&
			settings.Metrics.TemperatureThresholdCelsius <= models.MaxTemperatureThresholdCelsius &&
			settings.Metrics.TemperatureDurationMinutes >= 0 &&
			settings.Metrics.TemperatureDurationMinutes <= models.MaxTemperatureDurationMinutes)
}
