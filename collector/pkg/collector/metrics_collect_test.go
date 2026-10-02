package collector

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	mock_shell "github.com/analogj/scrutiny/collector/pkg/common/shell/mock"
	collectorconfig "github.com/analogj/scrutiny/collector/pkg/config"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

const someSmartPayload = `{"smartctl":{"exit_status":4},"model_name":"WDC WD40EFRX-68N32N0"}`

// fixes #663: `jmb39x-q,0` must reach smartctl as a single --device argument on the
// SMART collection call, exactly as it does on the detection call.
func TestMetricsCollector_Collect_PassesCommaDeviceTypeAsOneArgument(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	fakeShell := mock_shell.NewMockInterface(ctrl)
	fakeShell.EXPECT().
		CommandContext(gomock.Any(), gomock.Any(), "smartctl",
			[]string{"--xall", "--json", "--device", "jmb39x-q,0", "/dev/sda"},
			"", gomock.Any()).
		Return(someSmartPayload, nil)

	var published int32
	mc := newTestCollectCollector(t, fakeShell, func(*http.Request) {
		atomic.AddInt32(&published, 1)
	})

	mc.Collect("some-device-id", "sda", "jmb39x-q,0")

	require.Equal(t, int32(1), atomic.LoadInt32(&published), "SMART payload should be published")
}

// The FARM call is the third site that appends --device and must stay identical to
// the other two.
func TestMetricsCollector_Collect_PassesCommaDeviceTypeToFarmCall(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	fakeShell := mock_shell.NewMockInterface(ctrl)
	fakeShell.EXPECT().
		CommandContext(gomock.Any(), gomock.Any(), "smartctl",
			[]string{"--xall", "--json", "--device", "jmb39x-q,0", "/dev/sda"},
			"", gomock.Any()).
		Return(someSmartPayload, nil)
	fakeShell.EXPECT().
		CommandContext(gomock.Any(), gomock.Any(), "smartctl",
			[]string{"-l", "farm", "--json", "--device", "jmb39x-q,0", "/dev/sda"},
			"", gomock.Any()).
		Return(`{"seagate_farm_log":{}}`, nil)

	mc := newTestCollectCollector(t, fakeShell, nil)
	mc.config.Set("commands.metrics_farm_enabled", true)

	mc.Collect("some-device-id", "sda", "jmb39x-q,0")
}

// fixes #664: an explicit type is the documented remedy for Hitachi and Toshiba
// drives that report a false failure and zero SMART values under auto-detection, so
// it has to reach the SMART and FARM calls, not only the detection call.
func TestMetricsCollector_Collect_PassesConfiguredDeviceType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	fakeShell := mock_shell.NewMockInterface(ctrl)
	fakeShell.EXPECT().
		CommandContext(gomock.Any(), gomock.Any(), "smartctl",
			[]string{"--xall", "--json", "--device", "sat", "/dev/sda"},
			"", gomock.Any()).
		Return(someSmartPayload, nil)
	fakeShell.EXPECT().
		CommandContext(gomock.Any(), gomock.Any(), "smartctl",
			[]string{"-l", "farm", "--json", "--device", "sat", "/dev/sda"},
			"", gomock.Any()).
		Return(`{"seagate_farm_log":{}}`, nil)

	mc := newTestCollectCollector(t, fakeShell, nil)
	mc.config.Set("commands.metrics_farm_enabled", true)
	setDeviceOverrides(mc, map[string]interface{}{"device": "/dev/sda", "type": "sat"})

	mc.Collect("some-device-id", "sda", "sat")
}

// fixes #664: "scsi" and "ata" are suppressed only because `smartctl --scan`
// mislabels ATA drives as scsi in docker. A configured type is an instruction and is
// passed even when it is one of those two.
func TestMetricsCollector_Collect_PassesConfiguredStandardDeviceType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	fakeShell := mock_shell.NewMockInterface(ctrl)
	fakeShell.EXPECT().
		CommandContext(gomock.Any(), gomock.Any(), "smartctl",
			[]string{"--xall", "--json", "--device", "scsi", "/dev/sda"},
			"", gomock.Any()).
		Return(someSmartPayload, nil)

	mc := newTestCollectCollector(t, fakeShell, nil)
	setDeviceOverrides(mc, map[string]interface{}{"device": "/dev/sda", "type": "scsi"})

	mc.Collect("some-device-id", "sda", "scsi")
}

// A scanned "scsi" with nothing configured keeps today's behavior: no --device, so a
// drive that docker mislabels as scsi still reports its ATA metadata.
func TestMetricsCollector_Collect_SuppressesScannedStandardDeviceType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	fakeShell := mock_shell.NewMockInterface(ctrl)
	fakeShell.EXPECT().
		CommandContext(gomock.Any(), gomock.Any(), "smartctl",
			[]string{"--xall", "--json", "/dev/sda"},
			"", gomock.Any()).
		Return(someSmartPayload, nil)
	fakeShell.EXPECT().
		CommandContext(gomock.Any(), gomock.Any(), "smartctl",
			[]string{"-l", "farm", "--json", "/dev/sda"},
			"", gomock.Any()).
		Return(`{"seagate_farm_log":{}}`, nil)

	mc := newTestCollectCollector(t, fakeShell, nil)
	mc.config.Set("commands.metrics_farm_enabled", true)

	mc.Collect("some-device-id", "sda", "scsi")
}

// Only exit-status bits 0-1 invalidate the payload. Bit 2 accompanies the QNAP
// TR-004 "Read Device Statistics page 0x00 failed" warning and must still publish.
func TestMetricsCollector_Collect_PublishesOnNonFatalExitStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	fakeShell := mock_shell.NewMockInterface(ctrl)
	fakeShell.EXPECT().
		CommandContext(gomock.Any(), gomock.Any(), "smartctl", gomock.Any(), "", gomock.Any()).
		Return(someSmartPayload, collectExitErrorWithCode(t, 4))

	var publishedPaths []string
	mc := newTestCollectCollector(t, fakeShell, func(req *http.Request) {
		publishedPaths = append(publishedPaths, req.URL.Path)
	})

	mc.Collect("some-device-id", "sda", "jmb39x-q,0")

	require.Equal(t, []string{"/api/device/some-device-id/smart"}, publishedPaths)
}

func TestMetricsCollector_Collect_ReportsErrorOnFatalExitStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	fakeShell := mock_shell.NewMockInterface(ctrl)
	fakeShell.EXPECT().
		CommandContext(gomock.Any(), gomock.Any(), "smartctl", gomock.Any(), "", gomock.Any()).
		Return(someSmartPayload, collectExitErrorWithCode(t, 2))

	var publishedPaths []string
	mc := newTestCollectCollector(t, fakeShell, func(req *http.Request) {
		publishedPaths = append(publishedPaths, req.URL.Path)
	})

	mc.Collect("some-device-id", "sda", "jmb39x-q,0")

	// the SMART payload must not be published, only the collector-error report
	require.Equal(t, []string{"/api/device/some-device-id/collector-error"}, publishedPaths)
}

// fixes #911: with `-n standby` smartctl skips a sleeping drive and exits 2. That
// is not a failure: nothing is published and no collector error is reported. The
// `-n standby,0` form exits 0 and must be skipped the same way.
func TestMetricsCollector_Collect_SkipsDriveInStandby(t *testing.T) {
	const standbyPayload = `{"smartctl":{"exit_status":%d,"messages":[{"string":"Device is in STANDBY mode, exit(%d)","severity":"information"}]}}`

	for _, code := range []int{2, 0} {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			var runErr error
			if code != 0 {
				runErr = collectExitErrorWithCode(t, code)
			}
			fakeShell := mock_shell.NewMockInterface(ctrl)
			fakeShell.EXPECT().
				CommandContext(gomock.Any(), gomock.Any(), "smartctl", gomock.Any(), "", gomock.Any()).
				Return(fmt.Sprintf(standbyPayload, code, code), runErr)

			var publishedPaths []string
			mc := newTestCollectCollector(t, fakeShell, func(req *http.Request) {
				publishedPaths = append(publishedPaths, req.URL.Path)
			})

			mc.Collect("some-device-id", "sda", "sat")

			require.Empty(t, publishedPaths)
		})
	}
}

// newTestCollectCollector builds a MetricsCollector wired to the given shell, with
// every HTTP request answered 200 and optionally handed to observe first.
func newTestCollectCollector(t *testing.T, fakeShell *mock_shell.MockInterface, observe func(*http.Request)) MetricsCollector {
	t.Helper()

	cfg, err := collectorconfig.Create()
	require.NoError(t, err)
	cfg.Set(configKeyMetricsAPIRetryCount, 0)
	cfg.Set(configKeyMetricsAPIRetryDelay, 0)

	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if observe != nil {
				observe(req)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"success":true}`)),
			}, nil
		}),
	}

	logger := logrus.New()
	logger.SetOutput(io.Discard)

	parsedURL, err := url.Parse("http://example.com/")
	require.NoError(t, err)

	return MetricsCollector{
		config:      cfg,
		apiEndpoint: parsedURL,
		shell:       fakeShell,
		BaseCollector: BaseCollector{
			logger:     logrus.NewEntry(logger),
			httpClient: client,
		},
	}
}

// setDeviceOverrides writes a 'devices' section into the collector's config, the way
// a user would in collector.yaml. GetDeviceOverrides memoizes its result, so this has
// to run before the first call that reads it.
func setDeviceOverrides(mc MetricsCollector, overrides ...map[string]interface{}) {
	mc.config.Set("devices", overrides)
}

// collectExitErrorWithCode returns a genuine *exec.ExitError carrying the requested
// exit status. os.ProcessState cannot be constructed directly, so a child process is
// required; re-executing the test binary keeps this portable.
func collectExitErrorWithCode(t *testing.T, code int) *exec.ExitError {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestCollectExitCodeHelperProcess$")
	cmd.Env = append(os.Environ(), fmt.Sprintf("GO_COLLECT_HELPER_EXIT_CODE=%d", code))

	var exitErr *exec.ExitError
	require.ErrorAs(t, cmd.Run(), &exitErr, "helper process should fail with an ExitError")
	require.Equal(t, code, exitErr.ExitCode())
	return exitErr
}

// TestCollectExitCodeHelperProcess is not a real test. It is the child process that
// collectExitErrorWithCode re-executes to produce a real *exec.ExitError.
func TestCollectExitCodeHelperProcess(t *testing.T) {
	code := os.Getenv("GO_COLLECT_HELPER_EXIT_CODE")
	if code == "" {
		t.Skip("helper process; not meant to be run directly")
	}
	parsed, err := strconv.Atoi(code)
	require.NoError(t, err)
	os.Exit(parsed)
}
