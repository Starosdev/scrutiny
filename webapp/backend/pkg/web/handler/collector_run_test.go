package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"
)

func triggerCollectorsRequest() int {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("LOGGER", logrus.NewEntry(logrus.New()))
	TriggerCollectors(c)
	return w.Code
}

func collectorRunStatus(t *testing.T) bool {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	GetCollectorRunStatus(c)
	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Running bool `json:"running"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body.Running
}

// restoreCollectorSeams puts the test seams back once no run is left using them. Cleanups run
// last-in first-out, so a test that registers this first can release a blocked run in a later
// cleanup and the run still sees the fakes until it exits. A run that never exits fails here
// instead of leaving collectorRunning set and turning the next test's POST into a 409.
func restoreCollectorSeams(t *testing.T) {
	t.Helper()
	origLook, origExec, origGap := collectorLookPath, collectorExec, collectorGap
	t.Cleanup(func() {
		require.Eventually(t, func() bool { return !collectorRunning.Load() }, 5*time.Second, 5*time.Millisecond, "a manual collector run outlived its test")
		collectorLookPath, collectorExec, collectorGap = origLook, origExec, origGap
	})
}

func TestTriggerCollectors_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	orig := collectorLookPath
	defer func() { collectorLookPath = orig }()
	collectorLookPath = func(string) (string, error) { return "", errors.New("missing") }

	require.Equal(t, http.StatusNotFound, triggerCollectorsRequest())
	require.False(t, collectorRunStatus(t))
}

func TestTriggerCollectors_RunsAllAndRejectsConcurrent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	restoreCollectorSeams(t)

	collectorLookPath = func(name string) (string, error) { return "/bin/" + name, nil }
	collectorGap = 0
	// Enable the optional collectors, so the run covers the whole list.
	t.Setenv("COLLECTOR_ZFS_CRON_SCHEDULE", "*/15 * * * *")
	t.Setenv("COLLECTOR_MDADM_CRON_SCHEDULE", "*/15 * * * *")

	var mu sync.Mutex
	var ran []string
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseRun := func() { releaseOnce.Do(func() { close(release) }) }
	// Unblock the run even when an assertion below fails first.
	t.Cleanup(releaseRun)
	started := make(chan struct{}, len(manualCollectors))
	collectorExec = func(_ context.Context, path string, _ []string) ([]byte, error) {
		mu.Lock()
		ran = append(ran, path)
		mu.Unlock()
		started <- struct{}{}
		<-release
		return nil, nil
	}

	require.Equal(t, http.StatusAccepted, triggerCollectorsRequest())
	<-started
	require.Equal(t, http.StatusConflict, triggerCollectorsRequest(), "second request during a run must be rejected")
	require.True(t, collectorRunStatus(t), "status must report the run in progress")

	releaseRun()
	require.Eventually(t, func() bool { return !collectorRunning.Load() }, 2*time.Second, 5*time.Millisecond)
	require.False(t, collectorRunStatus(t), "status must report no run once the run has finished")

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, []string{
		"/bin/scrutiny-collector-metrics",
		"/bin/scrutiny-collector-zfs",
		"/bin/scrutiny-collector-mdadm",
	}, ran)
}

func TestTriggerCollectors_NoGapAfterLastCollector(t *testing.T) {
	gin.SetMode(gin.TestMode)
	restoreCollectorSeams(t)

	// Only the SMART collector exists, so the run executes exactly one collector.
	collectorLookPath = func(name string) (string, error) {
		if name == manualCollectors[0] {
			return "/bin/" + name, nil
		}
		return "", errors.New("missing")
	}
	collectorExec = func(context.Context, string, []string) ([]byte, error) { return nil, nil }
	// Long enough that a gap after the last collector fails the 1-second check below, short enough
	// that the cleanup in restoreCollectorSeams outlasts it, so a regression cannot leak the run flag.
	collectorGap = 3 * time.Second

	require.Equal(t, http.StatusAccepted, triggerCollectorsRequest())
	require.Eventually(t, func() bool { return !collectorRunning.Load() }, time.Second, 5*time.Millisecond, "run flag must clear without waiting for a gap after the last collector")
}

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
	origGap := collectorGap
	t.Cleanup(func() { collectorGap = origGap })
	collectorGap = 0
	for _, tc := range []struct {
		name       string
		only       string
		schedule   string
		startup    string
		enabledEnv string
		enabled    bool
		fail       bool
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
		// COLLECTOR_*_ENABLED belongs to the standalone omnibus manager only; the all-in-one image
		// ignores it, so manual runs must too (#1116).
		{name: "explicit false ignored with schedule", schedule: "*/15 * * * *", enabledEnv: "false", enabled: true},
		{name: "explicit true ignored without schedule", enabledEnv: "true"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, prefix := range []string{"COLLECTOR_ZFS", "COLLECTOR_MDADM"} {
				t.Setenv(prefix+"_CRON_SCHEDULE", tc.schedule)
				t.Setenv(prefix+"_RUN_STARTUP", tc.startup)
				t.Setenv(prefix+"_ENABLED", tc.enabledEnv)
				if tc.only != "" && prefix != "COLLECTOR_"+tc.only {
					t.Setenv(prefix+"_CRON_SCHEDULE", "")
					t.Setenv(prefix+"_RUN_STARTUP", "")
				}
			}
			dir := t.TempDir()
			t.Setenv("PATH", dir)
			t.Setenv("COLLECTOR_CRON_SCHEDULE", "0 0 * * *")
			t.Setenv("COLLECTOR_METRICS_ENABLED", tc.enabledEnv)
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
			// The finished log fires just before the run flag clears; wait so the next case is not rejected.
			require.Eventually(t, func() bool { return !collectorRunning.Load() }, 2*time.Second, 5*time.Millisecond)
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
