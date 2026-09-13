package detect_test

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
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

// fixes #663: a QNAP TR-004 in Individual mode addresses all four bays through one
// device file, one `jmb39x-q,N` device type per bay.
func TestDetect_TransformDetectedDevices_Jmb39xIndividualMode(t *testing.T) {
	// setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetString("host.id").AnyTimes().Return("")
	fakeConfig.EXPECT().IsAllowlistedDevice(gomock.Any()).AnyTimes().Return(true)
	fakeConfig.EXPECT().GetDeviceOverrides().AnyTimes().Return([]models.ScanOverride{
		{
			Device:     "/dev/sda",
			DeviceType: []string{"jmb39x-q,0", "jmb39x-q,1", "jmb39x-q,2", "jmb39x-q,3"},
			Ignore:     false,
		},
	})
	detectedDevices := models.Scan{
		Devices: []models.ScanDevice{
			{
				Name:     "/dev/sda",
				InfoName: "/dev/sda",
				Protocol: "ATA",
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
	require.Equal(t, 4, len(transformedDevices))
	deviceTypes := []string{}
	for _, transformedDevice := range transformedDevices {
		assert.Equal(t, "sda", transformedDevice.DeviceName)
		deviceTypes = append(deviceTypes, transformedDevice.DeviceType)
	}
	require.Equal(t, []string{"jmb39x-q,0", "jmb39x-q,1", "jmb39x-q,2", "jmb39x-q,3"}, deviceTypes)
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

func TestDetect_FilterRedundantDevices_PrefersControllerBackedEntry(t *testing.T) {
	devices := []models.Device{
		{
			DeviceName: "sda",
			DeviceType: "scsi",
		},
		{
			DeviceName:         "bus/0",
			DeviceType:         "megaraid,0",
			ResolvedDeviceName: "sda",
		},
		{
			DeviceName: "sdb",
			DeviceType: "scsi",
		},
	}

	filtered := detect.FilterRedundantDevices(devices)

	require.Equal(t, []models.Device{
		{
			DeviceName:         "bus/0",
			DeviceType:         "megaraid,0",
			ResolvedDeviceName: "sda",
		},
		{
			DeviceName: "sdb",
			DeviceType: "scsi",
		},
	}, filtered)
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
		fakeConfig.EXPECT().
			GetDeviceOverrides().
			Return(nil).
			AnyTimes()

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
		require.True(t, someDevice.SmartSupport.Available)
		require.NotNil(t, someDevice.SmartSupport.Enabled)
		require.True(t, *someDevice.SmartSupport.Enabled)
	})

	// fixes #663: a QNAP TR-004 enclosure reports "Read Device Statistics page 0x00
	// failed" and exits 4 on every run, while still printing a complete JSON document.
	t.Run("should accept usable JSON on a non-fatal exit status", func(t *testing.T) {
		fakeShell, fakeConfig, someLogger := setupSmartCtlInfoMocks(t, "sda", "jmb39x-q,0",
			"testdata/smartctl_info_jmb39x_exit4.json", exitErrorWithCode(t, 4))

		d := detect.Detect{Logger: someLogger, Shell: fakeShell, Config: fakeConfig}
		someDevice := &models.Device{DeviceName: "sda", DeviceType: "jmb39x-q,0"}

		require.NoError(t, d.SmartCtlInfo(someDevice))

		assert.Equal(t, "WDC WD40EFRX-68N32N0", someDevice.ModelName)
		assert.Equal(t, "WD-WCC7K0000000", someDevice.SerialNumber)
		assert.Equal(t, "82.00A82", someDevice.Firmware)
		assert.Equal(t, int64(4000787030016), someDevice.Capacity)
		assert.Equal(t, 5400, someDevice.RotationSpeed)
		// an empty WWN is what produced the "No Data" dashboard: every InfluxDB query
		// keys on the device_wwn tag.
		require.NotEmpty(t, someDevice.WWN)
		assert.Equal(t, strings.ToLower(someDevice.WWN), someDevice.WWN)
		// invariant: a user-supplied device type is never overwritten by smartctl's own
		assert.Equal(t, "jmb39x-q,0", someDevice.DeviceType)
	})

	// SCSI/SAS drives don't populate "firmware_version" in smartctl's JSON
	// output; the equivalent value is reported under "scsi_revision" instead.
	// SmartCtlInfo must fall back to it so device.Firmware is not left blank.
	t.Run("should populate firmware from scsi_revision for a SAS SSD", func(t *testing.T) {
		fakeShell, fakeConfig, someLogger := setupSmartCtlInfoArgvMocks(t, "sdp",
			[]string{"--info", "--json", "/dev/sdp"},
			"testdata/smartctl_info_sas_ssd.json", nil, nil)

		d := detect.Detect{Logger: someLogger, Shell: fakeShell, Config: fakeConfig}
		someDevice := &models.Device{DeviceName: "sdp"}

		require.NoError(t, d.SmartCtlInfo(someDevice))

		assert.Equal(t, "HPE VO003840JWZJK", someDevice.ModelName)
		assert.Equal(t, "ABCDEFGHIJKLMNOP", someDevice.SerialNumber)
		assert.Equal(t, "HPD5", someDevice.Firmware)
		assert.Equal(t, "SCSI", someDevice.DeviceProtocol)
	})

	// A run where smartctl reports neither firmware_version nor scsi_revision
	// (e.g. transient error, permissions, or args without -i/-x) must not blank
	// out a firmware value already on file for the device.
	t.Run("should not clear a previously known firmware when missing from payload", func(t *testing.T) {
		fakeShell, fakeConfig, someLogger := setupSmartCtlInfoArgvMocks(t, "sdp",
			[]string{"--info", "--json", "/dev/sdp"},
			"testdata/smartctl_info_sas_ssd_no_revision.json", nil, nil)

		d := detect.Detect{Logger: someLogger, Shell: fakeShell, Config: fakeConfig}
		someDevice := &models.Device{DeviceName: "sdp", Firmware: "HPD5"}

		require.NoError(t, d.SmartCtlInfo(someDevice))

		assert.Equal(t, "HPE VO003840JWZJK", someDevice.ModelName)
		assert.Equal(t, "HPD5", someDevice.Firmware)
	})

	t.Run("should reject output on a fatal exit status", func(t *testing.T) {
		fakeShell, fakeConfig, someLogger := setupSmartCtlInfoMocks(t, "sda", "jmb39x-q,0",
			"testdata/smartctl_info_jmb39x_exit4.json", exitErrorWithCode(t, 2))

		d := detect.Detect{Logger: someLogger, Shell: fakeShell, Config: fakeConfig}
		someDevice := &models.Device{DeviceName: "sda", DeviceType: "jmb39x-q,0"}

		require.Error(t, d.SmartCtlInfo(someDevice))

		// nothing may be copied out of output we decided not to trust
		assert.Empty(t, someDevice.ModelName)
		assert.Empty(t, someDevice.SerialNumber)
		assert.Empty(t, someDevice.WWN)
	})

	t.Run("should reject empty output on a non-fatal exit status", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		fakeConfig := mock_config.NewMockInterface(ctrl)
		fakeConfig.EXPECT().GetCommandMetricsInfoArgs("/dev/sda").Return("--info --json")
		fakeConfig.EXPECT().GetString("commands.metrics_smartctl_bin").Return("smartctl")
		fakeConfig.EXPECT().GetInt("commands.metrics_smartctl_timeout").Return(120)
		fakeConfig.EXPECT().GetDeviceOverrides().Return(nil).AnyTimes()

		someLogger := logrus.WithFields(logrus.Fields{})
		fakeShell := mock_shell.NewMockInterface(ctrl)
		fakeShell.EXPECT().
			CommandContext(gomock.Any(), someLogger, "smartctl", gomock.Any(), "", gomock.Any()).
			Return("   ", exitErrorWithCode(t, 4))

		d := detect.Detect{Logger: someLogger, Shell: fakeShell, Config: fakeConfig}
		someDevice := &models.Device{DeviceName: "sda"}

		require.Error(t, d.SmartCtlInfo(someDevice))
	})

	// fixes #663: `jmb39x-q,0` must reach smartctl as one --device argument. The nvme
	// subtest above uses an empty device type, so it never exercises this branch.
	t.Run("should pass a comma-containing device type as one --device argument", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		fakeConfig := mock_config.NewMockInterface(ctrl)
		fakeConfig.EXPECT().GetCommandMetricsInfoArgs("/dev/sda").Return("--info --json")
		fakeConfig.EXPECT().GetString("commands.metrics_smartctl_bin").Return("smartctl")
		fakeConfig.EXPECT().GetInt("commands.metrics_smartctl_timeout").Return(120)
		fakeConfig.EXPECT().GetDeviceOverrides().Return(nil).AnyTimes()

		smartctlInfoResults, err := os.ReadFile("testdata/smartctl_info_jmb39x_exit4.json")
		require.NoError(t, err)

		someLogger := logrus.WithFields(logrus.Fields{})
		fakeShell := mock_shell.NewMockInterface(ctrl)
		fakeShell.EXPECT().
			CommandContext(gomock.Any(), someLogger, "smartctl",
				[]string{"--info", "--json", "--device", "jmb39x-q,0", "/dev/sda"},
				"", gomock.Any()).
			Return(string(smartctlInfoResults), nil)

		d := detect.Detect{Logger: someLogger, Shell: fakeShell, Config: fakeConfig}
		someDevice := &models.Device{DeviceName: "sda", DeviceType: "jmb39x-q,0"}

		require.NoError(t, d.SmartCtlInfo(someDevice))
	})

	// fixes #664: some Hitachi and Toshiba drives report a false failure and zero
	// SMART values under auto-detection, and only return usable data with an explicit
	// type. The configured type must therefore reach smartctl on the detection call.
	t.Run("should pass an explicitly configured device type", func(t *testing.T) {
		fakeShell, fakeConfig, someLogger := setupSmartCtlInfoArgvMocks(t, "sda",
			[]string{"--info", "--json", "--device", "sat", "/dev/sda"},
			"testdata/smartctl_info_hitachi_sat.json", nil,
			[]models.ScanOverride{{Device: "/dev/sda", DeviceType: []string{"sat"}}})

		d := detect.Detect{Logger: someLogger, Shell: fakeShell, Config: fakeConfig}
		someDevice := &models.Device{DeviceName: "sda", DeviceType: "sat"}

		require.NoError(t, d.SmartCtlInfo(someDevice))

		// the zero-metric symptom: without the explicit type this drive answers with no
		// model, no serial, no WWN and smart_support.available false.
		assert.Equal(t, "Hitachi HDS723030ALA640", someDevice.ModelName)
		assert.Equal(t, "MK0000000000000", someDevice.SerialNumber)
		assert.Equal(t, int64(3000592982016), someDevice.Capacity)
		assert.Equal(t, 7200, someDevice.RotationSpeed)
		require.NotEmpty(t, someDevice.WWN)
		require.True(t, someDevice.SmartSupport.Available)
		assert.Equal(t, "sat", someDevice.DeviceType)
	})

	// the failure this fixture captures is what the user sees before configuring an
	// explicit type: smartctl auto-detects the drive as scsi and reports nothing usable.
	t.Run("should record the zero-metric response when the type is auto-detected", func(t *testing.T) {
		fakeShell, fakeConfig, someLogger := setupSmartCtlInfoArgvMocks(t, "sda",
			[]string{"--info", "--json", "/dev/sda"},
			"testdata/smartctl_info_hitachi_autodetect.json", exitErrorWithCode(t, 4), nil)

		d := detect.Detect{Logger: someLogger, Shell: fakeShell, Config: fakeConfig}
		someDevice := &models.Device{DeviceName: "sda", DeviceType: "scsi"}

		require.NoError(t, d.SmartCtlInfo(someDevice))

		assert.Empty(t, someDevice.ModelName)
		assert.Empty(t, someDevice.SerialNumber)
		assert.False(t, someDevice.SmartSupport.Available)
		// WWN is deliberately not asserted: with no wwn block in the response the
		// platform-specific wwnFallback runs, and on a Linux host it reports whatever
		// real disk happens to sit at /dev/sda.
	})

	// fixes #664: "scsi" and "ata" are suppressed only because `smartctl --scan`
	// mislabels ATA drives as scsi in docker. A type the user wrote down is an
	// instruction, not a guess, and must be passed even when it is one of those two.
	t.Run("should pass an explicitly configured standard device type", func(t *testing.T) {
		fakeShell, fakeConfig, someLogger := setupSmartCtlInfoArgvMocks(t, "sda",
			[]string{"--info", "--json", "--device", "scsi", "/dev/sda"},
			"testdata/smartctl_info_hitachi_sat.json", nil,
			[]models.ScanOverride{{Device: "/dev/sda", DeviceType: []string{"scsi"}}})

		d := detect.Detect{Logger: someLogger, Shell: fakeShell, Config: fakeConfig}
		someDevice := &models.Device{DeviceName: "sda", DeviceType: "scsi"}

		require.NoError(t, d.SmartCtlInfo(someDevice))
	})

	// a configured device with no type of its own inherits the scanned type, which is
	// still a guess, so the suppression has to survive the override lookup.
	t.Run("should suppress a scanned standard device type for a configured device", func(t *testing.T) {
		fakeShell, fakeConfig, someLogger := setupSmartCtlInfoArgvMocks(t, "sda",
			[]string{"--info", "--json", "/dev/sda"},
			"testdata/smartctl_info_hitachi_sat.json", nil,
			[]models.ScanOverride{{Device: "/dev/sda", Label: "some label"}})

		d := detect.Detect{Logger: someLogger, Shell: fakeShell, Config: fakeConfig}
		someDevice := &models.Device{DeviceName: "sda", DeviceType: "scsi"}

		require.NoError(t, d.SmartCtlInfo(someDevice))
	})

	// the old guard compared the raw string, so an uppercase scanned type slipped
	// through as `--device SCSI` while its lowercase twin was suppressed.
	t.Run("should suppress an uppercase scanned standard device type", func(t *testing.T) {
		fakeShell, fakeConfig, someLogger := setupSmartCtlInfoArgvMocks(t, "sda",
			[]string{"--info", "--json", "/dev/sda"},
			"testdata/smartctl_info_hitachi_sat.json", nil, nil)

		d := detect.Detect{Logger: someLogger, Shell: fakeShell, Config: fakeConfig}
		someDevice := &models.Device{DeviceName: "sda", DeviceType: "SCSI"}

		require.NoError(t, d.SmartCtlInfo(someDevice))
	})
}

func TestDetect_AppendDeviceTypeArgs(t *testing.T) {
	someOverrides := []models.ScanOverride{
		{Device: "/dev/sda", DeviceType: []string{"sat"}},
		{Device: "/dev/SDB", DeviceType: []string{"ata"}},
		{Device: "/dev/sdc", DeviceType: []string{"jmb39x-q,0", "jmb39x-q,1"}},
	}

	testCases := []struct {
		name           string
		fullDeviceName string
		deviceType     string
		expected       []string
	}{
		{"no type", "/dev/sdz", "", []string{"--info"}},
		{"whitespace only type", "/dev/sdz", "   ", []string{"--info"}},
		{"scanned scsi is suppressed", "/dev/sdz", "scsi", []string{"--info"}},
		{"scanned ata is suppressed", "/dev/sdz", "ata", []string{"--info"}},
		{"scanned nvme is passed", "/dev/sdz", "nvme", []string{"--info", "--device", "nvme"}},
		{"scanned controller type is passed", "/dev/sdz", "megaraid,4", []string{"--info", "--device", "megaraid,4"}},
		{"configured sat is passed", "/dev/sda", "sat", []string{"--info", "--device", "sat"}},
		{"configured ata is passed with a case-insensitive path match", "/dev/sdb", "ata", []string{"--info", "--device", "ata"}},
		{"configured type list member is passed", "/dev/sdc", "jmb39x-q,1", []string{"--info", "--device", "jmb39x-q,1"}},
		{"type configured for another device is not honored", "/dev/sdz", "sat", []string{"--info", "--device", "sat"}},
		{"standard type configured for another device is suppressed", "/dev/sdz", "ata", []string{"--info"}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.expected,
				detect.AppendDeviceTypeArgs([]string{"--info"}, someOverrides, testCase.fullDeviceName, testCase.deviceType))
		})
	}
}

// setupSmartCtlInfoArgvMocks wires a shell that only answers the exact argv given,
// with the configured device overrides in place. Use it when the assertion is about
// which arguments reach smartctl; an argv mismatch fails the test through gomock.
func setupSmartCtlInfoArgvMocks(t *testing.T, deviceName string, expectedArgs []string, fixturePath string, shellErr error, overrides []models.ScanOverride) (*mock_shell.MockInterface, *mock_config.MockInterface, *logrus.Entry) {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	fakeConfig := mock_config.NewMockInterface(ctrl)
	fakeConfig.EXPECT().GetCommandMetricsInfoArgs("/dev/" + deviceName).Return("--info --json")
	fakeConfig.EXPECT().GetString("commands.metrics_smartctl_bin").Return("smartctl")
	fakeConfig.EXPECT().GetInt("commands.metrics_smartctl_timeout").Return(120)
	fakeConfig.EXPECT().GetDeviceOverrides().Return(overrides).AnyTimes()

	smartctlInfoResults, err := os.ReadFile(fixturePath)
	require.NoError(t, err)

	someLogger := logrus.WithFields(logrus.Fields{})
	fakeShell := mock_shell.NewMockInterface(ctrl)
	fakeShell.EXPECT().
		CommandContext(gomock.Any(), someLogger, "smartctl", expectedArgs, "", gomock.Any()).
		Return(string(smartctlInfoResults), shellErr)

	return fakeShell, fakeConfig, someLogger
}

// setupSmartCtlInfoMocks wires a shell that returns the given fixture and error for
// any smartctl invocation. Use it when the assertion is about the parsed result
// rather than about the exact argv.
func setupSmartCtlInfoMocks(t *testing.T, deviceName string, deviceType string, fixturePath string, shellErr error) (*mock_shell.MockInterface, *mock_config.MockInterface, *logrus.Entry) {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	fakeConfig := mock_config.NewMockInterface(ctrl)
	fakeConfig.EXPECT().GetCommandMetricsInfoArgs("/dev/" + deviceName).Return("--info --json")
	fakeConfig.EXPECT().GetString("commands.metrics_smartctl_bin").Return("smartctl")
	fakeConfig.EXPECT().GetInt("commands.metrics_smartctl_timeout").Return(120)
	fakeConfig.EXPECT().GetDeviceOverrides().Return(nil).AnyTimes()

	smartctlInfoResults, err := os.ReadFile(fixturePath)
	require.NoError(t, err)

	someLogger := logrus.WithFields(logrus.Fields{})
	fakeShell := mock_shell.NewMockInterface(ctrl)
	fakeShell.EXPECT().
		CommandContext(gomock.Any(), someLogger, "smartctl",
			[]string{"--info", "--json", "--device", deviceType, "/dev/" + deviceName},
			"", gomock.Any()).
		Return(string(smartctlInfoResults), shellErr)

	return fakeShell, fakeConfig, someLogger
}

// exitErrorWithCode returns a genuine *exec.ExitError carrying the requested exit
// status. os.ProcessState cannot be constructed directly, so a child process is
// required; re-executing the test binary keeps this working on every platform the
// collector builds for, unlike shelling out to `sh -c "exit N"`.
func exitErrorWithCode(t *testing.T, code int) *exec.ExitError {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestDetectExitCodeHelperProcess$")
	cmd.Env = append(os.Environ(), fmt.Sprintf("GO_DETECT_HELPER_EXIT_CODE=%d", code))

	var exitErr *exec.ExitError
	require.ErrorAs(t, cmd.Run(), &exitErr, "helper process should fail with an ExitError")
	require.Equal(t, code, exitErr.ExitCode())
	return exitErr
}

// TestDetectExitCodeHelperProcess is not a real test. It is the child process that
// exitErrorWithCode re-executes in order to produce a real *exec.ExitError.
func TestDetectExitCodeHelperProcess(t *testing.T) {
	code := os.Getenv("GO_DETECT_HELPER_EXIT_CODE")
	if code == "" {
		t.Skip("helper process; not meant to be run directly")
	}
	parsed, err := strconv.Atoi(code)
	require.NoError(t, err)
	os.Exit(parsed)
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

func TestDetect_TransformDetectedDevices_UppercaseByIDPath(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetString("host.id").AnyTimes().Return("")
	fakeConfig.EXPECT().GetDeviceOverrides().AnyTimes().Return([]models.ScanOverride{
		{Device: "/dev/disk/by-id/ata-HGST_HUH721212ALE600_ABC123"},
	})
	fakeConfig.EXPECT().IsAllowlistedDevice(gomock.Any()).AnyTimes().Return(true)

	// Simulate smartctl --scan returning no results; the device is config-only.
	detectedDevices := models.Scan{}

	d := detect.Detect{Config: fakeConfig}
	transformedDevices := d.TransformDetectedDevices(detectedDevices)

	require.Equal(t, 1, len(transformedDevices))
	// The uppercase by-id path must be preserved verbatim so smartctl can resolve it.
	require.Equal(t, "disk/by-id/ata-HGST_HUH721212ALE600_ABC123", transformedDevices[0].DeviceName)
	require.Equal(t, "ata", transformedDevices[0].DeviceType)
}

func TestDetect_TransformDetectedDevices_UppercaseByIDPathScanned(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetString("host.id").AnyTimes().Return("")
	fakeConfig.EXPECT().GetDeviceOverrides().AnyTimes().Return([]models.ScanOverride{})
	fakeConfig.EXPECT().IsAllowlistedDevice(gomock.Any()).AnyTimes().Return(true)

	// Simulate smartctl --scan returning a by-id path with uppercase characters.
	const byIDPath = "/dev/disk/by-id/ata-HGST_HUH721212ALE600_ABC123"
	detectedDevices := models.Scan{
		Devices: []models.ScanDevice{
			{Name: byIDPath, InfoName: byIDPath, Protocol: "ata", Type: "ata"},
		},
	}

	d := detect.Detect{Config: fakeConfig}
	transformedDevices := d.TransformDetectedDevices(detectedDevices)

	require.Equal(t, 1, len(transformedDevices))
	// Case must be preserved so smartctl receives the correct path.
	require.Equal(t, "disk/by-id/ata-HGST_HUH721212ALE600_ABC123", transformedDevices[0].DeviceName)
}

func TestDetect_TransformDetectedDevices_EmptyDeviceName(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetString("host.id").AnyTimes().Return("")
	fakeConfig.EXPECT().GetDeviceOverrides().AnyTimes().Return([]models.ScanOverride{})
	fakeConfig.EXPECT().IsAllowlistedDevice(gomock.Any()).AnyTimes().Return(true)

	detectedDevices := models.Scan{
		Devices: []models.ScanDevice{
			{Name: "/dev/sda", InfoName: "/dev/sda", Protocol: "scsi", Type: "scsi"},
			{Name: "", InfoName: "", Protocol: "ata", Type: "ata"},
		},
	}

	d := detect.Detect{
		Logger: logrus.WithFields(logrus.Fields{}),
		Config: fakeConfig,
	}

	transformedDevices := d.TransformDetectedDevices(detectedDevices)

	require.Equal(t, 1, len(transformedDevices))
	require.Equal(t, "sda", transformedDevices[0].DeviceName)
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
