package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Execute the real CLI in a child process because main owns signals and exits.
func TestCollectorCLIProcess(t *testing.T) {
	if os.Getenv("SCRUTINY_TEST_COLLECTOR_CLI") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{"scrutiny-collector-metrics"}, os.Args[i+1:]...)
			main()
			os.Exit(0)
		}
	}
	t.Fatal("missing CLI arguments")
}

func TestExplicitEmptyScheduleRunsOnce(t *testing.T) {
	for _, source := range []string{"environment", "yaml", "both"} {
		t.Run(source, func(t *testing.T) {
			t.Setenv("SCRUTINY_TEST_COLLECTOR_CLI", "1")
			t.Setenv("COLLECTOR_CRON_SCHEDULE", "")
			if source != "yaml" {
				t.Setenv("COLLECTOR_CRON_SCHEDULE", "invalid inherited schedule")
			}
			t.Setenv("COLLECTOR_RUN_STARTUP", "true")
			t.Setenv("COLLECTOR_RUN_STARTUP_SLEEP", "3600")
			t.Setenv("COLLECTOR_CRON_RUN_ON_STARTUP", "true")
			t.Setenv("COLLECTOR_CRON_STARTUP_SLEEP_SECS", "3600")
			// A missing tool makes one collection fail immediately without touching disks
			// or an API. Scheduler mode would wait instead of returning this error.
			missing := filepath.Join(t.TempDir(), "missing-smartctl")
			t.Setenv("COLLECTOR_COMMANDS_METRICS_SMARTCTL_BIN", missing)
			configPath := filepath.Join(t.TempDir(), "collector.yaml")
			contents := "log:\n  level: INFO\n"
			if source != "environment" {
				contents += "cron:\n  schedule: '0 0 1 1 *'\n  run_on_startup: true\n  startup_sleep_secs: 3600\n"
			}
			require.NoError(t, os.WriteFile(configPath, []byte(contents), 0600))
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCollectorCLIProcess$", "--", "run", "--config", configPath, "--cron-schedule=")
			output, err := cmd.CombinedOutput()
			require.NoError(t, ctx.Err(), string(output))
			require.Error(t, err)
			require.Contains(t, string(output), "missing-smartctl binary is missing")
			require.Equal(t, 1, strings.Count(string(output), "Verifying required tools"))
			require.NotContains(t, string(output), "Internal cron scheduler active")
			require.NotContains(t, string(output), "Waiting 3600 seconds")
		})
	}
}

func TestConfiguredScheduleRemainsEnabled(t *testing.T) {
	t.Setenv("SCRUTINY_TEST_COLLECTOR_CLI", "1")
	t.Setenv("COLLECTOR_CRON_SCHEDULE", "invalid schedule")
	t.Setenv("COLLECTOR_CRON_RUN_ON_STARTUP", "false")
	configPath := filepath.Join(t.TempDir(), "collector.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("cron:\n  run_on_startup: false\n"), 0600))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCollectorCLIProcess$", "--", "run", "--config", configPath).CombinedOutput()
	require.NoError(t, ctx.Err(), string(output))
	require.Error(t, err)
	require.Contains(t, string(output), "invalid cron schedule")
	require.NotContains(t, string(output), "Verifying required tools")
}
