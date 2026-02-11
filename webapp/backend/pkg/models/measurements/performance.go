package measurements

import (
	"time"
)

// Performance represents time-series performance benchmark metrics stored in InfluxDB
type Performance struct {
	Date           time.Time `json:"date"`
	DeviceWWN      string    `json:"device_wwn"`      // tag
	DeviceProtocol string    `json:"device_protocol"` // tag
	Profile        string    `json:"profile"`         // tag: "quick" or "comprehensive"

	// Sequential throughput (bytes/sec)
	SeqReadBwBytes  float64 `json:"seq_read_bw_bytes"`
	SeqWriteBwBytes float64 `json:"seq_write_bw_bytes"`

	// Random IOPS
	RandReadIOPS  float64 `json:"rand_read_iops"`
	RandWriteIOPS float64 `json:"rand_write_iops"`

	// Random read latency (nanoseconds)
	RandReadLatAvgNs float64 `json:"rand_read_lat_ns_avg"`
	RandReadLatP50Ns float64 `json:"rand_read_lat_ns_p50"`
	RandReadLatP95Ns float64 `json:"rand_read_lat_ns_p95"`
	RandReadLatP99Ns float64 `json:"rand_read_lat_ns_p99"`

	// Random write latency (nanoseconds)
	RandWriteLatAvgNs float64 `json:"rand_write_lat_ns_avg"`
	RandWriteLatP50Ns float64 `json:"rand_write_lat_ns_p50"`
	RandWriteLatP95Ns float64 `json:"rand_write_lat_ns_p95"`
	RandWriteLatP99Ns float64 `json:"rand_write_lat_ns_p99"`

	// Mixed random read/write IOPS (comprehensive profile only)
	MixedRwIOPS float64 `json:"mixed_rw_iops"`

	// Metadata
	FioVersion      string  `json:"fio_version"`
	TestDurationSec float64 `json:"test_duration_sec"`
}

// Flatten converts the Performance struct to tags and fields for InfluxDB
func (p *Performance) Flatten() (tags map[string]string, fields map[string]interface{}) {
	tags = map[string]string{
		"device_wwn":      p.DeviceWWN,
		"device_protocol": p.DeviceProtocol,
		"profile":         p.Profile,
	}

	fields = map[string]interface{}{
		"seq_read_bw_bytes":      p.SeqReadBwBytes,
		"seq_write_bw_bytes":     p.SeqWriteBwBytes,
		"rand_read_iops":         p.RandReadIOPS,
		"rand_write_iops":        p.RandWriteIOPS,
		"rand_read_lat_ns_avg":   p.RandReadLatAvgNs,
		"rand_read_lat_ns_p50":   p.RandReadLatP50Ns,
		"rand_read_lat_ns_p95":   p.RandReadLatP95Ns,
		"rand_read_lat_ns_p99":   p.RandReadLatP99Ns,
		"rand_write_lat_ns_avg":  p.RandWriteLatAvgNs,
		"rand_write_lat_ns_p50":  p.RandWriteLatP50Ns,
		"rand_write_lat_ns_p95":  p.RandWriteLatP95Ns,
		"rand_write_lat_ns_p99":  p.RandWriteLatP99Ns,
		"mixed_rw_iops":          p.MixedRwIOPS,
		"fio_version":            p.FioVersion,
		"test_duration_sec":      p.TestDurationSec,
	}

	return tags, fields
}

// NewPerformanceFromInfluxDB creates a Performance from InfluxDB query result
func NewPerformanceFromInfluxDB(attrs map[string]interface{}) (*Performance, error) {
	p := Performance{
		Date:           attrs["_time"].(time.Time),
		DeviceWWN:      attrs["device_wwn"].(string),
		DeviceProtocol: attrs["device_protocol"].(string),
	}

	if val, ok := attrs["profile"]; ok && val != nil {
		p.Profile = val.(string)
	}
	if val, ok := attrs["seq_read_bw_bytes"]; ok && val != nil {
		p.SeqReadBwBytes = val.(float64)
	}
	if val, ok := attrs["seq_write_bw_bytes"]; ok && val != nil {
		p.SeqWriteBwBytes = val.(float64)
	}
	if val, ok := attrs["rand_read_iops"]; ok && val != nil {
		p.RandReadIOPS = val.(float64)
	}
	if val, ok := attrs["rand_write_iops"]; ok && val != nil {
		p.RandWriteIOPS = val.(float64)
	}
	if val, ok := attrs["rand_read_lat_ns_avg"]; ok && val != nil {
		p.RandReadLatAvgNs = val.(float64)
	}
	if val, ok := attrs["rand_read_lat_ns_p50"]; ok && val != nil {
		p.RandReadLatP50Ns = val.(float64)
	}
	if val, ok := attrs["rand_read_lat_ns_p95"]; ok && val != nil {
		p.RandReadLatP95Ns = val.(float64)
	}
	if val, ok := attrs["rand_read_lat_ns_p99"]; ok && val != nil {
		p.RandReadLatP99Ns = val.(float64)
	}
	if val, ok := attrs["rand_write_lat_ns_avg"]; ok && val != nil {
		p.RandWriteLatAvgNs = val.(float64)
	}
	if val, ok := attrs["rand_write_lat_ns_p50"]; ok && val != nil {
		p.RandWriteLatP50Ns = val.(float64)
	}
	if val, ok := attrs["rand_write_lat_ns_p95"]; ok && val != nil {
		p.RandWriteLatP95Ns = val.(float64)
	}
	if val, ok := attrs["rand_write_lat_ns_p99"]; ok && val != nil {
		p.RandWriteLatP99Ns = val.(float64)
	}
	if val, ok := attrs["mixed_rw_iops"]; ok && val != nil {
		p.MixedRwIOPS = val.(float64)
	}
	if val, ok := attrs["fio_version"]; ok && val != nil {
		p.FioVersion = val.(string)
	}
	if val, ok := attrs["test_duration_sec"]; ok && val != nil {
		p.TestDurationSec = val.(float64)
	}

	return &p, nil
}

// PerformanceBaseline represents averaged performance metrics used as a comparison baseline
type PerformanceBaseline struct {
	SeqReadBwBytes    float64 `json:"seq_read_bw_bytes"`
	SeqWriteBwBytes   float64 `json:"seq_write_bw_bytes"`
	RandReadIOPS      float64 `json:"rand_read_iops"`
	RandWriteIOPS     float64 `json:"rand_write_iops"`
	RandReadLatAvgNs  float64 `json:"rand_read_lat_ns_avg"`
	RandWriteLatAvgNs float64 `json:"rand_write_lat_ns_avg"`
	SampleCount       int     `json:"sample_count"`
}

// DegradationStatus represents whether a metric has degraded relative to its baseline
type DegradationStatus struct {
	BaselineAvg  float64 `json:"baseline_avg"`
	Current      float64 `json:"current"`
	DeviationPct float64 `json:"deviation_pct"`
	Status       string  `json:"status"` // "passed", "warning", "failed"
}

// DegradationReport contains per-metric degradation analysis
type DegradationReport struct {
	Detected bool                         `json:"detected"`
	Metrics  map[string]DegradationStatus `json:"metrics,omitempty"`
}
