package database

import (
	"testing"

	mock_config "github.com/analogj/scrutiny/webapp/backend/pkg/config/mock"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestDeviceHistoryQueriesUseStableWWNInsteadOfHostID(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	fakeConfig := mock_config.NewMockInterface(mockCtrl)
	fakeConfig.EXPECT().GetString(cfgInfluxDBBucket).Return("metrics").AnyTimes()

	deviceRepo := scrutinyRepository{appConfig: fakeConfig}

	smartQuery := deviceRepo.aggregateSmartAttributesQuery("wwn-1", DURATION_KEY_FOREVER, 1, 0, nil)
	require.Contains(t, smartQuery, `r["device_wwn"] == "wwn-1"`)
	require.NotContains(t, smartQuery, "host_id")

	temperatureQuery := deviceRepo.aggregateTempQuery(DURATION_KEY_FOREVER, "wwn-1")
	require.Contains(t, temperatureQuery, `r["device_wwn"]`)
	require.NotContains(t, temperatureQuery, "host_id")
}
