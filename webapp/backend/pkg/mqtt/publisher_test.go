package mqtt

import (
	"testing"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	"github.com/stretchr/testify/require"
)

func TestDeviceStatusString_Passed(t *testing.T) {
	require.Equal(t, "Passed", DeviceStatusString(pkg.DeviceStatusPassed))
}

func TestDeviceStatusString_FailedSmart(t *testing.T) {
	require.Equal(t, "Failed (SMART)", DeviceStatusString(pkg.DeviceStatusFailedSmart))
}

func TestDeviceStatusString_FailedScrutiny(t *testing.T) {
	require.Equal(t, "Failed (Scrutiny)", DeviceStatusString(pkg.DeviceStatusFailedScrutiny))
}

func TestDeviceStatusString_FailedBoth(t *testing.T) {
	both := pkg.DeviceStatusSet(pkg.DeviceStatusFailedSmart, pkg.DeviceStatusFailedScrutiny)
	require.Equal(t, "Failed (Both)", DeviceStatusString(both))
}

func TestBuildStatePayload_Passed(t *testing.T) {
	now := time.Date(2026, 2, 24, 10, 30, 0, 0, time.UTC)
	device := models.Device{
		WWN:          "0x5000cca264eb01d7",
		DeviceStatus: pkg.DeviceStatusPassed,
	}
	smartData := measurements.Smart{
		Temp:            45,
		PowerOnHours:    12345,
		PowerCycleCount: 678,
		Date:            now,
	}

	payload := buildStatePayload(device, smartData)

	require.Equal(t, int64(45), payload.Temperature)
	require.Equal(t, "Passed", payload.Status)
	require.Equal(t, int64(12345), payload.PowerOnHours)
	require.Equal(t, int64(678), payload.PowerCycleCount)
	require.Equal(t, "OFF", payload.Problem)
	require.Equal(t, "2026-02-24T10:30:00Z", payload.LastUpdated)
}

func TestBuildStatePayload_Failed(t *testing.T) {
	now := time.Date(2026, 2, 24, 10, 30, 0, 0, time.UTC)
	device := models.Device{
		WWN:          "0x5000cca264eb01d7",
		DeviceStatus: pkg.DeviceStatusFailedSmart,
	}
	smartData := measurements.Smart{
		Temp:            55,
		PowerOnHours:    50000,
		PowerCycleCount: 1200,
		Date:            now,
	}

	payload := buildStatePayload(device, smartData)

	require.Equal(t, int64(55), payload.Temperature)
	require.Equal(t, "Failed (SMART)", payload.Status)
	require.Equal(t, int64(50000), payload.PowerOnHours)
	require.Equal(t, int64(1200), payload.PowerCycleCount)
	require.Equal(t, "ON", payload.Problem)
}

func TestBuildStatePayload_ZeroValues(t *testing.T) {
	device := models.Device{
		WWN:          "0x5000cca264eb01d7",
		DeviceStatus: pkg.DeviceStatusPassed,
	}
	smartData := measurements.Smart{
		Temp:            0,
		PowerOnHours:    0,
		PowerCycleCount: 0,
		Date:            time.Time{},
	}

	payload := buildStatePayload(device, smartData)

	require.Equal(t, int64(0), payload.Temperature)
	require.Equal(t, int64(0), payload.PowerOnHours)
	require.Equal(t, int64(0), payload.PowerCycleCount)
	require.Equal(t, "OFF", payload.Problem)
	require.Equal(t, "Passed", payload.Status)
}
