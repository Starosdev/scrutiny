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
func (sr *scrutinyRepository) SavePerformanceResults(ctx context.Context, wwn string, perfData *measurements.Performance) error {
	perfData.DeviceWWN = wwn

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
func (sr *scrutinyRepository) GetPerformanceHistory(ctx context.Context, wwn string, durationKey string) ([]measurements.Performance, error) {
	bucketName := sr.lookupBucketName(durationKey)
	duration := sr.lookupDuration(durationKey)

	queryStr := fmt.Sprintf(`
		from(bucket: "%s")
		|> range(start: %s, stop: %s)
		|> filter(fn: (r) => r["_measurement"] == "performance")
		|> filter(fn: (r) => r["device_wwn"] == "%s")
		|> aggregateWindow(every: 1h, fn: last, createEmpty: false)
		|> pivot(rowKey:["_time"], columnKey: ["_field"], valueColumn: "_value")
		|> sort(columns: ["_time"], desc: false)
	`, bucketName, duration[0], duration[1], wwn)

	result, err := sr.influxQueryApi.Query(ctx, queryStr)
	if err != nil {
		return nil, fmt.Errorf("failed to query performance metrics: %v", err)
	}

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
func (sr *scrutinyRepository) GetPerformanceBaseline(ctx context.Context, wwn string, count int) (*measurements.PerformanceBaseline, error) {
	bucketName := sr.appConfig.GetString("web.influxdb.bucket")

	queryStr := fmt.Sprintf(`
		from(bucket: "%s")
		|> range(start: -30d)
		|> filter(fn: (r) => r["_measurement"] == "performance")
		|> filter(fn: (r) => r["device_wwn"] == "%s")
		|> pivot(rowKey:["_time"], columnKey: ["_field"], valueColumn: "_value")
		|> sort(columns: ["_time"], desc: true)
		|> limit(n: %d)
	`, bucketName, wwn, count)

	result, err := sr.influxQueryApi.Query(ctx, queryStr)
	if err != nil {
		return nil, fmt.Errorf("failed to query performance baseline: %v", err)
	}

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
