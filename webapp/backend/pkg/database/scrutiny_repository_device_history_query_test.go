package database

import (
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

	temperatureQuery := deviceRepo.aggregateTempQuery(DURATION_KEY_FOREVER, []string{"device-1"}, []string{"wwn-1"})
	require.Contains(t, temperatureQuery, `r["device_wwn"]`)
	require.NotContains(t, temperatureQuery, "host_id")
}

// fixes #851: points match on device_id. Points without a device_id tag fall back to
// device_wwn only while a single device holds that WWN; a shared WWN must not pull in
// another device's untagged history.
func TestDeviceHistoryPredicate(t *testing.T) {
	t.Parallel()

	require.Equal(t,
		`(exists r["device_id"] and r["device_id"] == "device-1") or (not exists r["device_id"] and r["device_wwn"] == "wwn-1")`,
		deviceHistoryPredicate("device-1", "wwn-1", true))
	require.Equal(t, `(exists r["device_id"] and r["device_id"] == "device-1")`, deviceHistoryPredicate("device-1", "wwn-1", false))
}
