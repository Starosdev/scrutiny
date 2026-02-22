package database

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
)

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Workload Insights
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func (sr *scrutinyRepository) GetWorkloadInsights(ctx context.Context, durationKey string) (map[string]*models.WorkloadInsight, error) {
	devices, err := sr.GetDevices(ctx)
	if err != nil {
		return nil, err
	}

	insights := map[string]*models.WorkloadInsight{}
	deviceProtocols := map[string]string{}
	for _, device := range devices {
		if device.Archived {
			continue
		}
		insights[device.WWN] = &models.WorkloadInsight{
			DeviceWWN:      device.WWN,
			DeviceProtocol: device.DeviceProtocol,
			Intensity:      "unknown",
		}
		deviceProtocols[device.WWN] = device.DeviceProtocol
	}

	if len(insights) == 0 {
		return insights, nil
	}

	// Query 1: first and last data points for rate computation
	firstPoints, lastPoints, err := sr.queryWorkloadFirstLast(ctx, durationKey)
	if err != nil {
		sr.logger.Errorf("Error querying workload first/last points: %v", err)
		return insights, nil
	}

	// Query 2: recent points for spike detection (raw bucket only)
	recentPoints, err := sr.queryWorkloadRecent(ctx)
	if err != nil {
		sr.logger.Errorf("Error querying workload recent points: %v", err)
		// Non-fatal: continue without spike detection
	}

	// Compute insights per device
	for wwn, insight := range insights {
		first, hasFirst := firstPoints[wwn]
		last, hasLast := lastPoints[wwn]

		if !hasFirst || !hasLast {
			continue
		}

		sr.computeWorkloadInsight(insight, first, last, deviceProtocols[wwn])

		// Spike detection
		if recent, ok := recentPoints[wwn]; ok && len(recent) >= 2 {
			spike := sr.detectSpike(recent, insight.DailyWriteBytes, deviceProtocols[wwn])
			if spike != nil {
				insight.Spike = spike
			}
		}
	}

	return insights, nil
}

// workloadSnapshot holds extracted field values from a single InfluxDB data point
type workloadSnapshot struct {
	Time             time.Time
	PowerOnHours     int64
	LogicalBlockSize int64

	// ATA
	Attr241RawValue    int64 // Total LBAs Written
	Attr242RawValue    int64 // Total LBAs Read
	Devstat124Value    int64 // Logical Sectors Written
	Devstat140Value    int64 // Logical Sectors Read
	Devstat78Value     int64 // Percentage Used Endurance
	Attr177Value       int64 // Wearout (Samsung/Crucial)
	Attr231Value       int64 // Life Left
	Attr232Value       int64 // Endurance Remaining
	Attr233Value       int64 // Wearout (Intel)

	// NVMe
	DataUnitsWritten   int64
	DataUnitsRead      int64
	PercentageUsed     int64

	// Track which fields were present
	hasAttr241         bool
	hasAttr242         bool
	hasDevstat124      bool
	hasDevstat140      bool
	hasDevstat78       bool
	hasAttr177         bool
	hasAttr231         bool
	hasAttr232         bool
	hasAttr233         bool
	hasDataUnitsW      bool
	hasDataUnitsR      bool
	hasPercentageUsed  bool
}

func parseWorkloadSnapshot(values map[string]interface{}) *workloadSnapshot {
	snap := &workloadSnapshot{}

	if v, ok := values["_time"]; ok && v != nil {
		if t, ok := v.(time.Time); ok {
			snap.Time = t
		}
	}
	if v, ok := values["power_on_hours"]; ok && v != nil {
		if intVal, ok := v.(int64); ok {
			snap.PowerOnHours = intVal
		}
	}
	if v, ok := values["logical_block_size"]; ok && v != nil {
		switch val := v.(type) {
		case int64:
			snap.LogicalBlockSize = val
		case float64:
			snap.LogicalBlockSize = int64(val)
		}
	}
	if snap.LogicalBlockSize == 0 {
		snap.LogicalBlockSize = 512 // default
	}

	// ATA attributes
	if v, ok := values["attr.241.raw_value"]; ok && v != nil {
		if intVal, ok := v.(int64); ok {
			snap.Attr241RawValue = intVal
			snap.hasAttr241 = true
		}
	}
	if v, ok := values["attr.242.raw_value"]; ok && v != nil {
		if intVal, ok := v.(int64); ok {
			snap.Attr242RawValue = intVal
			snap.hasAttr242 = true
		}
	}
	if v, ok := values["attr.devstat_1_24.value"]; ok && v != nil {
		if intVal, ok := v.(int64); ok {
			snap.Devstat124Value = intVal
			snap.hasDevstat124 = true
		}
	}
	if v, ok := values["attr.devstat_1_40.value"]; ok && v != nil {
		if intVal, ok := v.(int64); ok {
			snap.Devstat140Value = intVal
			snap.hasDevstat140 = true
		}
	}
	if v, ok := values["attr.devstat_7_8.value"]; ok && v != nil {
		if intVal, ok := v.(int64); ok {
			snap.Devstat78Value = intVal
			snap.hasDevstat78 = true
		}
	}
	for _, attrInfo := range []struct {
		field string
		dest  *int64
		flag  *bool
	}{
		{"attr.177.value", &snap.Attr177Value, &snap.hasAttr177},
		{"attr.231.value", &snap.Attr231Value, &snap.hasAttr231},
		{"attr.232.value", &snap.Attr232Value, &snap.hasAttr232},
		{"attr.233.value", &snap.Attr233Value, &snap.hasAttr233},
	} {
		if v, ok := values[attrInfo.field]; ok && v != nil {
			if intVal, ok := v.(int64); ok {
				*attrInfo.dest = intVal
				*attrInfo.flag = true
			}
		}
	}

	// NVMe attributes
	if v, ok := values["attr.data_units_written.value"]; ok && v != nil {
		if intVal, ok := v.(int64); ok {
			snap.DataUnitsWritten = intVal
			snap.hasDataUnitsW = true
		}
	}
	if v, ok := values["attr.data_units_read.value"]; ok && v != nil {
		if intVal, ok := v.(int64); ok {
			snap.DataUnitsRead = intVal
			snap.hasDataUnitsR = true
		}
	}
	if v, ok := values["attr.percentage_used.value"]; ok && v != nil {
		if intVal, ok := v.(int64); ok {
			snap.PercentageUsed = intVal
			snap.hasPercentageUsed = true
		}
	}

	return snap
}

func (sr *scrutinyRepository) queryWorkloadFirstLast(ctx context.Context, durationKey string) (
	firstPoints map[string]*workloadSnapshot,
	lastPoints map[string]*workloadSnapshot,
	err error,
) {
	firstPoints = map[string]*workloadSnapshot{}
	lastPoints = map[string]*workloadSnapshot{}

	queryStr := sr.buildWorkloadFirstLastQuery(durationKey)
	sr.logger.Debugln("Workload first/last query:", queryStr)

	result, err := sr.influxQueryApi.Query(ctx, queryStr)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query workload data: %w", err)
	}

	for result.Next() {
		values := result.Record().Values()
		deviceWWN, ok := values["device_wwn"]
		if !ok || deviceWWN == nil {
			continue
		}
		wwn := deviceWWN.(string)

		snap := parseWorkloadSnapshot(values)

		// Determine if this is a "first" or "last" result based on the yield name
		resultName := result.TableMetadata().Column(0).Name()
		if result.Record().Result() == "first" {
			firstPoints[wwn] = snap
		} else {
			// "last" result or default
			lastPoints[wwn] = snap
		}
		_ = resultName
	}
	if result.Err() != nil {
		return nil, nil, fmt.Errorf("query iteration error: %w", result.Err())
	}

	return firstPoints, lastPoints, nil
}

func (sr *scrutinyRepository) buildWorkloadFirstLastQuery(durationKey string) string {
	bucketBaseName := sr.appConfig.GetString("web.influxdb.bucket")

	partialQueryStr := []string{
		`import "influxdata/influxdb/schema"`,
		``,
		`workloadFields = (r) =>`,
		`    r["_field"] == "power_on_hours" or`,
		`    r["_field"] == "logical_block_size" or`,
		`    r["_field"] == "attr.241.raw_value" or`,
		`    r["_field"] == "attr.242.raw_value" or`,
		`    r["_field"] == "attr.devstat_1_24.value" or`,
		`    r["_field"] == "attr.devstat_1_40.value" or`,
		`    r["_field"] == "attr.data_units_written.value" or`,
		`    r["_field"] == "attr.data_units_read.value" or`,
		`    r["_field"] == "attr.percentage_used.value" or`,
		`    r["_field"] == "attr.devstat_7_8.value" or`,
		`    r["_field"] == "attr.177.value" or`,
		`    r["_field"] == "attr.231.value" or`,
		`    r["_field"] == "attr.232.value" or`,
		`    r["_field"] == "attr.233.value"`,
		``,
	}

	nestedDurationKeys := sr.lookupNestedDurationKeys(durationKey)
	subQueryNames := []string{}

	for _, nestedDurationKey := range nestedDurationKeys {
		bucketName := sr.lookupBucketName(nestedDurationKey)
		durationRange := sr.lookupDuration(nestedDurationKey)
		subQueryName := fmt.Sprintf(`%sData`, nestedDurationKey)
		subQueryNames = append(subQueryNames, subQueryName)

		partialQueryStr = append(partialQueryStr, []string{
			fmt.Sprintf(`%s = from(bucket: "%s")`, subQueryName, bucketName),
			fmt.Sprintf(`|> range(start: %s, stop: %s)`, durationRange[0], durationRange[1]),
			`|> filter(fn: (r) => r["_measurement"] == "smart")`,
			`|> filter(fn: workloadFields)`,
			``,
		}...)
	}

	var combinedExpr string
	if len(subQueryNames) == 1 {
		combinedExpr = subQueryNames[0]
	} else {
		combinedExpr = fmt.Sprintf("union(tables: [%s])", strings.Join(subQueryNames, ", "))
	}

	partialQueryStr = append(partialQueryStr, []string{
		fmt.Sprintf(`combined = %s`, combinedExpr),
		`|> schema.fieldsAsCols()`,
		`|> group(columns: ["device_wwn"])`,
		``,
		`combined`,
		`|> sort(columns: ["_time"], desc: false)`,
		`|> limit(n: 1)`,
		`|> yield(name: "first")`,
		``,
		`combined`,
		`|> sort(columns: ["_time"], desc: true)`,
		`|> limit(n: 1)`,
		`|> yield(name: "last")`,
	}...)

	_ = bucketBaseName
	return strings.Join(partialQueryStr, "\n")
}

func (sr *scrutinyRepository) queryWorkloadRecent(ctx context.Context) (map[string][]*workloadSnapshot, error) {
	recentPoints := map[string][]*workloadSnapshot{}

	queryStr := sr.buildWorkloadRecentQuery()
	sr.logger.Debugln("Workload recent query:", queryStr)

	result, err := sr.influxQueryApi.Query(ctx, queryStr)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent workload data: %w", err)
	}

	for result.Next() {
		values := result.Record().Values()
		deviceWWN, ok := values["device_wwn"]
		if !ok || deviceWWN == nil {
			continue
		}
		wwn := deviceWWN.(string)
		snap := parseWorkloadSnapshot(values)
		recentPoints[wwn] = append(recentPoints[wwn], snap)
	}
	if result.Err() != nil {
		return nil, fmt.Errorf("query iteration error: %w", result.Err())
	}

	return recentPoints, nil
}

func (sr *scrutinyRepository) buildWorkloadRecentQuery() string {
	bucketName := sr.appConfig.GetString("web.influxdb.bucket")

	return strings.Join([]string{
		`import "influxdata/influxdb/schema"`,
		``,
		`workloadFields = (r) =>`,
		`    r["_field"] == "power_on_hours" or`,
		`    r["_field"] == "logical_block_size" or`,
		`    r["_field"] == "attr.241.raw_value" or`,
		`    r["_field"] == "attr.242.raw_value" or`,
		`    r["_field"] == "attr.devstat_1_24.value" or`,
		`    r["_field"] == "attr.devstat_1_40.value" or`,
		`    r["_field"] == "attr.data_units_written.value" or`,
		`    r["_field"] == "attr.data_units_read.value"`,
		``,
		fmt.Sprintf(`from(bucket: "%s")`, bucketName),
		`|> range(start: -1w, stop: now())`,
		`|> filter(fn: (r) => r["_measurement"] == "smart")`,
		`|> filter(fn: workloadFields)`,
		`|> schema.fieldsAsCols()`,
		`|> group(columns: ["device_wwn"])`,
		`|> sort(columns: ["_time"], desc: true)`,
		`|> limit(n: 3)`,
	}, "\n")
}

func (sr *scrutinyRepository) computeWorkloadInsight(insight *models.WorkloadInsight, first, last *workloadSnapshot, protocol string) {
	timeSpan := last.Time.Sub(first.Time)
	timeSpanHours := timeSpan.Hours()
	insight.TimeSpanHours = timeSpanHours
	insight.DataPoints = 2

	if timeSpanHours < 1 {
		insight.Intensity = "unknown"
		sr.computeEndurance(insight, last, protocol, 0)
		return
	}

	timeSpanDays := timeSpanHours / 24.0

	var totalWrittenBytes, totalReadBytes int64

	switch protocol {
	case pkg.DeviceProtocolAta:
		totalWrittenBytes, totalReadBytes = sr.computeATAWorkload(first, last)
	case pkg.DeviceProtocolNvme:
		totalWrittenBytes, totalReadBytes = sr.computeNVMeWorkload(first, last)
	default:
		// SCSI: no cumulative byte counters
		insight.Intensity = "unknown"
		sr.computeEndurance(insight, last, protocol, 0)
		return
	}

	// Handle counter wraparound or reset
	if totalWrittenBytes < 0 {
		totalWrittenBytes = 0
	}
	if totalReadBytes < 0 {
		totalReadBytes = 0
	}

	insight.TotalWriteBytes = totalWrittenBytes
	insight.TotalReadBytes = totalReadBytes
	insight.DailyWriteBytes = int64(float64(totalWrittenBytes) / timeSpanDays)
	insight.DailyReadBytes = int64(float64(totalReadBytes) / timeSpanDays)

	if insight.DailyWriteBytes > 0 {
		insight.ReadWriteRatio = float64(insight.DailyReadBytes) / float64(insight.DailyWriteBytes)
		insight.ReadWriteRatio = math.Round(insight.ReadWriteRatio*100) / 100
	}

	insight.Intensity = classifyIntensity(insight.DailyWriteBytes + insight.DailyReadBytes)

	// Compute cumulative bytes for endurance TBW calculation
	cumulativeWriteBytes := sr.getCumulativeWriteBytes(last, protocol)
	sr.computeEndurance(insight, last, protocol, cumulativeWriteBytes)
}

func (sr *scrutinyRepository) computeATAWorkload(first, last *workloadSnapshot) (writtenBytes, readBytes int64) {
	blockSize := last.LogicalBlockSize

	// Prefer attr 241/242 (Total LBAs Written/Read)
	if last.hasAttr241 && first.hasAttr241 {
		writtenBytes = (last.Attr241RawValue - first.Attr241RawValue) * blockSize
	} else if last.hasDevstat124 && first.hasDevstat124 {
		// Fallback to device statistics: Logical Sectors Written
		writtenBytes = (last.Devstat124Value - first.Devstat124Value) * blockSize
	}

	if last.hasAttr242 && first.hasAttr242 {
		readBytes = (last.Attr242RawValue - first.Attr242RawValue) * blockSize
	} else if last.hasDevstat140 && first.hasDevstat140 {
		readBytes = (last.Devstat140Value - first.Devstat140Value) * blockSize
	}

	return writtenBytes, readBytes
}

func (sr *scrutinyRepository) computeNVMeWorkload(first, last *workloadSnapshot) (writtenBytes, readBytes int64) {
	// NVMe data units are in units of 1000 x 512 bytes = 512,000 bytes per unit
	const nvmeUnitBytes int64 = 512000

	if last.hasDataUnitsW && first.hasDataUnitsW {
		writtenBytes = (last.DataUnitsWritten - first.DataUnitsWritten) * nvmeUnitBytes
	}
	if last.hasDataUnitsR && first.hasDataUnitsR {
		readBytes = (last.DataUnitsRead - first.DataUnitsRead) * nvmeUnitBytes
	}

	return writtenBytes, readBytes
}

func (sr *scrutinyRepository) getCumulativeWriteBytes(snap *workloadSnapshot, protocol string) int64 {
	switch protocol {
	case pkg.DeviceProtocolAta:
		if snap.hasAttr241 {
			return snap.Attr241RawValue * snap.LogicalBlockSize
		}
		if snap.hasDevstat124 {
			return snap.Devstat124Value * snap.LogicalBlockSize
		}
	case pkg.DeviceProtocolNvme:
		if snap.hasDataUnitsW {
			return snap.DataUnitsWritten * 512000
		}
	}
	return 0
}

func classifyIntensity(dailyTotalBytes int64) string {
	dailyGB := float64(dailyTotalBytes) / (1024 * 1024 * 1024)
	switch {
	case dailyGB < 1:
		return "idle"
	case dailyGB < 20:
		return "light"
	case dailyGB < 100:
		return "medium"
	default:
		return "heavy"
	}
}

func (sr *scrutinyRepository) computeEndurance(insight *models.WorkloadInsight, snap *workloadSnapshot, protocol string, cumulativeWriteBytes int64) {
	var percentageUsed int64
	var hasPercentage bool

	switch protocol {
	case pkg.DeviceProtocolNvme:
		if snap.hasPercentageUsed {
			percentageUsed = snap.PercentageUsed
			hasPercentage = true
		}
	case pkg.DeviceProtocolAta:
		if snap.hasDevstat78 {
			percentageUsed = snap.Devstat78Value
			hasPercentage = true
		} else {
			// Check wearout attributes (higher = healthier, invert to get percentage used)
			for _, info := range []struct {
				has   bool
				value int64
			}{
				{snap.hasAttr177, snap.Attr177Value},
				{snap.hasAttr233, snap.Attr233Value},
				{snap.hasAttr231, snap.Attr231Value},
				{snap.hasAttr232, snap.Attr232Value},
			} {
				if info.has && info.value > 0 {
					percentageUsed = 100 - info.value
					if percentageUsed < 0 {
						percentageUsed = 0
					}
					hasPercentage = true
					break
				}
			}
		}
	}

	if !hasPercentage {
		return
	}

	estimate := &models.EnduranceEstimate{
		Available:      true,
		PercentageUsed: percentageUsed,
	}

	if cumulativeWriteBytes > 0 {
		estimate.TBWrittenSoFar = float64(cumulativeWriteBytes) / (1024 * 1024 * 1024 * 1024)
		estimate.TBWrittenSoFar = math.Round(estimate.TBWrittenSoFar*100) / 100
	}

	if percentageUsed > 0 && snap.PowerOnHours > 0 {
		totalLifespanHours := float64(snap.PowerOnHours) / (float64(percentageUsed) / 100.0)
		remainingHours := totalLifespanHours - float64(snap.PowerOnHours)
		if remainingHours > 0 {
			estimate.EstimatedLifespanDays = int64(remainingHours / 24)
		}
	}

	insight.Endurance = estimate
}

func (sr *scrutinyRepository) detectSpike(recentPoints []*workloadSnapshot, baselineDailyWriteBytes int64, protocol string) *models.ActivitySpike {
	if len(recentPoints) < 2 || baselineDailyWriteBytes <= 0 {
		return nil
	}

	// recentPoints are sorted desc by time (newest first)
	newest := recentPoints[0]
	previous := recentPoints[1]

	elapsed := newest.Time.Sub(previous.Time).Hours()
	if elapsed < 0.5 {
		return nil
	}

	var recentWrittenBytes int64
	switch protocol {
	case pkg.DeviceProtocolAta:
		if newest.hasAttr241 && previous.hasAttr241 {
			delta := newest.Attr241RawValue - previous.Attr241RawValue
			recentWrittenBytes = delta * newest.LogicalBlockSize
		} else if newest.hasDevstat124 && previous.hasDevstat124 {
			delta := newest.Devstat124Value - previous.Devstat124Value
			recentWrittenBytes = delta * newest.LogicalBlockSize
		}
	case pkg.DeviceProtocolNvme:
		if newest.hasDataUnitsW && previous.hasDataUnitsW {
			delta := newest.DataUnitsWritten - previous.DataUnitsWritten
			recentWrittenBytes = delta * 512000
		}
	default:
		return nil
	}

	if recentWrittenBytes <= 0 {
		return nil
	}

	recentDailyWriteBytes := int64(float64(recentWrittenBytes) / (elapsed / 24.0))
	spikeFactor := float64(recentDailyWriteBytes) / float64(baselineDailyWriteBytes)

	if spikeFactor > 3.0 {
		return &models.ActivitySpike{
			Detected:                true,
			RecentDailyWriteBytes:   recentDailyWriteBytes,
			BaselineDailyWriteBytes: baselineDailyWriteBytes,
			SpikeFactor:             math.Round(spikeFactor*10) / 10,
			Description:             fmt.Sprintf("Write rate is %.1fx above baseline", spikeFactor),
		}
	}

	return nil
}
