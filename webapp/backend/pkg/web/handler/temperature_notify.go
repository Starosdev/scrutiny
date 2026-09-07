package handler

import (
	"context"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	"github.com/analogj/scrutiny/webapp/backend/pkg/notify"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

const temperatureHistoryTimeout = 10 * time.Second

func maybeNotifyTemperature(c *gin.Context, logger logrus.FieldLogger, appConfig config.Interface, repo database.DeviceRepo, device *models.Device, smart *measurements.Smart, settings *models.Settings, now time.Time) {
	gateValue, exists := c.Get("NOTIFICATION_GATE")
	gate, ok := gateValue.(*notify.NotificationGate)
	if !exists || !ok || gate == nil || settings == nil {
		return
	}
	if !settings.Metrics.NotifyOnTemperature || device.Muted || !validTemperatureNotifySettings(settings) {
		gate.Temperature().Forget(device.DeviceID)
		return
	}
	threshold := settings.Metrics.TemperatureThresholdCelsius
	duration := time.Duration(settings.Metrics.TemperatureDurationMinutes) * time.Minute
	gate.Temperature().Evaluate(device.DeviceID, smart.Temp, threshold, duration, now, func() time.Time {
		if !settings.Collector.StoreTempHistory {
			return now
		}
		if smart.Date.IsZero() || smart.Date.After(now) {
			logger.Debugf("Skipping temperature history seed for device %s: upload timestamp is missing or ahead of the server clock", device.DeviceID)
			return now
		}
		queryCtx, cancel := context.WithTimeout(c.Request.Context(), temperatureHistoryTimeout)
		defer cancel()
		history, err := repo.GetSmartTemperatureHistoryForDevices(queryCtx, database.DURATION_KEY_DAY, []string{device.DeviceID})
		if err != nil {
			logger.Warnf("Could not seed temperature notification for device %s: %v", device.DeviceID, err)
			return now
		}
		// The query must contain this upload, rather than only stale stored points.
		for _, point := range history[device.DeviceID] {
			if point.Date.Equal(smart.Date) && point.Temp == smart.Temp {
				return notify.ExceededSince(history[device.DeviceID], threshold, now)
			}
		}
		logger.Debugf("Skipping temperature history seed for device %s: current upload is not visible in history; check collector/InfluxDB clocks and write visibility", device.DeviceID)
		return now
	}, func(since time.Time) bool {
		n := notify.NewTemperatureNotify(logger, appConfig, device, smart.Temp, threshold, since, now, settings.TemperatureUnit)
		n.LoadDatabaseUrls(c, repo)
		return gate.TrySend(&n, settings, false)
	})
}
