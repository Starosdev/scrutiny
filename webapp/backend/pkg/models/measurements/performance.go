package measurements

import (
	"time"
)

// Performance represents time-series performance benchmark metrics stored in InfluxDB
type Performance struct {
	Date              time.Time `json:"date"`
	DeviceWWN         string    `json:"device_wwn"`      // tag
	DeviceID          string    `json:"device_id"`       // tag (deterministic UUIDv5)
	DeviceProtocol    string    `json:"device_protocol"` // tag
	Profile           string    `json:"profile"`         // tag
	FioVersion        string    `json:"fio_version"`
	SeqReadBwBytes    float64   `json:"seq_read_bw_bytes"`
	SeqWriteBwBytes   float64   `json:"seq_write_bw_bytes"`
	RandReadIOPS      float64   `json:"rand_read_iops"`
	RandWriteIOPS     float64   `json:"rand_write_iops"`
	RandReadLatAvgNs  float64   `json:"rand_read_lat_ns_avg"`
	RandReadLatP50Ns  float64   `json:"rand_read_lat_ns_p50"`
	RandReadLatP95Ns  float64   `json:"rand_read_lat_ns_p95"`
	RandReadLatP99Ns  float64   `json:"rand_read_lat_ns_p99"`
	RandWriteLatAvgNs float64   `json:"rand_write_lat_ns_avg"`
	RandWriteLatP50Ns float64   `json:"rand_write_lat_ns_p50"`
	RandWriteLatP95Ns float64   `json:"rand_write_lat_ns_p95"`
	RandWriteLatP99Ns float64   `json:"rand_write_lat_ns_p99"`
	MixedRwIOPS       float64   `json:"mixed_rw_iops"`
	TestDurationSec   float64   `json:"test_duration_sec"`
}

// Flatten converts the Performance struct to tags and fields for InfluxDB
func (p *Performance) Flatten() (tags map[string]string, fields map[string]interface{}) {
	tags = map[string]string{
		"device_wwn":      p.DeviceWWN,
		"device_id":       p.DeviceID,
		"device_protocol": p.DeviceProtocol,
		"profile":         p.Profile,
	}

	fields = map[string]interface{}{
		"seq_read_bw_bytes":     p.SeqReadBwBytes,
		"seq_write_bw_bytes":    p.SeqWriteBwBytes,
		"rand_read_iops":        p.RandReadIOPS,
		"rand_write_iops":       p.RandWriteIOPS,
		"rand_read_lat_ns_avg":  p.RandReadLatAvgNs,
		"rand_read_lat_ns_p50":  p.RandReadLatP50Ns,
		"rand_read_lat_ns_p95":  p.RandReadLatP95Ns,
		"rand_read_lat_ns_p99":  p.RandReadLatP99Ns,
		"rand_write_lat_ns_avg": p.RandWriteLatAvgNs,
		"rand_write_lat_ns_p50": p.RandWriteLatP50Ns,
		"rand_write_lat_ns_p95": p.RandWriteLatP95Ns,
		"rand_write_lat_ns_p99": p.RandWriteLatP99Ns,
		"mixed_rw_iops":         p.MixedRwIOPS,
		"fio_version":           p.FioVersion,
		"test_duration_sec":     p.TestDurationSec,
	}

	return tags, fields
}

// NewPerformanceFromInfluxDB creates a Performance from InfluxDB query result
func NewPerformanceFromInfluxDB(attrs map[string]interface{}) (*Performance, error) {
	return &Performance{
		Date:              attrs["_time"].(time.Time),
		DeviceWWN:         attrs["device_wwn"].(string),
		DeviceProtocol:    attrs["device_protocol"].(string),
		Profile:           influxString(attrs, "profile"),
		SeqReadBwBytes:    influxFloat64(attrs, "seq_read_bw_bytes"),
		SeqWriteBwBytes:   influxFloat64(attrs, "seq_write_bw_bytes"),
		RandReadIOPS:      influxFloat64(attrs, "rand_read_iops"),
		RandWriteIOPS:     influxFloat64(attrs, "rand_write_iops"),
		RandReadLatAvgNs:  influxFloat64(attrs, "rand_read_lat_ns_avg"),
		RandReadLatP50Ns:  influxFloat64(attrs, "rand_read_lat_ns_p50"),
		RandReadLatP95Ns:  influxFloat64(attrs, "rand_read_lat_ns_p95"),
		RandReadLatP99Ns:  influxFloat64(attrs, "rand_read_lat_ns_p99"),
		RandWriteLatAvgNs: influxFloat64(attrs, "rand_write_lat_ns_avg"),
		RandWriteLatP50Ns: influxFloat64(attrs, "rand_write_lat_ns_p50"),
		RandWriteLatP95Ns: influxFloat64(attrs, "rand_write_lat_ns_p95"),
		RandWriteLatP99Ns: influxFloat64(attrs, "rand_write_lat_ns_p99"),
		MixedRwIOPS:       influxFloat64(attrs, "mixed_rw_iops"),
		FioVersion:        influxString(attrs, "fio_version"),
		TestDurationSec:   influxFloat64(attrs, "test_duration_sec"),
	}, nil
}

// PerformanceBaseline represents averaged performance metrics used as a comparison baseline
type PerformanceBaseline struct {
	SeqReadBwBytes    float64 `json:"seq_read_bw_bytes"`
	SeqWriteBwBytes   float64 `json:"seq_write_bw_bytes"`
	RandReadIOPS      float64 `json:"rand_read_iops"`
	RandWriteIOPS     float64 `json:"rand_write_iops"`
	RandReadLatAvgNs  float64 `json:"rand_read_lat_ns_avg"`
	RandWriteLatAvgNs float64 `json:"rand_write_lat_ns_avg"`
	SampleCount       int     `json:"sample_count"` // already optimal: int at end after float64s
}

// DegradationStatus represents whether a metric has degraded relative to its baseline
type DegradationStatus struct {
	Status       string  `json:"status"`
	BaselineAvg  float64 `json:"baseline_avg"`
	Current      float64 `json:"current"`
	DeviationPct float64 `json:"deviation_pct"`
}

// DegradationReport contains per-metric degradation analysis
type DegradationReport struct {
	Metrics  map[string]DegradationStatus `json:"metrics,omitempty"`
	Detected bool                         `json:"detected"`
}
