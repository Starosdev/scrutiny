package performance

import (
	"os"
	"path/filepath"
	"testing"

	mock_config "github.com/analogj/scrutiny/collector/pkg/config/mock"
	"github.com/analogj/scrutiny/collector/pkg/models"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// newTestCollector builds a collector whose configuration answers only the keys
// resolveTargetPath reads, so each test states the configuration it depends on.
func newTestCollector(t *testing.T, mountPoint string, allowDirectIO bool) *Collector {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	fakeConfig := mock_config.NewMockInterface(ctrl)
	fakeConfig.EXPECT().GetString("performance.mount_points.sda").Return(mountPoint).AnyTimes()
	fakeConfig.EXPECT().GetString("performance.mount_points.0x5000c500a0a0a0a0").Return("").AnyTimes()
	fakeConfig.EXPECT().GetBool("performance.allow_direct_device_io").Return(allowDirectIO).AnyTimes()
	fakeConfig.EXPECT().GetString("performance.profile").Return("quick").AnyTimes()
	fakeConfig.EXPECT().GetString("performance.temp_file_size").Return("1K").AnyTimes()

	return &Collector{
		config: fakeConfig,
		logger: logrus.WithField("type", "performance_test"),
	}
}

func testDevice() *models.Device {
	return &models.Device{DeviceName: "sda", WWN: "0x5000c500a0a0a0a0"}
}

// requireBenchmarkableDir skips a test when the temporary directory cannot host
// a benchmark file. Some CI runners put the temporary directory on a tmpfs,
// which the collector rejects on purpose, and that is not a failure of the code
// under test.
func requireBenchmarkableDir(t *testing.T, collector *Collector, dir string) {
	t.Helper()
	if usable, reason := collector.mountPointUsable(dir); !usable {
		t.Skipf("temporary directory %s cannot host a benchmark file: %s", dir, reason)
	}
}

func TestResolveTargetPath_UsesConfiguredMountPoint(t *testing.T) {
	dir := t.TempDir()
	collector := newTestCollector(t, dir, false)
	requireBenchmarkableDir(t, collector, dir)

	target, cleanup, directIO, err := collector.resolveTargetPath(testDevice())

	require.NoError(t, err)
	require.False(t, directIO, "a configured mount point must enable write tests")
	require.Equal(t, filepath.Join(dir, ".scrutiny_perf_bench"), target)
	require.NotNil(t, cleanup)
}

func TestResolveTargetPath_ResolvesConfiguredSymlink(t *testing.T) {
	// The checks read the mount table lexically while the write follows the
	// link, so the benchmark must run against the resolved path.
	root := t.TempDir()
	realDir := filepath.Join(root, "real")
	require.NoError(t, os.Mkdir(realDir, 0o755))
	link := filepath.Join(root, "link")
	require.NoError(t, os.Symlink(realDir, link))

	collector := newTestCollector(t, link, false)
	requireBenchmarkableDir(t, collector, realDir)

	target, _, directIO, err := collector.resolveTargetPath(testDevice())

	require.NoError(t, err)
	require.False(t, directIO)
	resolvedRoot, err := filepath.EvalSymlinks(realDir)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(resolvedRoot, ".scrutiny_perf_bench"), target)
}

func TestResolveTargetPath_UnusableConfiguredMountFallsBack(t *testing.T) {
	// A configured path that cannot hold the test file must not be used, and
	// must not stop the device being benchmarked read-only.
	notADirectory := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(notADirectory, []byte("x"), 0o600))

	target, cleanup, directIO, err := newTestCollector(t, notADirectory, false).resolveTargetPath(testDevice())

	require.NoError(t, err)
	require.True(t, directIO, "an unusable configured mount point falls back to raw device I/O")
	require.Equal(t, "/dev/sda", target)
	require.Nil(t, cleanup, "raw device I/O creates no file to clean up")
}

func TestResolveTargetPath_MissingConfiguredMountFallsBack(t *testing.T) {
	target, _, directIO, err := newTestCollector(t, filepath.Join(t.TempDir(), "absent"), false).resolveTargetPath(testDevice())

	require.NoError(t, err)
	require.True(t, directIO)
	require.Equal(t, "/dev/sda", target)
}

func TestResolveTargetPath_DirectDeviceIOOptIn(t *testing.T) {
	target, cleanup, directIO, err := newTestCollector(t, "", true).resolveTargetPath(testDevice())

	require.NoError(t, err)
	require.True(t, directIO)
	require.Equal(t, "/dev/sda", target)
	require.Nil(t, cleanup)
}

func TestResolveTargetPath_ConfiguredMountPointBeatsDirectIOOptIn(t *testing.T) {
	// Documented precedence: a configured mount point is checked before the
	// raw device opt-in, so the flag alone does not guarantee read-only runs.
	dir := t.TempDir()
	collector := newTestCollector(t, dir, true)
	requireBenchmarkableDir(t, collector, dir)

	target, _, directIO, err := collector.resolveTargetPath(testDevice())

	require.NoError(t, err)
	require.False(t, directIO)
	require.Equal(t, filepath.Join(dir, ".scrutiny_perf_bench"), target)
}

func TestResolveTargetPath_CleanupRemovesTestFile(t *testing.T) {
	dir := t.TempDir()
	collector := newTestCollector(t, dir, false)
	requireBenchmarkableDir(t, collector, dir)

	target, cleanup, _, err := collector.resolveTargetPath(testDevice())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(target, []byte("benchmark"), 0o600))

	cleanup()

	_, statErr := os.Stat(target)
	require.True(t, os.IsNotExist(statErr), "cleanup must remove the benchmark file")
}
