package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

func postRegisterDevices(t *testing.T, repo *mock_database.MockDeviceRepo, devices []models.Device) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(models.DeviceWrapper{Data: devices})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/devices/register", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Set("DEVICE_REPOSITORY", repo)
	c.Set("LOGGER", logrus.NewEntry(logrus.New()))

	RegisterDevices(c)
	return w
}

// fixes #851: in #850 four of five drives failed registration, the handler answered
// 500 for the whole batch, and the collector then skipped SMART collection for the
// one drive that had registered too.
func TestRegisterDevicesReturnsRegisteredDevicesWhenSomeFail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mock_database.NewMockDeviceRepo(ctrl)
	repo.EXPECT().RegisterDevice(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, device models.Device) error {
		if device.SerialNumber == "PPK4ZTUB" {
			return errors.New("constraint failed: UNIQUE constraint failed: devices.wwn (2067)")
		}
		return nil
	}).Times(2)

	w := postRegisterDevices(t, repo, []models.Device{
		{WWN: "ppkja1zb", DeviceName: "sda", DeviceType: "cciss,1", ModelName: "HITACHI HUC106060CSS600", SerialNumber: "PPKJA1ZB", HostId: "dl365"},
		{WWN: "ppk4ztub", DeviceName: "sda", DeviceType: "cciss,2", ModelName: "HITACHI HUC106060CSS600", SerialNumber: "PPK4ZTUB", HostId: "dl365"},
	})

	require.Equal(t, http.StatusOK, w.Code)
	var response models.DeviceWrapper
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Len(t, response.Data, 1)
	require.Equal(t, "PPKJA1ZB", response.Data[0].SerialNumber)
	require.NotEmpty(t, response.Data[0].DeviceID)
}

func TestRegisterDevicesFailsWhenNoDeviceRegisters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mock_database.NewMockDeviceRepo(ctrl)
	repo.EXPECT().RegisterDevice(gomock.Any(), gomock.Any()).Return(errors.New("database is locked")).Times(1)

	w := postRegisterDevices(t, repo, []models.Device{
		{WWN: "0x5000cca264eb01d7", DeviceName: "sdd", ModelName: "Samsung SSD 870 EVO 4TB", SerialNumber: "35ELXMLGS", HostId: "TrueNAS"},
	})

	require.Equal(t, http.StatusInternalServerError, w.Code)
	var response models.DeviceWrapper
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.False(t, response.Success)
}
