package models

import (
	"testing"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models/collector"
	"github.com/stretchr/testify/require"
)

func TestUpdateFromCollectorSmartInfo_ShouldPopulateModelName(t *testing.T) {
	// setup
	device := Device{
		WWN:        "0x5000cca252c859cc",
		DeviceName: "sdg",
	}
	smartInfo := collector.SmartInfo{
		ModelName:       "SEAGATE ST4000NM0043",
		FirmwareVersion: "0004",
	}
	enabled := true
	smartInfo.Device.Protocol = "SCSI"
	smartInfo.SmartStatus.Passed = true
	smartInfo.SmartSupport.Available = true
	smartInfo.SmartSupport.Enabled = &enabled

	// test
	err := device.UpdateFromCollectorSmartInfo(smartInfo)

	// assert
	require.NoError(t, err)
	require.Equal(t, "SEAGATE ST4000NM0043", device.ModelName)
	require.Equal(t, "0004", device.Firmware)
	require.Equal(t, "SCSI", device.DeviceProtocol)
	require.True(t, device.SmartSupport.Available)
	require.NotNil(t, device.SmartSupport.Enabled)
	require.True(t, *device.SmartSupport.Enabled)
}

func TestUpdateFromCollectorSmartInfo_ShouldPopulateFirmwareFromScsiRevisionForSas(t *testing.T) {
	// setup
	device := Device{
		WWN:        "0x50000f0b005750f0",
		DeviceName: "sdp",
	}
	smartInfo := collector.SmartInfo{
		ModelName:    "HPE VO003840JWZJK",
		ScsiRevision: "HPD5",
	}
	smartInfo.Device.Protocol = "SCSI"
	smartInfo.SmartStatus.Passed = true

	// test
	err := device.UpdateFromCollectorSmartInfo(smartInfo)

	// assert
	require.NoError(t, err)
	require.Equal(t, "HPE VO003840JWZJK", device.ModelName)
	require.Equal(t, "HPD5", device.Firmware)
	require.Equal(t, "SCSI", device.DeviceProtocol)
}

func TestUpdateFromCollectorSmartInfo_ShouldNotClearFirmwareWhenMissingFromPayload(t *testing.T) {
	// setup: device already has a firmware value on file from a previous run
	device := Device{
		WWN:        "0x50000f0b005750f0",
		DeviceName: "sdp",
		Firmware:   "HPD5",
	}
	// this run's smartctl payload reports neither firmware_version nor scsi_revision
	// (e.g. transient error, permissions issue, or args without -i/-x)
	smartInfo := collector.SmartInfo{
		ModelName: "HPE VO003840JWZJK",
	}
	smartInfo.Device.Protocol = "SCSI"
	smartInfo.SmartStatus.Passed = true

	// test
	err := device.UpdateFromCollectorSmartInfo(smartInfo)

	// assert: previously known firmware is preserved, not blanked out
	require.NoError(t, err)
	require.Equal(t, "HPD5", device.Firmware)
}

func TestUpdateFromCollectorSmartInfo_ShouldPopulateModelNameForAta(t *testing.T) {
	// setup
	device := Device{
		WWN:        "0x5000cca264eb01d7",
		DeviceName: "sda",
	}
	smartInfo := collector.SmartInfo{
		ModelName:       "WDC WD140EDFZ-11A0VA0",
		FirmwareVersion: "81.00A81",
	}
	smartInfo.Device.Protocol = "ATA"
	smartInfo.SmartStatus.Passed = true
	smartInfo.SmartSupport.Available = true

	// test
	err := device.UpdateFromCollectorSmartInfo(smartInfo)

	// assert
	require.NoError(t, err)
	require.Equal(t, "WDC WD140EDFZ-11A0VA0", device.ModelName)
	require.Equal(t, "ATA", device.DeviceProtocol)
	require.True(t, device.SmartSupport.Available)
}
