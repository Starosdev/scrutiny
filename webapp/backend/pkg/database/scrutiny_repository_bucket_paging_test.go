package database

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	mock_config "github.com/analogj/scrutiny/webapp/backend/pkg/config/mock"
	"github.com/golang/mock/gomock"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/domain"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"
)

// bucketPagingServer is a scripted /api/v2/buckets endpoint that records the paging each request
// carries. The offset is only observable here: api.Paging's fields are unexported, so the stub
// BucketsAPI the other bucket tests use cannot read what api.PagingWithOffset set, and a
// regression to a constant offset leaves those tests green (#1015).
type bucketPagingServer struct {
	mu      sync.Mutex
	offsets []string
	limits  []string
	orgs    []string
}

// start serves one full page of unrelated buckets followed by a short page holding shortPageName,
// which is the layout that makes a second request necessary and a third request wrong.
func (b *bucketPagingServer) start(t *testing.T, shortPageName string) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/buckets" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
			http.NotFound(w, r)
			return
		}

		requests := b.record(r)

		listed := []domain.Bucket{}
		switch requests {
		case 1:
			for i := 0; i < bucketPageSize; i++ {
				listed = append(listed, domain.Bucket{Name: fmt.Sprintf("unrelated_%d", i)})
			}
		case 2:
			listed = append(listed, domain.Bucket{Name: shortPageName})
		default:
			// The walk is asking for a page it has already consumed, which is what a constant or
			// non-advancing offset looks like from the server. The empty page below ends the walk,
			// so the regression fails this test rather than hanging it.
			offsets, _, _ := b.snapshot()
			t.Errorf("bucket paging made %d requests for a 2-page organization; offsets so far: %v", requests, offsets)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"buckets": listed})
	}))
	t.Cleanup(server.Close)

	return server
}

// record stores the paging one request carried and returns how many requests have arrived. The
// handler runs on the server's own goroutine, so every read of these slices takes the lock too.
func (b *bucketPagingServer) record(r *http.Request) int {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.offsets = append(b.offsets, r.URL.Query().Get("offset"))
	b.limits = append(b.limits, r.URL.Query().Get("limit"))
	b.orgs = append(b.orgs, r.URL.Query().Get("org"))

	return len(b.offsets)
}

func (b *bucketPagingServer) snapshot() (offsets, limits, orgs []string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return append([]string(nil), b.offsets...), append([]string(nil), b.limits...), append([]string(nil), b.orgs...)
}

// requireTwoPageWalk asserts the paging a two-page walk has to put on the wire: the first page
// from offset 0 and the second from offset bucketPageSize, both asking for bucketPageSize
// buckets, both scoped to this deployment's organization.
func requireTwoPageWalk(t *testing.T, recorder *bucketPagingServer, orgName string) {
	t.Helper()

	offsets, limits, orgs := recorder.snapshot()
	pageSize := fmt.Sprintf("%d", bucketPageSize)

	require.Equal(t, []string{"0", pageSize}, offsets)
	require.Equal(t, []string{pageSize, pageSize}, limits)
	require.Equal(t, []string{orgName, orgName}, orgs, "every bucket lookup is scoped to this deployment's organization")
}

func newPagingTestRepository(t *testing.T, client influxdb2.Client, orgName string) *scrutinyRepository {
	t.Helper()

	mockCtrl := gomock.NewController(t)
	t.Cleanup(func() { mockCtrl.Finish() })

	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetString(cfgInfluxDBOrg).Return(orgName).AnyTimes()

	logger, _ := logrustest.NewNullLogger()
	return &scrutinyRepository{appConfig: fakeConfig, logger: logger, influxClient: client}
}

func newPagingTestClient(t *testing.T, recorder *bucketPagingServer, shortPageName string) influxdb2.Client {
	t.Helper()

	client := influxdb2.NewClient(recorder.start(t, shortPageName).URL, "test-token")
	t.Cleanup(client.Close)

	return client
}

// findBucketInOrg runs during MigrateDatabase, so an offset that does not advance hangs startup
// rather than returning a wrong answer. Drive the real client against an httptest server and
// assert the offset it sends, which no stub can see.
func TestOrgBucketPagingAdvancesTheOffset(t *testing.T) {
	const orgName = "scrutiny"

	t.Run("findBucketInOrg", func(t *testing.T) {
		recorder := &bucketPagingServer{}
		client := newPagingTestClient(t, recorder, "metrics_yearly")

		found, err := findBucketInOrg(context.Background(), client.BucketsAPI(), orgName, "metrics_yearly")

		require.NoError(t, err)
		require.NotNil(t, found)
		require.Equal(t, "metrics_yearly", found.Name)
		requireTwoPageWalk(t, recorder, orgName)
	})

	t.Run("GetAvailableInfluxDBBuckets", func(t *testing.T) {
		recorder := &bucketPagingServer{}
		client := newPagingTestClient(t, recorder, "metrics_yearly")

		names, err := newPagingTestRepository(t, client, orgName).GetAvailableInfluxDBBuckets(context.Background())

		require.NoError(t, err)
		require.Len(t, names, bucketPageSize+1, "the diagnostic reports buckets from every page, not just the first")
		require.Equal(t, "unrelated_0", names[0])
		require.Equal(t, "metrics_yearly", names[bucketPageSize])
		requireTwoPageWalk(t, recorder, orgName)
	})

	// the match is on the full first page here, so a walk that does not stop on it asks for the
	// second page and fails. Without this the early return is unbound: the other subtests put the
	// match on the short page, which ends the walk whether or not the match does.
	t.Run("findBucketInOrg stops on the page holding the match", func(t *testing.T) {
		recorder := &bucketPagingServer{}
		client := newPagingTestClient(t, recorder, "metrics_yearly")

		found, err := findBucketInOrg(context.Background(), client.BucketsAPI(), orgName, "unrelated_7")

		require.NoError(t, err)
		require.NotNil(t, found)
		require.Equal(t, "unrelated_7", found.Name)

		offsets, _, _ := recorder.snapshot()
		require.Equal(t, []string{"0"}, offsets, "a match on a full page ends the walk without asking for the next one")
	})
}
