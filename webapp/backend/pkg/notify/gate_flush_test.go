package notify

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mock_config "github.com/analogj/scrutiny/webapp/backend/pkg/config/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"
)

const quietFlushTestTimeout = 5 * time.Second

func newQuietFlushNotify(t *testing.T, urls ...string) *Notify {
	t.Helper()
	cfg := mock_config.NewMockInterface(gomock.NewController(t))
	cfg.EXPECT().GetStringSlice("notify.urls").Return(nil).AnyTimes()
	cfg.EXPECT().GetString("notify.urls").Return("").AnyTimes()
	return &Notify{Logger: logrus.New(), Config: cfg, DatabaseUrls: urls}
}

func TestGateMissingEndpointsWarnsOnceAndKeepsRetrying(t *testing.T) {
	n := newQuietFlushNotify(t)
	logger, hook := logrustest.NewNullLogger()
	logger.SetLevel(logrus.WarnLevel)
	n.Logger = logger
	gate := NewNotificationGate(logger)
	settings := &models.Settings{}
	const attempts = 20
	var workers sync.WaitGroup
	var accepted atomic.Int32
	for range attempts {
		workers.Go(func() {
			if gate.TrySend(n, settings, false) {
				accepted.Add(1)
			}
		})
	}
	workers.Wait()
	require.Zero(t, accepted.Load())
	require.Len(t, hook.AllEntries(), 1, "all devices share one missing-endpoint warning")
	require.Contains(t, hook.LastEntry().Message, "no notification endpoints configured")

	require.True(t, gate.TrySend(&Notify{Payload: Payload{Subject: "Hot drive"}}, activeQuietSettings(), false))
	gate.FlushQuietQueue(n, settings)
	gate.FlushQuietQueue(n, settings)
	require.Len(t, hook.AllEntries(), 1, "digest retries must share the warning suppression")
	require.Equal(t, 1, gate.QueueLength())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	n.DatabaseUrls = []string{server.URL}
	gate.FlushQuietQueue(n, settings)
	require.Zero(t, gate.QueueLength(), "adding an endpoint must deliver the pending digest")
	require.True(t, gate.TrySend(n, settings, false))

	n.DatabaseUrls = nil
	require.False(t, gate.TrySend(n, settings, false))
	require.False(t, gate.TrySend(n, settings, false))
	require.Len(t, hook.AllEntries(), 2, "successful delivery re-arms the warning")
}

func activeQuietSettings() *models.Settings {
	now := time.Now()
	settings := &models.Settings{}
	settings.Metrics.NotificationQuietStart = now.Add(-time.Hour).Format("15:04")
	settings.Metrics.NotificationQuietEnd = now.Add(time.Hour).Format("15:04")
	return settings
}

func TestFlushQuietQueueRetainsUntilDelivered(t *testing.T) {
	for _, scenario := range []string{"quiet hours", "rate limited", "send failure", "no endpoints"} {
		t.Run(scenario, func(t *testing.T) {
			var calls atomic.Int32
			var status atomic.Int32
			status.Store(http.StatusNoContent)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(int(status.Load()))
			}))
			defer server.Close()
			n := newQuietFlushNotify(t, server.URL)
			gate := NewNotificationGate(n.Logger)
			n.Payload = Payload{Subject: "Hot drive", Message: "First line\nSecond line"}
			require.True(t, gate.TrySend(n, activeQuietSettings(), false))
			settings := &models.Settings{}
			var failedRequests int32
			switch scenario {
			case "quiet hours":
				settings = activeQuietSettings()
			case "rate limited":
				settings.Metrics.NotificationRateLimit = 1
				gate.recordSent()
			case "send failure":
				status.Store(http.StatusInternalServerError)
				failedRequests = 1
			case "no endpoints":
				n.DatabaseUrls = nil
			}

			gate.FlushQuietQueue(n, settings)
			require.Equal(t, 1, gate.QueueLength(), "undelivered alerts must remain queued")
			require.Equal(t, failedRequests, calls.Load())

			status.Store(http.StatusNoContent)
			n.DatabaseUrls = []string{server.URL}
			gate.FlushQuietQueue(n, &models.Settings{})
			require.Zero(t, gate.QueueLength())
			require.Equal(t, failedRequests+1, calls.Load())
			require.Equal(t, "Scrutiny: 1 notification(s) during quiet hours", n.Payload.Subject)
			require.Contains(t, n.Payload.Message, "Hot drive")
			require.Contains(t, n.Payload.Message, "First line")
			require.NotContains(t, n.Payload.Message, "Second line")

			gate.FlushQuietQueue(n, &models.Settings{})
			require.Equal(t, failedRequests+1, calls.Load(), "an empty queue must not resend")
		})
	}
}

func TestFlushQuietQueuePreservesConcurrentEnqueue(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var unblock sync.Once
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			close(started)
			<-release
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	defer unblock.Do(func() { close(release) })
	n := newQuietFlushNotify(t, server.URL)
	gate := NewNotificationGate(n.Logger)
	require.True(t, gate.TrySend(&Notify{Payload: Payload{Subject: "First drive"}}, activeQuietSettings(), false))
	done := make(chan struct{})
	go func() {
		gate.FlushQuietQueue(n, &models.Settings{})
		close(done)
	}()
	select {
	case <-started:
	case <-time.After(quietFlushTestTimeout):
		t.Fatal("digest delivery did not start")
	}

	enqueued := make(chan bool, 1)
	go func() {
		enqueued <- gate.TrySend(&Notify{Payload: Payload{Subject: "Second drive"}}, activeQuietSettings(), false)
	}()
	select {
	case accepted := <-enqueued:
		require.True(t, accepted)
	case <-time.After(quietFlushTestTimeout):
		t.Fatal("enqueue blocked behind delivery")
	}
	require.Equal(t, 2, gate.QueueLength(), "in-flight alerts remain queued until delivery succeeds")
	unblock.Do(func() { close(release) })
	select {
	case <-done:
	case <-time.After(quietFlushTestTimeout):
		t.Fatal("digest delivery did not finish")
	}
	require.Equal(t, 1, gate.QueueLength())
	require.Contains(t, n.Payload.Message, "First drive")
	require.NotContains(t, n.Payload.Message, "Second drive")
	gate.FlushQuietQueue(n, &models.Settings{})
	require.Zero(t, gate.QueueLength())
	require.EqualValues(t, 2, calls.Load())
	require.Contains(t, n.Payload.Message, "Second drive")
	require.NotContains(t, n.Payload.Message, "First drive")
}

func TestFlushQuietQueueConcurrentFlushesSendOnce(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	gate := NewNotificationGate(logrus.New())
	require.True(t, gate.TrySend(&Notify{Payload: Payload{Subject: "Hot drive"}}, activeQuietSettings(), false))
	const flushes = 20
	var workers sync.WaitGroup
	start := make(chan struct{})
	for range flushes {
		n := newQuietFlushNotify(t, server.URL)
		workers.Go(func() {
			<-start
			gate.FlushQuietQueue(n, &models.Settings{})
		})
	}
	close(start)
	workers.Wait()
	require.EqualValues(t, 1, calls.Load())
	require.Zero(t, gate.QueueLength())
}
