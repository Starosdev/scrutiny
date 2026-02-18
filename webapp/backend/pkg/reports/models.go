package reports

import (
	"time"
)

// ReportData is the central data structure for a scheduled report
type ReportData struct {
	GeneratedAt time.Time `json:"generated_at"`
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
	PeriodType  string    `json:"period_type"` // "daily", "weekly", "monthly"

	TotalDevices    int `json:"total_devices"`
	PassedDevices   int `json:"passed_devices"`
	WarningDevices  int `json:"warning_devices"`
	FailedDevices   int `json:"failed_devices"`
	ArchivedDevices int `json:"archived_devices"`

	Devices  []DeviceReport  `json:"devices"`
	ZFSPools []ZFSPoolReport `json:"zfs_pools"`
}

// NewReportData creates a ReportData with the given period
func NewReportData(periodType string, start, end time.Time) *ReportData {
	return &ReportData{
		GeneratedAt: time.Now(),
		PeriodStart: start,
		PeriodEnd:   end,
		PeriodType:  periodType,
		Devices:     []DeviceReport{},
		ZFSPools:    []ZFSPoolReport{},
	}
}

func (r *ReportData) HasFailures() bool {
	return r.FailedDevices > 0
}

func (r *ReportData) HasWarnings() bool {
	return r.WarningDevices > 0
}

func (r *ReportData) OverallStatus() string {
	if r.FailedDevices > 0 {
		return "critical"
	}
	if r.WarningDevices > 0 {
		return "warning"
	}
	return "healthy"
}

// DeviceReport contains health data for a single device within a report period
type DeviceReport struct {
	WWN      string `json:"wwn"`
	Name     string `json:"name"`
	Model    string `json:"model"`
	Serial   string `json:"serial"`
	Protocol string `json:"protocol"` // ATA, NVMe, SCSI
	HostID   string `json:"host_id"`
	Label    string `json:"label"`
	Status   int    `json:"status"` // bitwise: 0=pass, 1=smart fail, 2=scrutiny fail, 3=both

	TempCurrent int64   `json:"temp_current"`
	TempMin     int64   `json:"temp_min"`
	TempMax     int64   `json:"temp_max"`
	TempAvg     float64 `json:"temp_avg"`

	PowerOnHours    int64 `json:"power_on_hours"`
	PowerCycleCount int64 `json:"power_cycle_count"`

	PercentageUsed *int64 `json:"percentage_used,omitempty"`
	WearoutValue   *int64 `json:"wearout_value,omitempty"`

	NewAlerts      []AlertEntry `json:"new_alerts"`
	ActiveFailures []AlertEntry `json:"active_failures"`

	Performance *PerformanceSummary `json:"performance,omitempty"`
}

func (d *DeviceReport) DisplayName() string {
	if d.Label != "" {
		return d.Label + " (" + d.Name + ")"
	}
	return d.Name
}

func (d *DeviceReport) StatusString() string {
	switch d.Status {
	case 0:
		return "passed"
	case 1:
		return "failed (smart)"
	case 2:
		return "failed (scrutiny)"
	case 3:
		return "failed (smart+scrutiny)"
	default:
		return "unknown"
	}
}

// AlertEntry represents a SMART attribute in warning or failure state
type AlertEntry struct {
	AttributeID   string `json:"attribute_id"`
	AttributeName string `json:"attribute_name"`
	Status        string `json:"status"` // "warning", "failed"
	Value         int64  `json:"value"`
	Threshold     int64  `json:"threshold"`
	StatusReason  string `json:"status_reason"` // "smart" or "scrutiny"
}

// PerformanceSummary contains benchmark results for a device
type PerformanceSummary struct {
	SeqReadBW         float64  `json:"seq_read_bw"`
	SeqWriteBW        float64  `json:"seq_write_bw"`
	RandReadIOPS      float64  `json:"rand_read_iops"`
	RandWriteIOPS     float64  `json:"rand_write_iops"`
	BaselineDeviation *float64 `json:"baseline_deviation,omitempty"`
}

// ZFSPoolReport contains health data for a ZFS pool
type ZFSPoolReport struct {
	Name           string     `json:"name"`
	GUID           string     `json:"guid"`
	Health         string     `json:"health"`
	Capacity       float64    `json:"capacity"`
	ErrorsRead     int64      `json:"errors_read"`
	ErrorsWrite    int64      `json:"errors_write"`
	ErrorsChecksum int64      `json:"errors_checksum"`
	ScrubStatus    string     `json:"scrub_status"`
	LastScrubDate  *time.Time `json:"last_scrub_date,omitempty"`
}
