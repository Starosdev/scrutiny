package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	mock_config "github.com/analogj/scrutiny/webapp/backend/pkg/config/mock"
	mock_database "github.com/analogj/scrutiny/webapp/backend/pkg/database/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	"github.com/analogj/scrutiny/webapp/backend/pkg/web/handler"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// smartPayload builds a minimal smartctl JSON payload with the given exit_status.
func smartPayload(exitStatus int) string {
	payload := map[string]interface{}{
		"json_format_version": []int{1, 0},
		"smartctl": map[string]interface{}{
			"version":     []int{7, 3},
			"exit_status": exitStatus,
		},
		"device": map[string]interface{}{
			"name":     "/dev/sda",
			"type":     "ata",
			"protocol": "ATA",
		},
	}
	b, _ := json.Marshal(payload)
	return string(b)
}

// setupMetricsRouter creates a minimal router for UploadDeviceMetrics exit_status
// rejection tests. The mock repo resolves the device but no further DB calls are
// expected because the handler should reject before reaching them.
func setupMetricsRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	mockCtrl := gomock.NewController(t)
	t.Cleanup(func() { mockCtrl.Finish() })

	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeRepo := mock_database.NewMockDeviceRepo(mockCtrl)

	device := models.Device{DeviceID: testDeviceWWN, DeviceName: "/dev/sda"}
	fakeRepo.EXPECT().GetDeviceDetails(gomock.Any(), testDeviceWWN).Return(device, nil).AnyTimes()
	fakeRepo.EXPECT().GetDeviceByWWN(gomock.Any(), testDeviceWWN).Return(device, nil).AnyTimes()

	// CONFIG and DEVICE_REPOSITORY are required by the handler middleware.
	// No further mock expectations are set because the handler must reject
	// before calling UpdateDevice or SaveSmartAttributes.
	_ = fakeConfig

	logger := logrus.WithField("test", t.Name())
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("CONFIG", fakeConfig)
		c.Set("DEVICE_REPOSITORY", fakeRepo)
		c.Set("LOGGER", logger)
		c.Next()
	})
	r.POST("/api/device/:id/smart", handler.UploadDeviceMetrics)
	return r
}

// setupMetricsRouterAccept creates a router for UploadDeviceMetrics tests where the
// exit_status validation passes and the handler proceeds to persist data. The mock
// repo stubs all DB calls that occur after validation.
func setupMetricsRouterAccept(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	mockCtrl := gomock.NewController(t)
	t.Cleanup(func() { mockCtrl.Finish() })

	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeRepo := mock_database.NewMockDeviceRepo(mockCtrl)

	device := models.Device{DeviceID: testDeviceWWN, DeviceName: "/dev/sda", DeviceStatus: pkg.DeviceStatusPassed}
	fakeRepo.EXPECT().GetDeviceDetails(gomock.Any(), testDeviceWWN).Return(device, nil).AnyTimes()
	fakeRepo.EXPECT().GetDeviceByWWN(gomock.Any(), testDeviceWWN).Return(device, nil).AnyTimes()
	fakeRepo.EXPECT().UpdateDevice(gomock.Any(), gomock.Any(), gomock.Any()).Return(device, nil).AnyTimes()
	fakeRepo.EXPECT().SaveSmartAttributes(gomock.Any(), gomock.Any(), gomock.Any()).Return(measurements.Smart{Status: pkg.DeviceStatusPassed}, nil).AnyTimes()
	fakeRepo.EXPECT().UpdateDeviceHasForcedFailure(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	fakeRepo.EXPECT().SaveSmartTemperature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	fakeRepo.EXPECT().LoadSettings(gomock.Any()).Return(&models.Settings{}, nil).AnyTimes()

	fakeConfig.EXPECT().GetBool(gomock.Any()).Return(false).AnyTimes()
	fakeConfig.EXPECT().GetInt(gomock.Any()).Return(0).AnyTimes()

	logger := logrus.WithField("test", t.Name())
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("CONFIG", fakeConfig)
		c.Set("DEVICE_REPOSITORY", fakeRepo)
		c.Set("LOGGER", logger)
		c.Next()
	})
	r.POST("/api/device/:id/smart", handler.UploadDeviceMetrics)
	return r
}

func TestUploadDeviceMetrics_ExitStatus_FatalBit0(t *testing.T) {
	router := setupMetricsRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/device/%s/smart", testDeviceWWN), strings.NewReader(smartPayload(1)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	require.Contains(t, w.Body.String(), "unreliable data")
}

func TestUploadDeviceMetrics_ExitStatus_FatalBit1(t *testing.T) {
	router := setupMetricsRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/device/%s/smart", testDeviceWWN), strings.NewReader(smartPayload(2)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	require.Contains(t, w.Body.String(), "unreliable data")
}

func TestUploadDeviceMetrics_ExitStatus_FatalBits0And1(t *testing.T) {
	// exit_status 3 = bits 0x01 | 0x02; both are fatal
	router := setupMetricsRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/device/%s/smart", testDeviceWWN), strings.NewReader(smartPayload(3)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	require.Contains(t, w.Body.String(), "unreliable data")
}

func TestUploadDeviceMetrics_ExitStatus_ChecksumNotFatal(t *testing.T) {
	// exit_status 4 = bit 0x04 (checksum error). This is intentionally
	// treated as non-fatal because the JSON data is usually still valid
	// and many drives behind RAID/HBA controllers intermittently return it.
	router := setupMetricsRouterAccept(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/device/%s/smart", testDeviceWWN), strings.NewReader(smartPayload(4)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "success")
}

func TestUploadDeviceMetrics_ExitStatus_ChecksumWithInfoBitsNotFatal(t *testing.T) {
	// exit_status 0x44 = bit 0x04 (checksum) + bit 0x40 (error log has errors).
	// Neither is fatal; data should be accepted.
	router := setupMetricsRouterAccept(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/device/%s/smart", testDeviceWWN), strings.NewReader(smartPayload(0x44)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "success")
}

func TestUploadDeviceMetrics_ExitStatus_ErrorLogNotFatal(t *testing.T) {
	// exit_status 64 = bit 0x40 (error log contains records of errors).
	// This is informational and should not block data persistence.
	router := setupMetricsRouterAccept(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/device/%s/smart", testDeviceWWN), strings.NewReader(smartPayload(64)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "success")
}

func TestUploadDeviceMetrics_ExitStatus_SelfTestLogNotFatal(t *testing.T) {
	// exit_status 128 = bit 0x80 (self-test log contains errors).
	// This is informational and should not block data persistence.
	router := setupMetricsRouterAccept(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/device/%s/smart", testDeviceWWN), strings.NewReader(smartPayload(128)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "success")
}

func TestUploadDeviceMetrics_ExitStatus_FatalBitWithInfoBits(t *testing.T) {
	// exit_status 0x43 = bits 0, 1, and 6 set; bits 0-1 are fatal
	router := setupMetricsRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/device/%s/smart", testDeviceWWN), strings.NewReader(smartPayload(0x43)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
}
