package database

import (
	"strings"
	"testing"

	mock_config "github.com/analogj/scrutiny/webapp/backend/pkg/config/mock"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestDeviceHistoryQueriesUseDeviceIdentityInsteadOfHostID(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetString(cfgInfluxDBBucket).Return("metrics").AnyTimes()

	deviceRepo := scrutinyRepository{appConfig: fakeConfig}

	historyFilter := deviceHistoryPredicate("device-1", "wwn-1", true)
	smartQuery := deviceRepo.aggregateSmartAttributesQuery(historyFilter, DURATION_KEY_FOREVER, 1, 0, nil)
	require.Contains(t, smartQuery, `r["device_id"] == "device-1"`)
	require.NotContains(t, smartQuery, "host_id")
	// a device's series are pivoted and merged, and each day keeps its newest whole row
	mergeSeries := strings.Join([]string{
		"|> schema.fieldsAsCols()",
		"|> group()",
		`|> sort(columns: ["_time"])`,
		"|> window(every: 1d, createEmpty: false)",
		`|> last(column: "_time")`,
		`|> duplicate(column: "_stop", as: "_time")`,
		"|> group()",
		"|> tail(n: 1, offset: 0)",
	}, "\n")
	require.Contains(t, smartQuery, mergeSeries)
	require.NotContains(t, smartQuery, "aggregateWindow")

	temperatureQuery := deviceRepo.aggregateTempQuery(DURATION_KEY_FOREVER, []string{"device-1"}, []string{"wwn-1"})
	require.Contains(t, temperatureQuery, `r["device_wwn"]`)
	require.NotContains(t, temperatureQuery, "host_id")
}

// fixes #851: points match on device_id, and on device_wwn only while a single device holds that
// WWN. The WWN branch also reaches points tagged with an earlier device_id of the device; a shared
// WWN must not pull in another device's history.
func TestDeviceHistoryPredicate(t *testing.T) {
	t.Parallel()

	require.Equal(t,
		`(exists r["device_id"] and r["device_id"] == "device-1") or (exists r["device_wwn"] and r["device_wwn"] == "wwn-1")`,
		deviceHistoryPredicate("device-1", "wwn-1", true))
	require.Equal(t, `(exists r["device_id"] and r["device_id"] == "device-1")`, deviceHistoryPredicate("device-1", "wwn-1", false))
}
