package omnibus

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfigResolvesPathsAndAppliesEnvironment(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "bin", executableName("scrutiny-collector-zfs")))
	writeTestFile(t, filepath.Join(dir, "config", "collector-zfs.yaml"))
	configPath := filepath.Join(dir, "collector-omnibus.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
binary_dir: ./bin
collectors:
  zfs:
    enabled: false
    config: ./config/collector-zfs.yaml
`), 0o600))

	env := map[string]string{
		"COLLECTOR_ZFS_CRON_SCHEDULE":     "*/15 * * * *",
		"COLLECTOR_ZFS_RUN_STARTUP":       "true",
		"COLLECTOR_ZFS_RUN_STARTUP_SLEEP": "5",
	}
	cfg, err := LoadConfig(configPath, filepath.Join(dir, "unused"), mapLookup(env))
	require.NoError(t, err)

	zfs := cfg.Collectors["zfs"]
	assert.True(t, zfs.Enabled)
	assert.Equal(t, "*/15 * * * *", zfs.Schedule)
	assert.True(t, zfs.RunOnStartup)
	assert.Equal(t, 5, zfs.StartupSleepSecs)
	assert.Equal(t, filepath.Join(dir, "config", "collector-zfs.yaml"), zfs.ConfigPath)
	assert.Equal(t, filepath.Join(dir, "bin"), cfg.BinaryDir)
}

func TestLoadConfigExplicitDisableWinsOverScheduleOverride(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "collector-omnibus.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
collectors:
  metrics:
    enabled: true
    schedule: "0 0 * * *"
    config: collector.yaml
`), 0o600))

	env := map[string]string{
		"COLLECTOR_METRICS_ENABLED": "false",
		"COLLECTOR_CRON_SCHEDULE":   "*/5 * * * *",
	}
	_, err := LoadConfig(configPath, filepath.Join(dir, "scrutiny-collector-omnibus"), mapLookup(env))
	require.EqualError(t, err, "at least one collector must be enabled")
}

func TestLoadConfigRejectsUnknownCollector(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "collector-omnibus.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
collectors:
  unknown:
    enabled: true
`), 0o600))

	_, err := LoadConfig(configPath, filepath.Join(dir, "scrutiny-collector-omnibus"), mapLookup(nil))
	require.EqualError(t, err, `unknown collector "unknown"`)
}

func TestLoadConfigRejectsInvalidSchedule(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, executableName("scrutiny-collector-metrics")))
	writeTestFile(t, filepath.Join(dir, "collector.yaml"))
	configPath := filepath.Join(dir, "collector-omnibus.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
collectors:
  metrics:
    enabled: true
    schedule: invalid
    config: collector.yaml
`), 0o600))

	_, err := LoadConfig(configPath, filepath.Join(dir, "scrutiny-collector-omnibus"), mapLookup(nil))
	require.ErrorContains(t, err, `collector "metrics" has invalid schedule "invalid"`)
}

func TestLoadConfigRejectsMissingEnabledBinary(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "collector.yaml"))
	configPath := filepath.Join(dir, "collector-omnibus.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
collectors:
  metrics:
    enabled: true
    run_on_startup: true
    config: collector.yaml
`), 0o600))

	_, err := LoadConfig(configPath, filepath.Join(dir, "scrutiny-collector-omnibus"), mapLookup(nil))
	require.ErrorContains(t, err, `collector "metrics" binary`)
}

// A PowerShell operator cannot pass an empty schedule override: assigning ""
// deletes the variable. Documented workaround is a whitespace value, which the
// loader trims. Both halves are a documented contract in
// docs/INSTALL_COLLECTOR_OMNIBUS.md, so pin them here.
func TestLoadConfigScheduleOverrideClearingBehavior(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, executableName("scrutiny-collector-metrics")))
	writeTestFile(t, filepath.Join(dir, "collector.yaml"))
	configPath := filepath.Join(dir, "collector-omnibus.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
collectors:
  metrics:
    enabled: true
    schedule: "0 0 * * *"
    run_on_startup: true
    config: collector.yaml
`), 0o600))
	executablePath := filepath.Join(dir, "scrutiny-collector-omnibus")

	absent, err := LoadConfig(configPath, executablePath, mapLookup(nil))
	require.NoError(t, err)
	assert.Equal(t, "0 0 * * *", absent.Collectors["metrics"].Schedule)

	whitespace, err := LoadConfig(configPath, executablePath, mapLookup(map[string]string{
		"COLLECTOR_CRON_SCHEDULE": " ",
	}))
	require.NoError(t, err)
	metrics := whitespace.Collectors["metrics"]
	assert.Empty(t, metrics.Schedule)
	assert.True(t, metrics.Enabled)
	assert.True(t, metrics.RunOnStartup)
}

func mapLookup(values map[string]string) LookupEnv {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}

func writeTestFile(t *testing.T, path string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("test"), 0o755))
}
