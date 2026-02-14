package models

import (
	"encoding/json"
	"fmt"
)

// PerformanceResult is the JSON payload sent from collector to backend
type PerformanceResult struct {
	Profile           string  `json:"profile"`
	DeviceProtocol    string  `json:"device_protocol"`
	FioVersion        string  `json:"fio_version"`
	Date              int64   `json:"date"`
	SeqReadBwBytes    float64 `json:"seq_read_bw_bytes"`
	SeqWriteBwBytes   float64 `json:"seq_write_bw_bytes"`
	RandReadIOPS      float64 `json:"rand_read_iops"`
	RandWriteIOPS     float64 `json:"rand_write_iops"`
	RandReadLatAvgNs  float64 `json:"rand_read_lat_ns_avg"`
	RandReadLatP50Ns  float64 `json:"rand_read_lat_ns_p50"`
	RandReadLatP95Ns  float64 `json:"rand_read_lat_ns_p95"`
	RandReadLatP99Ns  float64 `json:"rand_read_lat_ns_p99"`
	RandWriteLatAvgNs float64 `json:"rand_write_lat_ns_avg"`
	RandWriteLatP50Ns float64 `json:"rand_write_lat_ns_p50"`
	RandWriteLatP95Ns float64 `json:"rand_write_lat_ns_p95"`
	RandWriteLatP99Ns float64 `json:"rand_write_lat_ns_p99"`
	MixedRwIOPS       float64 `json:"mixed_rw_iops"`
	TestDurationSec   float64 `json:"test_duration_sec"`
}

// FioOutput represents the top-level fio JSON output
type FioOutput struct {
	FioVersion string   `json:"fio version"`
	Jobs       []FioJob `json:"jobs"`
}

// FioJob represents a single fio job result
type FioJob struct {
	JobName    string     `json:"jobname"`
	Read       FioIOStats `json:"read"`
	Write      FioIOStats `json:"write"`
	JobRuntime int64      `json:"job_runtime"` // milliseconds
}

// FioIOStats represents read or write statistics from a fio job
type FioIOStats struct {
	ClatNs  FioClatency `json:"clat_ns"`
	LatNs   FioLatency  `json:"lat_ns"`
	BwBytes float64     `json:"bw_bytes"`
	IOPS    float64     `json:"iops"`
}

// FioLatency represents latency statistics
type FioLatency struct {
	Mean   float64 `json:"mean"`
	StdDev float64 `json:"stddev"`
}

// FioClatency represents completion latency with percentiles
type FioClatency struct {
	Percentile map[string]float64 `json:"percentile"`
	Mean       float64            `json:"mean"`
	StdDev     float64            `json:"stddev"`
}

// ParseFioOutput parses fio JSON output and returns the first job's stats
func ParseFioOutput(jsonBytes []byte) (*FioOutput, error) {
	var output FioOutput
	if err := json.Unmarshal(jsonBytes, &output); err != nil {
		return nil, fmt.Errorf("failed to parse fio output: %w", err)
	}

	if len(output.Jobs) == 0 {
		return nil, fmt.Errorf("fio output contains no job results")
	}

	return &output, nil
}

// ExtractReadStats extracts read throughput/IOPS/latency from a fio job
func ExtractReadStats(job *FioJob) (bwBytes, iops, latAvgNs, latP50Ns, latP95Ns, latP99Ns float64) {
	bwBytes = job.Read.BwBytes
	iops = job.Read.IOPS
	latAvgNs = job.Read.LatNs.Mean

	if job.Read.ClatNs.Percentile != nil {
		latP50Ns = job.Read.ClatNs.Percentile["50.000000"]
		latP95Ns = job.Read.ClatNs.Percentile["95.000000"]
		latP99Ns = job.Read.ClatNs.Percentile["99.000000"]
	}

	return
}

// ExtractWriteStats extracts write throughput/IOPS/latency from a fio job
func ExtractWriteStats(job *FioJob) (bwBytes, iops, latAvgNs, latP50Ns, latP95Ns, latP99Ns float64) {
	bwBytes = job.Write.BwBytes
	iops = job.Write.IOPS
	latAvgNs = job.Write.LatNs.Mean

	if job.Write.ClatNs.Percentile != nil {
		latP50Ns = job.Write.ClatNs.Percentile["50.000000"]
		latP95Ns = job.Write.ClatNs.Percentile["95.000000"]
		latP99Ns = job.Write.ClatNs.Percentile["99.000000"]
	}

	return
}
