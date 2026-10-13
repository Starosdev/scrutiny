package detect_test

import (
	"testing"

	mock_shell "github.com/analogj/scrutiny/collector/pkg/common/shell/mock"
	"github.com/analogj/scrutiny/collector/pkg/config"
	"github.com/analogj/scrutiny/collector/pkg/detect"
	"github.com/analogj/scrutiny/collector/pkg/models"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// infoJSON returns a minimal smartctl --info document for the given model.
func infoJSON(model, serial string) string {
	return `{"model_name":"` + model + `","serial_number":"` + serial + `","device":{"name":"/dev/x","type":"sat"}}`
}

type infoResult struct {
	out string
	err error
}

// inflate runs inflateDevices against a fake shell keyed by full device path.
func inflate(t *testing.T, cfgSetup func(config.Interface), results map[string]infoResult, names ...string) []models.Device {
	t.Helper()
	ctrl := gomock.NewController(t)
	cfg, err := config.Create()
	require.NoError(t, err)
	cfg.Set("commands.metrics_smartctl_timeout", 5)
	if cfgSetup != nil {
		cfgSetup(cfg)
	}

	fakeShell := mock_shell.NewMockInterface(ctrl)
	fakeShell.EXPECT().
		CommandContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, _ any, _ string, args []string, _ string, _ []string) (string, error) {
			res, ok := results[args[len(args)-1]]
			require.True(t, ok, "unexpected smartctl call for %v", args)
			return res.out, res.err
		}).
		AnyTimes()

	d := detect.Detect{Logger: logrus.NewEntry(logrus.New()), Shell: fakeShell, Config: cfg}
	d.SetBlockWWNFallback(func(dev *models.Device) { dev.WWN = dev.SerialNumber })

	devices := []models.Device{}
	for _, n := range names {
		devices = append(devices, models.Device{DeviceName: n, DeviceType: "sat"})
	}
	return d.InflateDevices(devices)
}

func names(devices []models.Device) []string {
	out := []string{}
	for _, d := range devices {
		out = append(out, d.DeviceName)
	}
	return out
}

func TestInflateDevices_SkipsFatalInfo(t *testing.T) {
	results := map[string]infoResult{
		"/dev/sda": {out: infoJSON("WDC WD40", "S1")},
		"/dev/sdb": {out: "", err: exitErrorWithCode(t, 2)},                         // device open failed
		"/dev/sdc": {out: infoJSON("WDC WD41", "S3"), err: exitErrorWithCode(t, 4)}, // checksum, non-fatal (#663)
	}
	got := inflate(t, nil, results, "sda", "sdb", "sdc")
	require.Equal(t, []string{"sda", "sdc"}, names(got))
	require.Equal(t, "WDC WD41", got[1].ModelName)
}

func TestInflateDevices_ExcludesVirtualDisksByDefault(t *testing.T) {
	results := map[string]infoResult{
		"/dev/sda": {out: infoJSON("QEMU HARDDISK", "q1")},
		"/dev/sdb": {out: infoJSON("Msft Virtual Disk", "m1")},
		"/dev/sdc": {out: infoJSON("VBOX HARDDISK", "v1")},
		"/dev/sdd": {out: infoJSON("Samsung SSD 860", "s1")},
	}
	require.Equal(t, []string{"sdd"}, names(inflate(t, nil, results, "sda", "sdb", "sdc", "sdd")))

	off := func(c config.Interface) { c.Set("exclude_virtual_disks", false) }
	require.Len(t, inflate(t, off, results, "sda", "sdb", "sdc", "sdd"), 4)
}

func TestInflateDevices_UserExcludePatterns(t *testing.T) {
	results := map[string]infoResult{
		"/dev/loop0": {out: infoJSON("Loop", "l0")},
		"/dev/sda":   {out: infoJSON("Longhorn Vol", "x")},
		"/dev/sdb":   {out: infoJSON("WDC WD40", "y")},
	}
	setup := func(c config.Interface) {
		c.Set("exclude", []string{`^/dev/loop`, `^Longhorn`, `([unclosed`})
	}
	// loop0 is excluded by path before any smartctl call; the invalid pattern is ignored.
	require.Equal(t, []string{"sdb"}, names(inflate(t, setup, results, "loop0", "sda", "sdb")))
}

func TestInflateDevices_ExcludeByDeviceType(t *testing.T) {
	results := map[string]infoResult{"/dev/sda": {out: infoJSON("WDC", "a")}}
	setup := func(c config.Interface) { c.Set("exclude", []string{`^sat$`}) }
	require.Empty(t, inflate(t, setup, results, "sda"))
}
