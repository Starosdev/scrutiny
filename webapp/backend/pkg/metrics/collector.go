package metrics

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	metricsModels "github.com/analogj/scrutiny/webapp/backend/pkg/models/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/sirupsen/logrus"
)

var zfsPoolStatusCodes = map[models.ZFSPoolStatus]float64{
	models.ZFSPoolStatusOnline:   1,
	models.ZFSPoolStatusDegraded: 2,
	models.ZFSPoolStatusFaulted:  3,
	models.ZFSPoolStatusOffline:  4,
	models.ZFSPoolStatusRemoved:  5,
	models.ZFSPoolStatusUnavail:  6,
}

var zfsScrubStateCodes = map[models.ZFSScrubState]float64{
	models.ZFSScrubStateNone:     1,
	models.ZFSScrubStateScanning: 2,
	models.ZFSScrubStateFinished: 3,
	models.ZFSScrubStateCanceled: 4,
}

var workloadIntensityCodes = map[string]float64{
	"unknown": 0,
	"idle":    1,
	"light":   2,
	"medium":  3,
	"heavy":   4,
}

const workloadMetricsDurationKey = "week"

var workloadIntensityOrder = []string{"unknown", "idle", "light", "medium", "heavy"}

// Collector manages Prometheus metrics for all devices and pools.
type Collector struct {
	mu        sync.RWMutex
	devices   map[string]*metricsModels.DeviceMetricsData
	zfsPools  map[string]*metricsModels.ZFSPoolMetricsData
	workloads map[string]*metricsModels.WorkloadMetricsData
	registry  *prometheus.Registry
	logger    *logrus.Entry
}

// NewCollector creates a new metrics collector.
func NewCollector(logger *logrus.Entry) *Collector {
	mc := &Collector{
		devices:   make(map[string]*metricsModels.DeviceMetricsData),
		zfsPools:  make(map[string]*metricsModels.ZFSPoolMetricsData),
		workloads: make(map[string]*metricsModels.WorkloadMetricsData),
		registry:  prometheus.NewRegistry(),
		logger:    logger,
	}

	mc.registry.MustRegister(collectors.NewGoCollector())
	mc.registry.MustRegister(mc)
	return mc
}

// UpdateDeviceMetrics updates device metrics after a SMART upload.
func (mc *Collector) UpdateDeviceMetrics(device *models.Device, smartData *measurements.Smart) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.devices[device.DeviceID] = &metricsModels.DeviceMetricsData{
		Device:    *device,
		SmartData: *smartData,
		UpdatedAt: time.Now(),
	}
	mc.logger.Debugf("Updated metrics for device %s", device.DeviceID)
}

// RefreshWorkloadMetrics refreshes workload metrics from the repository.
func (mc *Collector) RefreshWorkloadMetrics(deviceRepo database.DeviceRepo, ctx context.Context) error {
	// Use the same named duration key as the workload summary endpoint so exported
	// Prometheus workload metrics follow the existing backend "week" semantics.
	workloads, err := deviceRepo.GetWorkloadInsights(ctx, workloadMetricsDurationKey)
	if err != nil {
		return fmt.Errorf("failed to load workload insights: %w", err)
	}

	now := time.Now()
	next := make(map[string]*metricsModels.WorkloadMetricsData, len(workloads))
	for deviceID, insight := range workloads {
		if insight == nil {
			continue
		}
		next[deviceID] = &metricsModels.WorkloadMetricsData{
			Insight:   *insight,
			UpdatedAt: now,
		}
	}

	mc.mu.Lock()
	mc.workloads = next
	mc.mu.Unlock()

	mc.logger.Debugf("Refreshed workload metrics for %d devices", len(next))
	return nil
}

// RefreshZFSPoolMetrics refreshes ZFS pool metrics from the repository.
func (mc *Collector) RefreshZFSPoolMetrics(deviceRepo database.DeviceRepo, ctx context.Context) error {
	pools, err := deviceRepo.GetZFSPoolsSummary(ctx)
	if err != nil {
		return fmt.Errorf("failed to load zfs pool summary: %w", err)
	}

	now := time.Now()
	next := make(map[string]*metricsModels.ZFSPoolMetricsData, len(pools))
	for guid, pool := range pools {
		if pool == nil {
			continue
		}
		next[guid] = &metricsModels.ZFSPoolMetricsData{
			Pool:      *pool,
			UpdatedAt: now,
		}
	}

	mc.mu.Lock()
	mc.zfsPools = next
	mc.mu.Unlock()

	mc.logger.Debugf("Refreshed ZFS metrics for %d pools", len(next))
	return nil
}

// LoadInitialData loads device, workload, and ZFS data at startup.
func (mc *Collector) LoadInitialData(deviceRepo database.DeviceRepo, ctx context.Context) error {
	start := time.Now()
	mc.logger.Info("Loading initial metrics data from database...")

	summary, err := deviceRepo.GetSummary(ctx)
	if err != nil {
		return fmt.Errorf("failed to load device summary: %w", err)
	}

	smartDataMap := make(map[string][]measurements.Smart)
	var wg sync.WaitGroup
	var mapMu sync.Mutex

	for _, deviceSummary := range summary {
		wwn := deviceSummary.Device.WWN
		wg.Add(1)
		go func(w string) {
			defer wg.Done()
			smarts, historyErr := deviceRepo.GetSmartAttributeHistory(ctx, w, "forever", 1, 0, nil)
			if historyErr == nil && len(smarts) > 0 {
				mapMu.Lock()
				smartDataMap[w] = smarts
				mapMu.Unlock()
			}
		}(wwn)
	}

	wg.Wait()

	nextDevices := make(map[string]*metricsModels.DeviceMetricsData)
	for _, deviceSummary := range summary {
		device := deviceSummary.Device
		if smartResults, ok := smartDataMap[device.WWN]; ok && len(smartResults) > 0 {
			nextDevices[device.DeviceID] = &metricsModels.DeviceMetricsData{
				Device:    device,
				SmartData: smartResults[0],
				UpdatedAt: time.Now(),
			}
		}
	}

	mc.mu.Lock()
	mc.devices = nextDevices
	mc.mu.Unlock()

	if err := mc.RefreshWorkloadMetrics(deviceRepo, ctx); err != nil {
		return err
	}
	if err := mc.RefreshZFSPoolMetrics(deviceRepo, ctx); err != nil {
		return err
	}

	mc.logger.Infof(
		"Loaded metrics for %d devices, %d workloads, and %d ZFS pools in %v",
		len(mc.devices), len(mc.workloads), len(mc.zfsPools), time.Since(start),
	)
	return nil
}

// GetRegistry returns the Prometheus registry.
func (mc *Collector) GetRegistry() *prometheus.Registry {
	return mc.registry
}

// Describe implements prometheus.Collector interface.
func (mc *Collector) Describe(ch chan<- *prometheus.Desc) {}

// Collect implements prometheus.Collector interface.
func (mc *Collector) Collect(ch chan<- prometheus.Metric) {
	start := time.Now()
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	mc.collectDeviceInfo(ch)
	mc.collectDeviceCapacity(ch)
	mc.collectDeviceStatus(ch)
	mc.collectSmartAttributes(ch)
	mc.collectSummaryMetrics(ch)
	mc.collectStatistics(ch)
	mc.collectZFSPoolMetrics(ch)
	mc.collectWorkloadMetrics(ch)

	mc.logger.Debugf(
		"Metrics collected in %v for %d devices, %d workloads, and %d pools",
		time.Since(start), len(mc.devices), len(mc.workloads), len(mc.zfsPools),
	)
}

func (mc *Collector) collectDeviceInfo(ch chan<- prometheus.Metric) {
	for _, data := range mc.devices {
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc("scrutiny_device_info", "Device information",
				[]string{"device_id", "wwn", "device_name", "model_name", "serial_number",
					"firmware", "protocol", "host_id", "form_factor"}, nil),
			prometheus.GaugeValue, 1,
			data.Device.DeviceID, data.Device.WWN, data.Device.DeviceName, data.Device.ModelName,
			data.Device.SerialNumber, data.Device.Firmware,
			data.Device.DeviceProtocol, data.Device.HostId, data.Device.FormFactor,
		)
	}
}

func (mc *Collector) collectDeviceCapacity(ch chan<- prometheus.Metric) {
	for _, data := range mc.devices {
		if data.Device.Capacity > 0 {
			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc("scrutiny_device_capacity_bytes", "Device capacity in bytes",
					[]string{"device_id", "wwn", "device_name", "model_name", "protocol", "host_id"}, nil),
				prometheus.GaugeValue, float64(data.Device.Capacity),
				data.Device.DeviceID, data.Device.WWN, data.Device.DeviceName, data.Device.ModelName,
				data.Device.DeviceProtocol, data.Device.HostId,
			)
		}
	}
}

func (mc *Collector) collectDeviceStatus(ch chan<- prometheus.Metric) {
	for _, data := range mc.devices {
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc("scrutiny_device_status", "Device status (0=passed, 1=failed)",
				[]string{"device_id", "wwn", "device_name", "model_name", "protocol", "host_id"}, nil),
			prometheus.GaugeValue, float64(data.Device.DeviceStatus),
			data.Device.DeviceID, data.Device.WWN, data.Device.DeviceName, data.Device.ModelName,
			data.Device.DeviceProtocol, data.Device.HostId,
		)
	}
}

func (mc *Collector) collectSmartAttributes(ch chan<- prometheus.Metric) {
	for _, data := range mc.devices {
		baseLabels := []string{
			data.Device.DeviceID, data.Device.WWN, data.Device.DeviceName, data.Device.ModelName,
			data.Device.DeviceProtocol, data.Device.HostId,
		}

		for attrID, attr := range data.SmartData.Attributes {
			attrLabels := append(baseLabels, attrID)
			flattenedAttrs := attr.Flatten()

			for key, value := range flattenedAttrs {
				metricName := SanitizeMetricName(key)
				if floatVal, ok := TryParseFloat(value); ok {
					ch <- prometheus.MustNewConstMetric(
						prometheus.NewDesc(metricName, fmt.Sprintf("SMART attribute %s", key),
							[]string{"device_id", "wwn", "device_name", "model_name", "protocol", "host_id", "attribute_id"}, nil),
						prometheus.GaugeValue, floatVal, attrLabels...,
					)
				}
			}
		}
	}
}

func (mc *Collector) collectSummaryMetrics(ch chan<- prometheus.Metric) {
	for _, data := range mc.devices {
		labels := []string{
			data.Device.DeviceID, data.Device.WWN, data.Device.DeviceName, data.Device.ModelName,
			data.Device.DeviceProtocol, data.Device.HostId,
		}

		if data.SmartData.Temp > 0 {
			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc("scrutiny_smart_temperature_celsius",
					"Device temperature in Celsius",
					[]string{"device_id", "wwn", "device_name", "model_name", "protocol", "host_id"}, nil),
				prometheus.GaugeValue, float64(data.SmartData.Temp), labels...,
			)
		}

		if data.SmartData.PowerOnHours > 0 {
			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc("scrutiny_smart_power_on_hours", "Device power on hours",
					[]string{"device_id", "wwn", "device_name", "model_name", "protocol", "host_id"}, nil),
				prometheus.GaugeValue, float64(data.SmartData.PowerOnHours), labels...,
			)
		}

		if data.SmartData.PowerCycleCount > 0 {
			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc("scrutiny_smart_power_cycle_count", "Device power cycle count",
					[]string{"device_id", "wwn", "device_name", "model_name", "protocol", "host_id"}, nil),
				prometheus.GaugeValue, float64(data.SmartData.PowerCycleCount), labels...,
			)
		}

		timestampMs := float64(data.SmartData.Date.Unix() * 1000)
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc("scrutiny_smart_collector_timestamp",
				"Timestamp of last data collection",
				[]string{"device_id", "wwn", "device_name", "model_name", "protocol", "host_id"}, nil),
			prometheus.GaugeValue, timestampMs, labels...,
		)
	}
}

func (mc *Collector) collectStatistics(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("scrutiny_devices_total", "Total number of monitored devices", nil, nil),
		prometheus.GaugeValue, float64(len(mc.devices)),
	)

	protocolCount := make(map[string]int)
	for _, data := range mc.devices {
		protocol := data.Device.DeviceProtocol
		if protocol == "" {
			protocol = "unknown"
		}
		protocolCount[protocol]++
	}

	for protocol, count := range protocolCount {
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc("scrutiny_devices_by_protocol", "Number of devices by protocol",
				[]string{"protocol"}, nil),
			prometheus.GaugeValue, float64(count), protocol,
		)
	}

	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("scrutiny_zfs_pools_total", "Total number of monitored ZFS pools", nil, nil),
		prometheus.GaugeValue, float64(len(mc.zfsPools)),
	)
}

func (mc *Collector) collectZFSPoolMetrics(ch chan<- prometheus.Metric) {
	statusOptions := []models.ZFSPoolStatus{
		"",
		models.ZFSPoolStatusOnline,
		models.ZFSPoolStatusDegraded,
		models.ZFSPoolStatusFaulted,
		models.ZFSPoolStatusOffline,
		models.ZFSPoolStatusRemoved,
		models.ZFSPoolStatusUnavail,
	}
	scrubOptions := []models.ZFSScrubState{
		"",
		models.ZFSScrubStateNone,
		models.ZFSScrubStateScanning,
		models.ZFSScrubStateFinished,
		models.ZFSScrubStateCanceled,
	}

	for _, data := range mc.zfsPools {
		labels := []string{data.Pool.GUID, data.Pool.Name, data.Pool.HostID}

		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc("scrutiny_zfs_pool_size_bytes", "ZFS pool size in bytes",
				[]string{"guid", "pool_name", "host_id"}, nil),
			prometheus.GaugeValue, float64(data.Pool.Size), labels...,
		)
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc("scrutiny_zfs_pool_allocated_bytes", "ZFS pool allocated bytes",
				[]string{"guid", "pool_name", "host_id"}, nil),
			prometheus.GaugeValue, float64(data.Pool.Allocated), labels...,
		)
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc("scrutiny_zfs_pool_free_bytes", "ZFS pool free bytes",
				[]string{"guid", "pool_name", "host_id"}, nil),
			prometheus.GaugeValue, float64(data.Pool.Free), labels...,
		)
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc("scrutiny_zfs_pool_capacity_percent", "ZFS pool capacity percent",
				[]string{"guid", "pool_name", "host_id"}, nil),
			prometheus.GaugeValue, data.Pool.CapacityPercent, labels...,
		)
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc("scrutiny_zfs_pool_fragmentation_percent", "ZFS pool fragmentation percent",
				[]string{"guid", "pool_name", "host_id"}, nil),
			prometheus.GaugeValue, float64(data.Pool.Fragmentation), labels...,
		)
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc("scrutiny_zfs_pool_errors_read_total", "ZFS pool read errors",
				[]string{"guid", "pool_name", "host_id"}, nil),
			prometheus.GaugeValue, float64(data.Pool.TotalReadErrors), labels...,
		)
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc("scrutiny_zfs_pool_errors_write_total", "ZFS pool write errors",
				[]string{"guid", "pool_name", "host_id"}, nil),
			prometheus.GaugeValue, float64(data.Pool.TotalWriteErrors), labels...,
		)
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc("scrutiny_zfs_pool_errors_checksum_total", "ZFS pool checksum errors",
				[]string{"guid", "pool_name", "host_id"}, nil),
			prometheus.GaugeValue, float64(data.Pool.TotalChecksumErrors), labels...,
		)
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc("scrutiny_zfs_pool_scrub_scanned_bytes", "ZFS pool scrub scanned bytes",
				[]string{"guid", "pool_name", "host_id"}, nil),
			prometheus.GaugeValue, float64(data.Pool.ScrubScannedBytes), labels...,
		)
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc("scrutiny_zfs_pool_scrub_issued_bytes", "ZFS pool scrub issued bytes",
				[]string{"guid", "pool_name", "host_id"}, nil),
			prometheus.GaugeValue, float64(data.Pool.ScrubIssuedBytes), labels...,
		)
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc("scrutiny_zfs_pool_scrub_total_bytes", "ZFS pool scrub total bytes",
				[]string{"guid", "pool_name", "host_id"}, nil),
			prometheus.GaugeValue, float64(data.Pool.ScrubTotalBytes), labels...,
		)
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc("scrutiny_zfs_pool_scrub_errors_total", "ZFS pool scrub errors",
				[]string{"guid", "pool_name", "host_id"}, nil),
			prometheus.GaugeValue, float64(data.Pool.ScrubErrorsCount), labels...,
		)
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc("scrutiny_zfs_pool_scrub_percent_complete", "ZFS pool scrub percent complete",
				[]string{"guid", "pool_name", "host_id"}, nil),
			prometheus.GaugeValue, data.Pool.ScrubPercentComplete, labels...,
		)

		currentStatus := data.Pool.Status
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc("scrutiny_zfs_pool_status_code", "ZFS pool status code",
				[]string{"guid", "pool_name", "host_id"}, nil),
			prometheus.GaugeValue, zfsPoolStatusCodes[currentStatus], labels...,
		)
		for _, status := range statusOptions {
			statusLabel := string(status)
			if statusLabel == "" {
				statusLabel = "unknown"
			}
			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc("scrutiny_zfs_pool_status", "ZFS pool status as one-hot gauge",
					[]string{"guid", "pool_name", "host_id", "status"}, nil),
				prometheus.GaugeValue, metricValue(currentStatus, status),
				data.Pool.GUID, data.Pool.Name, data.Pool.HostID, statusLabel,
			)
		}

		currentScrub := data.Pool.ScrubState
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc("scrutiny_zfs_pool_scrub_state_code", "ZFS pool scrub state code",
				[]string{"guid", "pool_name", "host_id"}, nil),
			prometheus.GaugeValue, zfsScrubStateCodes[currentScrub], labels...,
		)
		for _, state := range scrubOptions {
			scrubLabel := string(state)
			if scrubLabel == "" {
				scrubLabel = "unknown"
			}
			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc("scrutiny_zfs_pool_scrub_state", "ZFS pool scrub state as one-hot gauge",
					[]string{"guid", "pool_name", "host_id", "scrub_state"}, nil),
				prometheus.GaugeValue, metricValue(currentScrub, state),
				data.Pool.GUID, data.Pool.Name, data.Pool.HostID, scrubLabel,
			)
		}
	}
}

func (mc *Collector) collectWorkloadMetrics(ch chan<- prometheus.Metric) {
	for _, data := range mc.workloads {
		labels := []string{
			data.Insight.DeviceID,
			data.Insight.DeviceWWN,
			data.Insight.DeviceName,
			data.Insight.ModelName,
			data.Insight.DeviceProtocol,
			data.Insight.HostId,
		}

		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc("scrutiny_workload_daily_read_bytes", "Estimated daily read bytes",
				[]string{"device_id", "wwn", "device_name", "model_name", "protocol", "host_id"}, nil),
			prometheus.GaugeValue, float64(data.Insight.DailyReadBytes), labels...,
		)
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc("scrutiny_workload_daily_write_bytes", "Estimated daily write bytes",
				[]string{"device_id", "wwn", "device_name", "model_name", "protocol", "host_id"}, nil),
			prometheus.GaugeValue, float64(data.Insight.DailyWriteBytes), labels...,
		)
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc("scrutiny_workload_total_read_bytes", "Total read bytes across the queried span",
				[]string{"device_id", "wwn", "device_name", "model_name", "protocol", "host_id"}, nil),
			prometheus.GaugeValue, float64(data.Insight.TotalReadBytes), labels...,
		)
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc("scrutiny_workload_total_write_bytes", "Total write bytes across the queried span",
				[]string{"device_id", "wwn", "device_name", "model_name", "protocol", "host_id"}, nil),
			prometheus.GaugeValue, float64(data.Insight.TotalWriteBytes), labels...,
		)
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc("scrutiny_workload_read_write_ratio", "Read/write ratio",
				[]string{"device_id", "wwn", "device_name", "model_name", "protocol", "host_id"}, nil),
			prometheus.GaugeValue, data.Insight.ReadWriteRatio, labels...,
		)
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc("scrutiny_workload_time_span_hours", "Time span used for workload calculation",
				[]string{"device_id", "wwn", "device_name", "model_name", "protocol", "host_id"}, nil),
			prometheus.GaugeValue, data.Insight.TimeSpanHours, labels...,
		)
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc("scrutiny_workload_data_points", "Data points used for workload calculation",
				[]string{"device_id", "wwn", "device_name", "model_name", "protocol", "host_id"}, nil),
			prometheus.GaugeValue, float64(data.Insight.DataPoints), labels...,
		)

		intensity := data.Insight.Intensity
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc("scrutiny_workload_intensity_code", "Workload intensity code",
				[]string{"device_id", "wwn", "device_name", "model_name", "protocol", "host_id"}, nil),
			prometheus.GaugeValue, workloadIntensityCodes[intensity], labels...,
		)
		for _, candidate := range workloadIntensityOrder {
			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc("scrutiny_workload_intensity", "Workload intensity as one-hot gauge",
					[]string{"device_id", "wwn", "device_name", "model_name", "protocol", "host_id", "intensity"}, nil),
				prometheus.GaugeValue, metricValue(intensity, candidate),
				data.Insight.DeviceID, data.Insight.DeviceWWN, data.Insight.DeviceName,
				data.Insight.ModelName, data.Insight.DeviceProtocol, data.Insight.HostId, candidate,
			)
		}

		if data.Insight.Endurance != nil {
			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc("scrutiny_workload_endurance_percentage_used", "SSD endurance percentage used",
					[]string{"device_id", "wwn", "device_name", "model_name", "protocol", "host_id"}, nil),
				prometheus.GaugeValue, float64(data.Insight.Endurance.PercentageUsed), labels...,
			)
			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc("scrutiny_workload_endurance_tb_written", "SSD terabytes written so far",
					[]string{"device_id", "wwn", "device_name", "model_name", "protocol", "host_id"}, nil),
				prometheus.GaugeValue, data.Insight.Endurance.TBWrittenSoFar, labels...,
			)
			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc("scrutiny_workload_endurance_estimated_lifespan_days", "Estimated remaining SSD lifespan in days",
					[]string{"device_id", "wwn", "device_name", "model_name", "protocol", "host_id"}, nil),
				prometheus.GaugeValue, float64(data.Insight.Endurance.EstimatedLifespanDays), labels...,
			)
		}

		if data.Insight.Spike != nil {
			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc("scrutiny_workload_spike_factor", "Workload spike factor compared to baseline",
					[]string{"device_id", "wwn", "device_name", "model_name", "protocol", "host_id"}, nil),
				prometheus.GaugeValue, data.Insight.Spike.SpikeFactor, labels...,
			)
			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc("scrutiny_workload_spike_recent_daily_write_bytes", "Recent daily write bytes used for spike detection",
					[]string{"device_id", "wwn", "device_name", "model_name", "protocol", "host_id"}, nil),
				prometheus.GaugeValue, float64(data.Insight.Spike.RecentDailyWriteBytes), labels...,
			)
			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc("scrutiny_workload_spike_baseline_daily_write_bytes", "Baseline daily write bytes used for spike detection",
					[]string{"device_id", "wwn", "device_name", "model_name", "protocol", "host_id"}, nil),
				prometheus.GaugeValue, float64(data.Insight.Spike.BaselineDailyWriteBytes), labels...,
			)
		}
	}
}
