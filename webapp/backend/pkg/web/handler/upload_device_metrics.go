package handler

import (
	"fmt"
	"net/http"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/metrics"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/collector"
	"github.com/analogj/scrutiny/webapp/backend/pkg/mqtt"
	"github.com/analogj/scrutiny/webapp/backend/pkg/notify"
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

	device, resolveErr := ResolveDevice(c, logger, deviceRepo)
	if resolveErr != nil {
		return
	}

	var collectorSmartData collector.SmartInfo
	err := c.BindJSON(&collectorSmartData)
	if err != nil {
		logger.Errorln("Cannot parse SMART data", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	// Validate smartctl exit_status bitmask before persisting data.
	// Bits 0x01 and 0x02 indicate conditions where the JSON data should not be trusted:
	//   0x01 = command line parse error
	//   0x02 = device open failed (includes standby)
	// Bit 0x04 (checksum error in response) is intentionally excluded because
	// the JSON data is usually still valid and many drives behind RAID/HBA
	// controllers intermittently return this code.
	exitStatus := collectorSmartData.Smartctl.ExitStatus
	if exitStatus&0x03 != 0 {
		logger.Warnf("Rejecting SMART data for device %s: smartctl exit_status %d has fatal bits set (mask 0x03)", device.WWN, exitStatus)
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"success": false,
			"error":   fmt.Sprintf("smartctl exit_status %d indicates unreliable data (bits 0-1 set)", exitStatus),
		})
		return
	}

	// Log informational exit status bits without rejecting data.
	// These indicate disk health issues which are exactly what we want to track:
	//   0x04 = checksum error in response (non-fatal, data usually valid)
	//   0x08 = SMART failure detected
	//   0x10 = prefail threshold exceeded
	//   0x20 = disk approaching failure
	//   0x40 = error log contains errors
	//   0x80 = self-test log contains errors
	if exitStatus != 0 {
		logger.Warnf("Device %s: smartctl exit_status %d has informational bits set; persisting data", device.WWN, exitStatus)
	}

	// update the device information if necessary (SQLite - uses deviceID)
	updatedDevice, err := deviceRepo.UpdateDevice(c, device.DeviceID, &collectorSmartData)
	if err != nil {
		logger.Errorln("An error occurred while updating device data from smartctl metrics:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	// insert smart info (InfluxDB - uses WWN)
	smartData, err := deviceRepo.SaveSmartAttributes(c, device.WWN, collectorSmartData)
	if err != nil {
		logger.Errorln("An error occurred while saving smartctl metrics", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	// Update device's forced failure flag based on override processing (SQLite - uses deviceID)
	if ffErr := deviceRepo.UpdateDeviceHasForcedFailure(c, device.DeviceID, smartData.HasForcedFailure); ffErr != nil {
		logger.Warnf("Failed to update has_forced_failure for device %s: %v", device.DeviceID, ffErr)
	}

	if smartData.Status != pkg.DeviceStatusPassed {
		//there is a failure detected by Scrutiny, update the device status on the homepage.
		updatedDevice, err = deviceRepo.UpdateDeviceStatus(c, device.DeviceID, smartData.Status)
		if err != nil {
			logger.Errorln("An error occurred while updating device status", err)
			c.JSON(http.StatusInternalServerError, gin.H{"success": false})
			return
		}
	} else if updatedDevice.DeviceStatus != pkg.DeviceStatusPassed {
		// Clear failure status when current SMART data shows all attributes passing
		updatedDevice, err = deviceRepo.ResetDeviceStatus(c, device.DeviceID)
		if err != nil {
			logger.Errorln("An error occurred while resetting device status", err)
			c.JSON(http.StatusInternalServerError, gin.H{"success": false})
			return
		}
		logger.Infof("Device %s status reset to passed - all SMART attributes now within thresholds", device.DeviceID)
	}

	// save smart temperature data (InfluxDB - uses WWN)
	err = deviceRepo.SaveSmartTemperature(c, device.WWN, updatedDevice.DeviceID, &collectorSmartData, appConfig.GetBool(fmt.Sprintf("%s.collector.retrieve_sct_temperature_history", config.DB_USER_SETTINGS_SUBKEY)))
	if err != nil {
		logger.Errorln("An error occurred while saving smartctl temp data", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	// check for error
	if notify.ShouldNotify(
		logger,
		&updatedDevice,
		&smartData,
		pkg.MetricsNotifyLevel(appConfig.GetInt(fmt.Sprintf("%s.metrics.notify_level", config.DB_USER_SETTINGS_SUBKEY))),
		pkg.MetricsStatusThreshold(appConfig.GetInt(fmt.Sprintf("%s.metrics.status_threshold", config.DB_USER_SETTINGS_SUBKEY))),
		pkg.MetricsStatusFilterAttributes(appConfig.GetInt(fmt.Sprintf("%s.metrics.status_filter_attributes", config.DB_USER_SETTINGS_SUBKEY))),
		appConfig.GetBool(fmt.Sprintf("%s.metrics.repeat_notifications", config.DB_USER_SETTINGS_SUBKEY)),
		device.WWN,
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
		liveNotify.LoadDatabaseUrls(c, deviceRepo)

		// Route through notification gate for rate limiting and quiet hours
		if gateVal, exists := c.Get("NOTIFICATION_GATE"); exists {
			if gate, ok := gateVal.(*notify.NotificationGate); ok {
				settings, settingsErr := deviceRepo.LoadSettings(c)
				if settingsErr != nil {
					logger.Warnf("Failed to load settings for notification gate: %v", settingsErr)
				}
				if settings != nil {
					gate.TrySend(&liveNotify, settings, false)
				} else {
					if sendErr := liveNotify.Send(); sendErr != nil {
						logger.Warnf("Failed to send notification for device %s: %v", device.DeviceID, sendErr)
					}
				}
			}
		} else {
			if sendErr := liveNotify.Send(); sendErr != nil {
				logger.Warnf("Failed to send notification for device %s: %v", device.DeviceID, sendErr)
			}
		}
	}

	// Check replacement risk and notify if configured threshold is met.
	riskSettings, riskSettingsErr := deviceRepo.LoadSettings(c)
	if riskSettingsErr != nil {
		logger.Warnf("Could not load settings for replacement risk notification: %v", riskSettingsErr)
	}
	if riskSettings != nil && riskSettings.Metrics.NotifyOnReplacementRisk {
		maybeNotifyReplacementRisk(c, logger, appConfig, deviceRepo, updatedDevice, smartData.Attributes, riskSettings)
	}

	// Update Prometheus metrics (if enabled)
	if collectorVal, exists := c.Get("METRICS_COLLECTOR"); exists {
		if collector, ok := collectorVal.(*metrics.Collector); ok && collector != nil {
			collector.UpdateDeviceMetrics(&updatedDevice, &smartData)
		}
	}

	// Publish to MQTT / Home Assistant (if enabled)
	if pubVal, exists := c.Get("MQTT_PUBLISHER"); exists {
		if pub, ok := pubVal.(*mqtt.Publisher); ok && pub != nil {
			pub.PublishDeviceState(device.DeviceID, &updatedDevice, &smartData)
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
