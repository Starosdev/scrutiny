package notify

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mock_config "github.com/analogj/scrutiny/webapp/backend/pkg/config/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// fakeOutbox mirrors the database outbox: conditional state transitions and oldest-first listing.
type fakeOutbox struct {
	mu         sync.Mutex
	rows       map[uint]*models.NotificationOutbox
	nextID     uint
	enqueueErr error
	// listBarrier, when set, holds each pending-row listing until every caller has listed,
	// so concurrent drains see the same rows before either claims one.
	listBarrier *sync.WaitGroup
}

func newFakeOutbox() *fakeOutbox { return &fakeOutbox{rows: map[uint]*models.NotificationOutbox{}} }

func (f *fakeOutbox) EnqueueNotification(_ context.Context, row *models.NotificationOutbox) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.enqueueErr != nil {
		return f.enqueueErr
	}
	f.nextID++
	copied := *row
	copied.ID = f.nextID
	f.rows[copied.ID] = &copied
	return nil
}

func (f *fakeOutbox) ListNotifications(_ context.Context, state string, limit int) ([]models.NotificationOutbox, error) {
	if f.listBarrier != nil && state == models.NotificationPending {
		defer func() {
			f.listBarrier.Done()
			f.listBarrier.Wait()
		}()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []models.NotificationOutbox
	for _, row := range f.rows {
		if row.State == state {
			out = append(out, *row)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeOutbox) TransitionNotification(_ context.Context, id uint, from string, to string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	row, ok := f.rows[id]
	if !ok || row.State != from {
		return false, nil
	}
	row.State = to
	return true, nil
}

func (f *fakeOutbox) DeleteNotifications(_ context.Context, ids []uint) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, id := range ids {
		delete(f.rows, id)
	}
	return nil
}

func (f *fakeOutbox) DeleteStaleNotifications(_ context.Context, states []string, before int64) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var removed int64
	for id, row := range f.rows {
		for _, state := range states {
			if row.State == state && row.CreatedAtUnixMs < before {
				delete(f.rows, id)
				removed++
			}
		}
	}
	return removed, nil
}

func (f *fakeOutbox) states() map[string]int {
	f.mu.Lock()
	defer f.mu.Unlock()
	counts := map[string]int{}
	for _, row := range f.rows {
		counts[row.State]++
	}
	return counts
}

func outboxTestConfig(t *testing.T) *mock_config.MockInterface {
	t.Helper()
	cfg := mock_config.NewMockInterface(gomock.NewController(t))
	cfg.EXPECT().GetStringSlice("notify.urls").Return(nil).AnyTimes()
	cfg.EXPECT().GetString("notify.urls").Return("").AnyTimes()
	return cfg
}

// countingEndpoint returns a notification URL and a counter of deliveries to it.
func countingEndpoint(t *testing.T, status int) (string, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(status)
	}))
	t.Cleanup(server.Close)
	return server.URL, &calls
}

func outboxNotify(cfg *mock_config.MockInterface, url string, subject string) *Notify {
	return &Notify{
		Logger:       logrus.New(),
		Config:       cfg,
		Payload:      Payload{Subject: subject, Message: subject + " details", HTMLMessage: "<b>" + subject + "</b>"},
		DatabaseUrls: []string{url},
	}
}

func TestOutbox_TrySendRecordsInsteadOfSending(t *testing.T) {
	cfg := outboxTestConfig(t)
	url, calls := countingEndpoint(t, http.StatusNoContent)
	store := newFakeOutbox()
	gate := NewNotificationGate(logrus.New())
	gate.UseOutbox(store, nil)

	require.True(t, gate.TrySend(outboxNotify(cfg, url, "Drive failed"), &models.Settings{}, false))
	require.Zero(t, calls.Load(), "the receiving replica must not send")
	require.Equal(t, map[string]int{models.NotificationPending: 1}, store.states())
	select {
	case <-gate.Wake():
	default:
		t.Fatal("enqueue must wake the local outbox worker")
	}

	n, err := gate.decodeOutboxRow(*store.rows[1], cfg)
	require.NoError(t, err)
	require.Equal(t, "Drive failed", n.Payload.Subject)
	require.Equal(t, "<b>Drive failed</b>", n.Payload.HTMLMessage, "the HTML body survives storage")
	require.Equal(t, []string{url}, n.DatabaseUrls)
}

func TestOutbox_DrainDeliversOnce(t *testing.T) {
	cfg := outboxTestConfig(t)
	url, calls := countingEndpoint(t, http.StatusNoContent)
	store := newFakeOutbox()
	receiver := NewNotificationGate(logrus.New())
	receiver.UseOutbox(store, nil)
	require.True(t, receiver.TrySend(outboxNotify(cfg, url, "Drive failed"), &models.Settings{}, false))

	// Two gates stand in for two replicas that both believe they lead during a lease handover.
	// Both see the row as pending before either claims it.
	leaders := []*NotificationGate{NewNotificationGate(logrus.New()), NewNotificationGate(logrus.New())}
	store.listBarrier = &sync.WaitGroup{}
	store.listBarrier.Add(len(leaders))
	var wg sync.WaitGroup
	for _, leader := range leaders {
		leader.UseOutbox(store, nil)
		wg.Add(1)
		go func(g *NotificationGate) {
			defer wg.Done()
			require.NoError(t, g.DrainOutbox(context.Background(), cfg, &models.Settings{}))
		}(leader)
	}
	wg.Wait()

	require.Equal(t, int32(1), calls.Load())
	require.Empty(t, store.states(), "a delivered row is removed")
}

func TestOutbox_QuietHoursDigestSurvivesLeaderChange(t *testing.T) {
	cfg := outboxTestConfig(t)
	url, calls := countingEndpoint(t, http.StatusNoContent)
	store := newFakeOutbox()
	receiver := NewNotificationGate(logrus.New())
	receiver.UseOutbox(store, nil)
	require.True(t, receiver.TrySend(outboxNotify(cfg, url, "Hot drive"), &models.Settings{}, false))
	require.True(t, receiver.TrySend(outboxNotify(cfg, url, "Failed drive"), &models.Settings{}, false))

	oldLeader := NewNotificationGate(logrus.New())
	oldLeader.UseOutbox(store, nil)
	require.NoError(t, oldLeader.DrainOutbox(context.Background(), cfg, activeQuietSettings()))
	require.Zero(t, calls.Load())
	require.Equal(t, map[string]int{models.NotificationQueued: 2}, store.states())

	// A different replica takes over after quiet hours and still sends the digest.
	newLeader := NewNotificationGate(logrus.New())
	newLeader.UseOutbox(store, nil)
	require.Equal(t, 2, newLeader.QueueLength())
	newLeader.FlushQuietQueue(outboxNotify(cfg, url, ""), &models.Settings{})
	require.Equal(t, int32(1), calls.Load(), "one digest for both notifications")
	require.Empty(t, store.states())
}

func TestOutbox_FailedDigestStaysQueued(t *testing.T) {
	cfg := outboxTestConfig(t)
	url, calls := countingEndpoint(t, http.StatusInternalServerError)
	store := newFakeOutbox()
	gate := NewNotificationGate(logrus.New())
	gate.UseOutbox(store, nil)
	require.True(t, gate.TrySend(outboxNotify(cfg, url, "Hot drive"), &models.Settings{}, false))
	require.NoError(t, gate.DrainOutbox(context.Background(), cfg, activeQuietSettings()))

	gate.FlushQuietQueue(outboxNotify(cfg, url, ""), &models.Settings{})
	require.Equal(t, int32(1), calls.Load())
	require.Equal(t, map[string]int{models.NotificationQueued: 1}, store.states(), "a failed digest is retried later")
}

func TestOutbox_DiscardsStaleNotifications(t *testing.T) {
	cfg := outboxTestConfig(t)
	url, calls := countingEndpoint(t, http.StatusNoContent)
	store := newFakeOutbox()
	gate := NewNotificationGate(logrus.New())
	gate.UseOutbox(store, nil)
	require.True(t, gate.TrySend(outboxNotify(cfg, url, "Old alert"), &models.Settings{}, false))
	store.rows[1].CreatedAtUnixMs = time.Now().Add(-2 * OutboxMaxAge).UnixMilli()

	require.NoError(t, gate.DrainOutbox(context.Background(), cfg, &models.Settings{}))
	require.Zero(t, calls.Load())
	require.Empty(t, store.states())
}

func TestOutbox_CollectorErrorsDeduplicatedByLeader(t *testing.T) {
	cfg := outboxTestConfig(t)
	url, calls := countingEndpoint(t, http.StatusNoContent)
	store := newFakeOutbox()
	// Two receiving replicas record the same collector error.
	for range 2 {
		receiver := NewNotificationGate(logrus.New())
		receiver.UseOutbox(store, nil)
		require.True(t, receiver.TrySendCollectorError("device:abc", "smartctl", "exit 2", outboxNotify(cfg, url, "Collector error"), &models.Settings{}))
	}

	leader := NewNotificationGate(logrus.New())
	leader.UseOutbox(store, nil)
	require.NoError(t, leader.DrainOutbox(context.Background(), cfg, &models.Settings{}))
	require.Equal(t, int32(1), calls.Load())
	require.Empty(t, store.states())
}

func TestOutbox_EnqueueFailureSendsDirectly(t *testing.T) {
	cfg := outboxTestConfig(t)
	url, calls := countingEndpoint(t, http.StatusNoContent)
	store := newFakeOutbox()
	store.enqueueErr = errors.New("database is locked")
	gate := NewNotificationGate(logrus.New())
	gate.UseOutbox(store, nil)

	require.True(t, gate.TrySend(outboxNotify(cfg, url, "Drive failed"), &models.Settings{}, false))
	require.Equal(t, int32(1), calls.Load(), "a notification is not lost when the outbox is unavailable")
}

func leaderGate(store *fakeOutbox) *NotificationGate {
	gate := NewNotificationGate(logrus.New())
	gate.UseOutbox(store, func() bool { return true })
	return gate
}

// The leader delivers directly and reports the real outcome, so a failed or rate-limited
// notification is reported as not sent and the caller can retry.
func TestOutbox_LeaderDeliversDirectlyAndReportsFailures(t *testing.T) {
	cfg := outboxTestConfig(t)

	okURL, okCalls := countingEndpoint(t, http.StatusNoContent)
	store := newFakeOutbox()
	gate := leaderGate(store)
	require.True(t, gate.TrySend(outboxNotify(cfg, okURL, "Drive failed"), &models.Settings{}, false))
	require.Equal(t, int32(1), okCalls.Load())
	require.Empty(t, store.states(), "the leader does not route its own notifications through the table")

	badURL, _ := countingEndpoint(t, http.StatusInternalServerError)
	require.False(t, gate.TrySend(outboxNotify(cfg, badURL, "Drive failed"), &models.Settings{}, false), "a failed send is reported")

	limited := &models.Settings{}
	limited.Metrics.NotificationRateLimit = 1
	require.False(t, gate.TrySend(outboxNotify(cfg, okURL, "Drive failed"), limited, false), "a rate-limited send is reported")
	require.Equal(t, int32(1), okCalls.Load())
}

// During quiet hours the leader stores the notification as a queued row, so the digest, which
// reads the table, still includes it after a restart or a leader change.
func TestOutbox_LeaderQuietHoursQueuesInTable(t *testing.T) {
	cfg := outboxTestConfig(t)
	url, calls := countingEndpoint(t, http.StatusNoContent)
	store := newFakeOutbox()
	gate := leaderGate(store)

	require.True(t, gate.TrySend(outboxNotify(cfg, url, "Hot drive"), activeQuietSettings(), false))
	require.Zero(t, calls.Load())
	require.Equal(t, map[string]int{models.NotificationQueued: 1}, store.states())
	select {
	case <-gate.Wake():
		t.Fatal("a queued row waits for the digest, not the drain")
	default:
	}

	gate.FlushQuietQueue(outboxNotify(cfg, url, ""), &models.Settings{})
	require.Equal(t, int32(1), calls.Load())
	require.Empty(t, store.states())
}

// A temperature alert whose delivery fails is retried on the next hot upload. On the leader
// that holds with an outbox, because TrySend reports the failed delivery.
func TestOutbox_LeaderTemperatureAlertRetriesAfterFailure(t *testing.T) {
	cfg := outboxTestConfig(t)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)
	gate := leaderGate(newFakeOutbox())

	send := func(time.Time) bool {
		return gate.TrySend(outboxNotify(cfg, server.URL, "Hot drive"), &models.Settings{}, false)
	}
	now := time.Now()
	gate.Temperature().Evaluate("dev", 60, 55, 0, now, nil, send)
	gate.Temperature().Evaluate("dev", 60, 55, 0, now.Add(time.Minute), nil, send)
	gate.Temperature().Evaluate("dev", 60, 55, 0, now.Add(2*time.Minute), nil, send)
	require.Equal(t, int32(2), calls.Load(), "the failed first alert is retried once, then the excursion is notified")
}
