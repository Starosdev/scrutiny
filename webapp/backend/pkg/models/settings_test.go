package models

import (
	"reflect"
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
	require.Equal(t, "comfortable", s.DashboardDensity)
	require.Equal(t, "celsius", s.TemperatureUnit)
	require.Equal(t, "smooth", s.LineStroke)
	require.Equal(t, "humanize", s.PoweredOnHoursUnit)
	require.Equal(t, 2, s.DashboardColumns)
	require.Equal(t, 25, s.DashboardPageSize)

	// Metrics numeric defaults
	require.Equal(t, 2, s.Metrics.NotifyLevel)
	require.Equal(t, 0, s.Metrics.StatusFilterAttributes) // 0 = All, valid default
	require.Equal(t, 3, s.Metrics.StatusThreshold)
	require.Equal(t, 60, s.Metrics.MissedPingTimeoutMinutes)
	require.Equal(t, 5, s.Metrics.MissedPingCheckIntervalMins)
	require.Equal(t, 24, s.Metrics.HeartbeatIntervalHours)
	require.Equal(t, DefaultTemperatureThresholdCelsius, s.Metrics.TemperatureThresholdCelsius)
	require.Zero(t, s.Metrics.TemperatureDurationMinutes)
	require.False(t, s.Metrics.NotifyOnTemperature)

	// Metrics scheduled report defaults
	require.Equal(t, "08:00", s.Metrics.ReportDailyTime)
	require.Equal(t, 1, s.Metrics.ReportWeeklyDay)
	require.Equal(t, "08:00", s.Metrics.ReportWeeklyTime)
	require.Equal(t, 1, s.Metrics.ReportMonthlyDay)
	require.Equal(t, "08:00", s.Metrics.ReportMonthlyTime)

	// Bool fields should remain false (valid default)
	require.False(t, s.FileSizeSIUnits)
	require.False(t, s.Collector.RetrieveSCTHistory)
	require.False(t, s.Metrics.RepeatNotifications)
	require.False(t, s.Metrics.NotifyOnMissedPing)
	require.False(t, s.Metrics.HeartbeatEnabled)
	require.False(t, s.Metrics.ReportEnabled)
	require.False(t, s.Metrics.ReportDailyEnabled)
	require.False(t, s.Metrics.ConsumerDriveProfilesEnabled)
	require.False(t, s.Metrics.ReportWeeklyEnabled)
	require.False(t, s.Metrics.ReportMonthlyEnabled)
	require.False(t, s.Metrics.ReportPDFEnabled)
	require.False(t, s.Metrics.NotifyOnReplacementRisk)
	require.Equal(t, "replace_soon", s.Metrics.ReplacementRiskNotifyCategory)
}

func TestApplyDefaults_PreservesExistingValues(t *testing.T) {
	s := Settings{
		Theme:              "dark",
		Layout:             "empty",
		DashboardDisplay:   "serial_id",
		DashboardSort:      "title",
		DashboardDensity:   "compact",
		TemperatureUnit:    "fahrenheit",
		LineStroke:         "straight",
		PoweredOnHoursUnit: "device_hours",
		DashboardColumns:   4,
	}
	s.Metrics.NotifyLevel = 1
	s.Metrics.StatusThreshold = 1
	s.Metrics.MissedPingTimeoutMinutes = 30
	s.Metrics.MissedPingCheckIntervalMins = 10
	s.Metrics.HeartbeatIntervalHours = 12
	s.Metrics.TemperatureThresholdCelsius = 65
	s.Metrics.TemperatureDurationMinutes = 0
	s.Metrics.ReportDailyTime = "03:00"
	s.Metrics.ReportWeeklyDay = 5
	s.Metrics.ReportWeeklyTime = "09:00"
	s.Metrics.ReportMonthlyDay = 15
	s.Metrics.ReportMonthlyTime = "10:00"

	s.ApplyDefaults()

	// All user-set values preserved
	require.Equal(t, "dark", s.Theme)
	require.Equal(t, "empty", s.Layout)
	require.Equal(t, "serial_id", s.DashboardDisplay)
	require.Equal(t, "title", s.DashboardSort)
	require.Equal(t, "compact", s.DashboardDensity)
	require.Equal(t, "fahrenheit", s.TemperatureUnit)
	require.Equal(t, "straight", s.LineStroke)
	require.Equal(t, "device_hours", s.PoweredOnHoursUnit)
	require.Equal(t, 4, s.DashboardColumns)
	require.Equal(t, 1, s.Metrics.NotifyLevel)
	require.Equal(t, 1, s.Metrics.StatusThreshold)
	require.Equal(t, 30, s.Metrics.MissedPingTimeoutMinutes)
	require.Equal(t, 10, s.Metrics.MissedPingCheckIntervalMins)
	require.Equal(t, 12, s.Metrics.HeartbeatIntervalHours)
	require.Equal(t, 65, s.Metrics.TemperatureThresholdCelsius)
	require.Zero(t, s.Metrics.TemperatureDurationMinutes)
	require.Equal(t, "03:00", s.Metrics.ReportDailyTime)
	require.Equal(t, 5, s.Metrics.ReportWeeklyDay)
	require.Equal(t, "09:00", s.Metrics.ReportWeeklyTime)
	require.Equal(t, 15, s.Metrics.ReportMonthlyDay)
	require.Equal(t, "10:00", s.Metrics.ReportMonthlyTime)
}

func TestApplyDefaults_PartiallyPopulated(t *testing.T) {
	s := Settings{
		Theme:  "dark",
		Layout: "", // empty - should get defaulted
	}
	s.Metrics.NotifyLevel = 1 // set
	// StatusThreshold left at 0 - should get defaulted

	s.ApplyDefaults()

	require.Equal(t, "dark", s.Theme)            // preserved
	require.Equal(t, "material", s.Layout)       // defaulted
	require.Equal(t, "name", s.DashboardDisplay) // defaulted
	require.Equal(t, "comfortable", s.DashboardDensity)
	require.Equal(t, 1, s.Metrics.NotifyLevel)     // preserved
	require.Equal(t, 3, s.Metrics.StatusThreshold) // defaulted
	require.Equal(t, 2, s.DashboardColumns)
}

// nonSecretStringSettings lists every string setting reviewed as safe to show a read-scope caller.
// A new string field fails TestSettingsRedactedClearsEverythingNotAllowlisted until someone decides
// whether it can carry a credential (URL with a token, password, webhook) and either redacts it in
// Settings.Redacted or adds it here.
var nonSecretStringSettings = map[string]bool{
	"Metrics.NotificationQuietStart": true, "Metrics.NotificationQuietEnd": true,
	"Metrics.ReportMonthlyTime": true, "Metrics.ReportWeeklyTime": true, "Metrics.ReportDailyTime": true,
	"Metrics.ReplacementRiskNotifyCategory": true, "Metrics.ConsumerDriveProfilesDenylist": true,
	"Theme": true, "Layout": true, "DashboardDisplay": true, "DashboardSort": true, "DashboardDensity": true,
	"TemperatureUnit": true, "LineStroke": true, "PoweredOnHoursUnit": true, "TimeFormat": true,
}

func fillStrings(v reflect.Value, path string, out map[string]bool) {
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		name := v.Type().Field(i).Name
		if path != "" {
			name = path + "." + name
		}
		switch f.Kind() {
		case reflect.String:
			f.SetString("SENTINEL")
			out[name] = true
		case reflect.Struct:
			fillStrings(f, name, out)
		}
	}
}

func collectSurvivors(v reflect.Value, path string, survivors map[string]bool) {
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		name := v.Type().Field(i).Name
		if path != "" {
			name = path + "." + name
		}
		switch f.Kind() {
		case reflect.String:
			if f.String() == "SENTINEL" {
				survivors[name] = true
			}
		case reflect.Struct:
			collectSurvivors(f, name, survivors)
		}
	}
}

func TestSettingsRedactedClearsEverythingNotAllowlisted(t *testing.T) {
	var s Settings
	all := map[string]bool{}
	fillStrings(reflect.ValueOf(&s).Elem(), "", all)

	red := s.Redacted()
	survivors := map[string]bool{}
	collectSurvivors(reflect.ValueOf(&red).Elem(), "", survivors)

	require.Empty(t, red.Metrics.UptimeKumaPushURL)
	require.Equal(t, "SENTINEL", s.Metrics.UptimeKumaPushURL, "Redacted must work on a copy")
	for name := range survivors {
		require.True(t, nonSecretStringSettings[name], "string setting %s survives redaction but is not on the reviewed allowlist", name)
	}
	for name := range nonSecretStringSettings {
		require.True(t, all[name], "allowlist names %s, which is not a string setting", name)
	}
}
