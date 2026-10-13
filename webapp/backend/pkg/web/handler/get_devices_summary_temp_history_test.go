package handler_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	mock_database "github.com/analogj/scrutiny/webapp/backend/pkg/database/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	"github.com/analogj/scrutiny/webapp/backend/pkg/web/handler"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func setupTemperatureSummaryRouter(t *testing.T, repo *mock_database.MockDeviceRepo) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("LOGGER", logrus.WithField("test", t.Name()))
		c.Set("DEVICE_REPOSITORY", repo)
		c.Next()
	})
	router.GET("/api/summary/temp", handler.GetDevicesSummaryTempHistory)
	router.GET("/api/summary/temp/devices", handler.GetTemperatureDeviceOptions)
	return router
}

func TestGetTemperatureDeviceOptionsReturnsActiveDevices(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	repo := mock_database.NewMockDeviceRepo(mockCtrl)
	repo.EXPECT().GetDevices(gomock.Any()).Return([]models.Device{
		{DeviceID: "active", HostId: "host-b", DeviceName: "sdb", ModelName: "Model B"},
		{DeviceID: "archived", HostId: "host-a", DeviceName: "sda", Archived: true},
	}, nil)

	response := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodGet, "/api/summary/temp/devices", nil)
	setupTemperatureSummaryRouter(t, repo).ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	require.Contains(t, response.Body.String(), `"device_id":"active"`)
	require.NotContains(t, response.Body.String(), `"device_id":"archived"`)
}

func TestGetDevicesSummaryTempHistoryFiltersSelectedDeviceIDs(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	repo := mock_database.NewMockDeviceRepo(mockCtrl)
	repo.EXPECT().
		GetSmartTemperatureHistoryForDevices(gomock.Any(), "week", []string{"device-1", "device-2"}).
		Return(map[string][]measurements.SmartTemperature{}, nil)

	response := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodGet, "/api/summary/temp?duration_key=week&device_id=device-1&device_id=device-2", nil)
	setupTemperatureSummaryRouter(t, repo).ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	require.Contains(t, response.Body.String(), `"success":true`)
}

func TestGetDevicesSummaryTempHistoryPreservesLegacyUnfilteredRequest(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	repo := mock_database.NewMockDeviceRepo(mockCtrl)
	repo.EXPECT().
		GetSmartTemperatureHistory(gomock.Any(), "week").
		Return(map[string][]measurements.SmartTemperature{}, nil)

	response := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodGet, "/api/summary/temp", nil)
	setupTemperatureSummaryRouter(t, repo).ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
}

func TestGetDevicesSummaryTempHistoryRejectsMoreThanTwentyDevices(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	repo := mock_database.NewMockDeviceRepo(mockCtrl)
	query := url.Values{"duration_key": []string{"week"}}
	for i := 0; i < 21; i++ {
		query.Add("device_id", fmt.Sprintf("device-%d", i))
	}

	response := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodGet, "/api/summary/temp?"+query.Encode(), nil)
	setupTemperatureSummaryRouter(t, repo).ServeHTTP(response, request)

	require.Equal(t, http.StatusBadRequest, response.Code)
	require.Contains(t, response.Body.String(), `"success":false`)
}
