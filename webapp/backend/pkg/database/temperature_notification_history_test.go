package database

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	mock_config "github.com/analogj/scrutiny/webapp/backend/pkg/config/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	"github.com/golang/mock/gomock"
	influxapi "github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/stretchr/testify/require"
)

const notificationHistoryCSV = `#datatype,string,long,dateTime:RFC3339,long,string,string
#group,false,false,false,false,true,true
#default,_result,,,,,
,result,table,_time,_value,device_id,device_wwn
`

type notificationHistoryQuery struct {
	stubQueryAPI
	response string
	query    string
	err      error
}

func (q *notificationHistoryQuery) Query(ctx context.Context, query string) (*influxapi.QueryTableResult, error) {
	q.query = query
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if q.err != nil {
		return nil, q.err
	}
	return influxapi.NewQueryTableResult(io.NopCloser(strings.NewReader(q.response))), nil
}

func TestTemperatureNotificationHistoryUsesDeviceIdentity(t *testing.T) {
	for _, wwn := range []string{"shared", ""} {
		t.Run("wwn="+wwn, func(t *testing.T) {
			query := &notificationHistoryQuery{response: notificationHistoryCSV + fmt.Sprintf(`,,0,2026-09-07T11:00:00Z,60,neighbor,%s
,,0,2026-09-07T11:10:00Z,60,,%s
,,0,2026-09-07T11:20:00Z,60,obsolete-id,%s
,,0,2026-09-07T12:00:00Z,60,drive,%s
,,0,2026-09-07T12:00:00Z,60,neighbor,%s
,,0,2026-09-07T11:50:00Z,40,neighbor,%s
,,0,2026-09-07T11:40:00Z,55,drive,%s
`, wwn, wwn, wwn, wwn, wwn, wwn, wwn)}
			cfg := mock_config.NewMockInterface(gomock.NewController(t))
			cfg.EXPECT().GetString("web.influxdb.bucket").Return("metrics")
			repo := scrutinyRepository{appConfig: cfg, influxQueryApi: query}
			points, err := repo.GetTemperatureNotificationHistory(context.Background(), "drive")
			require.NoError(t, err)
			require.Equal(t, []measurements.SmartTemperature{
				{Date: time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC), Temp: 60},
				{Date: time.Date(2026, 9, 7, 11, 40, 0, 0, time.UTC), Temp: 55},
			}, points)
			require.Contains(t, query.query, `exists r["device_id"] and r["device_id"] == "drive"`)
			require.Contains(t, query.query, `range(start: -1d, stop: now())`)
			require.Contains(t, query.query, `r["_field"] == "temp"`)
			require.NotContains(t, query.query, "device_wwn")
			require.NotContains(t, query.query, "aggregateWindow")
		})
	}
}

func TestTemperatureNotificationHistoryErrorsAndEscaping(t *testing.T) {
	const escapedID = `drive-"\\id`
	for _, scenario := range []string{"escaped", "empty result", "missing tag", "empty id", "query error", "stream error", "canceled"} {
		t.Run(scenario, func(t *testing.T) {
			query := &notificationHistoryQuery{response: notificationHistoryCSV}
			deviceID := escapedID
			ctx := context.Background()
			switch scenario {
			case "missing tag":
				query.response = "#datatype,string,long,dateTime:RFC3339,long\n#group,false,false,false,false\n#default,_result,,,\n,result,table,_time,_value\n,,0,2026-09-07T12:00:00Z,60\n"
			case "empty id":
				deviceID = ""
			case "query error":
				query.err = errors.New("unavailable")
			case "stream error":
				query.response += ",,0,invalid-date,60,drive,shared\n"
			case "canceled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			cfg := mock_config.NewMockInterface(gomock.NewController(t))
			if deviceID != "" {
				cfg.EXPECT().GetString("web.influxdb.bucket").Return("metrics")
			}
			repo := scrutinyRepository{appConfig: cfg, influxQueryApi: query}
			points, err := repo.GetTemperatureNotificationHistory(ctx, deviceID)
			if scenario == "empty id" || scenario == "query error" || scenario == "stream error" || scenario == "canceled" {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Empty(t, points)
			if deviceID == "" {
				require.Empty(t, query.query)
			} else {
				require.Contains(t, query.query, `r["device_id"] == `+strconv.Quote(deviceID))
			}
		})
	}
}
