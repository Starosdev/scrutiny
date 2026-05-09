package database

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	mock_config "github.com/analogj/scrutiny/webapp/backend/pkg/config/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/deviceid"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/golang/mock/gomock"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestMergeDevices_Integration(t *testing.T) {
	influxHost := "localhost"
	if _, isGithubActions := os.LookupEnv("GITHUB_ACTIONS"); isGithubActions {
		influxHost = "influxdb"
	}

	client := &http.Client{Timeout: 2 * time.Second}
	_, err := client.Get(fmt.Sprintf("http://%s:8086/api/v2/setup", influxHost))
	if err != nil {
		t.Skip("Skipping integration test: InfluxDB not available at " + influxHost + ":8086")
	}

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "scrutiny_test.db")
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetString("web.database.location").Return(dbPath).AnyTimes()
	fakeConfig.EXPECT().GetString("web.database.journal_mode").Return("WAL").AnyTimes()
	fakeConfig.EXPECT().GetString("log.level").Return("INFO").AnyTimes()
	fakeConfig.EXPECT().GetString("web.influxdb.scheme").Return("http").AnyTimes()
	fakeConfig.EXPECT().GetString("web.influxdb.host").Return(influxHost).AnyTimes()
	fakeConfig.EXPECT().GetString("web.influxdb.port").Return("8086").AnyTimes()
	fakeConfig.EXPECT().GetString("web.influxdb.token").Return("my-super-secret-auth-token").AnyTimes()
	fakeConfig.EXPECT().GetString("web.influxdb.org").Return("scrutiny").AnyTimes()
	fakeConfig.EXPECT().GetString("web.influxdb.bucket").Return("metrics").AnyTimes()
	fakeConfig.EXPECT().GetString("web.influxdb.init_username").Return("admin").AnyTimes()
	fakeConfig.EXPECT().GetString("web.influxdb.init_password").Return("password12345").AnyTimes()
	fakeConfig.EXPECT().GetBool("web.influxdb.tls.insecure_skip_verify").Return(false).AnyTimes()
	fakeConfig.EXPECT().GetBool("web.influxdb.retention_policy").Return(false).AnyTimes()
	fakeConfig.EXPECT().GetInt("web.influxdb.retention_policy.default_retention_period_days").Return(0).AnyTimes()
	fakeConfig.EXPECT().GetInt("web.influxdb.retention_policy.monthly_retention_period_months").Return(0).AnyTimes()
	fakeConfig.EXPECT().GetInt("web.influxdb.retention_policy.yearly_retention_period_months").Return(0).AnyTimes()
	fakeConfig.EXPECT().GetIntSlice("failures.transient.ata").Return([]int{195}).AnyTimes()
	fakeConfig.EXPECT().GetStringSlice("failures.ignored.devstat").Return([]string{}).AnyTimes()
	fakeConfig.EXPECT().Get("smart.attribute_overrides").Return(nil).AnyTimes()

	repoIface, err := NewScrutinyRepository(fakeConfig, logrus.WithField("test", t.Name()))
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = repoIface.Close()
	})

	repo := repoIface.(*scrutinyRepository)
	ctx := context.Background()

	sourceDevice := models.Device{
		DeviceID:     deviceid.Generate("model-a", "serial-a", "wwn-source"),
		ModelName:    "model-a",
		SerialNumber: "serial-a",
		WWN:          "wwn-source",
	}
	destinationDevice := models.Device{
		DeviceID:     deviceid.Generate("model-a", "serial-a", "wwn-destination"),
		ModelName:    "model-a",
		SerialNumber: "serial-a",
		WWN:          "wwn-destination",
	}

	require.NoError(t, repo.RegisterDevice(ctx, sourceDevice))
	require.NoError(t, repo.RegisterDevice(ctx, destinationDevice))

	olderCreatedAt := time.Now().Add(-48 * time.Hour)
	require.NoError(t, repo.gormClient.Model(&models.Device{}).Where(queryDeviceID, sourceDevice.DeviceID).Update("created_at", olderCreatedAt).Error)

	tempPoint := influxdb2.NewPoint(
		"temp",
		map[string]string{
			"device_wwn": sourceDevice.WWN,
			"device_id":  sourceDevice.DeviceID,
		},
		map[string]interface{}{
			"temp": int64(42),
		},
		time.Now().Add(-1*time.Hour),
	)
	require.NoError(t, repo.influxWriteApi.WritePoint(ctx, tempPoint))

	require.NoError(t, repo.MergeDevices(ctx, sourceDevice.DeviceID, destinationDevice.DeviceID))

	_, err = repo.GetDeviceDetails(ctx, sourceDevice.DeviceID)
	require.Error(t, err)

	updatedDestination, err := repo.GetDeviceDetails(ctx, destinationDevice.DeviceID)
	require.NoError(t, err)
	require.True(t, updatedDestination.CreatedAt.Equal(olderCreatedAt) || updatedDestination.CreatedAt.Before(olderCreatedAt.Add(1*time.Second)))

	tempHistory, err := repo.GetSmartTemperatureHistory(ctx, DURATION_KEY_DAY)
	require.NoError(t, err)
	require.NotEmpty(t, tempHistory[destinationDevice.DeviceID])
	require.Empty(t, tempHistory[sourceDevice.DeviceID])
}
