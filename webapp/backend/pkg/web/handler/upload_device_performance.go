package handler

import (
	"math"
	"net/http"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	"github.com/analogj/scrutiny/webapp/backend/pkg/validation"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// performanceRequest is the JSON payload sent by the collector
type performanceRequest struct {
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

// UploadDevicePerformance receives performance benchmark results from the collector
func UploadDevicePerformance(c *gin.Context) {
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)
	logger := c.MustGet("LOGGER").(*logrus.Entry)

	wwn := c.Param("wwn")
	if err := validation.ValidateWWN(wwn); err != nil {
		logger.Warnf("Invalid WWN format: %s", wwn)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	var req performanceRequest
	if err := c.BindJSON(&req); err != nil {
		logger.Errorln("Cannot parse performance data", err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid request body"})
		return
	}

	perfData := measurements.Performance{
		Date:              time.Unix(req.Date, 0),
		DeviceWWN:         wwn,
		DeviceProtocol:    req.DeviceProtocol,
		Profile:           req.Profile,
		SeqReadBwBytes:    req.SeqReadBwBytes,
		SeqWriteBwBytes:   req.SeqWriteBwBytes,
		RandReadIOPS:      req.RandReadIOPS,
		RandWriteIOPS:     req.RandWriteIOPS,
		RandReadLatAvgNs:  req.RandReadLatAvgNs,
		RandReadLatP50Ns:  req.RandReadLatP50Ns,
		RandReadLatP95Ns:  req.RandReadLatP95Ns,
		RandReadLatP99Ns:  req.RandReadLatP99Ns,
		RandWriteLatAvgNs: req.RandWriteLatAvgNs,
		RandWriteLatP50Ns: req.RandWriteLatP50Ns,
		RandWriteLatP95Ns: req.RandWriteLatP95Ns,
		RandWriteLatP99Ns: req.RandWriteLatP99Ns,
		MixedRwIOPS:       req.MixedRwIOPS,
		FioVersion:        req.FioVersion,
		TestDurationSec:   req.TestDurationSec,
	}

	if err := deviceRepo.SavePerformanceResults(c, wwn, &perfData); err != nil {
		logger.Errorln("An error occurred while saving performance results", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	// Calculate degradation against baseline
	baseline, err := deviceRepo.GetPerformanceBaseline(c, wwn, 5)
	if err != nil {
		logger.Warnf("Could not retrieve performance baseline for %s: %v", wwn, err)
		c.JSON(http.StatusOK, gin.H{"success": true})
		return
	}

	if baseline == nil || baseline.SampleCount < 2 {
		c.JSON(http.StatusOK, gin.H{"success": true})
		return
	}

	report := calculateDegradation(&perfData, baseline)

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"degradation": report,
	})
}

// calculateDegradation compares current results against a baseline and reports degradation
func calculateDegradation(current *measurements.Performance, baseline *measurements.PerformanceBaseline) measurements.DegradationReport {
	report := measurements.DegradationReport{
		Metrics: make(map[string]measurements.DegradationStatus),
	}

	// For throughput and IOPS: lower is worse (negative deviation = degradation)
	checkThroughput := func(name string, currentVal, baselineVal float64) {
		if baselineVal == 0 {
			return
		}
		devPct := ((currentVal - baselineVal) / baselineVal) * 100
		status := "passed"
		if devPct < -40 {
			status = "failed"
			report.Detected = true
		} else if devPct < -20 {
			status = "warning"
			report.Detected = true
		}
		report.Metrics[name] = measurements.DegradationStatus{
			BaselineAvg:  math.Round(baselineVal*100) / 100,
			Current:      currentVal,
			DeviationPct: math.Round(devPct*100) / 100,
			Status:       status,
		}
	}

	// For latency: higher is worse (positive deviation = degradation)
	checkLatency := func(name string, currentVal, baselineVal float64) {
		if baselineVal == 0 {
			return
		}
		devPct := ((currentVal - baselineVal) / baselineVal) * 100
		status := "passed"
		if devPct > 60 {
			status = "failed"
			report.Detected = true
		} else if devPct > 30 {
			status = "warning"
			report.Detected = true
		}
		report.Metrics[name] = measurements.DegradationStatus{
			BaselineAvg:  math.Round(baselineVal*100) / 100,
			Current:      currentVal,
			DeviationPct: math.Round(devPct*100) / 100,
			Status:       status,
		}
	}

	checkThroughput("seq_read_bw_bytes", current.SeqReadBwBytes, baseline.SeqReadBwBytes)
	checkThroughput("seq_write_bw_bytes", current.SeqWriteBwBytes, baseline.SeqWriteBwBytes)
	checkThroughput("rand_read_iops", current.RandReadIOPS, baseline.RandReadIOPS)
	checkThroughput("rand_write_iops", current.RandWriteIOPS, baseline.RandWriteIOPS)
	checkLatency("rand_read_lat_ns_avg", current.RandReadLatAvgNs, baseline.RandReadLatAvgNs)
	checkLatency("rand_write_lat_ns_avg", current.RandWriteLatAvgNs, baseline.RandWriteLatAvgNs)

	return report
}
