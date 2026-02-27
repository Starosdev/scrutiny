package web

import (
	"strings"
	"testing"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestBuildPushMessage_AllHealthy(t *testing.T) {
	devices := []models.Device{
		{DeviceName: "/dev/sda", DeviceProtocol: "ATA", DeviceStatus: pkg.DeviceStatusPassed},
		{DeviceName: "/dev/sdb", DeviceProtocol: "ATA", DeviceStatus: pkg.DeviceStatusPassed},
		{DeviceName: "/dev/nvme0", DeviceProtocol: "NVMe", DeviceStatus: pkg.DeviceStatusPassed},
	}

	status, msg := BuildPushMessage(devices)

	require.Equal(t, "up", status)
	require.Equal(t, "All 3 monitored drives healthy", msg)
}

func TestBuildPushMessage_WithFailures(t *testing.T) {
	devices := []models.Device{
		{DeviceName: "/dev/sda", DeviceProtocol: "ATA", DeviceStatus: pkg.DeviceStatusPassed},
		{DeviceName: "/dev/sdb", DeviceProtocol: "ATA", DeviceStatus: pkg.DeviceStatusFailedSmart},
		{DeviceName: "/dev/nvme0", DeviceProtocol: "NVMe", DeviceStatus: pkg.DeviceStatusFailedScrutiny},
	}

	status, msg := BuildPushMessage(devices)

	require.Equal(t, "down", status)
	require.Contains(t, msg, "2 of 3 drives failing")
	require.Contains(t, msg, "/dev/sdb (ATA)")
	require.Contains(t, msg, "/dev/nvme0 (NVMe)")
}

func TestBuildPushMessage_MessageTruncation(t *testing.T) {
	// Create many failing devices with long names to exceed 250 chars
	var devices []models.Device
	for i := 0; i < 30; i++ {
		devices = append(devices, models.Device{
			DeviceName:     "/dev/very-long-device-name-" + strings.Repeat("x", 10),
			DeviceProtocol: "ATA",
			DeviceStatus:   pkg.DeviceStatusFailedSmart,
		})
	}

	status, msg := BuildPushMessage(devices)

	require.Equal(t, "down", status)
	require.LessOrEqual(t, len(msg), 250)
	require.True(t, strings.HasSuffix(msg, "..."), "truncated message should end with ellipsis")
}

func TestBuildPushMessage_NoDevices(t *testing.T) {
	var devices []models.Device

	status, msg := BuildPushMessage(devices)

	require.Equal(t, "up", status)
	require.Equal(t, "No monitored drives found", msg)
}

func TestBuildPushMessage_SkipsArchivedAndMuted(t *testing.T) {
	devices := []models.Device{
		{DeviceName: "/dev/sda", DeviceProtocol: "ATA", DeviceStatus: pkg.DeviceStatusPassed},
		{DeviceName: "/dev/sdb", DeviceProtocol: "ATA", DeviceStatus: pkg.DeviceStatusFailedSmart, Archived: true},
		{DeviceName: "/dev/sdc", DeviceProtocol: "ATA", DeviceStatus: pkg.DeviceStatusFailedSmart, Muted: true},
	}

	status, msg := BuildPushMessage(devices)

	require.Equal(t, "up", status)
	require.Equal(t, "All 1 monitored drives healthy", msg)
}

func TestBuildPushMessage_AllArchivedOrMuted(t *testing.T) {
	devices := []models.Device{
		{DeviceName: "/dev/sda", DeviceProtocol: "ATA", DeviceStatus: pkg.DeviceStatusPassed, Archived: true},
		{DeviceName: "/dev/sdb", DeviceProtocol: "ATA", DeviceStatus: pkg.DeviceStatusPassed, Muted: true},
	}

	status, msg := BuildPushMessage(devices)

	require.Equal(t, "up", status)
	require.Equal(t, "No monitored drives found", msg)
}
