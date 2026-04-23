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
	ArraySize   int64 `json:"array_size"`
	UsedDevSize int64 `json:"used_dev_size"`
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
		"used_dev_size":   m.UsedDevSize,
	}

	return tags, fields
}

// NewMDADMMetricsFromInfluxDB creates an MDADMMetrics from InfluxDB query result
func NewMDADMMetricsFromInfluxDB(attrs map[string]interface{}) (*MDADMMetrics, error) {
	m := MDADMMetrics{
		Date:      attrs["_time"].(time.Time),
		ArrayUUID: attrs["array_uuid"].(string),
		ArrayName: attrs["array_name"].(string),
	}

	// Parse optional fields
	if val, ok := attrs["active_devices"]; ok && val != nil {
		m.ActiveDevices = int(val.(int64))
	}
	if val, ok := attrs["working_devices"]; ok && val != nil {
		m.WorkingDevices = int(val.(int64))
	}
	if val, ok := attrs["failed_devices"]; ok && val != nil {
		m.FailedDevices = int(val.(int64))
	}
	if val, ok := attrs["spare_devices"]; ok && val != nil {
		m.SpareDevices = int(val.(int64))
	}
	if val, ok := attrs["state"]; ok && val != nil {
		m.State = val.(string)
	}
	if val, ok := attrs["sync_progress"]; ok && val != nil {
		m.SyncProgress = val.(float64)
	}
	if val, ok := attrs["raw_mdstat"]; ok && val != nil {
		m.RawMdstat = val.(string)
	}
	if val, ok := attrs["array_size"]; ok && val != nil {
		m.ArraySize = val.(int64)
	}
	if val, ok := attrs["used_dev_size"]; ok && val != nil {
		m.UsedDevSize = val.(int64)
	}

	return &m, nil
}
