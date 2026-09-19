package database

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	mock_config "github.com/analogj/scrutiny/webapp/backend/pkg/config/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/golang/mock/gomock"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// influxDeleteRecorder captures InfluxDB delete requests as "<bucket> <predicate>".
type influxDeleteRecorder struct {
	mu       sync.Mutex
	requests []string
	// failStatus, when set, is returned for every delete request instead of success.
	failStatus int
}

func (r *influxDeleteRecorder) all() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.requests...)
}

func (r *influxDeleteRecorder) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests = nil
}

func (r *influxDeleteRecorder) failWith(status int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failStatus = status
}

// withInfluxDeleteRecorder points repo's InfluxDB client at a server that records delete requests
// instead of executing them.
func withInfluxDeleteRecorder(t *testing.T, repo *scrutinyRepository) *influxDeleteRecorder {
	t.Helper()
	recorder := &influxDeleteRecorder{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body struct {
			Predicate string `json:"predicate"`
		}
		if request.URL.Path != "/api/v2/delete" || json.NewDecoder(request.Body).Decode(&body) != nil {
			http.Error(writer, "unexpected request", http.StatusBadRequest)
			return
		}
		recorder.mu.Lock()
		recorder.requests = append(recorder.requests, request.URL.Query().Get("bucket")+" "+body.Predicate)
		failStatus := recorder.failStatus
		recorder.mu.Unlock()
		if failStatus != 0 {
			http.Error(writer, "delete failed", failStatus)
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	appConfig := mock_config.NewMockInterface(gomock.NewController(t))
	appConfig.EXPECT().GetString(cfgInfluxDBBucket).Return("metrics").AnyTimes()
	appConfig.EXPECT().GetString(cfgInfluxDBOrg).Return("scrutiny").AnyTimes()
	repo.appConfig = appConfig
	repo.logger = logrus.WithField("test", t.Name())
	repo.influxClient = influxdb2.NewClient(server.URL, "token")
	t.Cleanup(repo.influxClient.Close)
	return recorder
}

// bucketPredicates is the delete requests expected when each predicate runs against every history bucket.
func bucketPredicates(predicates ...string) []string {
	var expected []string
	for _, bucket := range []string{"metrics", "metrics_weekly", "metrics_monthly", "metrics_yearly"} {
		for _, predicate := range predicates {
			expected = append(expected, bucket+" "+predicate)
		}
	}
	return expected
}

// fixes #851: deleting one of the devices that share a WWN must not delete the other device's history.
// A WWN predicate is the only way to reach untagged legacy points, so it is used when the WWN is unique.
func TestDeleteDeviceDeletesByWWNOnlyWhenUnique(t *testing.T) {
	repo := createDeviceRegisterTestRepository(t)
	recorder := withInfluxDeleteRecorder(t, repo)
	ctx := context.Background()
	createSharedWWNDevices(t, repo)

	require.NoError(t, repo.DeleteDevice(ctx, "device-a"))
	require.ElementsMatch(t, bucketPredicates(`device_id="device-a"`), recorder.all())

	recorder.reset()
	require.NoError(t, repo.DeleteDevice(ctx, "device-unique"))
	require.ElementsMatch(t, bucketPredicates(`device_id="device-unique"`, `device_wwn="wwn-unique"`), recorder.all())
}

// A device without a WWN still has device_id-tagged history; it used to be left behind.
func TestDeleteDeviceWithoutWWNDeletesByDeviceID(t *testing.T) {
	repo := createDeviceRegisterTestRepository(t)
	recorder := withInfluxDeleteRecorder(t, repo)
	ctx := context.Background()
	require.NoError(t, repo.gormClient.Create(&models.Device{DeviceID: "device-no-wwn"}).Error)

	require.NoError(t, repo.DeleteDevice(ctx, "device-no-wwn"))

	require.ElementsMatch(t, bucketPredicates(`device_id="device-no-wwn"`), recorder.all())
}

// fixes #861: a failed history delete must keep the device row, so the leftover history still belongs
// to the device and the delete can be retried.
func TestDeleteDeviceKeepsRowWhenHistoryDeleteFails(t *testing.T) {
	repo := createDeviceRegisterTestRepository(t)
	recorder := withInfluxDeleteRecorder(t, repo)
	ctx := context.Background()
	createSharedWWNDevices(t, repo)

	recorder.failWith(http.StatusInternalServerError)
	require.Error(t, repo.DeleteDevice(ctx, "device-unique"))
	require.NoError(t, repo.gormClient.Where(queryDeviceID, "device-unique").First(&models.Device{}).Error)

	recorder.failWith(0)
	recorder.reset()
	require.NoError(t, repo.DeleteDevice(ctx, "device-unique"))
	require.ElementsMatch(t, bucketPredicates(`device_id="device-unique"`, `device_wwn="wwn-unique"`), recorder.all())
	require.Error(t, repo.gormClient.Where(queryDeviceID, "device-unique").First(&models.Device{}).Error)
}

// Merging devices that share a WWN copies the source's points to the destination, which keeps that
// WWN, so deleting the source by WWN would delete the copied history too.
func TestMergeSourceDeleteSkipsWWNSharedWithDestination(t *testing.T) {
	repo := createDeviceRegisterTestRepository(t)
	recorder := withInfluxDeleteRecorder(t, repo)
	ctx := context.Background()
	createSharedWWNDevices(t, repo)

	require.NoError(t, repo.deleteInfluxDeviceHistory(ctx, &models.Device{DeviceID: "device-a", WWN: "wwn-shared"}))
	require.ElementsMatch(t, bucketPredicates(`device_id="device-a"`), recorder.all())

	recorder.reset()
	require.NoError(t, repo.deleteInfluxDeviceHistory(ctx, &models.Device{DeviceID: "device-unique", WWN: "wwn-unique"}))
	require.ElementsMatch(t, bucketPredicates(`device_id="device-unique"`, `device_wwn="wwn-unique"`), recorder.all())
}
