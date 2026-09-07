package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	mock_database "github.com/analogj/scrutiny/webapp/backend/pkg/database/mock"
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
	var deliveries atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { deliveries.Add(1); w.WriteHeader(http.StatusOK) }))
	defer server.Close()
	cfg, err := config.Create()
	require.NoError(t, err)
	cfg.Set("notify.urls", []string{server.URL})
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
	repo.EXPECT().GetSmartTemperatureHistoryForDevices(gomock.Any(), "day", []string{"drive"}).Return(map[string][]measurements.SmartTemperature{"drive": {{Date: now.Add(-time.Hour), Temp: 60}, {Date: now, Temp: 60}}}, nil).Times(1)
	repo.EXPECT().GetNotifyUrls(gomock.Any()).Return(nil, nil).Times(1)
	gate := notify.NewNotificationGate(logrus.New())
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("LOGGER", logrus.WithField("test", t.Name()))
		c.Set("CONFIG", cfg)
		c.Set("DEVICE_REPOSITORY", repo)
		c.Set("NOTIFICATION_GATE", gate)
	})
	router.POST("/api/device/:id/smart", handler.UploadDeviceMetrics)
	for range 2 {
		request := httptest.NewRequest(http.MethodPost, "/api/device/drive/smart", strings.NewReader(smartPayload(0)))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	}
	require.EqualValues(t, 1, deliveries.Load())
}
