package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/collector"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	"gorm.io/gorm"
)

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// MDADM Array
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// RegisterMdadmArray inserts or updates an MDADM array in the database
func (sr *scrutinyRepository) RegisterMdadmArray(ctx context.Context, array models.MDADMArray) error {
	// Ensure UpdatedAt is set to current time
	array.UpdatedAt = time.Now()

	// Check if array already exists
	var existing models.MDADMArray
	result := sr.gormClient.WithContext(ctx).Where("uuid = ?", array.UUID).First(&existing)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		// New array - create it
		if err := sr.gormClient.WithContext(ctx).Create(&array).Error; err != nil {
			return err
		}
	} else if result.Error != nil {
		return result.Error
	} else {
		// Existing array - update it
		if err := sr.gormClient.WithContext(ctx).Model(&existing).Updates(map[string]interface{}{
			"name":       array.Name,
			"level":      array.Level,
			"devices":    array.Devices,
			"updated_at": array.UpdatedAt,
		}).Error; err != nil {
			return err
		}
	}

	return nil
}

// GetMdadmArrays returns all non-archived MDADM arrays
func (sr *scrutinyRepository) GetMdadmArrays(ctx context.Context) ([]models.MDADMArray, error) {
	arrays := []models.MDADMArray{}
	if err := sr.gormClient.WithContext(ctx).Where("archived = ?", false).Find(&arrays).Error; err != nil {
		return nil, fmt.Errorf("could not get MDADM arrays from DB: %v", err)
	}
	return arrays, nil
}

// GetMdadmArrayDetails returns a single MDADM array
func (sr *scrutinyRepository) GetMdadmArrayDetails(ctx context.Context, uuid string) (models.MDADMArray, error) {
	var array models.MDADMArray
	if err := sr.gormClient.WithContext(ctx).Where("uuid = ?", uuid).First(&array).Error; err != nil {
		return models.MDADMArray{}, err
	}
	return array, nil
}

// UpdateMdadmArrayArchived updates the archived state of an MDADM array
func (sr *scrutinyRepository) UpdateMdadmArrayArchived(ctx context.Context, uuid string, archived bool) error {
	return sr.gormClient.WithContext(ctx).Model(&models.MDADMArray{}).Where("uuid = ?", uuid).Update("archived", archived).Error
}

// UpdateMdadmArrayMuted updates the muted state of an MDADM array
func (sr *scrutinyRepository) UpdateMdadmArrayMuted(ctx context.Context, uuid string, muted bool) error {
	return sr.gormClient.WithContext(ctx).Model(&models.MDADMArray{}).Where("uuid = ?", uuid).Update("muted", muted).Error
}

// UpdateMdadmArrayLabel updates the label of an MDADM array
func (sr *scrutinyRepository) UpdateMdadmArrayLabel(ctx context.Context, uuid string, label string) error {
	return sr.gormClient.WithContext(ctx).Model(&models.MDADMArray{}).Where("uuid = ?", uuid).Update("label", label).Error
}

// DeleteMdadmArray deletes an MDADM array and its associated data
func (sr *scrutinyRepository) DeleteMdadmArray(ctx context.Context, uuid string) error {
	// Delete relational metadata
	if err := sr.gormClient.WithContext(ctx).Where("uuid = ?", uuid).Delete(&models.MDADMArray{}).Error; err != nil {
		return err
	}

	// Delete data from InfluxDB
	buckets := []string{
		sr.appConfig.GetString(cfgInfluxDBBucket),
		sr.appConfig.GetString(cfgInfluxDBBucket) + "_weekly",
		sr.appConfig.GetString(cfgInfluxDBBucket) + "_monthly",
		sr.appConfig.GetString(cfgInfluxDBBucket) + "_yearly",
	}

	for _, bucket := range buckets {
		if err := sr.influxClient.DeleteAPI().DeleteWithName(
			ctx,
			sr.appConfig.GetString(cfgInfluxDBOrg),
			bucket,
			time.Now().AddDate(-10, 0, 0),
			time.Now(),
			fmt.Sprintf(`array_uuid="%s"`, uuid),
		); err != nil {
			return err
		}
	}

	return nil
}

// GetMdadmArraysSummary returns a summary of all non-archived MDADM arrays
func (sr *scrutinyRepository) GetMdadmArraysSummary(ctx context.Context) (map[string]*models.MDADMArray, error) {
	arrays, err := sr.GetMdadmArrays(ctx)
	if err != nil {
		return nil, err
	}

	summary := make(map[string]*models.MDADMArray)
	for i := range arrays {
		summary[arrays[i].UUID] = &arrays[i]
	}

	return summary, nil
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// MDADM Array Metrics (InfluxDB)
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// SaveMdadmMetrics saves MDADM array metrics to InfluxDB
func (sr *scrutinyRepository) SaveMdadmMetrics(ctx context.Context, uuid string, metrics collector.MDADMMetrics) error {
	// Get array name for tagging
	var array models.MDADMArray
	if err := sr.gormClient.WithContext(ctx).Where("uuid = ?", uuid).First(&array).Error; err != nil {
		return err
	}

	influxMetrics := measurements.MDADMMetrics{
		Date:           time.Now(),
		ArrayUUID:      uuid,
		ArrayName:      array.Name,
		ActiveDevices:  metrics.ActiveDevices,
		WorkingDevices: metrics.WorkingDevices,
		FailedDevices:  metrics.FailedDevices,
		SpareDevices:   metrics.SpareDevices,
		State:          metrics.State,
		SyncProgress:   metrics.SyncProgress,
		RawMdstat:      metrics.RawMdstat,
	}

	tags, fields := influxMetrics.Flatten()

	return sr.saveDatapoint(
		sr.influxWriteApi,
		"mdadm_array",
		tags,
		fields,
		influxMetrics.Date,
		ctx,
	)
}

// GetMdadmMetricsHistory retrieves historical metrics for an MDADM array.
// Uses schema.fieldsAsCols() instead of aggregateWindow+pivot to preserve string fields
// (state, raw_mdstat) that aggregateWindow(fn: last) silently drops.
func (sr *scrutinyRepository) GetMdadmMetricsHistory(ctx context.Context, uuid string, durationKey string) ([]measurements.MDADMMetrics, error) {
	bucketName := sr.lookupBucketName(durationKey)
	duration := sr.lookupDuration(durationKey)

	queryStr := fmt.Sprintf(`
		import "influxdata/influxdb/schema"
		from(bucket: "%s")
		|> range(start: %s, stop: %s)
		|> filter(fn: (r) => r["_measurement"] == "mdadm_array")
		|> filter(fn: (r) => r["array_uuid"] == params.uuid)
		|> schema.fieldsAsCols()
		|> group()
		|> sort(columns: ["_time"], desc: false)
	`, bucketName, duration[0], duration[1])

	params := map[string]interface{}{
		"uuid": uuid,
	}

	sr.logger.Debugf("GetMdadmMetricsHistory query for uuid=%s bucket=%s: %s", uuid, bucketName, queryStr)

	result, err := sr.influxQueryApi.QueryWithParams(ctx, queryStr, params)
	if err != nil {
		sr.logger.Errorf("GetMdadmMetricsHistory query failed: %v", err)
		return nil, fmt.Errorf("failed to query MDADM array metrics: %v", err)
	}
	defer result.Close()

	var metricsHistory []measurements.MDADMMetrics
	for result.Next() {
		record := result.Record()
		values := record.Values()

		metrics, err := measurements.NewMDADMMetricsFromInfluxDB(values)
		if err != nil {
			sr.logger.Warnf("Failed to parse MDADM array metrics: %v", err)
			continue
		}

		metricsHistory = append(metricsHistory, *metrics)
	}

	sr.logger.Debugf("GetMdadmMetricsHistory returned %d records, resultErr=%v", len(metricsHistory), result.Err())

	return metricsHistory, result.Err()
}

// GetLatestMdadmMetrics fetches the single most recent datapoint with all fields preserved.
// Uses schema.fieldsAsCols() to correctly merge string and numeric fields into a single row.
func (sr *scrutinyRepository) GetLatestMdadmMetrics(ctx context.Context, uuid string) (*measurements.MDADMMetrics, error) {
	bucketName := sr.appConfig.GetString(cfgInfluxDBBucket)

	queryStr := fmt.Sprintf(`
		import "influxdata/influxdb/schema"
		from(bucket: "%s")
		|> range(start: -7d)
		|> filter(fn: (r) => r["_measurement"] == "mdadm_array")
		|> filter(fn: (r) => r["array_uuid"] == params.uuid)
		|> schema.fieldsAsCols()
		|> group()
		|> sort(columns: ["_time"], desc: true)
		|> limit(n: 1)
	`, bucketName)

	params := map[string]interface{}{
		"uuid": uuid,
	}

	sr.logger.Debugf("GetLatestMdadmMetrics query for uuid=%s: %s", uuid, queryStr)

	result, err := sr.influxQueryApi.QueryWithParams(ctx, queryStr, params)
	if err != nil {
		sr.logger.Errorf("GetLatestMdadmMetrics query failed: %v", err)
		return nil, fmt.Errorf("failed to query latest MDADM array metrics: %v", err)
	}
	defer result.Close()

	if result.Next() {
		metrics, err := measurements.NewMDADMMetricsFromInfluxDB(result.Record().Values())
		if err != nil {
			return nil, err
		}
		return metrics, nil
	}

	return nil, nil
}
