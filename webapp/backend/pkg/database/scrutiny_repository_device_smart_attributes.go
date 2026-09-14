package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/collector"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
)

// //////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// SMART
// //////////////////////////////////////////////////////////////////////////////////////////////////////////////////
func (sr *scrutinyRepository) SaveSmartAttributes(ctx context.Context, deviceID string, collectorSmartData collector.SmartInfo) (measurements.Smart, error) {
	// Resolve the device by device_id, never by WWN: several devices can share a WWN
	// (#851), and the device_id tag written here is what keeps their history apart.
	// Looking the device up first also lets device-scoped overrides apply to the
	// incoming SMART result as well as read-time projections.
	device, historyFilter, err := sr.deviceHistoryFilter(ctx, deviceID)
	if err != nil {
		return measurements.Smart{}, err
	}
	deviceSmartData := measurements.Smart{DeviceID: device.DeviceID}

	// Get merged overrides (config + database) for SMART attribute processing
	mergedOverrides := sr.GetMergedOverrides(ctx)

	err = deviceSmartData.FromCollectorSmartInfoWithOverrides(sr.appConfig, device.WWN, collectorSmartData, mergedOverrides)
	if err != nil {
		sr.logger.Errorln("Could not process SMART metrics", err)
		return measurements.Smart{}, err
	}

	// Apply delta-based evaluation for cumulative counter attributes (e.g., UltraDMA CRC Error Count).
	// Fetch the most recent existing SMART submission to compare values. Uses GetLatestSmartSubmission
	// (offset=0) because this is called BEFORE the current data is written to InfluxDB.
	// If a cumulative counter hasn't increased, suppress the warning since the underlying issue
	// may have been resolved.
	previousSmartData, prevErr := sr.smartSubmission(ctx, historyFilter, 0)
	var previousSmart *measurements.Smart
	if prevErr != nil || len(previousSmartData) < 1 {
		sr.logger.Debugln("No previous SMART submission available for delta evaluation (expected for first submission)")
	} else {
		previousSmart = &previousSmartData[0]
		previousValues := extractPreviousRawValues(previousSmart)
		deviceSmartData.ApplyDeltaEvaluation(previousValues)
	}

	// Flag Power-On Hours rollover (16-bit counter wrap) on attribute 9. Uses the previous
	// submission (when available) plus cross-attribute hour-counters; safe with a nil previous.
	deviceSmartData.DetectPowerOnHoursRollover(previousSmart)

	tags, fields := deviceSmartData.Flatten()

	if err := sr.syncDeviceSelfTests(ctx, &device, &collectorSmartData, selfTestPowerOnHours(&deviceSmartData, previousSmart)); err != nil {
		return measurements.Smart{}, err
	}

	// write point immediately
	return deviceSmartData, sr.saveDatapoint(sr.influxWriteApi, "smart", tags, fields, deviceSmartData.Date, ctx)
}

// extractPreviousRawValues extracts raw values from a previous SMART submission into a map
// keyed by attribute ID string, for use in delta-based evaluation.
func extractPreviousRawValues(previousSmart *measurements.Smart) map[string]int64 {
	values := make(map[string]int64)
	for attrId, attr := range previousSmart.Attributes {
		if ataAttr, ok := attr.(*measurements.SmartAtaAttribute); ok {
			values[attrId] = ataAttr.RawValue
		}
	}
	return values
}

// GetSmartAttributeHistory MUST return in sorted order, where newest entries are at the beginning of the list, and oldest are at the end.
// When selectEntries is > 0, only the most recent selectEntries database entries are returned, starting from the selectEntriesOffset entry.
// For example, with selectEntries = 5, selectEntries = 0, the most recent 5 are returned. With selectEntries = 3, selectEntries = 2, entries
// 2 to 4 are returned (2 being the third newest, since it is zero-indexed)
func (sr *scrutinyRepository) GetSmartAttributeHistory(ctx context.Context, deviceID string, durationKey string, selectEntries int, selectEntriesOffset int, attributes []string) ([]measurements.Smart, error) {
	_, historyFilter, err := sr.deviceHistoryFilter(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	queryStr := sr.aggregateSmartAttributesQuery(historyFilter, durationKey, selectEntries, selectEntriesOffset, attributes)
	sr.logger.Infoln(queryStr)

	smartResults := []measurements.Smart{}

	result, err := sr.influxQueryApi.Query(ctx, queryStr)
	if err == nil {
		defer result.Close()
		// Use Next() to iterate over query result lines
		for result.Next() {
			smartData, err := measurements.NewSmartFromInfluxDB(result.Record().Values(), sr.logger)
			if err != nil {
				return nil, err
			}
			smartResults = append(smartResults, *smartData)

		}
		if result.Err() != nil {
			sr.logger.Errorf("Query error: %s", result.Err().Error())
		}
	} else {
		return nil, err
	}

	return smartResults, nil

	//if err := device.SquashHistory(); err != nil {
	//	logger.Errorln("An error occurred while squashing device history", err)
	//	c.JSON(http.StatusInternalServerError, gin.H{"success": false})
	//	return
	//}
	//
	//if err := device.ApplyMetadataRules(); err != nil {
	//	logger.Errorln("An error occurred while applying scrutiny thresholds & rules", err)
	//	c.JSON(http.StatusInternalServerError, gin.H{"success": false})
	//	return
	//}

}

// GetPreviousSmartSubmission returns the previous raw SMART submission without daily aggregation.
// This is used for repeat notification detection to compare against the actual previous submission,
// not the previous day's aggregated value.
// Returns the second most recent submission (skipping the one just saved).
func (sr *scrutinyRepository) GetPreviousSmartSubmission(ctx context.Context, deviceID string) ([]measurements.Smart, error) {
	_, historyFilter, err := sr.deviceHistoryFilter(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	return sr.smartSubmission(ctx, historyFilter, 1)
}

// GetLatestSmartSubmission returns the most recent raw SMART submission without daily aggregation.
// This is used for delta evaluation BEFORE writing the current data to InfluxDB, so offset=0
// returns the actual most recent existing entry (which is the previous submission).
func (sr *scrutinyRepository) GetLatestSmartSubmission(ctx context.Context, deviceID string) ([]measurements.Smart, error) {
	_, historyFilter, err := sr.deviceHistoryFilter(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	return sr.smartSubmission(ctx, historyFilter, 0)
}

// smartSubmission returns one raw SMART submission from the last week for the points
// selected by historyFilter, newest first, skipping offset submissions.
func (sr *scrutinyRepository) smartSubmission(ctx context.Context, historyFilter string, offset int) ([]measurements.Smart, error) {
	queryStr := fmt.Sprintf(`
import "influxdata/influxdb/schema"
from(bucket: "%s")
|> range(start: -1w, stop: now())
|> filter(fn: (r) => r["_measurement"] == "smart")
|> filter(fn: (r) => %s)
|> schema.fieldsAsCols()
|> group()
|> sort(columns: ["_time"], desc: true)
|> limit(n: 1, offset: %d)
`, sr.appConfig.GetString(cfgInfluxDBBucket), historyFilter, offset)

	sr.logger.Debugln("smartSubmission query:", queryStr)

	smartResults := []measurements.Smart{}

	result, err := sr.influxQueryApi.Query(ctx, queryStr)
	if err != nil {
		return nil, err
	}
	defer result.Close()

	for result.Next() {
		smartData, err := measurements.NewSmartFromInfluxDB(result.Record().Values(), sr.logger)
		if err != nil {
			return nil, err
		}
		smartResults = append(smartResults, *smartData)
	}

	if result.Err() != nil {
		return nil, result.Err()
	}

	return smartResults, nil
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Helper Methods
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func (sr *scrutinyRepository) saveDatapoint(influxWriteApi api.WriteAPIBlocking, measurement string, tags map[string]string, fields map[string]interface{}, date time.Time, ctx context.Context) error {
	//sr.logger.Debugf("Storing datapoint in measurement '%s'. tags: %d fields: %d", measurement, len(tags), len(fields))
	p := influxdb2.NewPoint(measurement,
		tags,
		fields,
		date)

	// write point immediately
	return influxWriteApi.WritePoint(ctx, p)
}

func (sr *scrutinyRepository) aggregateSmartAttributesQuery(historyFilter string, durationKey string, selectEntries int, selectEntriesOffset int, attributes []string) string {

	/*

		import "influxdata/influxdb/schema"
		weekData = from(bucket: "metrics")
		|> range(start: -1w, stop: now())
		|> filter(fn: (r) => r["_measurement"] == "smart" )
		|> filter(fn: (r) => r["device_wwn"] == "0x5000c5002df89099" )
		|> tail(n: 10, offset: 0)
		|> schema.fieldsAsCols()

		monthData = from(bucket: "metrics_weekly")
		|> range(start: -1mo, stop: -1w)
		|> filter(fn: (r) => r["_measurement"] == "smart" )
		|> filter(fn: (r) => r["device_wwn"] == "0x5000c5002df89099" )
		|> tail(n: 10, offset: 0)
		|> schema.fieldsAsCols()

		yearData = from(bucket: "metrics_monthly")
		|> range(start: -1y, stop: -1mo)
		|> filter(fn: (r) => r["_measurement"] == "smart" )
		|> filter(fn: (r) => r["device_wwn"] == "0x5000c5002df89099" )
		|> tail(n: 10, offset: 0)
		|> schema.fieldsAsCols()

		foreverData = from(bucket: "metrics_yearly")
		|> range(start: -10y, stop: -1y)
		|> filter(fn: (r) => r["_measurement"] == "smart" )
		|> filter(fn: (r) => r["device_wwn"] == "0x5000c5002df89099" )
		|> tail(n: 10, offset: 0)
		|> schema.fieldsAsCols()

		union(tables: [weekData, monthData, yearData, foreverData])
		|> group()
		|> sort(columns: ["_time"], desc: true)
		|> tail(n: 6, offset: 4)
		|> yield(name: "last")

	*/

	partialQueryStr := []string{
		`import "influxdata/influxdb/schema"`,
	}

	nestedDurationKeys := sr.lookupNestedDurationKeys(durationKey)

	if len(nestedDurationKeys) == 1 {
		//there's only one bucket being queried, no need to union, just aggregate the dataset and return
		subqueryParts := []string{
			sr.generateSmartAttributesSubquery(historyFilter, nestedDurationKeys[0], 0, 0, attributes),
			fmt.Sprintf(`%sData`, nestedDurationKeys[0]),
			`|> sort(columns: ["_time"], desc: true)`,
		}
		if selectEntries > 0 {
			// Use limit() instead of tail() after desc sort to get the newest entries
			subqueryParts = append(subqueryParts, fmt.Sprintf(`|> limit(n: %d, offset: %d)`, selectEntries, selectEntriesOffset))
		}
		subqueryParts = append(subqueryParts, `|> yield()`)
		partialQueryStr = append(partialQueryStr, subqueryParts...)
		return strings.Join(partialQueryStr, "\n")
	}

	subQueries := []string{}
	subQueryNames := []string{}
	for _, nestedDurationKey := range nestedDurationKeys {
		subQueryNames = append(subQueryNames, fmt.Sprintf(`%sData`, nestedDurationKey))
		if selectEntries > 0 {
			// We only need the last `n + offset` # of entries from each table to guarantee we can
			// get the last `n` # of entries starting from `offset` of the union
			subQueries = append(subQueries, sr.generateSmartAttributesSubquery(historyFilter, nestedDurationKey, selectEntries+selectEntriesOffset, 0, attributes))
		} else {
			subQueries = append(subQueries, sr.generateSmartAttributesSubquery(historyFilter, nestedDurationKey, 0, 0, attributes))
		}
	}
	partialQueryStr = append(partialQueryStr, subQueries...)
	partialQueryStr = append(partialQueryStr, []string{
		fmt.Sprintf("union(tables: [%s])", strings.Join(subQueryNames, ", ")),
		`|> group()`,
		`|> sort(columns: ["_time"], desc: true)`,
	}...)
	if selectEntries > 0 {
		// Use limit() instead of tail() after desc sort to get the newest entries
		// tail() would get the oldest entries from the end, but we want the newest from the beginning
		partialQueryStr = append(partialQueryStr, fmt.Sprintf(`|> limit(n: %d, offset: %d)`, selectEntries, selectEntriesOffset))
	}
	partialQueryStr = append(partialQueryStr, `|> yield(name: "last")`)

	return strings.Join(partialQueryStr, "\n")
}

// generateSmartAttributesSubquery generates a subquery for SMART attributes.
// historyFilter is a Flux predicate built by deviceHistoryPredicate.
func (sr *scrutinyRepository) generateSmartAttributesSubquery(historyFilter string, durationKey string, selectEntries int, selectEntriesOffset int, attributes []string) string {
	bucketName := sr.lookupBucketName(durationKey)
	durationRange := sr.lookupDuration(durationKey)

	partialQueryStr := []string{
		fmt.Sprintf(`%sData = from(bucket: "%s")`, durationKey, bucketName),
		fmt.Sprintf(`|> range(start: %s, stop: %s)`, durationRange[0], durationRange[1]),
		`|> filter(fn: (r) => r["_measurement"] == "smart" )`,
		fmt.Sprintf(`|> filter(fn: (r) => %s )`, historyFilter),
	}

	partialQueryStr = append(partialQueryStr, fmt.Sprintf(`|> aggregateWindow(every: %s, fn: last, createEmpty: false)`, RESOLUTION_1_DAY))

	if selectEntries > 0 {
		partialQueryStr = append(partialQueryStr, fmt.Sprintf(`|> tail(n: %d, offset: %d)`, selectEntries, selectEntriesOffset))
	}
	partialQueryStr = append(partialQueryStr, "|> schema.fieldsAsCols()")

	return strings.Join(partialQueryStr, "\n")
}

// Self-test epochs require a trustworthy absolute upper bound. A warning or a
// counter decrease is evidence against treating the reported hours as absolute.
func selfTestPowerOnHours(current, previous *measurements.Smart) int64 {
	if previous != nil && previous.PowerOnHours > current.PowerOnHours {
		return 0
	}
	if attribute, ok := current.Attributes["9"].(*measurements.SmartAtaAttribute); ok && pkg.AttributeStatusHas(attribute.Status, pkg.AttributeStatusWarningScrutiny) {
		return 0
	}
	return current.PowerOnHours
}
