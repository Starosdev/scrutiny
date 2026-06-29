package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mock_database "github.com/analogj/scrutiny/webapp/backend/pkg/database/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/web/handler"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// setupSettingsRouter creates a minimal Gin router wired to the settings handlers.
func setupSettingsRouter(t *testing.T, mockRepo *mock_database.MockDeviceRepo) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	logger := logrus.WithField("test", t.Name())

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("LOGGER", logger)
		c.Set("DEVICE_REPOSITORY", mockRepo)
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

	router := setupSettingsRouter(t, mockRepo)

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

	router := setupSettingsRouter(t, mockRepo)

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
}
