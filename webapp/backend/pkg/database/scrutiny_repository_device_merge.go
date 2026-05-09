package database

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
	"gorm.io/gorm"
)

func (sr *scrutinyRepository) MergeDevices(ctx context.Context, sourceDeviceID string, destinationDeviceID string) error {
	if sourceDeviceID == destinationDeviceID {
		return fmt.Errorf("source and destination devices must be different")
	}

	sourceDevice, err := sr.GetDeviceDetails(ctx, sourceDeviceID)
	if err != nil {
		return fmt.Errorf("could not find source device: %w", err)
	}

	destinationDevice, err := sr.GetDeviceDetails(ctx, destinationDeviceID)
	if err != nil {
		return fmt.Errorf("could not find destination device: %w", err)
	}

	if err := sr.copyInfluxDeviceHistory(ctx, sourceDevice, destinationDevice); err != nil {
		return err
	}

	if err := sr.deleteInfluxDeviceHistory(ctx, sourceDevice); err != nil {
		return err
	}

	return sr.gormClient.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if sourceDevice.CreatedAt.Before(destinationDevice.CreatedAt) {
			if err := tx.Model(&destinationDevice).Update("created_at", sourceDevice.CreatedAt).Error; err != nil {
				return fmt.Errorf("could not update destination device created_at: %w", err)
			}
		}

		if err := tx.Where(queryDeviceID, sourceDevice.DeviceID).Delete(&sourceDevice).Error; err != nil {
			return fmt.Errorf("could not delete source device: %w", err)
		}

		return nil
	})
}

func (sr *scrutinyRepository) copyInfluxDeviceHistory(ctx context.Context, sourceDevice, destinationDevice models.Device) error {
	for _, bucket := range sr.deviceHistoryBuckets() {
		for _, measurement := range []string{"smart", "temp", "performance"} {
			points, err := sr.queryDeviceMeasurementPoints(ctx, bucket, measurement, sourceDevice.WWN, destinationDevice)
			if err != nil {
				return fmt.Errorf("could not query %s history in bucket %s: %w", measurement, bucket, err)
			}
			for i := range points {
				if err := sr.influxWriteApi.WritePoint(ctx, points[i]); err != nil {
					return fmt.Errorf("could not write %s history in bucket %s: %w", measurement, bucket, err)
				}
			}
		}
	}
	return nil
}

func (sr *scrutinyRepository) deleteInfluxDeviceHistory(ctx context.Context, sourceDevice models.Device) error {
	if sourceDevice.WWN == "" {
		return nil
	}

	for _, bucket := range sr.deviceHistoryBuckets() {
		if err := sr.influxClient.DeleteAPI().DeleteWithName(
			ctx,
			sr.appConfig.GetString(cfgInfluxDBOrg),
			bucket,
			time.Now().AddDate(-10, 0, 0),
			time.Now().AddDate(10, 0, 0),
			fmt.Sprintf(`device_wwn=%q`, sourceDevice.WWN),
		); err != nil {
			return fmt.Errorf("could not delete source history from bucket %s: %w", bucket, err)
		}
	}

	return nil
}

func (sr *scrutinyRepository) deviceHistoryBuckets() []string {
	buckets := []string{
		sr.appConfig.GetString(cfgInfluxDBBucket),
		fmt.Sprintf("%s_weekly", sr.appConfig.GetString(cfgInfluxDBBucket)),
		fmt.Sprintf("%s_monthly", sr.appConfig.GetString(cfgInfluxDBBucket)),
		fmt.Sprintf("%s_yearly", sr.appConfig.GetString(cfgInfluxDBBucket)),
	}
	return buckets
}

func (sr *scrutinyRepository) queryDeviceMeasurementPoints(ctx context.Context, bucket string, measurement string, sourceWWN string, destinationDevice models.Device) ([]*write.Point, error) {
	if sourceWWN == "" {
		return nil, nil
	}

	queryStr := fmt.Sprintf(`
from(bucket: "%s")
|> range(start: -10y, stop: now())
|> filter(fn: (r) => r["_measurement"] == "%s")
|> filter(fn: (r) => r["device_wwn"] == "%s")
|> pivot(rowKey:["_time"], columnKey: ["_field"], valueColumn: "_value")
|> sort(columns: ["_time"], desc: false)
`, bucket, measurement, sourceWWN)

	result, err := sr.influxQueryApi.Query(ctx, queryStr)
	if err != nil {
		return nil, err
	}
	defer result.Close()

	points := []*write.Point{}
	for result.Next() {
		values := result.Record().Values()
		fields := measurementFields(measurement, values)
		if len(fields) == 0 {
			continue
		}
		tags := measurementTags(measurement, values, destinationDevice)
		point := influxdb2.NewPoint(
			measurement,
			tags,
			fields,
			values["_time"].(time.Time),
		)
		points = append(points, point)
	}

	if result.Err() != nil {
		return nil, result.Err()
	}

	return points, nil
}

func measurementTags(measurement string, values map[string]interface{}, destinationDevice models.Device) map[string]string {
	tags := map[string]string{
		"device_wwn": destinationDevice.WWN,
		"device_id":  destinationDevice.DeviceID,
	}

	switch measurement {
	case "smart", "performance":
		if val, ok := values["device_protocol"].(string); ok && val != "" {
			tags["device_protocol"] = val
		} else if destinationDevice.DeviceProtocol != "" {
			tags["device_protocol"] = destinationDevice.DeviceProtocol
		}
	}

	if measurement == "performance" {
		if val, ok := values["profile"].(string); ok && val != "" {
			tags["profile"] = val
		}
	}

	return tags
}

func measurementFields(measurement string, values map[string]interface{}) map[string]interface{} {
	fields := map[string]interface{}{}
	tagKeys := map[string]bool{
		"result":          true,
		"table":           true,
		"_start":          true,
		"_stop":           true,
		"_time":           true,
		"_measurement":    true,
		"device_wwn":      true,
		"device_id":       true,
		"device_protocol": true,
		"profile":         true,
	}

	fieldKeysByMeasurement := map[string][]string{
		"temp": {
			"temp",
		},
	}

	if explicitKeys, ok := fieldKeysByMeasurement[measurement]; ok {
		for _, key := range explicitKeys {
			if value, exists := values[key]; exists && value != nil {
				fields[key] = value
			}
		}
		return fields
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		if tagKeys[key] || strings.HasPrefix(key, "_") {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if values[key] != nil {
			fields[key] = values[key]
		}
	}

	return fields
}
