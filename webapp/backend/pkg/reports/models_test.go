package reports

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestReportData_HasFailures(t *testing.T) {
	report := &ReportData{
		FailedDevices:  1,
		WarningDevices: 0,
	}
	assert.True(t, report.HasFailures())

	report2 := &ReportData{
		FailedDevices:  0,
		WarningDevices: 0,
	}
	assert.False(t, report2.HasFailures())
}

func TestReportData_HasWarnings(t *testing.T) {
	report := &ReportData{
		WarningDevices: 2,
	}
	assert.True(t, report.HasWarnings())
}

func TestDeviceReport_DisplayName(t *testing.T) {
	dr := DeviceReport{Name: "/dev/sda", Label: "Main Drive"}
	assert.Equal(t, "Main Drive (/dev/sda)", dr.DisplayName())

	dr2 := DeviceReport{Name: "/dev/sdb", Label: ""}
	assert.Equal(t, "/dev/sdb", dr2.DisplayName())
}

func TestDeviceReport_StatusString(t *testing.T) {
	dr := DeviceReport{Status: 0}
	assert.Equal(t, "passed", dr.StatusString())

	dr2 := DeviceReport{Status: 1}
	assert.Equal(t, "failed (smart)", dr2.StatusString())

	dr3 := DeviceReport{Status: 2}
	assert.Equal(t, "failed (scrutiny)", dr3.StatusString())

	dr4 := DeviceReport{Status: 3}
	assert.Equal(t, "failed (smart+scrutiny)", dr4.StatusString())
}

func TestReportData_OverallStatus(t *testing.T) {
	healthy := &ReportData{TotalDevices: 5, PassedDevices: 5}
	assert.Equal(t, "healthy", healthy.OverallStatus())

	warning := &ReportData{TotalDevices: 5, PassedDevices: 4, WarningDevices: 1}
	assert.Equal(t, "warning", warning.OverallStatus())

	failed := &ReportData{TotalDevices: 5, PassedDevices: 3, FailedDevices: 2}
	assert.Equal(t, "critical", failed.OverallStatus())
}

func TestNewReportData(t *testing.T) {
	now := time.Now()
	rd := NewReportData("daily", now.Add(-24*time.Hour), now)
	assert.Equal(t, "daily", rd.PeriodType)
	assert.False(t, rd.GeneratedAt.IsZero())
}
