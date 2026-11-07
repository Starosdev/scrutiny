package reports

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// int64Ptr returns a pointer to v, for pointer-valued fields such as
// temperatures and alert thresholds.
func int64Ptr(v int64) *int64 {
	return &v
}

func TestFormatTextReport_HealthyDevices(t *testing.T) {
	report := &ReportData{
		GeneratedAt:   time.Date(2026, 2, 17, 8, 0, 0, 0, time.UTC),
		PeriodType:    "daily",
		TotalDevices:  3,
		PassedDevices: 3,
		Devices: []DeviceReport{
			{Name: "/dev/sda", Model: "WDC WD40EFRX", Status: 0, TempCurrent: int64Ptr(35), TempMin: 30, TempMax: 40, TempAvg: 35.0},
			{Name: "/dev/sdb", Model: "Samsung 860", Status: 0, TempCurrent: int64Ptr(32), TempMin: 28, TempMax: 36, TempAvg: 32.0},
			{Name: "/dev/nvme0", Model: "Samsung 970", Status: 0, TempCurrent: int64Ptr(45), TempMin: 40, TempMax: 50, TempAvg: 45.0},
		},
	}

	subject, message := FormatTextReport(report)

	assert.Contains(t, subject, "Daily Report")
	assert.Contains(t, subject, "All 3 drives healthy")
	assert.Contains(t, message, "3 total")
	assert.Contains(t, message, "3 passed")
	assert.NotContains(t, message, "WARNINGS")
	assert.NotContains(t, message, "FAILURES")
}

func TestFormatTextReport_WithFailures(t *testing.T) {
	report := &ReportData{
		GeneratedAt:    time.Date(2026, 2, 17, 8, 0, 0, 0, time.UTC),
		PeriodType:     "weekly",
		TotalDevices:   3,
		PassedDevices:  1,
		WarningDevices: 1,
		FailedDevices:  1,
		Devices: []DeviceReport{
			{Name: "/dev/sda", Model: "WDC WD40EFRX", Status: 0, TempCurrent: int64Ptr(35)},
			{
				Name: "/dev/sdb", Model: "Samsung 860", Status: 2, TempCurrent: int64Ptr(32),
				ActiveFailures: []AlertEntry{
					{AttributeID: "5", AttributeName: "Reallocated Sectors", Status: "failed", Value: 10, Threshold: int64Ptr(5), StatusReason: "scrutiny"},
				},
			},
			{
				Name: "/dev/nvme0", Model: "Samsung 970", Status: 1, TempCurrent: int64Ptr(45),
				ActiveFailures: []AlertEntry{
					{AttributeID: "percentage_used", AttributeName: "Percentage Used", Status: "warning", Value: 95, StatusReason: "smart"},
				},
			},
		},
	}

	subject, message := FormatTextReport(report)

	assert.Contains(t, subject, "Weekly Report")
	assert.Contains(t, subject, "1 failed")
	assert.Contains(t, message, "1 warning")
	assert.Contains(t, message, "FAILURES")
	assert.Contains(t, message, "Reallocated Sectors")
}

func TestFormatTextReport_WithZFS(t *testing.T) {
	report := &ReportData{
		GeneratedAt:   time.Date(2026, 2, 17, 8, 0, 0, 0, time.UTC),
		PeriodType:    "monthly",
		TotalDevices:  2,
		PassedDevices: 2,
		Devices:       []DeviceReport{},
		ZFSPools: []ZFSPoolReport{
			{Name: "tank", Health: "ONLINE", Capacity: 65.5},
			{Name: "backup", Health: "ONLINE", Presence: "missing", ErrorsRead: 5},
		},
	}

	_, message := FormatTextReport(report)
	assert.Contains(t, message, "ZFS Pools")
	assert.Contains(t, message, "tank")
	assert.Contains(t, message, "backup: MISSING")
}

func TestFormatTextReport_Truncation(t *testing.T) {
	devices := make([]DeviceReport, 50)
	for i := range devices {
		devices[i] = DeviceReport{
			Name: "/dev/sd" + string(rune('a'+i%26)),
			ActiveFailures: []AlertEntry{
				{AttributeID: "5", AttributeName: "Reallocated Sectors", Status: "failed", Value: 100},
			},
			Status: 1,
		}
	}
	report := &ReportData{
		GeneratedAt:   time.Date(2026, 2, 17, 8, 0, 0, 0, time.UTC),
		PeriodType:    "daily",
		TotalDevices:  50,
		FailedDevices: 50,
		Devices:       devices,
	}

	_, message := FormatTextReport(report)
	truncated := TruncateForNotification(message, 2000)
	require.LessOrEqual(t, len(truncated), 2000)
	assert.True(t, strings.HasSuffix(truncated, "..."))
}

func TestFormatTextReport_TemperatureSummary(t *testing.T) {
	report := &ReportData{
		GeneratedAt:   time.Date(2026, 2, 17, 8, 0, 0, 0, time.UTC),
		PeriodType:    "daily",
		TotalDevices:  2,
		PassedDevices: 2,
		Devices: []DeviceReport{
			{Name: "/dev/sda", TempCurrent: int64Ptr(52), TempAvg: 45.0, Status: 0},
			{Name: "/dev/nvme0", TempCurrent: int64Ptr(28), TempAvg: 31.0, Status: 0},
		},
	}

	_, message := FormatTextReport(report)
	assert.Contains(t, message, "Temperature Summary")
	assert.Contains(t, message, "52")
	assert.Contains(t, message, "28")
}

func TestFormatTextReport_ThresholdZeroVersusNone(t *testing.T) {
	report := &ReportData{
		GeneratedAt: time.Date(2026, 2, 17, 8, 0, 0, 0, time.UTC),
		PeriodType:  "daily",
		Devices: []DeviceReport{{
			Name: "/dev/sda", Model: "WDC",
			ActiveFailures: []AlertEntry{
				{AttributeID: "1", AttributeName: "Read Error Rate", Status: "failed", Value: 90, Threshold: int64Ptr(0)},
				{AttributeID: "194", AttributeName: "Temperature", Status: "failed", Value: 40, Threshold: nil},
			},
		}},
	}

	_, message := FormatTextReport(report)

	assert.Contains(t, message, "Attribute 1 (Read Error Rate) = 90 [threshold: 0]")
	assert.Contains(t, message, "Attribute 194 (Temperature) = 40")
	assert.NotContains(t, message, "Attribute 194 (Temperature) = 40 [threshold")
}
