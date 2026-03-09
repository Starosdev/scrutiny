package detect_test

import (
	"os"
	"strings"
	"testing"

	mock_shell "github.com/analogj/scrutiny/collector/pkg/common/shell/mock"
	mock_config "github.com/analogj/scrutiny/collector/pkg/config/mock"
	"github.com/analogj/scrutiny/collector/pkg/detect"
	"github.com/analogj/scrutiny/collector/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/version"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetect_SmartctlScan(t *testing.T) {
	// setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetString("host.id").AnyTimes().Return("")
	fakeConfig.EXPECT().GetDeviceOverrides().AnyTimes().Return([]models.ScanOverride{})
	fakeConfig.EXPECT().GetString("commands.metrics_smartctl_bin").AnyTimes().Return("smartctl")
	fakeConfig.EXPECT().GetString("commands.metrics_scan_args").AnyTimes().Return("--scan --json")
	fakeConfig.EXPECT().GetInt("commands.metrics_smartctl_timeout").AnyTimes().Return(120)
	fakeConfig.EXPECT().IsAllowlistedDevice(gomock.Any()).AnyTimes().Return(true)

	fakeShell := mock_shell.NewMockInterface(mockCtrl)
	testScanResults, err := os.ReadFile("testdata/smartctl_scan_simple.json")
	fakeShell.EXPECT().CommandContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(string(testScanResults), err)

	d := detect.Detect{
		Logger: logrus.WithFields(logrus.Fields{}),
		Shell:  fakeShell,
		Config: fakeConfig,
	}

	// test
	scannedDevices, err := d.SmartctlScan()

	// assert
	require.NoError(t, err)
	require.Equal(t, 7, len(scannedDevices))
	require.Equal(t, "scsi", scannedDevices[0].DeviceType)
}

func TestDetect_SmartctlScan_Megaraid(t *testing.T) {
	// setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetString("host.id").AnyTimes().Return("")
	fakeConfig.EXPECT().GetDeviceOverrides().AnyTimes().Return([]models.ScanOverride{})
	fakeConfig.EXPECT().GetString("commands.metrics_smartctl_bin").AnyTimes().Return("smartctl")
	fakeConfig.EXPECT().GetString("commands.metrics_scan_args").AnyTimes().Return("--scan --json")
	fakeConfig.EXPECT().GetInt("commands.metrics_smartctl_timeout").AnyTimes().Return(120)
	fakeConfig.EXPECT().IsAllowlistedDevice(gomock.Any()).AnyTimes().Return(true)

	fakeShell := mock_shell.NewMockInterface(mockCtrl)
	testScanResults, err := os.ReadFile("testdata/smartctl_scan_megaraid.json")
	fakeShell.EXPECT().CommandContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(string(testScanResults), err)

	d := detect.Detect{
		Logger: logrus.WithFields(logrus.Fields{}),
		Shell:  fakeShell,
		Config: fakeConfig,
	}

	// test
	scannedDevices, err := d.SmartctlScan()

	// assert
	require.NoError(t, err)
	require.Equal(t, 2, len(scannedDevices))
	require.Equal(t, []models.Device{
		{DeviceName: "bus/0", DeviceType: "megaraid,0", CollectorVersion: version.VERSION},
		{DeviceName: "bus/0", DeviceType: "megaraid,1", CollectorVersion: version.VERSION},
	}, scannedDevices)
}

func TestDetect_SmartctlScan_Nvme(t *testing.T) {
	// setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetString("host.id").AnyTimes().Return("")
	fakeConfig.EXPECT().GetDeviceOverrides().AnyTimes().Return([]models.ScanOverride{})
	fakeConfig.EXPECT().GetString("commands.metrics_smartctl_bin").AnyTimes().Return("smartctl")
	fakeConfig.EXPECT().GetString("commands.metrics_scan_args").AnyTimes().Return("--scan --json")
	fakeConfig.EXPECT().GetInt("commands.metrics_smartctl_timeout").AnyTimes().Return(120)
	fakeConfig.EXPECT().IsAllowlistedDevice(gomock.Any()).AnyTimes().Return(true)

	fakeShell := mock_shell.NewMockInterface(mockCtrl)
	testScanResults, err := os.ReadFile("testdata/smartctl_scan_nvme.json")
	fakeShell.EXPECT().CommandContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(string(testScanResults), err)

	d := detect.Detect{
		Logger: logrus.WithFields(logrus.Fields{}),
		Shell:  fakeShell,
		Config: fakeConfig,
	}

	// test
	scannedDevices, err := d.SmartctlScan()

	// assert
	require.NoError(t, err)
	require.Equal(t, 1, len(scannedDevices))
	require.Equal(t, []models.Device{
		{DeviceName: "nvme0", DeviceType: "nvme", CollectorVersion: version.VERSION},
	}, scannedDevices)
}

func TestDetect_TransformDetectedDevices_Empty(t *testing.T) {
	// setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetString("host.id").AnyTimes().Return("")
	fakeConfig.EXPECT().GetDeviceOverrides().AnyTimes().Return([]models.ScanOverride{})
	fakeConfig.EXPECT().GetString("commands.metrics_smartctl_bin").AnyTimes().Return("smartctl")
	fakeConfig.EXPECT().GetString("commands.metrics_scan_args").AnyTimes().Return("--scan --json")
	fakeConfig.EXPECT().IsAllowlistedDevice(gomock.Any()).AnyTimes().Return(true)

	detectedDevices := models.Scan{
		Devices: []models.ScanDevice{
			{
				Name:     "/dev/sda",
				InfoName: "/dev/sda",
				Protocol: "scsi",
				Type:     "scsi",
			},
		},
	}

	d := detect.Detect{
		Config: fakeConfig,
	}

	// test
	transformedDevices := d.TransformDetectedDevices(detectedDevices)

	// assert
	require.Equal(t, "sda", transformedDevices[0].DeviceName)
	require.Equal(t, "scsi", transformedDevices[0].DeviceType)
}

func TestDetect_TransformDetectedDevices_Ignore(t *testing.T) {
	// setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetString("host.id").AnyTimes().Return("")
	fakeConfig.EXPECT().GetDeviceOverrides().AnyTimes().Return([]models.ScanOverride{{Device: "/dev/sda", DeviceType: nil, Ignore: true}})
	fakeConfig.EXPECT().GetString("commands.metrics_smartctl_bin").AnyTimes().Return("smartctl")
	fakeConfig.EXPECT().GetString("commands.metrics_scan_args").AnyTimes().Return("--scan --json")
	fakeConfig.EXPECT().IsAllowlistedDevice(gomock.Any()).AnyTimes().Return(true)

	detectedDevices := models.Scan{
		Devices: []models.ScanDevice{
			{
				Name:     "/dev/sda",
				InfoName: "/dev/sda",
				Protocol: "scsi",
				Type:     "scsi",
			},
		},
	}

	d := detect.Detect{
		Config: fakeConfig,
	}

	// test
	transformedDevices := d.TransformDetectedDevices(detectedDevices)

	// assert
	require.Equal(t, []models.Device{}, transformedDevices)
}

func TestDetect_TransformDetectedDevices_Raid(t *testing.T) {
	// setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetString("host.id").AnyTimes().Return("")
	fakeConfig.EXPECT().GetString("commands.metrics_smartctl_bin").AnyTimes().Return("smartctl")
	fakeConfig.EXPECT().GetString("commands.metrics_scan_args").AnyTimes().Return("--scan --json")
	fakeConfig.EXPECT().IsAllowlistedDevice(gomock.Any()).AnyTimes().Return(true)
	fakeConfig.EXPECT().GetDeviceOverrides().AnyTimes().Return([]models.ScanOverride{
		{
			Device:     "/dev/bus/0",
			DeviceType: []string{"megaraid,14", "megaraid,15", "megaraid,18", "megaraid,19", "megaraid,20", "megaraid,21"},
			Ignore:     false,
		},
		{
			Device:     "/dev/twa0",
			DeviceType: []string{"3ware,0", "3ware,1", "3ware,2", "3ware,3", "3ware,4", "3ware,5"},
			Ignore:     false,
		},
	})
	detectedDevices := models.Scan{
		Devices: []models.ScanDevice{
			{
				Name:     "/dev/bus/0",
				InfoName: "/dev/bus/0",
				Protocol: "scsi",
				Type:     "scsi",
			},
		},
	}

	d := detect.Detect{
		Config: fakeConfig,
	}

	// test
	transformedDevices := d.TransformDetectedDevices(detectedDevices)

	// assert
	require.Equal(t, 12, len(transformedDevices))
}

func TestDetect_TransformDetectedDevices_Simple(t *testing.T) {
	// setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetString("host.id").AnyTimes().Return("")
	fakeConfig.EXPECT().GetString("commands.metrics_smartctl_bin").AnyTimes().Return("smartctl")
	fakeConfig.EXPECT().GetString("commands.metrics_scan_args").AnyTimes().Return("--scan --json")
	fakeConfig.EXPECT().GetDeviceOverrides().AnyTimes().Return([]models.ScanOverride{{Device: "/dev/sda", DeviceType: []string{"sat+megaraid"}}})
	fakeConfig.EXPECT().IsAllowlistedDevice(gomock.Any()).AnyTimes().Return(true)
	detectedDevices := models.Scan{
		Devices: []models.ScanDevice{
			{
				Name:     "/dev/sda",
				InfoName: "/dev/sda",
				Protocol: "ata",
				Type:     "ata",
			},
		},
	}

	d := detect.Detect{
		Config: fakeConfig,
	}

	// test
	transformedDevices := d.TransformDetectedDevices(detectedDevices)

	// assert
	require.Equal(t, 1, len(transformedDevices))
	require.Equal(t, "sat+megaraid", transformedDevices[0].DeviceType)
}

// test https://github.com/AnalogJ/scrutiny/issues/255#issuecomment-1164024126
func TestDetect_TransformDetectedDevices_WithoutDeviceTypeOverride(t *testing.T) {
	// setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetString("host.id").AnyTimes().Return("")
	fakeConfig.EXPECT().GetString("commands.metrics_smartctl_bin").AnyTimes().Return("smartctl")
	fakeConfig.EXPECT().GetString("commands.metrics_scan_args").AnyTimes().Return("--scan --json")
	fakeConfig.EXPECT().GetDeviceOverrides().AnyTimes().Return([]models.ScanOverride{{Device: "/dev/sda"}})
	fakeConfig.EXPECT().IsAllowlistedDevice(gomock.Any()).AnyTimes().Return(true)
	detectedDevices := models.Scan{
		Devices: []models.ScanDevice{
			{
				Name:     "/dev/sda",
				InfoName: "/dev/sda",
				Protocol: "ata",
				Type:     "scsi",
			},
		},
	}

	d := detect.Detect{
		Config: fakeConfig,
	}

	// test
	transformedDevices := d.TransformDetectedDevices(detectedDevices)

	// assert
	require.Equal(t, 1, len(transformedDevices))
	require.Equal(t, "scsi", transformedDevices[0].DeviceType)
}

func TestDetect_TransformDetectedDevices_WhenDeviceNotDetected(t *testing.T) {
	// setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetString("host.id").AnyTimes().Return("")
	fakeConfig.EXPECT().GetString("commands.metrics_smartctl_bin").AnyTimes().Return("smartctl")
	fakeConfig.EXPECT().GetString("commands.metrics_scan_args").AnyTimes().Return("--scan --json")
	fakeConfig.EXPECT().GetDeviceOverrides().AnyTimes().Return([]models.ScanOverride{{Device: "/dev/sda"}})
	detectedDevices := models.Scan{}

	d := detect.Detect{
		Config: fakeConfig,
	}

	// test
	transformedDevices := d.TransformDetectedDevices(detectedDevices)

	// assert
	require.Equal(t, 1, len(transformedDevices))
	require.Equal(t, "ata", transformedDevices[0].DeviceType)
}

func TestDetect_TransformDetectedDevices_AllowListFilters(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetString("host.id").AnyTimes().Return("")
	fakeConfig.EXPECT().GetString("commands.metrics_smartctl_bin").AnyTimes().Return("smartctl")
	fakeConfig.EXPECT().GetString("commands.metrics_scan_args").AnyTimes().Return("--scan --json")
	fakeConfig.EXPECT().GetDeviceOverrides().AnyTimes().Return([]models.ScanOverride{{Device: "/dev/sda", DeviceType: []string{"sat+megaraid"}}})
	fakeConfig.EXPECT().IsAllowlistedDevice("/dev/sda").Return(true)
	fakeConfig.EXPECT().IsAllowlistedDevice("/dev/sdb").Return(false)
	detectedDevices := models.Scan{
		Devices: []models.ScanDevice{
			{
				Name:     "/dev/sda",
				InfoName: "/dev/sda",
				Protocol: "ata",
				Type:     "ata",
			},
			{
				Name:     "/dev/sdb",
				InfoName: "/dev/sdb",
				Protocol: "ata",
				Type:     "ata",
			},
		},
	}

	d := detect.Detect{
		Config: fakeConfig,
	}

	// test
	transformedDevices := d.TransformDetectedDevices(detectedDevices)

	// assert
	require.Equal(t, 1, len(transformedDevices))
	require.Equal(t, "sda", transformedDevices[0].DeviceName)
}

func TestDetect_SmartCtlInfo(t *testing.T) {
	t.Run("should report nvme info", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		const (
			someArgs = "--info --json"

			// device info
			someDeviceName           = "some-device-name"
			someModelName            = "KCD61LUL3T84"
			someSerialNumber         = "61Q0A05UT7B8"
			someFirmware             = "8002"
			someDeviceProtocol       = "NVMe"
			someDeviceType           = "nvme"
			someCapacity       int64 = 3840755982336
		)

		fakeConfig := mock_config.NewMockInterface(ctrl)
		fakeConfig.EXPECT().
			GetCommandMetricsInfoArgs("/dev/" + someDeviceName).
			Return(someArgs)
		fakeConfig.EXPECT().
			GetString("commands.metrics_smartctl_bin").
			Return("smartctl")
		fakeConfig.EXPECT().
			GetInt("commands.metrics_smartctl_timeout").
			Return(120)

		someLogger := logrus.WithFields(logrus.Fields{})

		smartctlInfoResults, err := os.ReadFile("testdata/smartctl_info_nvme.json")
		require.NoError(t, err)

		fakeShell := mock_shell.NewMockInterface(ctrl)
		fakeShell.EXPECT().
			CommandContext(gomock.Any(), someLogger, "smartctl", append(strings.Split(someArgs, " "), "/dev/"+someDeviceName), "", gomock.Any()).
			Return(string(smartctlInfoResults), err)

		d := detect.Detect{
			Logger: someLogger,
			Shell:  fakeShell,
			Config: fakeConfig,
		}

		someDevice := &models.Device{
			WWN:        "some wwn",
			DeviceName: someDeviceName,
		}

		require.NoError(t, d.SmartCtlInfo(someDevice))

		assert.Equal(t, someDeviceName, someDevice.DeviceName)
		assert.Equal(t, someModelName, someDevice.ModelName)
		assert.Equal(t, someSerialNumber, someDevice.SerialNumber)
		assert.Equal(t, someFirmware, someDevice.Firmware)
		assert.Equal(t, someDeviceProtocol, someDevice.DeviceProtocol)
		assert.Equal(t, someDeviceType, someDevice.DeviceType)
		assert.Equal(t, someCapacity, someDevice.Capacity)
	})
}

func TestDetect_TransformDetectedDevices_LabelWithDeviceType(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetString("host.id").AnyTimes().Return("")
	fakeConfig.EXPECT().GetDeviceOverrides().AnyTimes().Return([]models.ScanOverride{
		{Device: "/dev/sda", DeviceType: []string{"sat"}, Label: "NAS Pool - Disk 1"},
	})
	fakeConfig.EXPECT().IsAllowlistedDevice(gomock.Any()).AnyTimes().Return(true)

	detectedDevices := models.Scan{
		Devices: []models.ScanDevice{
			{Name: "/dev/sda", InfoName: "/dev/sda", Protocol: "ata", Type: "ata"},
		},
	}

	d := detect.Detect{Config: fakeConfig}
	transformedDevices := d.TransformDetectedDevices(detectedDevices)

	require.Equal(t, 1, len(transformedDevices))
	require.Equal(t, "sat", transformedDevices[0].DeviceType)
	require.Equal(t, "NAS Pool - Disk 1", transformedDevices[0].Label)
}

func TestDetect_TransformDetectedDevices_LabelWithoutDeviceType(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetString("host.id").AnyTimes().Return("")
	fakeConfig.EXPECT().GetDeviceOverrides().AnyTimes().Return([]models.ScanOverride{
		{Device: "/dev/sda", Label: "Backup Drive"},
	})
	fakeConfig.EXPECT().IsAllowlistedDevice(gomock.Any()).AnyTimes().Return(true)

	detectedDevices := models.Scan{
		Devices: []models.ScanDevice{
			{Name: "/dev/sda", InfoName: "/dev/sda", Protocol: "ata", Type: "scsi"},
		},
	}

	d := detect.Detect{Config: fakeConfig}
	transformedDevices := d.TransformDetectedDevices(detectedDevices)

	require.Equal(t, 1, len(transformedDevices))
	require.Equal(t, "scsi", transformedDevices[0].DeviceType)
	require.Equal(t, "Backup Drive", transformedDevices[0].Label)
}

func TestDetect_TransformDetectedDevices_NoLabel(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetString("host.id").AnyTimes().Return("")
	fakeConfig.EXPECT().GetDeviceOverrides().AnyTimes().Return([]models.ScanOverride{
		{Device: "/dev/sda", DeviceType: []string{"sat"}},
	})
	fakeConfig.EXPECT().IsAllowlistedDevice(gomock.Any()).AnyTimes().Return(true)

	detectedDevices := models.Scan{
		Devices: []models.ScanDevice{
			{Name: "/dev/sda", InfoName: "/dev/sda", Protocol: "ata", Type: "ata"},
		},
	}

	d := detect.Detect{Config: fakeConfig}
	transformedDevices := d.TransformDetectedDevices(detectedDevices)

	require.Equal(t, 1, len(transformedDevices))
	require.Equal(t, "", transformedDevices[0].Label)
}

func TestDetect_TransformDetectedDevices_IOServicePath(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetString("host.id").AnyTimes().Return("")
	fakeConfig.EXPECT().GetDeviceOverrides().AnyTimes().Return([]models.ScanOverride{})
	fakeConfig.EXPECT().IsAllowlistedDevice(gomock.Any()).AnyTimes().Return(true)

	const ioServicePath = "IOService:/AppleARMPE/arm-io@10F00000/AppleT8110AHCIE@ba010000/IOAHCIBlockStorageDevice"

	detectedDevices := models.Scan{
		Devices: []models.ScanDevice{
			{Name: ioServicePath, InfoName: ioServicePath, Protocol: "ata", Type: "ata"},
		},
	}

	d := detect.Detect{Config: fakeConfig}
	transformedDevices := d.TransformDetectedDevices(detectedDevices)

	require.Equal(t, 1, len(transformedDevices))
	// Case must be preserved — smartctl requires the exact IOService path
	require.Equal(t, ioServicePath, transformedDevices[0].DeviceName)
	require.Equal(t, "ata", transformedDevices[0].DeviceType)
}

func TestDetect_TransformDetectedDevices_IODeviceTreePath(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetString("host.id").AnyTimes().Return("")
	fakeConfig.EXPECT().GetDeviceOverrides().AnyTimes().Return([]models.ScanOverride{})
	fakeConfig.EXPECT().IsAllowlistedDevice(gomock.Any()).AnyTimes().Return(true)

	const ioDeviceTreePath = "IODeviceTree:/arm-io@10F00000/SDIO@10F00000/IOSDHostDevice/IOSDBlockStorageDevice"

	detectedDevices := models.Scan{
		Devices: []models.ScanDevice{
			{Name: ioDeviceTreePath, InfoName: ioDeviceTreePath, Protocol: "ata", Type: "ata"},
		},
	}

	d := detect.Detect{Config: fakeConfig}
	transformedDevices := d.TransformDetectedDevices(detectedDevices)

	require.Equal(t, 1, len(transformedDevices))
	// Case must be preserved — smartctl requires the exact IODeviceTree path
	require.Equal(t, ioDeviceTreePath, transformedDevices[0].DeviceName)
}

func TestDetect_DeviceFullPath_IOServicePreservesPath(t *testing.T) {
	const ioServicePath = "IOService:/AppleARMPE/arm-io@10F00000/IOAHCIBlockStorageDevice"
	// DeviceFullPath must return the IOService path verbatim (no /dev/ prefix)
	require.Equal(t, ioServicePath, detect.DeviceFullPath(ioServicePath))
}

func TestDetect_DeviceFullPath_StandardDeviceGetsPrefixed(t *testing.T) {
	// Standard device names should still receive the platform device prefix
	result := detect.DeviceFullPath("sda")
	require.True(t, strings.HasSuffix(result, "sda"))
	require.NotEqual(t, "sda", result, "standard device should have a prefix added")
}

func TestDetect_TransformDetectedDevices_RaidWithLabel(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetString("host.id").AnyTimes().Return("")
	fakeConfig.EXPECT().GetDeviceOverrides().AnyTimes().Return([]models.ScanOverride{
		{
			Device:     "/dev/bus/0",
			DeviceType: []string{"megaraid,14", "megaraid,15", "megaraid,18"},
			Label:      "RAID Controller A",
		},
	})
	fakeConfig.EXPECT().IsAllowlistedDevice(gomock.Any()).AnyTimes().Return(true)

	detectedDevices := models.Scan{
		Devices: []models.ScanDevice{
			{Name: "/dev/bus/0", InfoName: "/dev/bus/0", Protocol: "scsi", Type: "scsi"},
		},
	}

	d := detect.Detect{Config: fakeConfig}
	transformedDevices := d.TransformDetectedDevices(detectedDevices)

	require.Equal(t, 3, len(transformedDevices))
	for _, dev := range transformedDevices {
		require.Equal(t, "RAID Controller A", dev.Label)
	}
}
