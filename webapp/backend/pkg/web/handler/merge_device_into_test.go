package handler_test

import (
	"encoding/json"
	"fmt"
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

const testDeviceID = "8f5f0d29-a9f2-4cf2-81f0-5f8fd509c001"
const testDestinationDeviceID = "8f5f0d29-a9f2-4cf2-81f0-5f8fd509c999"

func setupMergeDevicesRouter(t *testing.T, repo *mock_database.MockDeviceRepo) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	logger := logrus.WithField("test", t.Name())

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("LOGGER", logger)
		c.Set("DEVICE_REPOSITORY", repo)
		c.Next()
	})
	r.POST("/api/device/:id/merge_into", handler.MergeDeviceInto)
	return r
}

func TestMergeDeviceInto_BadJSON(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockRepo := mock_database.NewMockDeviceRepo(mockCtrl)
	device := models.Device{DeviceID: testDeviceID}

	mockRepo.EXPECT().GetDeviceDetails(gomock.Any(), testDeviceID).Return(device, nil)

	router := setupMergeDevicesRouter(t, mockRepo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/device/"+testDeviceID+"/merge_into", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestMergeDeviceInto_MissingDestinationDeviceID(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockRepo := mock_database.NewMockDeviceRepo(mockCtrl)
	device := models.Device{DeviceID: testDeviceID}

	mockRepo.EXPECT().GetDeviceDetails(gomock.Any(), testDeviceID).Return(device, nil)

	router := setupMergeDevicesRouter(t, mockRepo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/device/"+testDeviceID+"/merge_into", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	var response map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Equal(t, false, response["success"])
	require.Contains(t, response["error"], "new_device_id")
}

func TestMergeDeviceInto_Success(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockRepo := mock_database.NewMockDeviceRepo(mockCtrl)
	device := models.Device{DeviceID: testDeviceID}

	mockRepo.EXPECT().GetDeviceDetails(gomock.Any(), testDeviceID).Return(device, nil)
	mockRepo.EXPECT().MergeDevices(gomock.Any(), testDeviceID, testDestinationDeviceID).Return(nil)

	router := setupMergeDevicesRouter(t, mockRepo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/device/"+testDeviceID+"/merge_into", strings.NewReader(fmt.Sprintf(`{"new_device_id":"%s"}`, testDestinationDeviceID)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var response map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Equal(t, true, response["success"])
}

func TestMergeDeviceInto_DeviceNotFound(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockRepo := mock_database.NewMockDeviceRepo(mockCtrl)

	mockRepo.EXPECT().GetDeviceDetails(gomock.Any(), gomock.Any()).Return(models.Device{}, fmt.Errorf("not found")).AnyTimes()
	mockRepo.EXPECT().GetDeviceByWWN(gomock.Any(), gomock.Any()).Return(models.Device{}, fmt.Errorf("not found")).AnyTimes()

	router := setupMergeDevicesRouter(t, mockRepo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/device/"+testDeviceID+"/merge_into", strings.NewReader(fmt.Sprintf(`{"new_device_id":"%s"}`, testDestinationDeviceID)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
}
