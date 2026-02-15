package handler

import (
	"fmt"
	"net/http"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/metrics"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/collector"
	"github.com/analogj/scrutiny/webapp/backend/pkg/notify"
	"github.com/analogj/scrutiny/webapp/backend/pkg/validation"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func UploadDeviceMetrics(c *gin.Context) {
	//db := c.MustGet("DB").(*gorm.DB)
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	appConfig := c.MustGet("CONFIG").(config.Interface)
	//influxWriteDb := c.MustGet("INFLUXDB_WRITE").(*api.WriteAPIBlocking)
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)

	//appConfig := c.MustGet("CONFIG").(config.Interface)

	wwn := c.Param("wwn")
	if err := validation.ValidateWWN(wwn); err != nil {
		logger.Warnf("Invalid WWN format: %s", wwn)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	var collectorSmartData collector.SmartInfo
	err := c.BindJSON(&collectorSmartData)
	if err != nil {
		logger.Errorln("Cannot parse SMART data", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	//update the device information if necessary
	updatedDevice, err := deviceRepo.UpdateDevice(c, wwn, collectorSmartData)
	if err != nil {
		logger.Errorln("An error occurred while updating device data from smartctl metrics:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	// insert smart info
	smartData, err := deviceRepo.SaveSmartAttributes(c, wwn, collectorSmartData)
	if err != nil {
		logger.Errorln("An error occurred while saving smartctl metrics", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	// Update device's forced failure flag based on override processing
	if err := deviceRepo.UpdateDeviceHasForcedFailure(c, wwn, smartData.HasForcedFailure); err != nil {
		logger.Warnf("Failed to update has_forced_failure for device %s: %v", wwn, err)
	}

	if smartData.Status != pkg.DeviceStatusPassed {
		//there is a failure detected by Scrutiny, update the device status on the homepage.
		updatedDevice, err = deviceRepo.UpdateDeviceStatus(c, wwn, smartData.Status)
		if err != nil {
			logger.Errorln("An error occurred while updating device status", err)
			c.JSON(http.StatusInternalServerError, gin.H{"success": false})
			return
		}
	} else if updatedDevice.DeviceStatus != pkg.DeviceStatusPassed {
		// Clear failure status when current SMART data shows all attributes passing
		updatedDevice, err = deviceRepo.ResetDeviceStatus(c, wwn)
		if err != nil {
			logger.Errorln("An error occurred while resetting device status", err)
			c.JSON(http.StatusInternalServerError, gin.H{"success": false})
			return
		}
		logger.Infof("Device %s status reset to passed - all SMART attributes now within thresholds", wwn)
	}

	// save smart temperature data (ignore failures)
	err = deviceRepo.SaveSmartTemperature(c, wwn, collectorSmartData, appConfig.GetBool(fmt.Sprintf("%s.collector.retrieve_sct_temperature_history", config.DB_USER_SETTINGS_SUBKEY)))
	if err != nil {
		logger.Errorln("An error occurred while saving smartctl temp data", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	//check for error
	if notify.ShouldNotify(
		logger,
		updatedDevice,
		smartData,
		pkg.MetricsStatusThreshold(appConfig.GetInt(fmt.Sprintf("%s.metrics.status_threshold", config.DB_USER_SETTINGS_SUBKEY))),
		pkg.MetricsStatusFilterAttributes(appConfig.GetInt(fmt.Sprintf("%s.metrics.status_filter_attributes", config.DB_USER_SETTINGS_SUBKEY))),
		appConfig.GetBool(fmt.Sprintf("%s.metrics.repeat_notifications", config.DB_USER_SETTINGS_SUBKEY)),
		c,
		deviceRepo,
		appConfig,
	) {
		//send notifications

		liveNotify := notify.New(
			logger,
			appConfig,
			updatedDevice,
			false,
		)
		if err := liveNotify.Send(); err != nil {
			logger.Warnf("Failed to send notification for device %s: %v", wwn, err)
		}
	}

	// Update Prometheus metrics (if enabled)
	if collectorVal, exists := c.Get("METRICS_COLLECTOR"); exists {
		if collector, ok := collectorVal.(*metrics.Collector); ok && collector != nil {
			collector.UpdateDeviceMetrics(wwn, updatedDevice, smartData)
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
