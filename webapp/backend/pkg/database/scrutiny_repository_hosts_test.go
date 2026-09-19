package database

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	mock_config "github.com/analogj/scrutiny/webapp/backend/pkg/config/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/glebarez/sqlite"
	"github.com/golang/mock/gomock"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newHostManagementTestRepository(t *testing.T) *scrutinyRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Device{}))
	return &scrutinyRepository{gormClient: db}
}

func TestGetHostsCountsSMARTDevicesAndExcludesEmptyHostIDs(t *testing.T) {
	repo := newHostManagementTestRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.gormClient.Create(&[]models.Device{
		{DeviceID: "a-active", HostId: "alpha"},
		{DeviceID: "a-archived", HostId: "alpha", Archived: true},
		{DeviceID: "b-active", HostId: "beta"},
		{DeviceID: "empty", HostId: ""},
		{DeviceID: "spaces", HostId: "   "},
	}).Error)

	hosts, err := repo.GetHosts(ctx)

	require.NoError(t, err)
	require.Equal(t, []models.HostSummary{
		{HostID: "alpha", ActiveDevices: 1, ArchivedDevices: 1, TotalDevices: 2},
		{HostID: "beta", ActiveDevices: 1, ArchivedDevices: 0, TotalDevices: 1},
	}, hosts)
}

func TestUpdateHostArchivedUpdatesEveryDeviceInHost(t *testing.T) {
	repo := newHostManagementTestRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.gormClient.Create(&[]models.Device{
		{DeviceID: "a-1", HostId: "alpha"},
		{DeviceID: "a-2", HostId: "alpha"},
		{DeviceID: "b-1", HostId: "beta"},
	}).Error)

	count, err := repo.UpdateHostArchived(ctx, "alpha", true)

	require.NoError(t, err)
	require.Equal(t, int64(2), count)
	var activeAlpha int64
	require.NoError(t, repo.gormClient.Model(&models.Device{}).Where("host_id = ? AND archived = false", "alpha").Count(&activeAlpha).Error)
	require.Zero(t, activeAlpha)
}

// A WWN may be deleted by predicate only when every holder is being purged; host-less devices count
// as outside the selection.
func TestWWNsHeldOnlyBySelectedHosts(t *testing.T) {
	devices := []models.Device{
		{DeviceID: "a", HostId: "alpha", WWN: "shared-selected"},
		{DeviceID: "b", HostId: "beta", WWN: "shared-selected"},
		{DeviceID: "a-outside", HostId: "alpha", WWN: "shared-outside"},
		{DeviceID: "c", HostId: "gamma", WWN: "shared-outside"},
		{DeviceID: "a-hostless", HostId: "alpha", WWN: "shared-hostless"},
		{DeviceID: "hostless", HostId: "", WWN: "shared-hostless"},
		{DeviceID: "a-no-wwn", HostId: "alpha"},
	}
	selected := map[string]struct{}{"alpha": {}, "beta": {}}

	require.Equal(t, map[string]struct{}{"shared-selected": {}}, wwnsHeldOnlyBySelectedHosts(devices, selected))
}

// fixes #851: history is deleted by device_id, so a WWN shared with a device on another host no longer
// blocks the purge. Deleting by that WWN would remove the other device's points, so it is skipped.
func TestPurgeHostsDeletesByDeviceIDWhenWWNSharedOutsideSelection(t *testing.T) {
	repo := newHostManagementTestRepository(t)
	recorder := withInfluxDeleteRecorder(t, repo)
	ctx := context.Background()
	require.NoError(t, repo.gormClient.Create(&[]models.Device{
		{DeviceID: "a", HostId: "alpha", WWN: "shared"},
		{DeviceID: "b", HostId: "beta", WWN: "shared"},
	}).Error)

	results, err := repo.PurgeHosts(ctx, []string{"alpha"})

	require.NoError(t, err)
	require.Len(t, results, 1)
	require.True(t, results[0].Success, results[0].Error)
	require.ElementsMatch(t, bucketPredicates(`device_id="a"`), recorder.all())
	var remaining []models.Device
	require.NoError(t, repo.gormClient.Find(&remaining).Error)
	require.Len(t, remaining, 1)
	require.Equal(t, "b", remaining[0].DeviceID)
}

func TestPurgeHostsWithoutWWNIsRetrySafe(t *testing.T) {
	repo := newHostManagementTestRepository(t)
	recorder := withInfluxDeleteRecorder(t, repo)
	ctx := context.Background()
	require.NoError(t, repo.gormClient.Create(&models.Device{DeviceID: "a", HostId: "alpha"}).Error)

	first, err := repo.PurgeHosts(ctx, []string{"alpha"})
	require.NoError(t, err)
	require.True(t, first[0].Success)
	require.Equal(t, int64(1), first[0].DeviceCount)
	require.ElementsMatch(t, bucketPredicates(`device_id="a"`), recorder.all())

	second, err := repo.PurgeHosts(ctx, []string{"alpha"})
	require.NoError(t, err)
	require.False(t, second[0].Success)
	require.Equal(t, "host not found", second[0].Error)
}

func TestPurgeHostsKeepsSQLiteRowsAfterInfluxFailureAndCanRetry(t *testing.T) {
	var failDeletes atomic.Bool
	failDeletes.Store(true)
	deleteServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "/api/v2/delete", request.URL.Path)
		if failDeletes.Load() {
			http.Error(writer, "temporary failure", http.StatusServiceUnavailable)
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(deleteServer.Close)

	ctrl := gomock.NewController(t)
	appConfig := mock_config.NewMockInterface(ctrl)
	appConfig.EXPECT().GetString(cfgInfluxDBBucket).Return("metrics").AnyTimes()
	appConfig.EXPECT().GetString(cfgInfluxDBOrg).Return("scrutiny").AnyTimes()

	repo := newHostManagementTestRepository(t)
	repo.appConfig = appConfig
	repo.logger = logrus.WithField("test", t.Name())
	repo.influxClient = influxdb2.NewClient(deleteServer.URL, "token")
	t.Cleanup(repo.influxClient.Close)
	ctx := context.Background()
	require.NoError(t, repo.gormClient.Create(&models.Device{DeviceID: "a", HostId: "alpha", WWN: "wwn-a"}).Error)

	first, err := repo.PurgeHosts(ctx, []string{"alpha"})
	require.NoError(t, err)
	require.False(t, first[0].Success)
	require.Contains(t, first[0].Error, "could not delete history")
	var count int64
	require.NoError(t, repo.gormClient.Model(&models.Device{}).Where("host_id = ?", "alpha").Count(&count).Error)
	require.Equal(t, int64(1), count)

	failDeletes.Store(false)
	second, err := repo.PurgeHosts(ctx, []string{"alpha"})
	require.NoError(t, err)
	require.True(t, second[0].Success)
	require.NoError(t, repo.gormClient.Model(&models.Device{}).Where("host_id = ?", "alpha").Count(&count).Error)
	require.Zero(t, count)
}
