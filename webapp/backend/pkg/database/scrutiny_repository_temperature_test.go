package database

import (
	"context"
	mock_config "github.com/analogj/scrutiny/webapp/backend/pkg/config/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/collector"
	"github.com/golang/mock/gomock"
	influxapi "github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
	"github.com/stretchr/testify/require"
	"io"
	"strings"
	"testing"
)

type temperatureErrorQuery struct{ stubQueryAPI }

func (s *temperatureErrorQuery) Query(_ context.Context, _ string) (*influxapi.QueryTableResult, error) {
	return influxapi.NewQueryTableResult(io.NopCloser(strings.NewReader("invalid,csv\n"))), nil
}

func TestTemperatureHistoryPropagatesStreamErrors(t *testing.T) {
	repo := createDeviceSelfTestRepository(t)
	repo.influxQueryApi = &temperatureErrorQuery{}
	history, err := repo.GetSmartTemperatureHistory(context.Background(), DURATION_KEY_DAY)
	require.Error(t, err)
	require.Nil(t, history)
}

type countingWriteAPI struct {
	stubWriteAPI
	points int
}

func (s *countingWriteAPI) WritePoint(_ context.Context, points ...*write.Point) error {
	s.points += len(points)
	return nil
}

func TestSaveSmartTemperatureSkipsWritesWhenHistoryStorageDisabled(t *testing.T) {
	writeAPI := &countingWriteAPI{}
	deviceRepo := scrutinyRepository{influxWriteApi: writeAPI}

	err := deviceRepo.SaveSmartTemperature(context.Background(), "wwn-1", "device-1", &collector.SmartInfo{}, true, false)

	require.NoError(t, err)
	require.Zero(t, writeAPI.points)
}

func TestSaveSmartTemperatureStoresCurrentPointWhenHistoryStorageEnabled(t *testing.T) {
	writeAPI := &countingWriteAPI{}
	deviceRepo := scrutinyRepository{influxWriteApi: writeAPI}

	err := deviceRepo.SaveSmartTemperature(context.Background(), "wwn-1", "device-1", &collector.SmartInfo{}, false, true)

	require.NoError(t, err)
	require.Equal(t, 1, writeAPI.points)
}

func TestAggregateTempQueryFiltersSelectedWWNs(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetString("web.influxdb.bucket").Return("metrics").AnyTimes()

	deviceRepo := scrutinyRepository{appConfig: fakeConfig}
	influxDBScript := deviceRepo.aggregateTempQuery(DURATION_KEY_WEEK, "wwn-1", `wwn-"2`)

	require.Contains(t, influxDBScript, `contains(value: r["device_wwn"], set: ["wwn-1", "wwn-\"2"])`)
	require.Equal(t, 1, strings.Count(influxDBScript, `contains(value: r["device_wwn"]`))
}

func Test_aggregateTempQuery_Day(t *testing.T) {
	t.Parallel()

	//setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetString("web.influxdb.bucket").Return("metrics").AnyTimes()
	fakeConfig.EXPECT().GetString("web.influxdb.org").Return("scrutiny").AnyTimes()

	deviceRepo := scrutinyRepository{
		appConfig: fakeConfig,
	}

	aggregationType := DURATION_KEY_DAY

	//test
	influxDbScript := deviceRepo.aggregateTempQuery(aggregationType)

	//assert
	require.Equal(t, `import "influxdata/influxdb/schema"
dayData = from(bucket: "metrics")
|> range(start: -1d, stop: now())
|> filter(fn: (r) => r["_measurement"] == "temp" )
|> group(columns: ["device_wwn"])
|> toInt()

dayData
|> schema.fieldsAsCols()
|> yield()`, influxDbScript)
}

func Test_aggregateTempQuery_Week(t *testing.T) {
	t.Parallel()

	//setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetString("web.influxdb.bucket").Return("metrics").AnyTimes()
	fakeConfig.EXPECT().GetString("web.influxdb.org").Return("scrutiny").AnyTimes()

	deviceRepo := scrutinyRepository{
		appConfig: fakeConfig,
	}

	aggregationType := DURATION_KEY_WEEK

	//test
	influxDbScript := deviceRepo.aggregateTempQuery(aggregationType)

	//assert
	require.Equal(t, `import "influxdata/influxdb/schema"
weekData = from(bucket: "metrics")
|> range(start: -1w, stop: now())
|> filter(fn: (r) => r["_measurement"] == "temp" )
|> aggregateWindow(every: 1h, fn: mean, createEmpty: false)
|> group(columns: ["device_wwn"])
|> toInt()

weekData
|> schema.fieldsAsCols()
|> yield()`, influxDbScript)
}

func Test_aggregateTempQuery_Month(t *testing.T) {
	t.Parallel()

	//setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetString("web.influxdb.bucket").Return("metrics").AnyTimes()
	fakeConfig.EXPECT().GetString("web.influxdb.org").Return("scrutiny").AnyTimes()

	deviceRepo := scrutinyRepository{
		appConfig: fakeConfig,
	}

	aggregationType := DURATION_KEY_MONTH

	//test
	influxDbScript := deviceRepo.aggregateTempQuery(aggregationType)

	//assert
	require.Equal(t, `import "influxdata/influxdb/schema"
weekData = from(bucket: "metrics")
|> range(start: -1w, stop: now())
|> filter(fn: (r) => r["_measurement"] == "temp" )
|> aggregateWindow(every: 1h, fn: mean, createEmpty: false)
|> group(columns: ["device_wwn"])
|> toInt()

monthData = from(bucket: "metrics_weekly")
|> range(start: -1mo, stop: -1w)
|> filter(fn: (r) => r["_measurement"] == "temp" )
|> aggregateWindow(every: 1h, fn: mean, createEmpty: false)
|> group(columns: ["device_wwn"])
|> toInt()

union(tables: [weekData, monthData])
|> group(columns: ["device_wwn"])
|> sort(columns: ["_time"], desc: false)
|> schema.fieldsAsCols()`, influxDbScript)
}

func Test_aggregateTempQuery_Year(t *testing.T) {
	t.Parallel()

	//setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetString("web.influxdb.bucket").Return("metrics").AnyTimes()
	fakeConfig.EXPECT().GetString("web.influxdb.org").Return("scrutiny").AnyTimes()

	deviceRepo := scrutinyRepository{
		appConfig: fakeConfig,
	}

	aggregationType := DURATION_KEY_YEAR

	//test
	influxDbScript := deviceRepo.aggregateTempQuery(aggregationType)

	//assert
	require.Equal(t, `import "influxdata/influxdb/schema"
weekData = from(bucket: "metrics")
|> range(start: -1w, stop: now())
|> filter(fn: (r) => r["_measurement"] == "temp" )
|> aggregateWindow(every: 1h, fn: mean, createEmpty: false)
|> group(columns: ["device_wwn"])
|> toInt()

monthData = from(bucket: "metrics_weekly")
|> range(start: -1mo, stop: -1w)
|> filter(fn: (r) => r["_measurement"] == "temp" )
|> aggregateWindow(every: 1h, fn: mean, createEmpty: false)
|> group(columns: ["device_wwn"])
|> toInt()

yearData = from(bucket: "metrics_monthly")
|> range(start: -1y, stop: -1mo)
|> filter(fn: (r) => r["_measurement"] == "temp" )
|> aggregateWindow(every: 1h, fn: mean, createEmpty: false)
|> group(columns: ["device_wwn"])
|> toInt()

union(tables: [weekData, monthData, yearData])
|> group(columns: ["device_wwn"])
|> sort(columns: ["_time"], desc: false)
|> schema.fieldsAsCols()`, influxDbScript)
}

func Test_aggregateTempQuery_Forever(t *testing.T) {
	t.Parallel()

	//setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetString("web.influxdb.bucket").Return("metrics").AnyTimes()
	fakeConfig.EXPECT().GetString("web.influxdb.org").Return("scrutiny").AnyTimes()

	deviceRepo := scrutinyRepository{
		appConfig: fakeConfig,
	}

	aggregationType := DURATION_KEY_FOREVER

	//test
	influxDbScript := deviceRepo.aggregateTempQuery(aggregationType)

	//assert
	require.Equal(t, `import "influxdata/influxdb/schema"
weekData = from(bucket: "metrics")
|> range(start: -1w, stop: now())
|> filter(fn: (r) => r["_measurement"] == "temp" )
|> aggregateWindow(every: 1h, fn: mean, createEmpty: false)
|> group(columns: ["device_wwn"])
|> toInt()

monthData = from(bucket: "metrics_weekly")
|> range(start: -1mo, stop: -1w)
|> filter(fn: (r) => r["_measurement"] == "temp" )
|> aggregateWindow(every: 1h, fn: mean, createEmpty: false)
|> group(columns: ["device_wwn"])
|> toInt()

yearData = from(bucket: "metrics_monthly")
|> range(start: -1y, stop: -1mo)
|> filter(fn: (r) => r["_measurement"] == "temp" )
|> aggregateWindow(every: 1h, fn: mean, createEmpty: false)
|> group(columns: ["device_wwn"])
|> toInt()

foreverData = from(bucket: "metrics_yearly")
|> range(start: -10y, stop: -1y)
|> filter(fn: (r) => r["_measurement"] == "temp" )
|> aggregateWindow(every: 1h, fn: mean, createEmpty: false)
|> group(columns: ["device_wwn"])
|> toInt()

union(tables: [weekData, monthData, yearData, foreverData])
|> group(columns: ["device_wwn"])
|> sort(columns: ["_time"], desc: false)
|> schema.fieldsAsCols()`, influxDbScript)
}
