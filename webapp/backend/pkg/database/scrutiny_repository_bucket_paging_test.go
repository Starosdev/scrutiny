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

// maxBucketPageRequests bounds the scripted organization so a non-advancing offset fails the
// test instead of hanging it. Two requests serve the whole organization; a third means the walk
// asked for a page it had already been given.
const maxBucketPageRequests = 3

// bucketPagingServer is a scripted /api/v2/buckets endpoint that records the paging each request
// carries. The offset is only observable here: api.Paging's fields are unexported, so the stub
// BucketsAPI the other bucket tests use cannot read what api.PagingWithOffset set, and a
// regression to a constant offset leaves those tests green (#1015).
type bucketPagingServer struct {
	mu       sync.Mutex
	offsets  []string
	limits   []string
	orgs     []string
	requests int
}

// start serves one full page of unrelated buckets followed by a short page holding shortPageName,
// which is the only layout that makes the second request necessary and the third request wrong.
func (b *bucketPagingServer) start(t *testing.T, orgName, shortPageName string) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/buckets" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
			http.NotFound(w, r)
			return
		}

		b.mu.Lock()
		b.offsets = append(b.offsets, r.URL.Query().Get("offset"))
		b.limits = append(b.limits, r.URL.Query().Get("limit"))
		b.orgs = append(b.orgs, r.URL.Query().Get("org"))
		b.requests++
		requests := b.requests
		b.mu.Unlock()

		listed := []domain.Bucket{}
		switch requests {
		case 1:
			for i := 0; i < bucketPageSize; i++ {
				name := fmt.Sprintf("unrelated_%d", i)
				listed = append(listed, domain.Bucket{Name: name})
			}
		case 2:
			listed = append(listed, domain.Bucket{Name: shortPageName})
		default:
			// the walk is asking for a page it has already consumed, which is what a constant
			// or non-advancing offset looks like from the server
			t.Errorf("bucket paging made %d requests for a 2-page organization; offsets so far: %v", requests, b.snapshotOffsets())
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"buckets": listed})
	}))
	t.Cleanup(server.Close)

	return server
}

func (b *bucketPagingServer) snapshotOffsets() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.offsets...)
}

// expectedPaging is the paging a two-page walk has to put on the wire: the first page from
// offset 0 and the second from offset bucketPageSize, both asking for bucketPageSize buckets.
func expectedPaging() (offsets, limits []string) {
	limit := fmt.Sprintf("%d", bucketPageSize)
	return []string{"0", limit}, []string{limit, limit}
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

// findBucketInOrg runs during MigrateDatabase, so an offset that does not advance hangs startup
// rather than returning a wrong answer. Drive the real client against an httptest server and
// assert the offset it sends, which no stub can see.
func TestOrgBucketPagingAdvancesTheOffset(t *testing.T) {
	const orgName = "scrutiny"

	t.Run("findBucketInOrg", func(t *testing.T) {
		recorder := &bucketPagingServer{}
		server := recorder.start(t, orgName, "metrics_yearly")
		client := influxdb2.NewClient(server.URL, "test-token")
		t.Cleanup(client.Close)

		found, err := findBucketInOrg(context.Background(), client.BucketsAPI(), orgName, "metrics_yearly")

		require.NoError(t, err)
		require.NotNil(t, found)
		require.Equal(t, "metrics_yearly", found.Name)

		offsets, limits := expectedPaging()
		require.Equal(t, offsets, recorder.offsets)
		require.Equal(t, limits, recorder.limits)
		require.Equal(t, []string{orgName, orgName}, recorder.orgs, "every bucket lookup is scoped to this deployment's organization")
	})

	t.Run("GetAvailableInfluxDBBuckets", func(t *testing.T) {
		recorder := &bucketPagingServer{}
		server := recorder.start(t, orgName, "metrics_yearly")
		client := influxdb2.NewClient(server.URL, "test-token")
		t.Cleanup(client.Close)

		names, err := newPagingTestRepository(t, client, orgName).GetAvailableInfluxDBBuckets(context.Background())

		require.NoError(t, err)
		require.Len(t, names, bucketPageSize+1, "the diagnostic reports buckets from every page, not just the first")
		require.Equal(t, "unrelated_0", names[0])
		require.Equal(t, "metrics_yearly", names[bucketPageSize])

		offsets, limits := expectedPaging()
		require.Equal(t, offsets, recorder.offsets)
		require.Equal(t, limits, recorder.limits)
		require.Equal(t, []string{orgName, orgName}, recorder.orgs)
	})
}
