package reports

import (
	"context"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
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
	for wwn, summary := range summaries {
		if summary.Device.Archived {
			archivedCount++
			continue
		}

		deviceReport := buildDeviceReport(summary, tempHistory[wwn])

		// Populate active failures from latest SMART attribute data
		g.populateAlerts(ctx, &deviceReport, wwn, durationKey)

		report.Devices = append(report.Devices, deviceReport)

		report.TotalDevices++
		if summary.Device.DeviceStatus == pkg.DeviceStatusPassed {
			report.PassedDevices++
		} else if pkg.DeviceStatusHas(summary.Device.DeviceStatus, pkg.DeviceStatusFailedSmart) || pkg.DeviceStatusHas(summary.Device.DeviceStatus, pkg.DeviceStatusFailedScrutiny) {
			report.FailedDevices++
		} else {
			report.PassedDevices++
		}
	}

	report.ArchivedDevices = archivedCount
	report.WarningDevices = report.TotalDevices - report.PassedDevices - report.FailedDevices

	// Populate ZFS pool data
	g.populateZFSPools(ctx, report)

	return report, nil
}

func (g *Generator) populateAlerts(ctx context.Context, dr *DeviceReport, wwn string, durationKey string) {
	smartHistory, err := g.repo.GetSmartAttributeHistory(ctx, wwn, durationKey, 1, 0, nil)
	if err != nil || len(smartHistory) == 0 {
		return
	}

	latest := smartHistory[0]
	for attrID, attr := range latest.Attributes {
		status := attr.GetStatus()
		if status == pkg.AttributeStatusPassed {
			continue
		}

		entry := AlertEntry{
			AttributeID: attrID,
			Value:       attr.GetTransformedValue(),
		}

		if pkg.AttributeStatusHas(status, pkg.AttributeStatusFailedSmart) {
			entry.Status = "failed"
			entry.StatusReason = "smart"
			dr.ActiveFailures = append(dr.ActiveFailures, entry)
		} else if pkg.AttributeStatusHas(status, pkg.AttributeStatusFailedScrutiny) {
			entry.Status = "failed"
			entry.StatusReason = "scrutiny"
			dr.ActiveFailures = append(dr.ActiveFailures, entry)
		} else if pkg.AttributeStatusHas(status, pkg.AttributeStatusWarningScrutiny) {
			entry.Status = "warning"
			entry.StatusReason = "scrutiny"
			dr.ActiveFailures = append(dr.ActiveFailures, entry)
		}
	}
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
		dr.TempCurrent = summary.SmartResults.Temp
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
	} else {
		dr.TempMin = dr.TempCurrent
		dr.TempMax = dr.TempCurrent
		dr.TempAvg = float64(dr.TempCurrent)
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
