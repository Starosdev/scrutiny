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
	mock_config "github.com/analogj/scrutiny/webapp/backend/pkg/config/mock"
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
			"version":       []int{7, 3},
			"exit_status":   exitStatus,
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

func TestUploadDeviceMetrics_ExitStatus_FatalBit2(t *testing.T) {
	router := setupMetricsRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/device/%s/smart", testDeviceWWN), strings.NewReader(smartPayload(4)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	require.Contains(t, w.Body.String(), "unreliable data")
}

func TestUploadDeviceMetrics_ExitStatus_AllFatalBits(t *testing.T) {
	router := setupMetricsRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/device/%s/smart", testDeviceWWN), strings.NewReader(smartPayload(7)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	require.Contains(t, w.Body.String(), "unreliable data")
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
