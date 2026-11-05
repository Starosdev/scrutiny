package handler

import (
	"net/http"
	"strings"

	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/collector"
	"github.com/analogj/scrutiny/webapp/backend/pkg/notify"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// UploadMdadmMetrics handles MDADM metrics uploaded by the collector
func UploadMdadmMetrics(c *gin.Context) {
	dbRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	// A series keyed by an identifier the other routes refuse would be a
	// series nothing could later read or delete.
	uuid, ok := mdadmArrayUUID(c, logger)
	if !ok {
		return
	}

	metrics, bound := bindMDADMMetrics(c)
	if !bound {
		return
	}

	if err := dbRepo.SaveMdadmMetrics(c.Request.Context(), uuid, metrics); err != nil {
		logger.Errorf("Failed to save MDADM metrics for array %s: %v", uuid, err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "errors": []string{err.Error()}})
		return
	}

	tracker := mdadmTracker(c)
	if shouldNotifyForMDADMFailure(&metrics) {
		handleMDADMNotification(c, dbRepo, logger, tracker, uuid, &metrics)
	} else if tracker != nil {
		tracker.Forget(uuid)
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

func notificationGate(c *gin.Context) *notify.NotificationGate {
	gate, _ := c.Value("NOTIFICATION_GATE").(*notify.NotificationGate)
	return gate
}

func mdadmTracker(c *gin.Context) *notify.MDADMTracker {
	if gate := notificationGate(c); gate != nil {
		return gate.MDADM()
	}
	return nil
}

func handleMDADMNotification(c *gin.Context, dbRepo database.DeviceRepo, logger *logrus.Entry, tracker *notify.MDADMTracker, uuid string, metrics *collector.MDADMMetrics) {
	array, err := dbRepo.GetMdadmArrayDetails(c.Request.Context(), uuid)
	if err != nil {
		logger.Errorf("Failed to retrieve details for MDADM array %s during notification process: %v", uuid, err)
		return
	}
	// Muting an array is what the mute route writes, and this is the only
	// place that can honor it.
	if array.Muted {
		logger.Debugf("Skipping notification for MDADM array %s: array is muted", uuid)
		return
	}

	settings := loadNotificationSettings(c, logger, dbRepo)
	send := func() bool {
		appConfig := c.MustGet("CONFIG").(config.Interface)
		notification := notify.NewMDADMNotify(logger, appConfig, array, *metrics)
		notification.LoadDatabaseUrls(c.Request.Context(), dbRepo)
		return sendMDADMNotification(c, logger, uuid, &notification, settings)
	}
	if tracker == nil {
		send()
		return
	}
	tracker.Evaluate(uuid, metrics, settings != nil && settings.Metrics.RepeatNotifications, send)
}

// sendMDADMNotification reports whether the notification was delivered or queued.
func sendMDADMNotification(c *gin.Context, logger *logrus.Entry, uuid string, notification *notify.Notify, settings *models.Settings) bool {
	if gate := notificationGate(c); gate != nil && settings != nil {
		return gate.TrySend(notification, settings, false)
	}
	if err := notification.Send(); err != nil {
		logger.Warnf("Failed to send MDADM notification for array %s: %v", uuid, err)
		return false
	}
	return true
}
