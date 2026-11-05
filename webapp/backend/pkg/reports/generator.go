package reports

import (
	"context"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	"github.com/analogj/scrutiny/webapp/backend/pkg/thresholds"
)

// SummaryProvider is the subset of database.DeviceRepo needed by the generator.
type SummaryProvider interface {
	GetSummary(ctx context.Context) (map[string]*models.DeviceSummary, error)
	GetSmartTemperatureHistory(ctx context.Context, durationKey string) (map[string][]measurements.SmartTemperature, error)
	GetSmartAttributeHistory(ctx context.Context, wwn string, durationKey string, selectEntries int, selectEntriesOffset int, attributes []string) ([]measurements.Smart, error)
	GetZFSPoolsSummary(ctx context.Context) (map[string]*models.ZFSPool, error)
}

// Generator builds ReportData by querying the database
type Generator struct {
	repo SummaryProvider
}

// NewGenerator creates a report generator
func NewGenerator(repo SummaryProvider) *Generator {
	return &Generator{repo: repo}
}

// Generate builds a complete ReportData for the given period
func (g *Generator) Generate(ctx context.Context, periodType string, start, end time.Time) (*ReportData, error) {
	report := NewReportData(periodType, start, end)

	summaries, err := g.repo.GetSummary(ctx)
	if err != nil {
		return nil, err
	}

	durationKey := periodToDurationKey(periodType)
	tempHistory, err := g.repo.GetSmartTemperatureHistory(ctx, durationKey)
	if err != nil {
		return nil, err
	}

	archivedCount := 0
	for devID, summary := range summaries {
		if summary.Device.Archived {
			archivedCount++
			continue
		}

		deviceReport := buildDeviceReport(summary, tempHistory[devID])

		// Populate active failures from latest SMART attribute data
		g.populateAlerts(ctx, &deviceReport, summary.Device.DeviceID, durationKey)

		report.Devices = append(report.Devices, deviceReport)

		hasWarning := false
		for _, failure := range deviceReport.ActiveFailures {
			if failure.Status == "warning" {
				hasWarning = true
				break
			}
		}

		report.TotalDevices++
		switch {
		case pkg.DeviceStatusHas(summary.Device.DeviceStatus, pkg.DeviceStatusFailedSmart) || pkg.DeviceStatusHas(summary.Device.DeviceStatus, pkg.DeviceStatusFailedScrutiny):
			report.FailedDevices++
		case hasWarning:
			report.WarningDevices++
		default:
			report.PassedDevices++
		}
	}

	report.ArchivedDevices = archivedCount

	// Populate ZFS pool data
	g.populateZFSPools(ctx, report)

	return report, nil
}

func (g *Generator) populateAlerts(ctx context.Context, dr *DeviceReport, deviceID string, durationKey string) {
	smartHistory, err := g.repo.GetSmartAttributeHistory(ctx, deviceID, durationKey, 1, 0, nil)
	if err != nil || len(smartHistory) == 0 {
		return
	}

	latest := smartHistory[0]
	for attrID, attr := range latest.Attributes {
		status := attr.GetStatus()
		if status == pkg.AttributeStatusPassed {
			continue
		}

		name, value, threshold := alertAttributeDetails(attrID, attr)
		entry := AlertEntry{
			AttributeID:   attrID,
			AttributeName: name,
			Value:         value,
			Threshold:     threshold,
		}

		switch {
		case pkg.AttributeStatusHas(status, pkg.AttributeStatusFailedSmart):
			entry.Status = "failed"
			entry.StatusReason = "smart"
			dr.ActiveFailures = append(dr.ActiveFailures, entry)
		case pkg.AttributeStatusHas(status, pkg.AttributeStatusFailedScrutiny):
			entry.Status = "failed"
			entry.StatusReason = "scrutiny"
			dr.ActiveFailures = append(dr.ActiveFailures, entry)
		case pkg.AttributeStatusHas(status, pkg.AttributeStatusWarningScrutiny):
			entry.Status = "warning"
			entry.StatusReason = "scrutiny"
			dr.ActiveFailures = append(dr.ActiveFailures, entry)
		}
	}
}

// alertAttributeDetails returns the display name, the value the attribute is evaluated on and the
// manufacturer threshold for one SMART attribute. It follows the detail page: ATA attributes use the
// metadata display name and the value the status is evaluated on (normalized, transformed or raw), and show the threshold
// only for normalized values or attributes without metadata. Device-statistics and FARM attributes
// prefer the metadata threshold over the drive-reported one.
//
// The threshold is 0 when there is nothing to show. NVMe and SCSI attributes store -1 for "no
// threshold"; it is mapped to 0 here so every formatter (text, HTML, PDF) sees the same thing.
//
// ATA value rule: when the attribute has metadata, the value is the one the status is evaluated on.
// That choice belongs to measurements.AttributeEvaluatedValue. The rule kept here is only for an ATA
// attribute with no metadata: it reports the normalized Value, as the detail page does, whereas the
// helper would fall through to RawValue. ATA attributes 174, 204, 227 and 228 have metadata without a
// display type, so they report RawValue; that is deliberate, because an override pins the raw
// value and the status reason refers to it.
func alertAttributeDetails(attrID string, attribute measurements.SmartAttribute) (name string, value int64, threshold int64) {
	switch attr := attribute.(type) {
	case *measurements.SmartAtaAttribute:
		name, value, threshold = attr.Name, int64(attr.Value), attr.Threshold
		if metadata, ok := thresholds.AtaMetadata[attr.AttributeId]; ok {
			name = metadata.DisplayName
			value, _ = measurements.AttributeEvaluatedValue(attr)
			if metadata.DisplayType != thresholds.AtaSmartAttributeDisplayTypeNormalized {
				threshold = 0
			}
		}
	case *measurements.SmartAtaDeviceStatAttribute:
		name, value, threshold = attr.AttributeId, attr.Value, attr.Threshold
		if metadata, ok := thresholds.AtaDeviceStatsMetadata[attr.AttributeId]; ok {
			name = metadata.DisplayName
			if metadata.Threshold != 0 {
				threshold = metadata.Threshold
			}
		}
	case *measurements.SmartFarmAttribute:
		name, value, threshold = attr.AttributeId, attr.Value, attr.Threshold
		if metadata, ok := thresholds.FarmMetadata[attr.AttributeId]; ok {
			name = metadata.DisplayName
			if metadata.Threshold != 0 {
				threshold = metadata.Threshold
			}
		}
	case *measurements.SmartNvmeAttribute:
		name, value, threshold = attr.AttributeId, attr.Value, attr.Threshold
		if metadata, ok := thresholds.NmveMetadata[attr.AttributeId]; ok {
			name = metadata.DisplayName
		}
	case *measurements.SmartScsiAttribute:
		name, value, threshold = attr.AttributeId, attr.Value, attr.Threshold
		if metadata, ok := thresholds.ScsiMetadataFor(attr.AttributeId); ok {
			name = metadata.DisplayName
		}
	}
	if threshold < 0 {
		threshold = 0
	}
	if name == "" {
		name = attrID
	}
	return name, value, threshold
}

func (g *Generator) populateZFSPools(ctx context.Context, report *ReportData) {
	poolsSummary, err := g.repo.GetZFSPoolsSummary(ctx)
	if err != nil {
		return
	}

	for _, pool := range poolsSummary {
		if pool == nil {
			continue
		}

		poolReport := ZFSPoolReport{
			Name:           pool.Name,
			GUID:           pool.GUID,
			Health:         pool.Health,
			Presence:       string(pool.Presence),
			Capacity:       pool.CapacityPercent,
			ErrorsRead:     pool.TotalReadErrors,
			ErrorsWrite:    pool.TotalWriteErrors,
			ErrorsChecksum: pool.TotalChecksumErrors,
		}

		if pool.ScrubState != "" {
			poolReport.ScrubStatus = string(pool.ScrubState)
		}
		if pool.ScrubEndTime != nil {
			t := *pool.ScrubEndTime
			poolReport.LastScrubDate = &t
		}

		report.ZFSPools = append(report.ZFSPools, poolReport)
	}
}

func buildDeviceReport(summary *models.DeviceSummary, temps []measurements.SmartTemperature) DeviceReport {
	dr := DeviceReport{
		WWN:      summary.Device.WWN,
		Name:     summary.Device.DeviceName,
		Model:    summary.Device.ModelName,
		Serial:   summary.Device.SerialNumber,
		Protocol: summary.Device.DeviceProtocol,
		HostID:   summary.Device.HostId,
		Label:    summary.Device.Label,
		Status:   int(summary.Device.DeviceStatus),

		NewAlerts:      []AlertEntry{},
		ActiveFailures: []AlertEntry{},
	}

	if summary.SmartResults != nil {
		if summary.SmartResults.Temp != nil {
			val := *summary.SmartResults.Temp
			dr.TempCurrent = &val
		}
		dr.PowerOnHours = summary.SmartResults.PowerOnHours

		if summary.SmartResults.PercentageUsed != nil {
			val := *summary.SmartResults.PercentageUsed
			dr.PercentageUsed = &val
		}
		if summary.SmartResults.WearoutValue != nil {
			val := *summary.SmartResults.WearoutValue
			dr.WearoutValue = &val
		}
	}

	if len(temps) > 0 {
		dr.TempMin, dr.TempMax, dr.TempAvg = aggregateTemps(temps)
	} else if dr.TempCurrent != nil {
		dr.TempMin = *dr.TempCurrent
		dr.TempMax = *dr.TempCurrent
		dr.TempAvg = float64(*dr.TempCurrent)
	}

	return dr
}

func aggregateTemps(temps []measurements.SmartTemperature) (min, max int64, avg float64) {
	if len(temps) == 0 {
		return 0, 0, 0
	}

	min = temps[0].Temp
	max = temps[0].Temp
	var sum int64

	for _, t := range temps {
		if t.Temp < min {
			min = t.Temp
		}
		if t.Temp > max {
			max = t.Temp
		}
		sum += t.Temp
	}

	avg = float64(sum) / float64(len(temps))
	return min, max, avg
}

func periodToDurationKey(periodType string) string {
	switch periodType {
	case "daily":
		return "day"
	case "weekly":
		return "week"
	case "monthly":
		return "month"
	default:
		return "week"
	}
}
