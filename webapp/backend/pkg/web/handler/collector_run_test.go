package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"
)

type collectorFinishedHook struct{ done chan struct{} }

func (h collectorFinishedHook) Levels() []logrus.Level { return logrus.AllLevels }
func (h collectorFinishedHook) Fire(entry *logrus.Entry) error {
	if entry.Message == "Manual sequential collector run finished" {
		close(h.done)
	}
	return nil
}

func TestTriggerCollectorsEnablement(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX executable fixture")
	}
	for _, tc := range []struct {
		name     string
		only     string
		schedule string
		startup  string
		enabled  bool
		fail     bool
	}{
		{name: "optional disabled"},
		{name: "quoted-empty double", schedule: `""`},
		{name: "quoted-empty single", schedule: "''"},
		{name: "quoted-empty padded", schedule: " \t\"  \"\r\n"},
		{name: "whitespace-only schedule", schedule: " \t\r\n"},
		{name: "quoted-empty startup enabled", schedule: `""`, startup: "true", enabled: true},
		{name: "quoted schedule enabled", schedule: `"*/15 * * * *"`, enabled: true},
		{name: "startup enabled", startup: "true", enabled: true},
		{name: "zfs only", only: "ZFS", startup: "true", enabled: true},
		{name: "mdadm only", only: "MDADM", schedule: "*/15 * * * *", enabled: true},
		{name: "normalized startup", startup: " \" TRUE \" ", enabled: true},
		{name: "numeric startup", startup: "1", enabled: true},
		{name: "startup disabled", startup: "false"},
		{name: "schedule survives startup false", schedule: "*/15 * * * *", startup: "false", enabled: true},
		{name: "enabled failure remains visible", startup: "true", enabled: true, fail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, prefix := range []string{"COLLECTOR_ZFS", "COLLECTOR_MDADM"} {
				t.Setenv(prefix+"_CRON_SCHEDULE", tc.schedule)
				t.Setenv(prefix+"_RUN_STARTUP", tc.startup)
				if tc.only != "" && prefix != "COLLECTOR_"+tc.only {
					t.Setenv(prefix+"_CRON_SCHEDULE", "")
					t.Setenv(prefix+"_RUN_STARTUP", "")
				}
			}
			dir := t.TempDir()
			t.Setenv("PATH", dir)
			t.Setenv("COLLECTOR_CRON_SCHEDULE", "0 0 * * *")
			t.Setenv("COLLECTOR_API_ENDPOINT", "http://example.invalid")
			t.Setenv("COLLECTOR_API_TOKEN", "test-token")
			for _, name := range []string{"metrics", "zfs", "mdadm"} {
				script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$0.args\"\nprintf '%s\\n' \"$COLLECTOR_API_ENDPOINT\" \"$COLLECTOR_API_TOKEN\" >> \"$0.args\"\n"
				if tc.fail && name == "zfs" {
					script += "exit 1\n"
				}
				require.NoError(t, os.WriteFile(filepath.Join(dir, "scrutiny-collector-"+name), []byte(script), 0700))
			}
			done := make(chan struct{})
			logger := logrus.New()
			logger.SetOutput(io.Discard)
			logs := logtest.NewLocal(logger)
			logger.AddHook(collectorFinishedHook{done: done})
			response := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(response)
			c.Set("LOGGER", logrus.NewEntry(logger))
			TriggerCollectors(c)
			require.Equal(t, http.StatusAccepted, response.Code)
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Fatal("manual collector run did not finish")
			}
			for _, name := range []string{"metrics", "zfs", "mdadm"} {
				args, err := os.ReadFile(filepath.Join(dir, "scrutiny-collector-"+name+".args"))
				if name != "metrics" && (!tc.enabled || (tc.only != "" && strings.ToUpper(name) != tc.only)) {
					require.ErrorIs(t, err, os.ErrNotExist)
					continue
				}
				require.NoError(t, err)
				want := "run\n"
				if name == "metrics" {
					want += "--cron-schedule=\n"
				}
				require.Equal(t, want+"http://example.invalid\ntest-token\n", string(args))
			}
			var failures, skipped int
			for _, entry := range logs.AllEntries() {
				if entry.Level == logrus.ErrorLevel {
					failures++
				}
				if strings.Contains(entry.Message, "disabled, skipping") {
					skipped++
				}
			}
			if tc.fail {
				require.Equal(t, 1, failures)
			} else {
				require.Zero(t, failures)
			}
			if !tc.enabled {
				require.Equal(t, 2, skipped)
			} else if tc.only != "" {
				require.Equal(t, 1, skipped)
			} else {
				require.Zero(t, skipped)
			}
		})
	}
}
