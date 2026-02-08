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
	smartInfo.Device.Protocol = "SCSI"
	smartInfo.SmartStatus.Passed = true

	// test
	err := device.UpdateFromCollectorSmartInfo(smartInfo)

	// assert
	require.NoError(t, err)
	require.Equal(t, "SEAGATE ST4000NM0043", device.ModelName)
	require.Equal(t, "0004", device.Firmware)
	require.Equal(t, "SCSI", device.DeviceProtocol)
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

	// test
	err := device.UpdateFromCollectorSmartInfo(smartInfo)

	// assert
	require.NoError(t, err)
	require.Equal(t, "WDC WD140EDFZ-11A0VA0", device.ModelName)
	require.Equal(t, "ATA", device.DeviceProtocol)
}
