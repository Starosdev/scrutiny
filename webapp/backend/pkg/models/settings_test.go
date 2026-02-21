package models

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplyDefaults_AllZeroValues(t *testing.T) {
	s := Settings{}
	s.ApplyDefaults()

	// Top-level string settings
	require.Equal(t, "system", s.Theme)
	require.Equal(t, "material", s.Layout)
	require.Equal(t, "name", s.DashboardDisplay)
	require.Equal(t, "status", s.DashboardSort)
	require.Equal(t, "celsius", s.TemperatureUnit)
	require.Equal(t, "smooth", s.LineStroke)
	require.Equal(t, "humanize", s.PoweredOnHoursUnit)

	// Metrics numeric defaults
	require.Equal(t, 2, s.Metrics.NotifyLevel)
	require.Equal(t, 0, s.Metrics.StatusFilterAttributes) // 0 = All, valid default
	require.Equal(t, 3, s.Metrics.StatusThreshold)
	require.Equal(t, 60, s.Metrics.MissedPingTimeoutMinutes)
	require.Equal(t, 5, s.Metrics.MissedPingCheckIntervalMins)
	require.Equal(t, 24, s.Metrics.HeartbeatIntervalHours)

	// Metrics scheduled report defaults
	require.Equal(t, "08:00", s.Metrics.ReportDailyTime)
	require.Equal(t, 1, s.Metrics.ReportWeeklyDay)
	require.Equal(t, "08:00", s.Metrics.ReportWeeklyTime)
	require.Equal(t, 1, s.Metrics.ReportMonthlyDay)
	require.Equal(t, "08:00", s.Metrics.ReportMonthlyTime)
	require.Equal(t, "/opt/scrutiny/reports", s.Metrics.ReportPDFPath)

	// Bool fields should remain false (valid default)
	require.False(t, s.FileSizeSIUnits)
	require.False(t, s.Collector.RetrieveSCTHistory)
	require.False(t, s.Metrics.RepeatNotifications)
	require.False(t, s.Metrics.NotifyOnMissedPing)
	require.False(t, s.Metrics.HeartbeatEnabled)
	require.False(t, s.Metrics.ReportEnabled)
	require.False(t, s.Metrics.ReportDailyEnabled)
	require.False(t, s.Metrics.ReportWeeklyEnabled)
	require.False(t, s.Metrics.ReportMonthlyEnabled)
	require.False(t, s.Metrics.ReportPDFEnabled)
}

func TestApplyDefaults_PreservesExistingValues(t *testing.T) {
	s := Settings{
		Theme:              "dark",
		Layout:             "empty",
		DashboardDisplay:   "serial_id",
		DashboardSort:      "title",
		TemperatureUnit:    "fahrenheit",
		LineStroke:         "straight",
		PoweredOnHoursUnit: "device_hours",
	}
	s.Metrics.NotifyLevel = 1
	s.Metrics.StatusThreshold = 1
	s.Metrics.MissedPingTimeoutMinutes = 30
	s.Metrics.MissedPingCheckIntervalMins = 10
	s.Metrics.HeartbeatIntervalHours = 12
	s.Metrics.ReportDailyTime = "03:00"
	s.Metrics.ReportWeeklyDay = 5
	s.Metrics.ReportWeeklyTime = "09:00"
	s.Metrics.ReportMonthlyDay = 15
	s.Metrics.ReportMonthlyTime = "10:00"
	s.Metrics.ReportPDFPath = "/custom/path"

	s.ApplyDefaults()

	// All user-set values preserved
	require.Equal(t, "dark", s.Theme)
	require.Equal(t, "empty", s.Layout)
	require.Equal(t, "serial_id", s.DashboardDisplay)
	require.Equal(t, "title", s.DashboardSort)
	require.Equal(t, "fahrenheit", s.TemperatureUnit)
	require.Equal(t, "straight", s.LineStroke)
	require.Equal(t, "device_hours", s.PoweredOnHoursUnit)
	require.Equal(t, 1, s.Metrics.NotifyLevel)
	require.Equal(t, 1, s.Metrics.StatusThreshold)
	require.Equal(t, 30, s.Metrics.MissedPingTimeoutMinutes)
	require.Equal(t, 10, s.Metrics.MissedPingCheckIntervalMins)
	require.Equal(t, 12, s.Metrics.HeartbeatIntervalHours)
	require.Equal(t, "03:00", s.Metrics.ReportDailyTime)
	require.Equal(t, 5, s.Metrics.ReportWeeklyDay)
	require.Equal(t, "09:00", s.Metrics.ReportWeeklyTime)
	require.Equal(t, 15, s.Metrics.ReportMonthlyDay)
	require.Equal(t, "10:00", s.Metrics.ReportMonthlyTime)
	require.Equal(t, "/custom/path", s.Metrics.ReportPDFPath)
}

func TestApplyDefaults_PartiallyPopulated(t *testing.T) {
	s := Settings{
		Theme:  "dark",
		Layout: "", // empty - should get defaulted
	}
	s.Metrics.NotifyLevel = 1 // set
	// StatusThreshold left at 0 - should get defaulted

	s.ApplyDefaults()

	require.Equal(t, "dark", s.Theme)              // preserved
	require.Equal(t, "material", s.Layout)          // defaulted
	require.Equal(t, "name", s.DashboardDisplay)    // defaulted
	require.Equal(t, 1, s.Metrics.NotifyLevel)      // preserved
	require.Equal(t, 3, s.Metrics.StatusThreshold)  // defaulted
}
