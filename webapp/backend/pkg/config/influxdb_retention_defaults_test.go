package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The weekly task reads data 7-14 days old and the monthly task reads up to 2 months old. A missed
// run loses data unless the source bucket keeps it for a margin beyond the end of that window.
func TestInfluxDBRetentionDefaultsLeaveMarginForMissedTaskRuns(t *testing.T) {
	testConfig, err := Create()
	require.NoError(t, err)

	const day = 24 * 60 * 60
	require.GreaterOrEqual(t, testConfig.GetInt("web.influxdb.retention.daily"), (14+7)*day)
	require.GreaterOrEqual(t, testConfig.GetInt("web.influxdb.retention.weekly"), (62+7)*day)
	require.GreaterOrEqual(t, testConfig.GetInt("web.influxdb.retention.monthly"), (2*366+7)*day)
}
