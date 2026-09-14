package database

import (
	"context"
	"testing"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/deviceid"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
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
	require.Equal(t, `(exists r["device_id"] and r["device_id"] == "device-a")`, filter)

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

// Aggregated queries group by device_id and device_wwn. A record belongs to the device in its
// device_id tag, or to the single device holding its WWN; anything else is dropped.
func TestHistoryRecordDeviceIDAttribution(t *testing.T) {
	uniqueWWNs := uniqueWWNDeviceIDs([]models.Device{
		{DeviceID: "device-unique", WWN: "wwn-unique"},
		{DeviceID: "device-a", WWN: "wwn-shared"},
		{DeviceID: "device-b", WWN: "wwn-shared"},
		{DeviceID: "device-no-wwn"},
	})
	require.Equal(t, map[string]string{"wwn-unique": "device-unique"}, uniqueWWNs)

	for name, testCase := range map[string]struct {
		values   map[string]interface{}
		deviceID string
		ok       bool
	}{
		"tagged":               {map[string]interface{}{"device_id": "device-a", "device_wwn": "wwn-shared"}, "device-a", true},
		"untagged unique wwn":  {map[string]interface{}{"device_wwn": "wwn-unique"}, "device-unique", true},
		"nil device_id tag":    {map[string]interface{}{"device_id": nil, "device_wwn": "wwn-unique"}, "device-unique", true},
		"untagged shared wwn":  {map[string]interface{}{"device_wwn": "wwn-shared"}, "", false},
		"untagged unknown wwn": {map[string]interface{}{"device_wwn": "wwn-other"}, "", false},
		"no identifiers":       {map[string]interface{}{"temp": int64(40)}, "", false},
	} {
		t.Run(name, func(t *testing.T) {
			deviceID, ok := historyRecordDeviceID(testCase.values, uniqueWWNs)
			require.Equal(t, testCase.ok, ok)
			require.Equal(t, testCase.deviceID, deviceID)
		})
	}
}

// One device can return a device_id-tagged row and an older untagged row; the summary keeps the
// newest whichever order they arrive in.
func TestApplySummaryRecordKeepsNewestRowPerDevice(t *testing.T) {
	older := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := older.Add(24 * time.Hour)
	tagged := map[string]interface{}{"device_id": "dev-1", "device_wwn": "wwn-1", "power_on_hours": int64(200), "_time": newer}
	legacy := map[string]interface{}{"device_wwn": "wwn-1", "power_on_hours": int64(100), "_time": older}

	for name, order := range map[string][]map[string]interface{}{
		"tagged first": {tagged, legacy},
		"legacy first": {legacy, tagged},
	} {
		t.Run(name, func(t *testing.T) {
			summaries, wwnToDeviceID := summaryRecordFixture()
			for _, values := range order {
				summaryRecordRepository().applySummaryRecord(summaries, wwnToDeviceID, values)
			}
			require.Equal(t, int64(200), summaries["dev-1"].SmartResults.PowerOnHours)
		})
	}
}

func TestApplyLastSeenRecordDropsUnattributableRecords(t *testing.T) {
	seen := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	lastSeen := map[string]time.Time{}
	uniqueWWNs := map[string]string{"wwn-unique": "device-unique"}

	applyLastSeenRecord(lastSeen, uniqueWWNs, map[string]interface{}{"device_wwn": "wwn-shared", "_time": seen})
	applyLastSeenRecord(lastSeen, uniqueWWNs, map[string]interface{}{"device_wwn": "wwn-unique", "_time": seen})
	applyLastSeenRecord(lastSeen, uniqueWWNs, map[string]interface{}{"device_id": "device-a", "device_wwn": "wwn-shared", "_time": seen.Add(time.Hour)})

	require.Equal(t, map[string]time.Time{"device-unique": seen, "device-a": seen.Add(time.Hour)}, lastSeen)
}

func TestAppendTempRecordAttributesByDeviceID(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	history := map[string][]measurements.SmartTemperature{}
	uniqueWWNs := map[string]string{"wwn-unique": "device-unique"}

	appendTempRecord(history, map[string]interface{}{"device_id": "device-a", "device_wwn": "wwn-shared", "temp": int64(40), "_time": at}, uniqueWWNs)
	appendTempRecord(history, map[string]interface{}{"device_wwn": "wwn-shared", "temp": int64(41), "_time": at}, uniqueWWNs)
	appendTempRecord(history, map[string]interface{}{"device_wwn": "wwn-unique", "temp": int64(42), "_time": at}, uniqueWWNs)

	require.Len(t, history, 2)
	require.Len(t, history["device-a"], 1)
	require.Len(t, history["device-unique"], 1)
}
