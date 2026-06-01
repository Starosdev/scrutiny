package handler

import (
	"net/http"
	"strings"

	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/collector"
	"github.com/analogj/scrutiny/webapp/backend/pkg/notify"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// UploadMdadmMetrics handles MDADM metrics uploaded by the collector
func UploadMdadmMetrics(c *gin.Context) {
	dbRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	uuid := c.Param("uuid")
	if uuid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "errors": []string{"UUID is required"}})
		return
	}

	metrics, ok := bindMDADMMetrics(c)
	if !ok {
		return
	}

	if err := dbRepo.SaveMdadmMetrics(c.Request.Context(), uuid, metrics); err != nil {
		logger.Errorf("Failed to save MDADM metrics for array %s: %v", uuid, err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "errors": []string{err.Error()}})
		return
	}

	if shouldNotifyForMDADMFailure(&metrics) {
		handleMDADMNotification(c, dbRepo, logger, uuid, &metrics)
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func bindMDADMMetrics(c *gin.Context) (collector.MDADMMetrics, bool) {
	var metrics collector.MDADMMetrics
	if err := c.ShouldBindJSON(&metrics); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "errors": []error{err}})
		return collector.MDADMMetrics{}, false
	}
	return metrics, true
}

func shouldNotifyForMDADMFailure(metrics *collector.MDADMMetrics) bool {
	return metrics.FailedDevices > 0 || strings.Contains(strings.ToLower(metrics.State), "degraded")
}

func handleMDADMNotification(c *gin.Context, dbRepo database.DeviceRepo, logger *logrus.Entry, uuid string, metrics *collector.MDADMMetrics) {
	array, err := dbRepo.GetMdadmArrayDetails(c.Request.Context(), uuid)
	if err != nil {
		logger.Errorf("Failed to retrieve details for MDADM array %s during notification process: %v", uuid, err)
		return
	}
	if !shouldSendMDADMNotification(c, dbRepo, uuid, metrics) {
		return
	}

	appConfig := c.MustGet("CONFIG").(config.Interface)
	notification := notify.NewMDADMNotify(logger, appConfig, array, *metrics)
	notification.LoadDatabaseUrls(c.Request.Context(), dbRepo)
	sendNotificationWithGate(c, dbRepo, logger, uuid, &notification)
}

func shouldSendMDADMNotification(c *gin.Context, dbRepo database.DeviceRepo, uuid string, metrics *collector.MDADMMetrics) bool {
	history, err := dbRepo.GetMdadmMetricsHistory(c.Request.Context(), uuid, "day")
	if err != nil || len(history) <= 1 {
		return true
	}
	lastMetric := history[len(history)-2]
	return !shouldNotifyForMDADMState(lastMetric.State, lastMetric.FailedDevices)
}

func sendNotificationWithGate(c *gin.Context, dbRepo database.DeviceRepo, logger *logrus.Entry, uuid string, notification *notify.Notify) {
	if gateVal, exists := c.Get("NOTIFICATION_GATE"); exists {
		if gate, ok := gateVal.(*notify.NotificationGate); ok {
			settings, settingsErr := dbRepo.LoadSettings(c.Request.Context())
			if settingsErr != nil {
				logger.Warnf("Failed to load settings for notification gate: %v", settingsErr)
			}
			if settings != nil {
				gate.TrySend(notification, settings, false)
				return
			}
		}
	}
	if sendErr := notification.Send(); sendErr != nil {
		logger.Warnf("Failed to send MDADM notification for array %s: %v", uuid, sendErr)
	}
}

func shouldNotifyForMDADMState(state string, failedDevices int) bool {
	return failedDevices > 0 || strings.Contains(strings.ToLower(state), "degraded")
}
