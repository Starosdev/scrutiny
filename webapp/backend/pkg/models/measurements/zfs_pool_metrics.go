package measurements

import (
	"time"
)

// ZFSPoolMetrics represents time-series metrics for a ZFS pool stored in InfluxDB
type ZFSPoolMetrics struct {
	Date     time.Time `json:"date"`
	PoolGUID string    `json:"pool_guid"` // tag
	PoolName string    `json:"pool_name"` // tag

	// Capacity metrics (fields)
	Size            int64   `json:"size"`
	Allocated       int64   `json:"allocated"`
	Free            int64   `json:"free"`
	CapacityPercent float64 `json:"capacity_percent"`
	Fragmentation   int     `json:"fragmentation"`

	// Health status (field - stored as string)
	Status string `json:"status"`

	// Error counts (fields)
	ReadErrors     int64 `json:"read_errors"`
	WriteErrors    int64 `json:"write_errors"`
	ChecksumErrors int64 `json:"checksum_errors"`

	// Scrub metrics (fields)
	ScrubState   string  `json:"scrub_state"`
	ScrubPercent float64 `json:"scrub_percent"`
	ScrubErrors  int64   `json:"scrub_errors"`
}

// Flatten converts the ZFSPoolMetrics struct to tags and fields for InfluxDB
func (m *ZFSPoolMetrics) Flatten() (tags map[string]string, fields map[string]interface{}) {
	tags = map[string]string{
		"pool_guid": m.PoolGUID,
		"pool_name": m.PoolName,
	}

	fields = map[string]interface{}{
		"size":             m.Size,
		"allocated":        m.Allocated,
		"free":             m.Free,
		"capacity_percent": m.CapacityPercent,
		"fragmentation":    m.Fragmentation,
		"status":           m.Status,
		"read_errors":      m.ReadErrors,
		"write_errors":     m.WriteErrors,
		"checksum_errors":  m.ChecksumErrors,
		"scrub_state":      m.ScrubState,
		"scrub_percent":    m.ScrubPercent,
		"scrub_errors":     m.ScrubErrors,
	}

	return tags, fields
}

// NewZFSPoolMetricsFromInfluxDB creates a ZFSPoolMetrics from InfluxDB query result
func NewZFSPoolMetricsFromInfluxDB(attrs map[string]interface{}) (*ZFSPoolMetrics, error) {
	m := ZFSPoolMetrics{
		Date:     attrs["_time"].(time.Time),
		PoolGUID: attrs["pool_guid"].(string),
		PoolName: attrs["pool_name"].(string),
	}

	m.Size = influxInt64(attrs, "size")
	m.Allocated = influxInt64(attrs, "allocated")
	m.Free = influxInt64(attrs, "free")
	m.CapacityPercent = influxFloat64(attrs, "capacity_percent")
	m.Fragmentation = int(influxInt64(attrs, "fragmentation"))
	m.Status = influxString(attrs, "status")
	m.ReadErrors = influxInt64(attrs, "read_errors")
	m.WriteErrors = influxInt64(attrs, "write_errors")
	m.ChecksumErrors = influxInt64(attrs, "checksum_errors")
	m.ScrubState = influxString(attrs, "scrub_state")
	m.ScrubPercent = influxFloat64(attrs, "scrub_percent")
	m.ScrubErrors = influxInt64(attrs, "scrub_errors")

	return &m, nil
}

func influxInt64(attrs map[string]interface{}, key string) int64 {
	if val, ok := attrs[key]; ok && val != nil {
		return val.(int64)
	}
	return 0
}

func influxFloat64(attrs map[string]interface{}, key string) float64 {
	if val, ok := attrs[key]; ok && val != nil {
		return val.(float64)
	}
	return 0
}

func influxString(attrs map[string]interface{}, key string) string {
	if val, ok := attrs[key]; ok && val != nil {
		return val.(string)
	}
	return ""
}

// ZFSPoolCapacityHistory represents a simplified capacity data point for charts
type ZFSPoolCapacityHistory struct {
	Date            time.Time `json:"date"`
	CapacityPercent float64   `json:"capacity_percent"`
}
