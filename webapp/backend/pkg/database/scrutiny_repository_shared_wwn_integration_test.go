package database

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	mock_config "github.com/analogj/scrutiny/webapp/backend/pkg/config/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	"github.com/golang/mock/gomock"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// fixes #851: exercises the device_id history rules against a real InfluxDB (CI runs influxdb:2.9).
// One device holds its WWN alone and has device_id-tagged, untagged and stale-tagged points (an
// earlier device_id of the same device). Two devices share a WWN, next to an untagged point that
// neither of them can own.
func TestSharedWWNHistory_Integration(t *testing.T) {
	ResetMigrationGuardForTests()

	repo := newSharedWWNIntegrationRepository(t, integrationInfluxHost(t))
	ctx := context.Background()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	uniqueWWN := "wwn-unique-" + suffix
	sharedWWN := "wwn-shared-" + suffix
	unique := models.Device{DeviceID: "unique-" + suffix, WWN: uniqueWWN, DeviceProtocol: "ATA"}
	sharedA := models.Device{DeviceID: "shared-a-" + suffix, WWN: sharedWWN, DeviceProtocol: "ATA"}
	sharedB := models.Device{DeviceID: "shared-b-" + suffix, WWN: sharedWWN, DeviceProtocol: "ATA"}
	for _, device := range []models.Device{unique, sharedA, sharedB} {
		require.NoError(t, repo.gormClient.Create(&device).Error)
	}
	t.Cleanup(func() {
		for _, bucket := range repo.deviceHistoryBuckets() {
			for _, wwn := range []string{uniqueWWN, sharedWWN} {
				_ = repo.influxClient.DeleteAPI().DeleteWithName(context.Background(), "scrutiny", bucket,
					time.Now().AddDate(-10, 0, 0), time.Now().AddDate(10, 0, 0), fmt.Sprintf("device_wwn=%q", wwn))
			}
		}
	})

	now := time.Now().Truncate(time.Second)
	writePoints := func(tags map[string]string, temp int64, at time.Time) {
		t.Helper()
		smartTags := map[string]string{"device_protocol": "ATA"}
		for key, value := range tags {
			smartTags[key] = value
		}
		require.NoError(t, repo.influxWriteApi.WritePoint(ctx, influxdb2.NewPoint("smart", smartTags,
			map[string]interface{}{"temp": temp, "power_on_hours": int64(1000)}, at)))
		require.NoError(t, repo.influxWriteApi.WritePoint(ctx, influxdb2.NewPoint("temp", tags,
			map[string]interface{}{"temp": temp}, at)))
	}
	writePoints(map[string]string{"device_id": unique.DeviceID, "device_wwn": uniqueWWN}, 30, now.Add(-1*time.Hour))
	writePoints(map[string]string{"device_wwn": uniqueWWN}, 31, now.Add(-2*time.Hour))
	writePoints(map[string]string{"device_id": unique.DeviceID + "-legacy", "device_wwn": uniqueWWN}, 32, now.Add(-3*time.Hour))
	writePoints(map[string]string{"device_id": sharedA.DeviceID, "device_wwn": sharedWWN}, 40, now.Add(-1*time.Hour))
	writePoints(map[string]string{"device_id": sharedB.DeviceID, "device_wwn": sharedWWN}, 50, now.Add(-1*time.Hour))
	writePoints(map[string]string{"device_wwn": sharedWWN}, 60, now.Add(-2*time.Hour))

	smartTemps := func(deviceID string) []int64 {
		t.Helper()
		history, err := repo.GetSmartAttributeHistory(ctx, deviceID, DURATION_KEY_WEEK, 0, 0, nil)
		require.NoError(t, err)
		temps := []int64{}
		for _, smart := range history {
			temps = append(temps, smart.Temp)
		}
		return temps
	}
	temperatureTemps := func(history []measurements.SmartTemperature) []int64 {
		temps := []int64{}
		for _, point := range history {
			temps = append(temps, point.Temp)
		}
		return temps
	}

	// single-device history
	require.ElementsMatch(t, []int64{30, 31, 32}, smartTemps(unique.DeviceID))
	require.ElementsMatch(t, []int64{40}, smartTemps(sharedA.DeviceID))
	require.ElementsMatch(t, []int64{50}, smartTemps(sharedB.DeviceID))

	// summary keeps each device's newest row
	summaries, err := repo.GetSummary(ctx)
	require.NoError(t, err)
	for deviceID, expected := range map[string]int64{unique.DeviceID: 30, sharedA.DeviceID: 40, sharedB.DeviceID: 50} {
		require.NotNil(t, summaries[deviceID].SmartResults, deviceID)
		require.NotNil(t, summaries[deviceID].SmartResults.Temp, deviceID)
		require.Equal(t, expected, *summaries[deviceID].SmartResults.Temp, deviceID)
	}

	// temperature history, for every device and for a selection
	temperatures, err := repo.GetSmartTemperatureHistory(ctx, DURATION_KEY_DAY)
	require.NoError(t, err)
	require.ElementsMatch(t, []int64{30, 31, 32}, temperatureTemps(temperatures[unique.DeviceID]))
	require.ElementsMatch(t, []int64{40}, temperatureTemps(temperatures[sharedA.DeviceID]))
	require.ElementsMatch(t, []int64{50}, temperatureTemps(temperatures[sharedB.DeviceID]))

	selected, err := repo.GetSmartTemperatureHistoryForDevices(ctx, DURATION_KEY_DAY, []string{unique.DeviceID, sharedA.DeviceID})
	require.NoError(t, err)
	require.ElementsMatch(t, []int64{30, 31, 32}, temperatureTemps(selected[unique.DeviceID]))
	require.ElementsMatch(t, []int64{40}, temperatureTemps(selected[sharedA.DeviceID]))
	require.NotContains(t, selected, sharedB.DeviceID)

	// deleting one of the devices that share a WWN keeps the other device's points and the unowned one
	require.NoError(t, repo.DeleteDevice(ctx, sharedA.DeviceID))
	require.ElementsMatch(t, []int64{50}, smartTemps(sharedB.DeviceID))
	require.Equal(t, int64(2), countSmartTempPoints(t, repo, sharedWWN))

	// deleting the device that alone holds its WWN also removes its untagged and stale-tagged points
	require.NoError(t, repo.DeleteDevice(ctx, unique.DeviceID))
	require.Zero(t, countSmartTempPoints(t, repo, uniqueWWN))
}

func countSmartTempPoints(t *testing.T, repo *scrutinyRepository, wwn string) int64 {
	t.Helper()
	result, err := repo.influxQueryApi.Query(context.Background(), fmt.Sprintf(`from(bucket: "metrics")
|> range(start: -1d)
|> filter(fn: (r) => r["_measurement"] == "smart" and r["_field"] == "temp" and r["device_wwn"] == %q)
|> count()`, wwn))
	require.NoError(t, err)
	defer result.Close()
	var total int64
	for result.Next() {
		if count, ok := result.Record().Value().(int64); ok {
			total += count
		}
	}
	require.NoError(t, result.Err())
	return total
}

func newSharedWWNIntegrationRepository(t *testing.T, influxHost string) *scrutinyRepository {
	t.Helper()
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetString("web.database.location").Return(filepath.Join(t.TempDir(), "scrutiny_test.db")).AnyTimes()
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
	// Summary, history and delete paths read further settings; zero values keep them at defaults.
	fakeConfig.EXPECT().GetString(gomock.Any()).Return("").AnyTimes()
	fakeConfig.EXPECT().GetBool(gomock.Any()).Return(false).AnyTimes()
	fakeConfig.EXPECT().GetInt(gomock.Any()).Return(0).AnyTimes()
	fakeConfig.EXPECT().GetIntSlice(gomock.Any()).Return(nil).AnyTimes()
	fakeConfig.EXPECT().GetStringSlice(gomock.Any()).Return(nil).AnyTimes()
	fakeConfig.EXPECT().Get(gomock.Any()).Return(nil).AnyTimes()
	fakeConfig.EXPECT().IsSet(gomock.Any()).Return(false).AnyTimes()

	repoIface, err := NewScrutinyRepository(fakeConfig, logrus.WithField("test", t.Name()))
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = repoIface.Close()
	})
	return repoIface.(*scrutinyRepository)
}

// integrationInfluxHost returns a reachable InfluxDB host, or skips the test. The CI "Test Backend"
// job runs on the runner rather than in a job container, so its influxdb service is published on
// localhost:8086; the "influxdb" host name resolves only from inside a job container. Choosing
// "influxdb" whenever GITHUB_ACTIONS was set made every integration test skip in CI.
func integrationInfluxHost(t *testing.T) string {
	t.Helper()
	client := &http.Client{Timeout: 2 * time.Second}
	for _, host := range []string{"localhost", "influxdb"} {
		response, err := client.Get(fmt.Sprintf("http://%s:8086/api/v2/setup", host))
		if err == nil {
			_ = response.Body.Close()
			return host
		}
	}
	t.Skip("Skipping integration test: InfluxDB not available at localhost:8086 or influxdb:8086")
	return ""
}
