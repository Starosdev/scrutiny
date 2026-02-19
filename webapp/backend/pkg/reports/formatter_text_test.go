package reports

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatTextReport_HealthyDevices(t *testing.T) {
	report := &ReportData{
		GeneratedAt:   time.Date(2026, 2, 17, 8, 0, 0, 0, time.UTC),
		PeriodType:    "daily",
		TotalDevices:  3,
		PassedDevices: 3,
		Devices: []DeviceReport{
			{Name: "/dev/sda", Model: "WDC WD40EFRX", Status: 0, TempCurrent: 35, TempMin: 30, TempMax: 40, TempAvg: 35.0},
			{Name: "/dev/sdb", Model: "Samsung 860", Status: 0, TempCurrent: 32, TempMin: 28, TempMax: 36, TempAvg: 32.0},
			{Name: "/dev/nvme0", Model: "Samsung 970", Status: 0, TempCurrent: 45, TempMin: 40, TempMax: 50, TempAvg: 45.0},
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
			{Name: "/dev/sda", Model: "WDC WD40EFRX", Status: 0, TempCurrent: 35},
			{
				Name: "/dev/sdb", Model: "Samsung 860", Status: 2, TempCurrent: 32,
				ActiveFailures: []AlertEntry{
					{AttributeID: "5", AttributeName: "Reallocated Sectors", Status: "failed", Value: 10, Threshold: 5, StatusReason: "scrutiny"},
				},
			},
			{
				Name: "/dev/nvme0", Model: "Samsung 970", Status: 1, TempCurrent: 45,
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
			{Name: "backup", Health: "DEGRADED", ErrorsRead: 5},
		},
	}

	_, message := FormatTextReport(report)
	assert.Contains(t, message, "ZFS Pools")
	assert.Contains(t, message, "tank")
	assert.Contains(t, message, "DEGRADED")
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
			{Name: "/dev/sda", TempCurrent: 52, TempAvg: 45.0, Status: 0},
			{Name: "/dev/nvme0", TempCurrent: 28, TempAvg: 31.0, Status: 0},
		},
	}

	_, message := FormatTextReport(report)
	assert.Contains(t, message, "Temperature Summary")
	assert.Contains(t, message, "52")
	assert.Contains(t, message, "28")
}
