package database

import (
	"context"
	"fmt"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
)

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Performance Benchmarks (InfluxDB)
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// SavePerformanceResults saves performance benchmark results to InfluxDB
func (sr *scrutinyRepository) SavePerformanceResults(ctx context.Context, deviceID string, perfData *measurements.Performance) error {
	// Tag points with both identifiers. This used to look the device up by passing the
	// WWN as a device_id, which never matched, so performance points carried no device_id.
	device, err := sr.GetDeviceDetails(ctx, deviceID)
	if err != nil {
		return fmt.Errorf("could not find device %s: %w", deviceID, err)
	}
	perfData.DeviceID = device.DeviceID
	perfData.DeviceWWN = device.WWN

	tags, fields := perfData.Flatten()

	return sr.saveDatapoint(
		sr.influxWriteApi,
		"performance",
		tags,
		fields,
		perfData.Date,
		ctx,
	)
}

// GetPerformanceHistory retrieves historical performance metrics for a device
func (sr *scrutinyRepository) GetPerformanceHistory(ctx context.Context, deviceID string, durationKey string) ([]measurements.Performance, error) {
	_, historyFilter, err := sr.deviceHistoryFilter(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	bucketName := sr.lookupBucketName(durationKey)
	duration := sr.lookupDuration(durationKey)

	queryStr := fmt.Sprintf(`
		from(bucket: "%s")
		|> range(start: %s, stop: %s)
		|> filter(fn: (r) => r["_measurement"] == "performance")
		|> filter(fn: (r) => %s)
		|> aggregateWindow(every: 1h, fn: last, createEmpty: false)
		|> pivot(rowKey:["_time"], columnKey: ["_field"], valueColumn: "_value")
		|> sort(columns: ["_time"], desc: false)
	`, bucketName, duration[0], duration[1], historyFilter)

	result, err := sr.influxQueryApi.Query(ctx, queryStr)
	if err != nil {
		return nil, fmt.Errorf("failed to query performance metrics: %v", err)
	}
	defer result.Close()

	var history []measurements.Performance
	for result.Next() {
		record := result.Record()
		values := record.Values()

		perf, err := measurements.NewPerformanceFromInfluxDB(values)
		if err != nil {
			sr.logger.Warnf("Failed to parse performance metrics: %v", err)
			continue
		}

		history = append(history, *perf)
	}

	if result.Err() != nil {
		return nil, fmt.Errorf("query error: %v", result.Err())
	}

	return history, nil
}

// GetPerformanceBaseline calculates a baseline from the last N performance results
func (sr *scrutinyRepository) GetPerformanceBaseline(ctx context.Context, deviceID string, count int) (*measurements.PerformanceBaseline, error) {
	_, historyFilter, err := sr.deviceHistoryFilter(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	bucketName := sr.appConfig.GetString(cfgInfluxDBBucket)

	queryStr := fmt.Sprintf(`
		from(bucket: "%s")
		|> range(start: -30d)
		|> filter(fn: (r) => r["_measurement"] == "performance")
		|> filter(fn: (r) => %s)
		|> pivot(rowKey:["_time"], columnKey: ["_field"], valueColumn: "_value")
		|> sort(columns: ["_time"], desc: true)
		|> limit(n: %d)
	`, bucketName, historyFilter, count)

	result, err := sr.influxQueryApi.Query(ctx, queryStr)
	if err != nil {
		return nil, fmt.Errorf("failed to query performance baseline: %v", err)
	}
	defer result.Close()

	var results []measurements.Performance
	for result.Next() {
		record := result.Record()
		values := record.Values()

		perf, err := measurements.NewPerformanceFromInfluxDB(values)
		if err != nil {
			sr.logger.Warnf("Failed to parse performance baseline: %v", err)
			continue
		}

		results = append(results, *perf)
	}

	if result.Err() != nil {
		return nil, fmt.Errorf("query error: %v", result.Err())
	}

	if len(results) == 0 {
		return nil, nil
	}

	baseline := &measurements.PerformanceBaseline{
		SampleCount: len(results),
	}

	for i := range results {
		baseline.SeqReadBwBytes += results[i].SeqReadBwBytes
		baseline.SeqWriteBwBytes += results[i].SeqWriteBwBytes
		baseline.RandReadIOPS += results[i].RandReadIOPS
		baseline.RandWriteIOPS += results[i].RandWriteIOPS
		baseline.RandReadLatAvgNs += results[i].RandReadLatAvgNs
		baseline.RandWriteLatAvgNs += results[i].RandWriteLatAvgNs
	}

	n := float64(len(results))
	baseline.SeqReadBwBytes /= n
	baseline.SeqWriteBwBytes /= n
	baseline.RandReadIOPS /= n
	baseline.RandWriteIOPS /= n
	baseline.RandReadLatAvgNs /= n
	baseline.RandWriteLatAvgNs /= n

	return baseline, nil
}
