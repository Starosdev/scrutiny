package measurements

import (
	"time"
)

// MDADMMetrics represents time-series metrics for an MDADM array stored in InfluxDB
type MDADMMetrics struct {
	Date      time.Time `json:"date"`
	ArrayUUID string    `json:"array_uuid"` // tag
	ArrayName string    `json:"array_name"` // tag

	// Device counts (fields)
	ActiveDevices  int `json:"active_devices"`
	WorkingDevices int `json:"working_devices"`
	FailedDevices  int `json:"failed_devices"`
	SpareDevices   int `json:"spare_devices"`

	// Status (fields)
	State        string  `json:"state"`
	SyncProgress float64 `json:"sync_progress"`
	RawMdstat    string  `json:"raw_mdstat"`

	// Storage sizes in bytes (fields)
	ArraySize int64 `json:"array_size"`
	// UsedBytes is the filesystem-level used space from statfs on the mount point
	UsedBytes int64 `json:"used_bytes"`
}

// Flatten converts the MDADMMetrics struct to tags and fields for InfluxDB
func (m *MDADMMetrics) Flatten() (tags map[string]string, fields map[string]interface{}) {
	tags = map[string]string{
		"array_uuid": m.ArrayUUID,
		"array_name": m.ArrayName,
	}

	fields = map[string]interface{}{
		"active_devices":  m.ActiveDevices,
		"working_devices": m.WorkingDevices,
		"failed_devices":  m.FailedDevices,
		"spare_devices":   m.SpareDevices,
		"state":           m.State,
		"sync_progress":   m.SyncProgress,
		"raw_mdstat":      m.RawMdstat,
		"array_size":      m.ArraySize,
		"used_bytes":      m.UsedBytes,
	}

	return tags, fields
}

// NewMDADMMetricsFromInfluxDB creates an MDADMMetrics from InfluxDB query result
func NewMDADMMetricsFromInfluxDB(attrs map[string]interface{}) (*MDADMMetrics, error) {
	return &MDADMMetrics{
		Date:           attrs["_time"].(time.Time),
		ArrayUUID:      attrs["array_uuid"].(string),
		ArrayName:      attrs["array_name"].(string),
		ActiveDevices:  int(influxInt64(attrs, "active_devices")),
		WorkingDevices: int(influxInt64(attrs, "working_devices")),
		FailedDevices:  int(influxInt64(attrs, "failed_devices")),
		SpareDevices:   int(influxInt64(attrs, "spare_devices")),
		State:          influxString(attrs, "state"),
		SyncProgress:   influxFloat64(attrs, "sync_progress"),
		RawMdstat:      influxString(attrs, "raw_mdstat"),
		ArraySize:      influxInt64(attrs, "array_size"),
		UsedBytes:      influxInt64(attrs, "used_bytes"),
	}, nil
}
