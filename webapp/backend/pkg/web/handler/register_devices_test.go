package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	mock_database "github.com/analogj/scrutiny/webapp/backend/pkg/database/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestRegisterDevicesKeepsDeviceIDStableWhenHostIDChanges(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mock_database.NewMockDeviceRepo(ctrl)
	var registered []models.Device
	repo.EXPECT().RegisterDevice(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, device models.Device) error {
		registered = append(registered, device)
		return nil
	}).Times(2)

	register := func(hostID string) models.Device {
		payload, err := json.Marshal(models.DeviceWrapper{Data: []models.Device{{
			WWN:          "0x5000cca264eb01d7",
			DeviceName:   "sdd",
			ModelName:    "Samsung SSD 870 EVO 4TB",
			SerialNumber: "35ELXMLGS",
			HostId:       hostID,
		}}})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/api/devices/register", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Set("DEVICE_REPOSITORY", repo)
		c.Set("LOGGER", logrus.NewEntry(logrus.New()))

		RegisterDevices(c)

		require.Equal(t, http.StatusOK, w.Code)
		var response models.DeviceWrapper
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		require.True(t, response.Success)
		require.Len(t, response.Data, 1)
		return response.Data[0]
	}

	initial := register("TrueNAS")
	moved := register("homeserver2")

	require.Len(t, registered, 2)
	require.Equal(t, "TrueNAS", registered[0].HostId)
	require.Equal(t, "homeserver2", registered[1].HostId)
	require.Equal(t, initial.DeviceID, moved.DeviceID)
	require.Equal(t, registered[0].DeviceID, registered[1].DeviceID)
}
