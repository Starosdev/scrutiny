package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	mock_database "github.com/analogj/scrutiny/webapp/backend/pkg/database/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/web/handler"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestGetDeviceSelfTests(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	fakeRepo := mock_database.NewMockDeviceRepo(mockCtrl)
	device := models.Device{DeviceID: "device-1", WWN: testDeviceWWN, DeviceName: "/dev/sda"}
	effectiveHours := int64(68000)
	selfTests := []models.DeviceSelfTest{
		{
			DeviceID:               "device-1",
			DeviceWWN:              testDeviceWWN,
			TypeValue:              1,
			TypeString:             "Short offline",
			StatusValue:            0,
			StatusString:           "Completed without error",
			StatusPassed:           true,
			LifetimeHours:          2464,
			EffectiveLifetimeHours: &effectiveHours,
		},
		{LifetimeHours: 100},
	}

	fakeRepo.EXPECT().GetDeviceDetails(gomock.Any(), "device-1").Return(device, nil)
	fakeRepo.EXPECT().GetDeviceSelfTests(gomock.Any(), "device-1").Return(selfTests, nil)

	logger := logrus.WithField("test", t.Name())
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("DEVICE_REPOSITORY", fakeRepo)
		c.Set("LOGGER", logger)
		c.Next()
	})
	r.GET("/api/device/:id/selftest", handler.GetDeviceSelfTests)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/device/device-1/selftest", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			SelfTests []models.DeviceSelfTest     `json:"self_tests"`
			Health    models.DeviceSelfTestHealth `json:"health"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Len(t, response.Data.SelfTests, 2)
	require.Nil(t, response.Data.SelfTests[1].EffectiveLifetimeHours)
	require.Contains(t, w.Body.String(), `"effective_lifetime_hours":null`)
	require.Equal(t, 2464, response.Data.SelfTests[0].LifetimeHours)
	require.Equal(t, int64(68000), *response.Data.SelfTests[0].EffectiveLifetimeHours)
	require.Equal(t, "Short offline", response.Data.SelfTests[0].TypeString)
	require.Equal(t, models.DeviceSelfTestStatusPassed, response.Data.Health.Status)
	require.True(t, response.Data.Health.HasResult)
	require.True(t, response.Data.Health.HasFailures)
}

func TestGetDeviceSelfTestsReturnsServerErrorOnRepositoryFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	fakeRepo := mock_database.NewMockDeviceRepo(mockCtrl)
	device := models.Device{DeviceID: "device-1", WWN: testDeviceWWN, DeviceName: "/dev/sda"}

	fakeRepo.EXPECT().GetDeviceDetails(gomock.Any(), "device-1").Return(device, nil)
	fakeRepo.EXPECT().GetDeviceSelfTests(gomock.Any(), "device-1").Return(nil, fmt.Errorf("db failed"))

	logger := logrus.WithField("test", t.Name())
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("DEVICE_REPOSITORY", fakeRepo)
		c.Set("LOGGER", logger)
		c.Next()
	})
	r.GET("/api/device/:id/selftest", handler.GetDeviceSelfTests)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/device/device-1/selftest", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Contains(t, w.Body.String(), `"success":false`)
}
