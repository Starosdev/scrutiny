package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
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

func TestTriggerCollectorsForcesSmartOneShot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX executable fixture")
	}
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	t.Setenv("COLLECTOR_CRON_SCHEDULE", "0 0 * * *")
	t.Setenv("COLLECTOR_API_ENDPOINT", "http://example.invalid")
	t.Setenv("COLLECTOR_API_TOKEN", "test-token")
	for _, name := range []string{"metrics", "zfs", "mdadm"} {
		script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$0.args\"\nprintf '%s\\n' \"$COLLECTOR_API_ENDPOINT\" \"$COLLECTOR_API_TOKEN\" >> \"$0.args\"\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, "scrutiny-collector-"+name), []byte(script), 0700))
	}
	done := make(chan struct{})
	logger := logrus.New()
	logger.SetOutput(io.Discard)
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
		require.NoError(t, err)
		want := "run\n"
		if name == "metrics" {
			want += "--cron-schedule=\n"
		}
		require.Equal(t, want+"http://example.invalid\ntest-token\n", string(args))
	}
}
