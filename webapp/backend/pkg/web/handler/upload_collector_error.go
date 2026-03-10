package handler

import (
	"fmt"
	"net/http"

	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/notify"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// CollectorErrorRequest is the JSON payload sent by the collector when smartctl
// returns an unrecoverable error during --scan, --info, or --xall.
type CollectorErrorRequest struct {
	// ErrorType is a short tag identifying which operation failed: "scan", "info", or "xall".
	ErrorType string `json:"error_type" binding:"required"`
	// ErrorMessage is the human-readable description of the failure.
	ErrorMessage string `json:"error_message" binding:"required"`
}

// UploadCollectorError handles POST /api/device/:id/collector-error.
// The collector calls this endpoint when smartctl returns an error during
// device detection or SMART data collection. When the notify_on_collector_error
// setting is enabled the backend forwards the error through the notification pipeline.
func UploadCollectorError(c *gin.Context) {
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	appConfig := c.MustGet("CONFIG").(config.Interface)
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)

	device, resolveErr := ResolveDevice(c, logger, deviceRepo)
	if resolveErr != nil {
		return
	}

	var req CollectorErrorRequest
	if err := c.BindJSON(&req); err != nil {
		logger.Errorln("Cannot parse collector error payload", err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false})
		return
	}

	notifyEnabled := appConfig.GetBool(fmt.Sprintf("%s.metrics.notify_on_collector_error", config.DB_USER_SETTINGS_SUBKEY))
	if !notifyEnabled {
		logger.Debugf("notify_on_collector_error is disabled; skipping notification for device %s", device.DeviceID)
		c.JSON(http.StatusOK, gin.H{"success": true})
		return
	}

	if device.Muted {
		logger.Debugf("Device %s is muted; skipping collector error notification", device.DeviceID)
		c.JSON(http.StatusOK, gin.H{"success": true})
		return
	}

	errorNotify := notify.NewCollectorError(logger, appConfig, device, req.ErrorType, req.ErrorMessage)
	errorNotify.LoadDatabaseUrls(c, deviceRepo)

	if gateVal, exists := c.Get("NOTIFICATION_GATE"); exists {
		if gate, ok := gateVal.(*notify.NotificationGate); ok {
			settings, settingsErr := deviceRepo.LoadSettings(c)
			if settingsErr != nil {
				logger.Warnf("Failed to load settings for notification gate: %v", settingsErr)
			}
			if settings != nil {
				gate.TrySend(&errorNotify, settings, false)
			} else {
				if sendErr := errorNotify.Send(); sendErr != nil {
					logger.Warnf("Failed to send collector error notification for device %s: %v", device.DeviceID, sendErr)
				}
			}
		}
	} else {
		if sendErr := errorNotify.Send(); sendErr != nil {
			logger.Warnf("Failed to send collector error notification for device %s: %v", device.DeviceID, sendErr)
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// UploadCollectorScanError handles POST /api/collector/scan-error.
// The collector calls this endpoint when smartctl --scan itself fails (no devices
// available to attach the error to). The notification is sent host-scoped rather
// than device-scoped.
func UploadCollectorScanError(c *gin.Context) {
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	appConfig := c.MustGet("CONFIG").(config.Interface)
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)

	var req CollectorErrorRequest
	if err := c.BindJSON(&req); err != nil {
		logger.Errorln("Cannot parse collector scan error payload", err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false})
		return
	}

	notifyEnabled := appConfig.GetBool(fmt.Sprintf("%s.metrics.notify_on_collector_error", config.DB_USER_SETTINGS_SUBKEY))
	if !notifyEnabled {
		logger.Debugf("notify_on_collector_error is disabled; skipping scan error notification")
		c.JSON(http.StatusOK, gin.H{"success": true})
		return
	}

	// For scan errors we have no specific device, so we use a minimal Device struct.
	device := models.Device{}
	errorNotify := notify.NewCollectorError(logger, appConfig, device, req.ErrorType, req.ErrorMessage)
	errorNotify.LoadDatabaseUrls(c, deviceRepo)

	if gateVal, exists := c.Get("NOTIFICATION_GATE"); exists {
		if gate, ok := gateVal.(*notify.NotificationGate); ok {
			settings, settingsErr := deviceRepo.LoadSettings(c)
			if settingsErr != nil {
				logger.Warnf("Failed to load settings for notification gate: %v", settingsErr)
			}
			if settings != nil {
				gate.TrySend(&errorNotify, settings, false)
			} else {
				if sendErr := errorNotify.Send(); sendErr != nil {
					logger.Warnf("Failed to send collector scan error notification: %v", sendErr)
				}
			}
		}
	} else {
		if sendErr := errorNotify.Send(); sendErr != nil {
			logger.Warnf("Failed to send collector scan error notification: %v", sendErr)
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
