package database

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models/collector"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
)

// //////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Temperature Data
// //////////////////////////////////////////////////////////////////////////////////////////////////////////////////
func (sr *scrutinyRepository) SaveSmartTemperature(ctx context.Context, wwn string, deviceID string, collectorSmartData *collector.SmartInfo, retrieveSCTTemperatureHistory bool, storeTemperatureHistory bool) error {
	if !storeTemperatureHistory {
		return nil
	}
	if len(collectorSmartData.AtaSctTemperatureHistory.Table) > 0 && retrieveSCTTemperatureHistory {

		for ndx, temp := range collectorSmartData.AtaSctTemperatureHistory.Table {
			//temp value may be null, we must skip/ignore them. See #393
			if temp == 0 {
				continue
			}

			intervalSec := collectorSmartData.AtaSctTemperatureHistory.LoggingIntervalMinutes * 60
			datapointTime := collectorSmartData.LocalTime.TimeT - int64(ndx)*intervalSec
			alignedDatapointTime := datapointTime - datapointTime%intervalSec
			smartTemp := measurements.SmartTemperature{
				Date: time.Unix(alignedDatapointTime, 0),
				Temp: temp,
			}

			tags, fields := smartTemp.Flatten()
			tags["device_wwn"] = wwn
			tags["device_id"] = deviceID
			p := influxdb2.NewPoint("temp",
				tags,
				fields,
				smartTemp.Date)
			err := sr.influxWriteApi.WritePoint(ctx, p)
			if err != nil {
				return err
			}
		}
	}

	// Even if ata_sct_temperature_history is present, also add current temperature. See #824
	smartTemp := measurements.SmartTemperature{
		Date: time.Unix(collectorSmartData.LocalTime.TimeT, 0),
		Temp: measurements.CorrectedTemperature(collectorSmartData),
	}

	tags, fields := smartTemp.Flatten()
	tags["device_wwn"] = wwn
	tags["device_id"] = deviceID
	p := influxdb2.NewPoint("temp",
		tags,
		fields,
		smartTemp.Date)
	return sr.influxWriteApi.WritePoint(ctx, p)
}

func (sr *scrutinyRepository) GetSmartTemperatureHistory(ctx context.Context, durationKey string) (map[string][]measurements.SmartTemperature, error) {
	return sr.getSmartTemperatureHistory(ctx, durationKey, nil)
}

func (sr *scrutinyRepository) GetSmartTemperatureHistoryForDevices(ctx context.Context, durationKey string, deviceIDs []string) (map[string][]measurements.SmartTemperature, error) {
	return sr.getSmartTemperatureHistory(ctx, durationKey, deviceIDs)
}

// GetTemperatureNotificationHistory uses only raw points tagged with the current
// device ID. WWNs and legacy untagged points cannot safely establish an excursion.
func (sr *scrutinyRepository) GetTemperatureNotificationHistory(ctx context.Context, deviceID string) ([]measurements.SmartTemperature, error) {
	if deviceID == "" {
		return nil, fmt.Errorf("temperature notification history requires a device ID")
	}
	query := fmt.Sprintf(`from(bucket: %s)
|> range(start: %s, stop: %s)
|> filter(fn: (r) => r["_measurement"] == "temp" and r["_field"] == "temp")
|> filter(fn: (r) => exists r["device_id"] and r["device_id"] == %s)`,
		strconv.Quote(sr.lookupBucketName(DURATION_KEY_DAY)), INFLUX_DURATION_1_DAY, INFLUX_NOW, strconv.Quote(deviceID))
	result, err := sr.influxQueryApi.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer result.Close()
	var history []measurements.SmartTemperature
	for result.Next() {
		record := result.Record()
		if record.ValueByKey("device_id") != deviceID || record.Time().IsZero() {
			continue
		}
		point := measurements.SmartTemperature{Date: record.Time()}
		point.Inflate("temp", record.Value())
		history = append(history, point)
	}
	if err := result.Err(); err != nil {
		return nil, fmt.Errorf("temperature notification history query failed: %w", err)
	}
	return history, nil
}

func (sr *scrutinyRepository) getSmartTemperatureHistory(ctx context.Context, durationKey string, deviceIDs []string) (map[string][]measurements.SmartTemperature, error) {
	//we can get temp history for "week", "month", DURATION_KEY_YEAR, "forever"

	devices, devErr := sr.GetDevices(ctx)
	if devErr != nil && len(deviceIDs) > 0 {
		return nil, fmt.Errorf("failed to resolve selected temperature devices: %w", devErr)
	}
	uniqueWWNs := uniqueWWNDeviceIDs(devices)

	// With a selection, filter on the selected device_ids, plus the WWNs that selected devices
	// hold alone, which reach their untagged legacy points.
	var selectedIDs, legacyWWNs []string
	if len(deviceIDs) > 0 {
		wanted := map[string]struct{}{}
		for _, deviceID := range deviceIDs {
			wanted[deviceID] = struct{}{}
		}
		for i := range devices {
			if _, selected := wanted[devices[i].DeviceID]; !selected {
				continue
			}
			selectedIDs = append(selectedIDs, devices[i].DeviceID)
			if uniqueWWNs[devices[i].WWN] == devices[i].DeviceID {
				legacyWWNs = append(legacyWWNs, devices[i].WWN)
			}
		}
	}

	deviceTempHistory := map[string][]measurements.SmartTemperature{}
	if len(deviceIDs) > 0 && len(selectedIDs) == 0 {
		return deviceTempHistory, nil
	}

	queryStr := sr.aggregateTempQuery(durationKey, selectedIDs, legacyWWNs)

	result, err := sr.influxQueryApi.Query(ctx, queryStr)
	if err != nil {
		return nil, err
	}
	defer result.Close()

	// Use Next() to iterate over query result lines
	for result.Next() {
		appendTempRecord(deviceTempHistory, result.Record().Values(), uniqueWWNs)
	}
	if result.Err() != nil {
		return nil, fmt.Errorf("temperature history query failed: %w", result.Err())
	}
	// A device's tagged and untagged series arrive separately; merge them in time order.
	for deviceID := range deviceTempHistory {
		history := deviceTempHistory[deviceID]
		sort.SliceStable(history, func(i, j int) bool { return history[i].Date.Before(history[j].Date) })
	}
	return deviceTempHistory, nil
}

// appendTempRecord attributes a single InfluxDB temperature record to a device (see
// historyRecordDeviceID) and appends the inflated SmartTemperature to its history.
func appendTempRecord(deviceTempHistory map[string][]measurements.SmartTemperature, values map[string]interface{}, uniqueWWNs map[string]string) {
	key, ok := historyRecordDeviceID(values, uniqueWWNs)
	if !ok {
		return
	}
	date, ok := values["_time"].(time.Time)
	if !ok {
		return
	}

	smartTemp := measurements.SmartTemperature{}
	for k, val := range values {
		smartTemp.Inflate(k, val)
	}
	smartTemp.Date = date
	deviceTempHistory[key] = append(deviceTempHistory[key], smartTemp)
}

// temperatureSelectionPredicate selects temperature points tagged with one of deviceIDs, and
// untagged legacy points carrying one of legacyWWNs.
func temperatureSelectionPredicate(deviceIDs []string, legacyWWNs []string) string {
	predicate := fmt.Sprintf(`(exists r["device_id"] and contains(value: r["device_id"], set: [%s]))`, quotedFluxSet(deviceIDs))
	if len(legacyWWNs) > 0 {
		predicate += fmt.Sprintf(` or (not exists r["device_id"] and contains(value: r["device_wwn"], set: [%s]))`, quotedFluxSet(legacyWWNs))
	}
	return predicate
}

func quotedFluxSet(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, strconv.Quote(value))
	}
	return strings.Join(quoted, ", ")
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Helper Methods
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// aggregateTempQuery builds the temperature history query. deviceIDs restricts it to those
// devices, and legacyWWNs adds their untagged points (see temperatureSelectionPredicate); with no
// deviceIDs every device is queried.
func (sr *scrutinyRepository) aggregateTempQuery(durationKey string, deviceIDs []string, legacyWWNs []string) string {

	/*
		import "influxdata/influxdb/schema"
		weekData = from(bucket: "metrics")
		  |> range(start: -1w, stop: now())
		  |> filter(fn: (r) => r["_measurement"] == "temp" )
		  |> aggregateWindow(every: 1h, fn: mean, createEmpty: false)
		  |> group(columns: ["device_id", "device_wwn"])
		  |> toInt()

		monthData = from(bucket: "metrics_weekly")
		  |> range(start: -1mo, stop: now())
		  |> filter(fn: (r) => r["_measurement"] == "temp" )
		  |> aggregateWindow(every: 1h, fn: mean, createEmpty: false)
		  |> group(columns: ["device_id", "device_wwn"])
		  |> toInt()

		union(tables: [weekData, monthData])
		  |> group(columns: ["device_id", "device_wwn"])
		  |> sort(columns: ["_time"], desc: false)
		  |> schema.fieldsAsCols()

	*/

	partialQueryStr := []string{
		`import "influxdata/influxdb/schema"`,
	}

	nestedDurationKeys := sr.lookupNestedDurationKeys(durationKey)

	subQueryNames := []string{}
	for _, nestedDurationKey := range nestedDurationKeys {
		bucketName := sr.lookupBucketName(nestedDurationKey)
		durationRange := sr.lookupDuration(nestedDurationKey)
		durationResolution := sr.lookupResolution(nestedDurationKey)

		subQueryNames = append(subQueryNames, fmt.Sprintf(`%sData`, nestedDurationKey))
		subQuery := []string{
			fmt.Sprintf(`%sData = from(bucket: "%s")`, nestedDurationKey, bucketName),
			fmt.Sprintf(`|> range(start: %s, stop: %s)`, durationRange[0], durationRange[1]),
			`|> filter(fn: (r) => r["_measurement"] == "temp" )`,
		}
		if len(deviceIDs) > 0 {
			subQuery = append(subQuery, fmt.Sprintf(`|> filter(fn: (r) => %s)`, temperatureSelectionPredicate(deviceIDs, legacyWWNs)))
		}
		if durationResolution != "" {
			subQuery = append(subQuery,
				fmt.Sprintf(`|> aggregateWindow(every: %s, fn: mean, createEmpty: false)`, durationResolution))
		}
		subQuery = append(subQuery, `|> group(columns: ["device_id", "device_wwn"])`, `|> toInt()`, "")
		partialQueryStr = append(partialQueryStr, subQuery...)
	}

	if len(subQueryNames) == 1 {
		//there's only one bucket being queried, no need to union, just aggregate the dataset and return
		partialQueryStr = append(partialQueryStr, []string{
			subQueryNames[0],
			"|> schema.fieldsAsCols()",
			"|> yield()",
		}...)
	} else {
		partialQueryStr = append(partialQueryStr, []string{
			fmt.Sprintf("union(tables: [%s])", strings.Join(subQueryNames, ", ")),
			`|> group(columns: ["device_id", "device_wwn"])`,
			`|> sort(columns: ["_time"], desc: false)`,
			"|> schema.fieldsAsCols()",
		}...)
	}

	return strings.Join(partialQueryStr, "\n")
}
