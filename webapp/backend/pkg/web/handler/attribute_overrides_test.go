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

// setupOverridesRouter creates a minimal Gin router wired to the attribute override handlers.
func setupOverridesRouter(t *testing.T, mockRepo *mock_database.MockDeviceRepo) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	logger := logrus.WithField("test", t.Name())

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("LOGGER", logger)
		c.Set("DEVICE_REPOSITORY", mockRepo)
		c.Next()
	})
	r.GET("/api/settings/overrides", handler.GetAttributeOverrides)
	r.POST("/api/settings/overrides", handler.SaveAttributeOverride)
	r.DELETE("/api/settings/overrides/:id", handler.DeleteAttributeOverride)
	return r
}

// --- GetAttributeOverrides ---

func TestGetAttributeOverrides_ReturnsEmptyList(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockRepo := mock_database.NewMockDeviceRepo(mockCtrl)
	mockRepo.EXPECT().GetAllOverridesForDisplay(gomock.Any()).Return([]models.AttributeOverride{}, nil)

	router := setupOverridesRouter(t, mockRepo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/settings/overrides", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var response map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Equal(t, true, response["success"])
	require.NotNil(t, response["data"])
}

func TestGetAttributeOverrides_ReturnsBothUIAndConfigOverrides(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockRepo := mock_database.NewMockDeviceRepo(mockCtrl)

	warnVal := int64(10)
	mockRepo.EXPECT().GetAllOverridesForDisplay(gomock.Any()).Return([]models.AttributeOverride{
		{Protocol: "ATA", AttributeId: "5", Action: "ignore", Source: "ui"},
		{Protocol: "NVMe", AttributeId: "media_errors", WarnAbove: &warnVal, Source: "config"},
	}, nil)

	router := setupOverridesRouter(t, mockRepo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/settings/overrides", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var response map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Equal(t, true, response["success"])
	data, ok := response["data"].([]interface{})
	require.True(t, ok)
	require.Len(t, data, 2)
}

// --- SaveAttributeOverride validation ---

func TestSaveAttributeOverride_MissingProtocol(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockRepo := mock_database.NewMockDeviceRepo(mockCtrl)

	router := setupOverridesRouter(t, mockRepo)

	body := strings.NewReader(`{"attribute_id": "5", "action": "ignore"}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/settings/overrides", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	var response map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Equal(t, false, response["success"])
	require.Contains(t, response["error"], "Protocol")
}

func TestSaveAttributeOverride_MissingAttributeId(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockRepo := mock_database.NewMockDeviceRepo(mockCtrl)

	router := setupOverridesRouter(t, mockRepo)

	body := strings.NewReader(`{"protocol": "ATA", "action": "ignore"}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/settings/overrides", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSaveAttributeOverride_InvalidProtocol(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockRepo := mock_database.NewMockDeviceRepo(mockCtrl)

	router := setupOverridesRouter(t, mockRepo)

	body := strings.NewReader(`{"protocol": "SATA", "attribute_id": "5", "action": "ignore"}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/settings/overrides", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	var response map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Contains(t, response["error"], "protocol")
}

func TestSaveAttributeOverride_InvalidAction(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockRepo := mock_database.NewMockDeviceRepo(mockCtrl)

	router := setupOverridesRouter(t, mockRepo)

	body := strings.NewReader(`{"protocol": "ATA", "attribute_id": "5", "action": "unknown"}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/settings/overrides", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSaveAttributeOverride_ForceStatusMissingStatus(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockRepo := mock_database.NewMockDeviceRepo(mockCtrl)

	router := setupOverridesRouter(t, mockRepo)

	body := strings.NewReader(`{"protocol": "ATA", "attribute_id": "5", "action": "force_status"}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/settings/overrides", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	var response map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Contains(t, response["error"], "Status")
}

func TestSaveAttributeOverride_ForceStatusInvalidStatus(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockRepo := mock_database.NewMockDeviceRepo(mockCtrl)

	router := setupOverridesRouter(t, mockRepo)

	body := strings.NewReader(`{"protocol": "ATA", "attribute_id": "5", "action": "force_status", "status": "unknown"}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/settings/overrides", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSaveAttributeOverride_CustomThreshold_NoThresholds(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockRepo := mock_database.NewMockDeviceRepo(mockCtrl)

	router := setupOverridesRouter(t, mockRepo)

	body := strings.NewReader(`{"protocol": "ATA", "attribute_id": "5", "action": ""}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/settings/overrides", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	var response map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Contains(t, response["error"], "warn_above or fail_above")
}

func TestSaveAttributeOverride_CustomThreshold_NegativeWarn(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockRepo := mock_database.NewMockDeviceRepo(mockCtrl)

	router := setupOverridesRouter(t, mockRepo)

	body := strings.NewReader(`{"protocol": "ATA", "attribute_id": "5", "action": "", "warn_above": -1}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/settings/overrides", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	var response map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Contains(t, response["error"], "non-negative")
}

func TestSaveAttributeOverride_CustomThreshold_WarnNotLessThanFail(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockRepo := mock_database.NewMockDeviceRepo(mockCtrl)

	router := setupOverridesRouter(t, mockRepo)

	body := strings.NewReader(`{"protocol": "ATA", "attribute_id": "5", "action": "", "warn_above": 10, "fail_above": 5}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/settings/overrides", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	var response map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Contains(t, response["error"], "warn_above must be less than fail_above")
}

func TestSaveAttributeOverride_InvalidWWN(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockRepo := mock_database.NewMockDeviceRepo(mockCtrl)

	router := setupOverridesRouter(t, mockRepo)

	body := strings.NewReader(`{"protocol": "ATA", "attribute_id": "5", "action": "ignore", "wwn": "not-a-wwn"}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/settings/overrides", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	var response map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Contains(t, response["error"], "WWN")
}

func TestSaveAttributeOverride_ValidIgnore(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockRepo := mock_database.NewMockDeviceRepo(mockCtrl)
	mockRepo.EXPECT().SaveAttributeOverride(gomock.Any(), gomock.Any()).Return(nil)
	mockRepo.EXPECT().GetDevices(gomock.Any()).Return([]models.Device{}, nil)

	router := setupOverridesRouter(t, mockRepo)

	body := strings.NewReader(`{"protocol": "ATA", "attribute_id": "5", "action": "ignore"}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/settings/overrides", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var response map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Equal(t, true, response["success"])
}

func TestSaveAttributeOverride_ValidForceStatus(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockRepo := mock_database.NewMockDeviceRepo(mockCtrl)
	mockRepo.EXPECT().SaveAttributeOverride(gomock.Any(), gomock.Any()).Return(nil)
	mockRepo.EXPECT().GetDevices(gomock.Any()).Return([]models.Device{}, nil)

	router := setupOverridesRouter(t, mockRepo)

	body := strings.NewReader(`{"protocol": "NVMe", "attribute_id": "media_errors", "action": "force_status", "status": "passed"}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/settings/overrides", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestSaveAttributeOverride_ValidCustomThreshold(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockRepo := mock_database.NewMockDeviceRepo(mockCtrl)
	mockRepo.EXPECT().SaveAttributeOverride(gomock.Any(), gomock.Any()).Return(nil)
	mockRepo.EXPECT().GetDevices(gomock.Any()).Return([]models.Device{}, nil)

	router := setupOverridesRouter(t, mockRepo)

	body := strings.NewReader(`{"protocol": "ATA", "attribute_id": "187", "action": "", "warn_above": 5, "fail_above": 10}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/settings/overrides", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestSaveAttributeOverride_ValidWWN(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockRepo := mock_database.NewMockDeviceRepo(mockCtrl)
	mockRepo.EXPECT().SaveAttributeOverride(gomock.Any(), gomock.Any()).Return(nil)
	mockRepo.EXPECT().GetDevices(gomock.Any()).Return([]models.Device{}, nil)

	router := setupOverridesRouter(t, mockRepo)

	body := strings.NewReader(`{"protocol": "ATA", "attribute_id": "5", "action": "ignore", "wwn": "0x5000cca264eb01d7"}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/settings/overrides", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

// --- DeleteAttributeOverride ---

func TestDeleteAttributeOverride_InvalidID(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockRepo := mock_database.NewMockDeviceRepo(mockCtrl)

	router := setupOverridesRouter(t, mockRepo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/settings/overrides/not-a-number", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeleteAttributeOverride_Success(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockRepo := mock_database.NewMockDeviceRepo(mockCtrl)
	mockRepo.EXPECT().GetAttributeOverrideByID(gomock.Any(), uint(1)).Return(&models.AttributeOverride{
		Protocol: "ATA", AttributeId: "5", Action: "ignore", Source: "ui",
	}, nil)
	mockRepo.EXPECT().DeleteAttributeOverride(gomock.Any(), uint(1)).Return(nil)
	mockRepo.EXPECT().GetDevices(gomock.Any()).Return([]models.Device{}, nil)

	router := setupOverridesRouter(t, mockRepo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/settings/overrides/1", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var response map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Equal(t, true, response["success"])
}
