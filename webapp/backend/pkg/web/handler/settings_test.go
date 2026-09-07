package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	mock_database "github.com/analogj/scrutiny/webapp/backend/pkg/database/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/web/handler"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestSaveSettingsTemperatureValidation(t *testing.T) {
	for _, tc := range []struct {
		name                        string
		enabled                     bool
		threshold, duration, status int
	}{
		{"zero threshold", true, 0, 30, http.StatusBadRequest},
		{"negative threshold", true, -1, 30, http.StatusBadRequest},
		{"high threshold", true, 151, 30, http.StatusBadRequest},
		{"negative duration", true, 55, -1, http.StatusBadRequest},
		{"overflow duration", true, 55, models.MaxTemperatureDurationMinutes + 1, http.StatusBadRequest},
		{"immediate", true, 55, 0, http.StatusOK},
		{"minimum", true, 1, 30, http.StatusOK},
		{"maximum", true, 150, models.MaxTemperatureDurationMinutes, http.StatusOK},
		{"disabled", false, -1, -1, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := mock_database.NewMockDeviceRepo(gomock.NewController(t))
			if tc.status == http.StatusOK {
				repo.EXPECT().SaveSettings(gomock.Any(), gomock.Any()).Return(nil)
			}
			router := setupSettingsRouter(t, repo, false)
			body := fmt.Sprintf(`{"metrics":{"notify_on_temperature":%t,"temperature_threshold_celsius":%d,"temperature_duration_minutes":%d}}`, tc.enabled, tc.threshold, tc.duration)
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/settings", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(response, request)
			require.Equal(t, tc.status, response.Code, response.Body.String())
		})
	}
}

// setupSettingsRouter creates a minimal Gin router wired to the settings handlers.
func setupSettingsRouter(t *testing.T, mockRepo *mock_database.MockDeviceRepo, zfsPoolModificationsAllowed bool) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	logger := logrus.WithField("test", t.Name())
	appConfig, err := config.Create()
	require.NoError(t, err)
	appConfig.Set(config.WebZFSAllowPoolModificationsKey, zfsPoolModificationsAllowed)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("LOGGER", logger)
		c.Set("DEVICE_REPOSITORY", mockRepo)
		c.Set("CONFIG", appConfig)
		c.Next()
	})
	r.GET("/api/settings", handler.GetSettings)
	r.POST("/api/settings", handler.SaveSettings)
	return r
}

func TestGetSettings_IncludesServerCapabilityFlags(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockRepo := mock_database.NewMockDeviceRepo(mockCtrl)
	mockRepo.EXPECT().LoadSettings(gomock.Any()).Return(&models.Settings{}, nil)

	router := setupSettingsRouter(t, mockRepo, false)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/settings", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var response map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Equal(t, true, response["success"])
	require.NotNil(t, response["settings"])
	require.NotEmpty(t, response["server_version"])
	_, hasFlag := response["collector_trigger_enabled"]
	require.True(t, hasFlag, "GET response must carry collector_trigger_enabled")
	_, isBool := response["collector_trigger_enabled"].(bool)
	require.True(t, isBool, "collector_trigger_enabled must be a boolean")
	require.Equal(t, false, response["zfs_pool_modifications_allowed"])
}

// TestSaveSettings_PreservesServerCapabilityFlags is the regression guard for the
// "Run collectors" button vanishing after any settings save. The button is gated
// on collector_trigger_enabled; the save response must echo the same server
// capability flags as GET, otherwise the frontend overwrites them with undefined.
func TestSaveSettings_PreservesServerCapabilityFlags(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockRepo := mock_database.NewMockDeviceRepo(mockCtrl)
	mockRepo.EXPECT().SaveSettings(gomock.Any(), gomock.Any()).Return(nil)

	router := setupSettingsRouter(t, mockRepo, false)

	body := strings.NewReader(`{"temperature_unit": "celsius"}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/settings", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var response map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Equal(t, true, response["success"])
	require.NotNil(t, response["settings"])
	require.NotEmpty(t, response["server_version"], "save response must echo server_version")
	_, hasFlag := response["collector_trigger_enabled"]
	require.True(t, hasFlag, "save response must carry collector_trigger_enabled so the Run collectors button survives a save")
	_, isBool := response["collector_trigger_enabled"].(bool)
	require.True(t, isBool, "collector_trigger_enabled must be a boolean")
	require.Equal(t, false, response["zfs_pool_modifications_allowed"])
}

func TestSaveSettingsRejectsUnsupportedDashboardPageSize(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockRepo := mock_database.NewMockDeviceRepo(mockCtrl)
	router := setupSettingsRouter(t, mockRepo, true)

	body := strings.NewReader(`{"dashboard_page_size": 10}`)
	response := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodPost, "/api/settings", body)
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusBadRequest, response.Code)
	require.Contains(t, response.Body.String(), "dashboard_page_size")
}

func TestSaveSettingsRejectsUnsupportedDashboardHostPageSize(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockRepo := mock_database.NewMockDeviceRepo(mockCtrl)
	router := setupSettingsRouter(t, mockRepo, true)

	body := strings.NewReader(`{"dashboard_host_page_size": 100}`)
	response := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodPost, "/api/settings", body)
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusBadRequest, response.Code)
	require.Contains(t, response.Body.String(), "dashboard_host_page_size")
}
