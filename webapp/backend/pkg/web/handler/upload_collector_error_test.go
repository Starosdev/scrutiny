package handler_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mock_config "github.com/analogj/scrutiny/webapp/backend/pkg/config/mock"
	mock_database "github.com/analogj/scrutiny/webapp/backend/pkg/database/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/web/handler"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

const testDeviceWWN = "0x5000cca264eb01d7"
const testNotifySettingKey = "user.metrics.notify_on_collector_error"

// setupCollectorErrorRouter creates a minimal router for UploadCollectorError tests.
// device is the Device returned by the mock repo; if nil the mock returns an error (device not found).
func setupCollectorErrorRouter(t *testing.T, device *models.Device, notifyEnabled bool) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	mockCtrl := gomock.NewController(t)
	t.Cleanup(func() { mockCtrl.Finish() })

	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeRepo := mock_database.NewMockDeviceRepo(mockCtrl)

	if device != nil {
		fakeRepo.EXPECT().GetDeviceDetails(gomock.Any(), testDeviceWWN).Return(*device, nil).AnyTimes()
		fakeConfig.EXPECT().GetBool(testNotifySettingKey).Return(notifyEnabled).AnyTimes()
	} else {
		fakeRepo.EXPECT().GetDeviceDetails(gomock.Any(), gomock.Any()).Return(models.Device{}, fmt.Errorf("not found")).AnyTimes()
		fakeRepo.EXPECT().GetDeviceByWWN(gomock.Any(), gomock.Any()).Return(models.Device{}, fmt.Errorf("not found")).AnyTimes()
	}

	logger := logrus.WithField("test", t.Name())
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("CONFIG", fakeConfig)
		c.Set("DEVICE_REPOSITORY", fakeRepo)
		c.Set("LOGGER", logger)
		c.Next()
	})
	r.POST("/api/device/:id/collector-error", handler.UploadCollectorError)
	return r
}

// setupScanErrorRouter creates a minimal router for UploadCollectorScanError tests.
func setupScanErrorRouter(t *testing.T, notifyEnabled bool) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	mockCtrl := gomock.NewController(t)
	t.Cleanup(func() { mockCtrl.Finish() })

	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeRepo := mock_database.NewMockDeviceRepo(mockCtrl)

	fakeConfig.EXPECT().GetBool(testNotifySettingKey).Return(notifyEnabled).AnyTimes()

	logger := logrus.WithField("test", t.Name())
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("CONFIG", fakeConfig)
		c.Set("DEVICE_REPOSITORY", fakeRepo)
		c.Set("LOGGER", logger)
		c.Next()
	})
	r.POST("/api/collector/scan-error", handler.UploadCollectorScanError)
	return r
}

// --- UploadCollectorError tests ---

func TestUploadCollectorError_BadJSON(t *testing.T) {
	device := &models.Device{DeviceID: testDeviceWWN, DeviceName: "/dev/sda"}
	router := setupCollectorErrorRouter(t, device, true)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/device/"+testDeviceWWN+"/collector-error", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUploadCollectorError_NotifyDisabled(t *testing.T) {
	device := &models.Device{DeviceID: testDeviceWWN, DeviceName: "/dev/sda"}
	router := setupCollectorErrorRouter(t, device, false)

	w := httptest.NewRecorder()
	body := `{"error_type":"xall","error_message":"smartctl exited with fatal code 2"}`
	req, _ := http.NewRequest("POST", "/api/device/"+testDeviceWWN+"/collector-error", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestUploadCollectorError_DeviceNotFound(t *testing.T) {
	router := setupCollectorErrorRouter(t, nil, true)

	w := httptest.NewRecorder()
	body := `{"error_type":"xall","error_message":"some error"}`
	req, _ := http.NewRequest("POST", "/api/device/"+testDeviceWWN+"/collector-error", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestUploadCollectorError_MutedDevice(t *testing.T) {
	device := &models.Device{DeviceID: testDeviceWWN, DeviceName: "/dev/sda", Muted: true}
	router := setupCollectorErrorRouter(t, device, true)

	w := httptest.NewRecorder()
	body := `{"error_type":"xall","error_message":"smartctl exited with fatal code 2"}`
	req, _ := http.NewRequest("POST", "/api/device/"+testDeviceWWN+"/collector-error", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// Muted device: handler returns 200 but skips notification
	require.Equal(t, http.StatusOK, w.Code)
}

// --- UploadCollectorScanError tests ---

func TestUploadCollectorScanError_BadJSON(t *testing.T) {
	router := setupScanErrorRouter(t, true)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/collector/scan-error", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUploadCollectorScanError_NotifyDisabled(t *testing.T) {
	router := setupScanErrorRouter(t, false)

	w := httptest.NewRecorder()
	body := `{"error_type":"scan","error_message":"permission denied"}`
	req, _ := http.NewRequest("POST", "/api/collector/scan-error", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}
