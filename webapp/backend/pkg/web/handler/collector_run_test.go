package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/analogj/scrutiny/collector/pkg/omnibus"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func triggerCollectorsRequest() int {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("LOGGER", logrus.NewEntry(logrus.New()))
	TriggerCollectors(c)
	return w.Code
}

func TestTriggerCollectors_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	orig := collectorLookPath
	defer func() { collectorLookPath = orig }()
	collectorLookPath = func(string) (string, error) { return "", errors.New("missing") }

	require.Equal(t, http.StatusNotFound, triggerCollectorsRequest())
}

func TestTriggerCollectors_RunsAllSpecsAndRejectsConcurrent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	origLook, origExec, origGap := collectorLookPath, collectorExec, collectorGap
	defer func() { collectorLookPath, collectorExec, collectorGap = origLook, origExec, origGap }()

	collectorLookPath = func(name string) (string, error) { return "/bin/" + name, nil }
	collectorGap = 0

	var mu sync.Mutex
	var ran []string
	release := make(chan struct{})
	started := make(chan struct{}, len(omnibus.CollectorSpecs))
	collectorExec = func(_ context.Context, path string) ([]byte, error) {
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

	close(release)
	require.Eventually(t, func() bool {
		if !collectorRunMu.TryLock() {
			return false
		}
		collectorRunMu.Unlock()
		return true
	}, 2*time.Second, 5*time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, ran, len(omnibus.CollectorSpecs))
	require.Equal(t, "/bin/scrutiny-collector-btrfs", ran[3])
}
