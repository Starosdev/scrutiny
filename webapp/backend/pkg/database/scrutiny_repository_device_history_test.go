package database

import (
	"context"
	"testing"

	"github.com/analogj/scrutiny/webapp/backend/pkg/deviceid"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func createSharedWWNDevices(t *testing.T, repo *scrutinyRepository) {
	t.Helper()
	for _, device := range []models.Device{
		{DeviceID: "device-unique", WWN: "wwn-unique"},
		{DeviceID: "device-a", WWN: "wwn-shared"},
		{DeviceID: "device-b", WWN: "wwn-shared"},
	} {
		require.NoError(t, repo.gormClient.Create(&device).Error)
	}
}

// fixes #851: drives behind one controller can register with the same WWN, so a WWN
// lookup must refuse to pick one of them.
func TestGetDeviceByWWNRejectsSharedWWN(t *testing.T) {
	repo := createDeviceRegisterTestRepository(t)
	ctx := context.Background()
	createSharedWWNDevices(t, repo)

	device, err := repo.GetDeviceByWWN(ctx, "wwn-unique")
	require.NoError(t, err)
	require.Equal(t, "device-unique", device.DeviceID)

	_, err = repo.GetDeviceByWWN(ctx, "wwn-shared")
	require.ErrorIs(t, err, ErrAmbiguousWWN)

	_, err = repo.GetDeviceByWWN(ctx, "wwn-missing")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// Untagged legacy points carry only device_wwn. They may be read as a device's history
// only while that device is the sole holder of the WWN.
func TestDeviceHistoryFilterFallsBackToWWNOnlyWhenUnique(t *testing.T) {
	repo := createDeviceRegisterTestRepository(t)
	ctx := context.Background()
	createSharedWWNDevices(t, repo)

	_, filter, err := repo.deviceHistoryFilter(ctx, "device-unique")
	require.NoError(t, err)
	require.Equal(t, deviceHistoryPredicate("device-unique", "wwn-unique", true), filter)

	_, filter, err = repo.deviceHistoryFilter(ctx, "device-a")
	require.NoError(t, err)
	require.Equal(t, `r["device_id"] == "device-a"`, filter)

	_, _, err = repo.deviceHistoryFilter(ctx, "device-missing")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// Legacy reconciliation re-keys a device row to its new device_id. Self-test history is
// keyed by device_id now, so it has to move with the row or the device loses it.
func TestRegisterDeviceMovesSelfTestsWhenRekeyingLegacyRow(t *testing.T) {
	repo := createDeviceRegisterTestRepository(t)
	ctx := context.Background()

	legacy := models.Device{DeviceID: "legacy-id", WWN: "wwn-1", ModelName: "model-a"}
	require.NoError(t, repo.gormClient.Create(&legacy).Error)
	require.NoError(t, repo.gormClient.Create(&models.DeviceSelfTest{
		DeviceID: legacy.DeviceID, DeviceWWN: legacy.WWN, DeviceIdentity: legacy.DeviceID, LifetimeHours: 100,
	}).Error)

	incoming := models.Device{WWN: "wwn-1", ModelName: "model-a", SerialNumber: "serial-a"}
	require.NoError(t, repo.RegisterDevice(ctx, incoming))

	expectedID := deviceid.GenerateWithFallback(incoming.ModelName, incoming.SerialNumber, incoming.WWN, incoming.DeviceName, incoming.HostId)
	require.NotEqual(t, legacy.DeviceID, expectedID)

	var selfTests []models.DeviceSelfTest
	require.NoError(t, repo.gormClient.Find(&selfTests).Error)
	require.Len(t, selfTests, 1)
	require.Equal(t, expectedID, selfTests[0].DeviceID)
	require.Equal(t, expectedID, selfTests[0].DeviceIdentity)
}
