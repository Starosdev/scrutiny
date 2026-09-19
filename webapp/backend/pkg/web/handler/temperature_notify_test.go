package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	mock_database "github.com/analogj/scrutiny/webapp/backend/pkg/database/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	"github.com/analogj/scrutiny/webapp/backend/pkg/notify"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"
)

func TestMaybeNotifyTemperature(t *testing.T) {
	for _, scenario := range []string{"seeded", "disabled", "muted", "storage disabled", "history error", "stale history", "empty history", "unknown", "quiet hours", "retry", "no gate", "missing gate", "wrong gate", "invalid settings", "missing date", "future date", "immediate", "rate limit", "webhook error", "request canceled"} {
		t.Run(scenario, func(t *testing.T) {
			cfg, err := config.Create()
			require.NoError(t, err)
			logger, hook := logrustest.NewNullLogger()
			logger.SetLevel(logrus.DebugLevel)
			var deliveries atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempt := deliveries.Add(1)
				if scenario == "webhook error" && attempt == 1 {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()
			cfg.Set("notify.urls", []string{server.URL})
			settings := &models.Settings{}
			settings.ApplyDefaults()
			settings.Metrics.NotifyOnTemperature = true
			settings.Metrics.TemperatureDurationMinutes = models.DefaultTemperatureDurationMinutes
			settings.Collector.StoreTempHistory = true
			device := &models.Device{DeviceID: "drive", DeviceName: "/dev/sda"}
			now := time.Now().UTC().Truncate(time.Second)
			smart := &measurements.Smart{Temp: 60, Date: now}
			repo := mock_database.NewMockDeviceRepo(gomock.NewController(t))
			gate := notify.NewNotificationGate(logrus.New())
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/api/device/drive/smart", nil)
			c.Set("NOTIFICATION_GATE", gate)
			history := []measurements.SmartTemperature{{Date: now.Add(-time.Hour), Temp: 60}, {Date: now, Temp: 60}}
			var historyErr error
			switch scenario {
			case "disabled":
				settings.Metrics.NotifyOnTemperature = false
			case "muted":
				device.Muted = true
			case "storage disabled":
				settings.Collector.StoreTempHistory = false
			case "history error":
				historyErr = errors.New("influx unavailable")
			case "request canceled":
				requestCtx, cancel := context.WithCancel(c.Request.Context())
				cancel()
				c.Request = c.Request.WithContext(requestCtx)
				historyErr = context.Canceled
			case "stale history":
				history = history[:1]
			case "empty history":
				history = nil
			case "unknown":
				smart.Temp = 0
			case "quiet hours":
				settings.Metrics.NotificationQuietStart = time.Now().Add(-time.Minute).Format("15:04")
				settings.Metrics.NotificationQuietEnd = time.Now().Add(time.Minute).Format("15:04")
			case "retry":
				cfg.Set("notify.urls", []string{})
			case "no gate":
				c.Set("NOTIFICATION_GATE", (*notify.NotificationGate)(nil))
			case "missing gate":
				c.Keys = nil
			case "wrong gate":
				c.Set("NOTIFICATION_GATE", "invalid")
			case "invalid settings":
				settings.Metrics.TemperatureDurationMinutes = -1
			case "missing date":
				smart.Date = time.Time{}
			case "future date":
				smart.Date = now.Add(time.Hour)
			case "immediate":
				settings.Metrics.TemperatureDurationMinutes = 0
			case "rate limit":
				settings.Metrics.NotificationRateLimit = 1
				n := notify.NewTemperatureNotify(logrus.New(), cfg, device, smart.Temp, settings.Metrics.TemperatureThresholdCelsius, now, now, settings.TemperatureUnit)
				require.True(t, gate.TrySend(&n, settings, false))
				deliveries.Store(0)
			}
			validGate := scenario != "no gate" && scenario != "missing gate" && scenario != "wrong gate"
			if settings.Metrics.NotifyOnTemperature && !device.Muted && settings.Collector.StoreTempHistory && smart.Temp > 0 && validGate && scenario != "invalid settings" && !smart.Date.IsZero() && !smart.Date.After(now) {
				repo.EXPECT().GetTemperatureNotificationHistory(gomock.Any(), "drive").Do(func(ctx context.Context, _ string) {
					deadline, exists := ctx.Deadline()
					require.True(t, exists, "history lookup must have a deadline")
					require.Positive(t, time.Until(deadline))
					require.LessOrEqual(t, time.Until(deadline), temperatureHistoryTimeout)
					if scenario == "request canceled" {
						require.ErrorIs(t, ctx.Err(), context.Canceled)
					}
				}).Return(history, historyErr).Times(1)
			}
			if scenario == "seeded" || scenario == "quiet hours" || scenario == "retry" || scenario == "immediate" || scenario == "rate limit" || scenario == "webhook error" {
				repo.EXPECT().GetNotifyUrls(gomock.Any()).Return(nil, nil).AnyTimes()
			}
			for range 2 {
				maybeNotifyTemperature(c, logger, cfg, repo, device, smart, settings, now)
			}
			switch scenario {
			case "webhook error":
				require.EqualValues(t, 2, deliveries.Load(), "HTTP failure must be retried on the next upload")
			case "seeded", "immediate":
				require.EqualValues(t, 1, deliveries.Load())
			case "quiet hours":
				require.Equal(t, 1, gate.QueueLength())
				require.Zero(t, deliveries.Load())
			case "retry":
				cfg.Set("notify.urls", []string{server.URL})
				maybeNotifyTemperature(c, logrus.New(), cfg, repo, device, smart, settings, now)
				require.EqualValues(t, 1, deliveries.Load())
			case "rate limit":
				require.Zero(t, deliveries.Load())
				settings.Metrics.NotificationRateLimit = 0
				maybeNotifyTemperature(c, logrus.New(), cfg, repo, device, smart, settings, now)
				require.EqualValues(t, 1, deliveries.Load())
			default:
				require.Zero(t, deliveries.Load())
			}
			if scenario == "stale history" || scenario == "empty history" || scenario == "missing date" || scenario == "future date" {
				require.NotNil(t, hook.LastEntry())
				require.Equal(t, logrus.DebugLevel, hook.LastEntry().Level)
				require.Contains(t, hook.LastEntry().Message, "temperature history")
			}
		})
	}
}

func TestLoadNotificationSettingsHandlesErrors(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	repo := mock_database.NewMockDeviceRepo(gomock.NewController(t))
	repo.EXPECT().LoadSettings(gomock.Any()).Return(nil, errors.New("unavailable"))
	require.Nil(t, loadNotificationSettings(c, logrus.NewEntry(logrus.New()), repo))
}

func TestTemperatureMuteAndDisableResetActiveExcursion(t *testing.T) {
	for _, muted := range []bool{true, false} {
		cfg, err := config.Create()
		require.NoError(t, err)
		var deliveries atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { deliveries.Add(1); w.WriteHeader(http.StatusOK) }))
		defer server.Close()
		cfg.Set("notify.urls", []string{server.URL})
		settings := &models.Settings{}
		settings.ApplyDefaults()
		settings.Metrics.NotifyOnTemperature = true
		settings.Metrics.TemperatureDurationMinutes = models.DefaultTemperatureDurationMinutes
		gate := notify.NewNotificationGate(logrus.New())
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set("NOTIFICATION_GATE", gate)
		device := &models.Device{DeviceID: "drive"}
		smart := &measurements.Smart{Temp: 60}
		now := time.Now()
		repo := mock_database.NewMockDeviceRepo(gomock.NewController(t))
		maybeNotifyTemperature(c, logrus.New(), cfg, repo, device, smart, settings, now)
		// Hold a failed send in flight while the handler processes mute/disable.
		const deliveryTestAllowance = 2 * time.Second
		started, release, deliveryDone := make(chan struct{}), make(chan struct{}), make(chan struct{})
		go func() {
			defer close(deliveryDone)
			gate.Temperature().Evaluate("drive", 60, settings.Metrics.TemperatureThresholdCelsius,
				time.Duration(models.DefaultTemperatureDurationMinutes)*time.Minute, now.Add(time.Hour), nil,
				func(time.Time) bool {
					close(started)
					select {
					case <-release:
					case <-time.After(deliveryTestAllowance):
					}
					return false
				})
		}()
		select {
		case <-started:
		case <-time.After(deliveryTestAllowance):
			t.Fatal("send did not start")
		}
		device.Muted = muted
		settings.Metrics.NotifyOnTemperature = muted
		resetDone := make(chan struct{})
		go func() {
			defer close(resetDone)
			maybeNotifyTemperature(c, logrus.New(), cfg, repo, device, smart, settings, now.Add(time.Hour))
		}()
		close(release)
		for _, done := range []chan struct{}{deliveryDone, resetDone} {
			select {
			case <-done:
			case <-time.After(deliveryTestAllowance):
				t.Fatal("mute/disable remained blocked after delivery failed")
			}
		}
		device.Muted = false
		settings.Metrics.NotifyOnTemperature = true
		settings.Collector.StoreTempHistory = true
		smart.Date = now.Add(time.Hour)
		// No history or notification calls are expected when resuming.
		maybeNotifyTemperature(c, logrus.New(), cfg, repo, device, smart, settings, now.Add(time.Hour))
		require.Zero(t, deliveries.Load())
		repo.EXPECT().GetNotifyUrls(gomock.Any()).Return(nil, nil).Times(1)
		maybeNotifyTemperature(c, logrus.New(), cfg, repo, device, smart, settings, now.Add(time.Hour+time.Duration(models.DefaultTemperatureDurationMinutes)*time.Minute))
		require.EqualValues(t, 1, deliveries.Load(), "resumed excursion must wait the full duration")
	}
}
