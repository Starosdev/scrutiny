package handler_test

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	mock_database "github.com/analogj/scrutiny/webapp/backend/pkg/database/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/metrics"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	"github.com/analogj/scrutiny/webapp/backend/pkg/notify"
	"github.com/analogj/scrutiny/webapp/backend/pkg/web/handler"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestUploadTemperatureDeliversOnce(t *testing.T) {
	for _, stalled := range []bool{false, true} {
		t.Run(fmt.Sprint("stalled=", stalled), func(t *testing.T) {
			var deliveries atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { deliveries.Add(1); w.WriteHeader(http.StatusOK) }))
			defer server.Close()
			cfg, err := config.Create()
			require.NoError(t, err)
			cfg.Set("notify.urls", []string{server.URL})
			if stalled {
				listener, listenErr := net.Listen("tcp", "127.0.0.1:0")
				require.NoError(t, listenErr)
				defer listener.Close()
				const smtpBudget = 100 * time.Millisecond
				const fixtureSafety = 3 * time.Second
				go func() {
					for {
						conn, acceptErr := listener.Accept()
						if acceptErr != nil {
							return
						}
						deliveries.Add(1)
						_ = conn.SetDeadline(time.Now().Add(fixtureSafety))
						_, _ = io.Copy(io.Discard, conn)
						_ = conn.Close()
					}
				}()
				cfg.Set("notify.urls", []string{"smtp://" + listener.Addr().String() + "/?fromaddress=sender@example.com&toaddresses=recipient@example.com&usestarttls=No&auth=None&timeout=" + smtpBudget.String()})
			}
			settings := &models.Settings{}
			settings.ApplyDefaults()
			settings.Metrics.NotifyOnTemperature = true
			settings.Metrics.TemperatureDurationMinutes = models.DefaultTemperatureDurationMinutes
			settings.Collector.StoreTempHistory = true
			device := models.Device{DeviceID: "drive", WWN: "wwn", DeviceName: "/dev/sda", DeviceStatus: pkg.DeviceStatusPassed}
			now := time.Now().UTC().Truncate(time.Second)
			repo := mock_database.NewMockDeviceRepo(gomock.NewController(t))
			repo.EXPECT().GetDeviceDetails(gomock.Any(), "drive").Return(device, nil).Times(2)
			repo.EXPECT().UpdateDevice(gomock.Any(), "drive", gomock.Any()).Return(device, nil).Times(2)
			repo.EXPECT().SaveSmartAttributes(gomock.Any(), "wwn", gomock.Any()).Return(measurements.Smart{Temp: 60, Date: now, Status: pkg.DeviceStatusPassed}, nil).Times(2)
			repo.EXPECT().UpdateDeviceHasForcedFailure(gomock.Any(), "drive", false).Return(nil).Times(2)
			repo.EXPECT().SaveSmartTemperature(gomock.Any(), "wwn", "drive", gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(2)
			repo.EXPECT().LoadSettings(gomock.Any()).Return(settings, nil).Times(2)
			repo.EXPECT().GetTemperatureNotificationHistory(gomock.Any(), "drive").Return([]measurements.SmartTemperature{{Date: now.Add(-time.Hour), Temp: 60}, {Date: now, Temp: 60}}, nil).Times(1)
			wantDeliveries := 1
			if stalled {
				wantDeliveries = 2
			}
			repo.EXPECT().GetNotifyUrls(gomock.Any()).Return(nil, nil).Times(wantDeliveries)
			repo.EXPECT().GetWorkloadInsights(gomock.Any(), "week").Return(nil, nil).Times(2)
			gate := notify.NewNotificationGate(logrus.New())
			collector := metrics.NewCollector(logrus.NewEntry(logrus.New()))
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("LOGGER", logrus.WithField("test", t.Name()))
				c.Set("CONFIG", cfg)
				c.Set("DEVICE_REPOSITORY", repo)
				c.Set("NOTIFICATION_GATE", gate)
				c.Set("METRICS_COLLECTOR", collector)
			})
			router.POST("/api/device/:id/smart", handler.UploadDeviceMetrics)
			for range 2 {
				start := time.Now()
				request := httptest.NewRequest(http.MethodPost, "/api/device/drive/smart", strings.NewReader(smartPayload(0)))
				request.Header.Set("Content-Type", "application/json")
				response := httptest.NewRecorder()
				router.ServeHTTP(response, request)
				require.Equal(t, http.StatusOK, response.Code, response.Body.String())
				const uploadTestAllowance = 2 * time.Second
				require.Less(t, time.Since(start), uploadTestAllowance)
			}
			require.EqualValues(t, wantDeliveries, deliveries.Load())
		})
	}
}

func TestUploadSharesNotificationSettings(t *testing.T) {
	for _, scenario := range []string{"available", "missing", "load error", "no gate", "nil gate", "quiet hours", "rate limit"} {
		t.Run(scenario, func(t *testing.T) {
			var deliveries atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				deliveries.Add(1)
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()
			cfg, err := config.Create()
			require.NoError(t, err)
			cfg.Set("notify.urls", []string{server.URL})
			cfg.Set(config.DB_USER_SETTINGS_SUBKEY+".metrics.notify_level", pkg.MetricsNotifyLevelFail)
			cfg.Set(config.DB_USER_SETTINGS_SUBKEY+".metrics.status_threshold", pkg.MetricsStatusThresholdSmart)
			cfg.Set(config.DB_USER_SETTINGS_SUBKEY+".metrics.repeat_notifications", true)
			settings := &models.Settings{}
			settings.ApplyDefaults()
			settings.Metrics.NotifyOnTemperature = true
			if scenario == "quiet hours" {
				now := time.Now()
				settings.Metrics.NotificationQuietStart = now.Add(-time.Hour).Format("15:04")
				settings.Metrics.NotificationQuietEnd = now.Add(time.Hour).Format("15:04")
			}
			if scenario == "rate limit" {
				settings.Metrics.NotificationRateLimit = 1
			}
			var settingsError error
			if scenario == "missing" || scenario == "load error" {
				settings = nil
			}
			if scenario == "load error" {
				settingsError = errors.New("settings unavailable")
			}
			device := models.Device{DeviceID: "drive", WWN: "wwn", DeviceName: "/dev/sda", DeviceStatus: pkg.DeviceStatusFailedSmart}
			repo := mock_database.NewMockDeviceRepo(gomock.NewController(t))
			repo.EXPECT().GetDeviceDetails(gomock.Any(), "drive").Return(device, nil)
			repo.EXPECT().UpdateDevice(gomock.Any(), "drive", gomock.Any()).Return(device, nil)
			repo.EXPECT().SaveSmartAttributes(gomock.Any(), "wwn", gomock.Any()).Return(measurements.Smart{Temp: 60, Status: device.DeviceStatus}, nil)
			repo.EXPECT().UpdateDeviceHasForcedFailure(gomock.Any(), "drive", false).Return(nil)
			repo.EXPECT().UpdateDeviceStatus(gomock.Any(), "drive", device.DeviceStatus).Return(device, nil)
			repo.EXPECT().SaveSmartTemperature(gomock.Any(), "wwn", "drive", gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			repo.EXPECT().LoadSettings(gomock.Any()).Return(settings, settingsError).Times(1)
			urlLoads := 1
			if scenario == "available" || scenario == "quiet hours" || scenario == "rate limit" {
				urlLoads++
			}
			repo.EXPECT().GetNotifyUrls(gomock.Any()).Return(nil, nil).Times(urlLoads)
			gate := notify.NewNotificationGate(logrus.New())
			if scenario == "nil gate" {
				gate = nil
			}
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("LOGGER", logrus.WithField("test", t.Name()))
				c.Set("CONFIG", cfg)
				c.Set("DEVICE_REPOSITORY", repo)
				if scenario != "no gate" {
					c.Set("NOTIFICATION_GATE", gate)
				}
			})
			router.POST("/api/device/:id/smart", handler.UploadDeviceMetrics)
			request := httptest.NewRequest(http.MethodPost, "/api/device/drive/smart", strings.NewReader(smartPayload(0)))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			require.Equal(t, http.StatusOK, response.Code)
			wantDeliveries := urlLoads
			if scenario == "quiet hours" {
				require.Equal(t, urlLoads, gate.QueueLength(), "SMART and temperature must both respect quiet hours")
				wantDeliveries = 0
			} else if scenario == "rate limit" {
				wantDeliveries = settings.Metrics.NotificationRateLimit
			}
			require.EqualValues(t, wantDeliveries, deliveries.Load())
		})
	}
}
