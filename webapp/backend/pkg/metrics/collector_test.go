package metrics

import (
	"strings"
	"testing"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	metricsModels "github.com/analogj/scrutiny/webapp/backend/pkg/models/metrics"
	dto "github.com/prometheus/client_model/go"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectorIncludesZFSAndWorkloadMetrics(t *testing.T) {
	collector := NewCollector(logrus.New().WithField("test", "collector"))
	collector.devices["dev-1"] = &metricsModels.DeviceMetricsData{
		Device: models.Device{
			DeviceID:       "dev-1",
			WWN:            "wwn-1",
			DeviceName:     "nvme0n1",
			ModelName:      "FastDrive",
			DeviceProtocol: "nvme",
			HostId:         "host-a",
			Capacity:       1000,
		},
		SmartData: measurements.Smart{
			DeviceID:       "dev-1",
			DeviceWWN:      "wwn-1",
			DeviceProtocol: "nvme",
			Temp:           42,
		},
	}
	collector.zfsPools["pool-guid"] = &metricsModels.ZFSPoolMetricsData{
		Pool: models.ZFSPool{
			GUID:                 "pool-guid",
			Name:                 "tank",
			HostID:               "host-a",
			Status:               models.ZFSPoolStatusDegraded,
			Size:                 1000,
			Allocated:            600,
			Free:                 400,
			Fragmentation:        8,
			CapacityPercent:      60.5,
			ScrubState:           models.ZFSScrubStateScanning,
			ScrubScannedBytes:    120,
			ScrubIssuedBytes:     110,
			ScrubTotalBytes:      500,
			ScrubErrorsCount:     2,
			ScrubPercentComplete: 24.5,
			TotalReadErrors:      3,
			TotalWriteErrors:     4,
			TotalChecksumErrors:  5,
		},
	}
	collector.workloads["dev-1"] = &metricsModels.WorkloadMetricsData{
		Insight: models.WorkloadInsight{
			DeviceID:        "dev-1",
			DeviceWWN:       "wwn-1",
			DeviceName:      "nvme0n1",
			ModelName:       "FastDrive",
			DeviceProtocol:  "nvme",
			HostId:          "host-a",
			Intensity:       "medium",
			DailyReadBytes:  100,
			DailyWriteBytes: 200,
			TotalReadBytes:  1000,
			TotalWriteBytes: 2000,
			ReadWriteRatio:  0.5,
			TimeSpanHours:   48,
			DataPoints:      2,
			Endurance: &models.EnduranceEstimate{
				Available:             true,
				PercentageUsed:        12,
				EstimatedLifespanDays: 345,
				TBWrittenSoFar:        7.5,
			},
			Spike: &models.ActivitySpike{
				Detected:                true,
				SpikeFactor:             3.4,
				RecentDailyWriteBytes:   444,
				BaselineDailyWriteBytes: 111,
			},
		},
	}

	families := gatherMetricFamilies(t, collector)

	assertMetricValue(t, families, "scrutiny_zfs_pool_capacity_percent", 60.5, map[string]string{
		"guid":      "pool-guid",
		"pool_name": "tank",
		"host_id":   "host-a",
	})
	assertMetricValue(t, families, "scrutiny_zfs_pool_status", 1, map[string]string{
		"guid":      "pool-guid",
		"pool_name": "tank",
		"host_id":   "host-a",
		"status":    "DEGRADED",
	})
	assertMetricValue(t, families, "scrutiny_zfs_pool_status", 0, map[string]string{
		"guid":      "pool-guid",
		"pool_name": "tank",
		"host_id":   "host-a",
		"status":    "ONLINE",
	})
	assertMetricValue(t, families, "scrutiny_zfs_pool_status_code", 2, map[string]string{
		"guid":      "pool-guid",
		"pool_name": "tank",
		"host_id":   "host-a",
	})
	assertMetricValue(t, families, "scrutiny_zfs_pool_scrub_state", 1, map[string]string{
		"guid":        "pool-guid",
		"pool_name":   "tank",
		"host_id":     "host-a",
		"scrub_state": "scanning",
	})
	assertMetricValue(t, families, "scrutiny_zfs_pool_scrub_state_code", 2, map[string]string{
		"guid":      "pool-guid",
		"pool_name": "tank",
		"host_id":   "host-a",
	})
	assertMetricValue(t, families, "scrutiny_zfs_pool_scrub_state", 0, map[string]string{
		"guid":        "pool-guid",
		"pool_name":   "tank",
		"host_id":     "host-a",
		"scrub_state": "unknown",
	})
	assertMetricValue(t, families, "scrutiny_workload_daily_write_bytes", 200, map[string]string{
		"device_id":   "dev-1",
		"wwn":         "wwn-1",
		"device_name": "nvme0n1",
		"model_name":  "FastDrive",
		"protocol":    "nvme",
		"host_id":     "host-a",
	})
	assertMetricValue(t, families, "scrutiny_workload_intensity", 1, map[string]string{
		"device_id":   "dev-1",
		"wwn":         "wwn-1",
		"device_name": "nvme0n1",
		"model_name":  "FastDrive",
		"protocol":    "nvme",
		"host_id":     "host-a",
		"intensity":   "medium",
	})
	assertMetricValue(t, families, "scrutiny_workload_intensity", 0, map[string]string{
		"device_id":   "dev-1",
		"wwn":         "wwn-1",
		"device_name": "nvme0n1",
		"model_name":  "FastDrive",
		"protocol":    "nvme",
		"host_id":     "host-a",
		"intensity":   "idle",
	})
	assertMetricValue(t, families, "scrutiny_workload_intensity_code", 3, map[string]string{
		"device_id":   "dev-1",
		"wwn":         "wwn-1",
		"device_name": "nvme0n1",
		"model_name":  "FastDrive",
		"protocol":    "nvme",
		"host_id":     "host-a",
	})
	assertMetricValue(t, families, "scrutiny_workload_endurance_percentage_used", 12, map[string]string{
		"device_id":   "dev-1",
		"wwn":         "wwn-1",
		"device_name": "nvme0n1",
		"model_name":  "FastDrive",
		"protocol":    "nvme",
		"host_id":     "host-a",
	})
	assertMetricValue(t, families, "scrutiny_workload_spike_factor", 3.4, map[string]string{
		"device_id":   "dev-1",
		"wwn":         "wwn-1",
		"device_name": "nvme0n1",
		"model_name":  "FastDrive",
		"protocol":    "nvme",
		"host_id":     "host-a",
	})
}

func TestCollectorOmitsOptionalWorkloadMetricsAndHandlesUnknownStates(t *testing.T) {
	collector := NewCollector(logrus.New().WithField("test", "collector"))
	collector.zfsPools["pool-guid"] = &metricsModels.ZFSPoolMetricsData{
		Pool: models.ZFSPool{
			GUID:       "pool-guid",
			Name:       "tank",
			HostID:     "host-a",
			Status:     "",
			ScrubState: "",
		},
	}
	collector.workloads["dev-2"] = &metricsModels.WorkloadMetricsData{
		Insight: models.WorkloadInsight{
			DeviceID:       "dev-2",
			DeviceWWN:      "wwn-2",
			DeviceProtocol: "scsi",
			Intensity:      "unknown",
		},
	}

	families := gatherMetricFamilies(t, collector)

	assertMetricValue(t, families, "scrutiny_zfs_pool_status_code", 0, map[string]string{
		"guid":      "pool-guid",
		"pool_name": "tank",
		"host_id":   "host-a",
	})
	assertMetricValue(t, families, "scrutiny_zfs_pool_status", 1, map[string]string{
		"guid":      "pool-guid",
		"pool_name": "tank",
		"host_id":   "host-a",
		"status":    "unknown",
	})
	assertMetricValue(t, families, "scrutiny_zfs_pool_scrub_state_code", 0, map[string]string{
		"guid":      "pool-guid",
		"pool_name": "tank",
		"host_id":   "host-a",
	})
	assertMetricValue(t, families, "scrutiny_zfs_pool_scrub_state", 1, map[string]string{
		"guid":        "pool-guid",
		"pool_name":   "tank",
		"host_id":     "host-a",
		"scrub_state": "unknown",
	})
	assertMetricValue(t, families, "scrutiny_workload_intensity_code", 0, map[string]string{
		"device_id":   "dev-2",
		"wwn":         "wwn-2",
		"device_name": "",
		"model_name":  "",
		"protocol":    "scsi",
		"host_id":     "",
	})
	assertMetricValue(t, families, "scrutiny_workload_intensity", 1, map[string]string{
		"device_id":   "dev-2",
		"wwn":         "wwn-2",
		"device_name": "",
		"model_name":  "",
		"protocol":    "scsi",
		"host_id":     "",
		"intensity":   "unknown",
	})

	assert.Nil(t, findMetric(families["scrutiny_workload_endurance_percentage_used"], map[string]string{
		"device_id":   "dev-2",
		"wwn":         "wwn-2",
		"device_name": "",
		"model_name":  "",
		"protocol":    "scsi",
		"host_id":     "",
	}))
	assert.Nil(t, findMetric(families["scrutiny_workload_spike_factor"], map[string]string{
		"device_id":   "dev-2",
		"wwn":         "wwn-2",
		"device_name": "",
		"model_name":  "",
		"protocol":    "scsi",
		"host_id":     "",
	}))
}

func gatherMetricFamilies(t *testing.T, collector *Collector) map[string]*dto.MetricFamily {
	t.Helper()

	families, err := collector.GetRegistry().Gather()
	require.NoError(t, err)

	indexed := make(map[string]*dto.MetricFamily, len(families))
	for _, family := range families {
		indexed[family.GetName()] = family
	}
	return indexed
}

func assertMetricValue(t *testing.T, families map[string]*dto.MetricFamily, metricName string, expected float64, labels map[string]string) {
	t.Helper()

	family, ok := families[metricName]
	require.Truef(t, ok, "metric family %s not found", metricName)

	metric := findMetric(family, labels)
	require.NotNilf(t, metric, "metric %s with labels %v not found", metricName, labels)
	assert.InDelta(t, expected, metric.GetGauge().GetValue(), 0.0001)
}

func findMetric(family *dto.MetricFamily, labels map[string]string) *dto.Metric {
	if family == nil {
		return nil
	}
	for _, metric := range family.Metric {
		if labelsMatch(metric, labels) {
			return metric
		}
	}
	return nil
}

func labelsMatch(metric *dto.Metric, expected map[string]string) bool {
	if len(expected) == 0 {
		return true
	}
	for key, value := range expected {
		found := false
		for _, label := range metric.Label {
			if label.GetName() == key && label.GetValue() == value {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func TestMetricValue(t *testing.T) {
	assert.Equal(t, 1.0, metricValue("idle", "idle"))
	assert.Equal(t, 0.0, metricValue("idle", "heavy"))
	assert.Equal(t, 1.0, metricValue(models.ZFSPoolStatusOnline, models.ZFSPoolStatusOnline))
}

func TestOrderedKeys(t *testing.T) {
	keys := orderedKeys(map[string]int{"b": 2, "a": 1, "c": 3})
	assert.Equal(t, "a,b,c", strings.Join(keys, ","))
}
