package mqtt

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/stretchr/testify/require"
)

type syncRepository struct {
	database.DeviceRepo
	calls   atomic.Int32
	entered chan struct{}
	release chan struct{}
	err     error
}

func (r *syncRepository) GetSummary(context.Context) (map[string]*models.DeviceSummary, error) {
	r.calls.Add(1)
	if r.entered != nil {
		r.entered <- struct{}{}
	}
	if r.release != nil {
		<-r.release
	}
	return map[string]*models.DeviceSummary{}, r.err
}

func TestSyncRejectsInitialConnectionRetry(t *testing.T) {
	transport := &connectionClient{}
	pub := &Publisher{client: &Client{client: transport}, logger: testLogger()}
	repo := &syncRepository{}
	published, cleaned, err := pub.SyncAllDevices(repo, context.Background())
	require.ErrorIs(t, err, ErrNotConnected)
	require.Zero(t, published)
	require.Zero(t, cleaned)
	pub.PublishDiscovery(&models.Device{DeviceID: "test"})
	pub.RemoveDevice(&models.Device{DeviceID: "test", WWN: "wwn"})
	require.Zero(t, transport.publishes.Load())
	require.Zero(t, repo.calls.Load())
}

func TestReconnectSyncWaitsForManualSync(t *testing.T) {
	transport := &connectionClient{open: true}
	pub := &Publisher{client: &Client{client: transport}, logger: testLogger()}
	repo := &syncRepository{release: make(chan struct{}), entered: make(chan struct{}, 2)}
	results := make(chan error, 2)
	go func() {
		_, _, err := pub.SyncAllDevices(repo, context.Background())
		results <- err
	}()
	select {
	case <-repo.entered:
	case <-time.After(time.Second):
		t.Fatal("manual sync did not start")
	}
	go func() { results <- pub.LoadInitialData(repo, context.Background()) }()
	select {
	case <-repo.entered:
		t.Error("reconnect overlapped the manual sync")
	case <-time.After(100 * time.Millisecond):
	}
	close(repo.release)
	for range 2 {
		select {
		case err := <-results:
			require.NoError(t, err)
		case <-time.After(time.Second):
			t.Fatal("sync did not finish")
		}
	}
	require.EqualValues(t, 2, repo.calls.Load(), "reconnect must not be dropped")
}

func TestSyncReleasesLockAfterRepositoryError(t *testing.T) {
	pub := &Publisher{client: &Client{client: &connectionClient{open: true}}}
	repo := &syncRepository{err: errors.New("database unavailable")}
	_, _, err := pub.SyncAllDevices(repo, context.Background())
	require.ErrorIs(t, err, repo.err)
	repo.err = nil
	_, _, err = pub.SyncAllDevices(repo, context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 2, repo.calls.Load())
}
