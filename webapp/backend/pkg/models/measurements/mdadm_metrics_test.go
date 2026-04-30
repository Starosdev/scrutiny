package measurements

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMDADMMetrics_Flatten(t *testing.T) {
	now := time.Now()
	metrics := MDADMMetrics{
		Date:           now,
		ArrayUUID:      "test-uuid",
		ArrayName:      "md0",
		ActiveDevices:  2,
		WorkingDevices: 2,
		FailedDevices:  0,
		SpareDevices:   1,
		State:          "clean",
		SyncProgress:   100.0,
	}

	tags, fields := metrics.Flatten()

	assert.Equal(t, "test-uuid", tags["array_uuid"])
	assert.Equal(t, "md0", tags["array_name"])
	assert.Equal(t, 2, fields["active_devices"])
	assert.Equal(t, 2, fields["working_devices"])
	assert.Equal(t, 0, fields["failed_devices"])
	assert.Equal(t, 1, fields["spare_devices"])
	assert.Equal(t, "clean", fields["state"])
	assert.Equal(t, 100.0, fields["sync_progress"])
}

func TestNewMDADMMetricsFromInfluxDB(t *testing.T) {
	now := time.Now()
	attrs := map[string]interface{}{
		"_time":           now,
		"array_uuid":      "test-uuid",
		"array_name":      "md0",
		"active_devices":  int64(2),
		"working_devices": int64(2),
		"failed_devices":  int64(0),
		"spare_devices":   int64(1),
		"state":           "clean",
		"sync_progress":   100.0,
	}

	metrics, err := NewMDADMMetricsFromInfluxDB(attrs)

	assert.NoError(t, err)
	assert.Equal(t, now, metrics.Date)
	assert.Equal(t, "test-uuid", metrics.ArrayUUID)
	assert.Equal(t, 2, metrics.ActiveDevices)
	assert.Equal(t, 2, metrics.WorkingDevices)
	assert.Equal(t, 0, metrics.FailedDevices)
	assert.Equal(t, 1, metrics.SpareDevices)
	assert.Equal(t, "clean", metrics.State)
	assert.Equal(t, 100.0, metrics.SyncProgress)
}
