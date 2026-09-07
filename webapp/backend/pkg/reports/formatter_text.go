package reports

import (
	"fmt"
	"strconv"
	"strings"
)

// FormatTextReport generates a plain text subject and message body for notification backends.
// Returns (subject, message).
func FormatTextReport(report *ReportData) (string, string) {
	if report == nil {
		return "Scrutiny Report", "No report data available."
	}
	periodLabel := capitalizePeriod(report.PeriodType)
	dateStr := report.GeneratedAt.Format("2006-01-02")

	subject := formatSubject(report, periodLabel)

	var parts []string

	// Header and device summary
	parts = append(parts,
		fmt.Sprintf("Scrutiny %s Report - %s", periodLabel, dateStr),
		"",
		fmt.Sprintf("Devices: %d total | %d passed | %d warning | %d failed",
			report.TotalDevices, report.PassedDevices, report.WarningDevices, report.FailedDevices),
	)
	if report.ArchivedDevices > 0 {
		parts = append(parts, fmt.Sprintf("  (%d archived, excluded from report)", report.ArchivedDevices))
	}

	// Alert sections
	parts = appendAlertSection(parts, report, "failed", "FAILURES:")
	parts = appendAlertSection(parts, report, "warning", "WARNINGS:")

	// Temperature summary
	parts = appendTempSummary(parts, report.Devices)

	// ZFS section
	parts = appendZFSSection(parts, report.ZFSPools)

	message := strings.Join(parts, "\n")
	return subject, message
}

// TruncateForNotification truncates a message to maxLen characters,
// ending with "..." if truncated.
func TruncateForNotification(message string, maxLen int) string {
	if len(message) <= maxLen {
		return message
	}
	if maxLen <= 3 {
		return message[:maxLen]
	}
	return message[:maxLen-3] + "..."
}

func capitalizePeriod(periodType string) string {
	if periodType == "" {
		return "Report"
	}
	return strings.ToUpper(periodType[:1]) + periodType[1:]
}

func formatSubject(report *ReportData, periodLabel string) string {
	if report.HasFailures() {
		return fmt.Sprintf("Scrutiny %s Report - %d failed, %d warning", periodLabel, report.FailedDevices, report.WarningDevices)
	}
	if report.HasWarnings() {
		return fmt.Sprintf("Scrutiny %s Report - %d warning", periodLabel, report.WarningDevices)
	}
	return fmt.Sprintf("Scrutiny %s Report - All %d drives healthy", periodLabel, report.TotalDevices)
}

func appendAlertSection(parts []string, report *ReportData, status string, header string) []string {
	alerts := collectAlerts(report, status)
	if len(alerts) == 0 {
		return parts
	}
	parts = append(parts, "", header)
	for _, entry := range alerts {
		parts = append(parts, fmt.Sprintf("  - %s: %s", entry.deviceName, entry.alertLine))
	}
	return parts
}

func appendTempSummary(parts []string, devices []DeviceReport) []string {
	if len(devices) == 0 {
		return parts
	}
	hottest, coldest := tempExtremes(devices)
	if hottest == nil {
		return parts
	}
	parts = append(parts,
		"",
		"Temperature Summary:",
		fmt.Sprintf("  Highest: %s at %sC (avg %.0fC)", hottest.DisplayName(), formatTemp(hottest.TempCurrent), hottest.TempAvg),
	)
	if coldest != nil && coldest != hottest {
		parts = append(parts, fmt.Sprintf("  Lowest: %s at %sC (avg %.0fC)", coldest.DisplayName(), formatTemp(coldest.TempCurrent), coldest.TempAvg))
	}
	return parts
}

func appendZFSSection(parts []string, pools []ZFSPoolReport) []string {
	if len(pools) == 0 {
		return parts
	}
	parts = append(parts, "", "ZFS Pools:")
	for i := range pools {
		parts = append(parts, formatZFSPoolLine(&pools[i]))
	}
	return parts
}

func formatZFSPoolLine(pool *ZFSPoolReport) string {
	details := fmt.Sprintf("capacity: %.1f%%", pool.Capacity)
	if pool.ErrorsRead > 0 || pool.ErrorsWrite > 0 || pool.ErrorsChecksum > 0 {
		details += fmt.Sprintf(", errors: %d read / %d write / %d checksum",
			pool.ErrorsRead, pool.ErrorsWrite, pool.ErrorsChecksum)
	}
	return fmt.Sprintf("  - %s: %s (%s)", pool.Name, pool.Health, details)
}

type alertLine struct {
	deviceName string
	alertLine  string
}

func collectAlerts(report *ReportData, status string) []alertLine {
	var results []alertLine
	for i := range report.Devices {
		device := &report.Devices[i]
		for _, alert := range device.ActiveFailures {
			if alert.Status != status {
				continue
			}
			line := fmt.Sprintf("Attribute %s (%s) = %d", alert.AttributeID, alert.AttributeName, alert.Value)
			if alert.Threshold > 0 {
				line += fmt.Sprintf(" [threshold: %d]", alert.Threshold)
			}
			results = append(results, alertLine{
				deviceName: device.DisplayName(),
				alertLine:  line,
			})
		}
	}
	return results
}

// formatTemp renders a temperature, or a placeholder when the device reported
// none. A device with no reading must not print as 0C.
const tempPlaceholder = "--"

func formatTemp(temp *int64) string {
	if temp == nil {
		return tempPlaceholder
	}
	return strconv.FormatInt(*temp, 10)
}

// tempExtremes returns the hottest and coldest device that actually reported a
// temperature. Devices without a reading are skipped rather than counted as 0C,
// which previously made any of them the coldest device.
func tempExtremes(devices []DeviceReport) (*DeviceReport, *DeviceReport) {
	var hottest, coldest *DeviceReport
	for i := range devices {
		if devices[i].TempCurrent == nil {
			continue
		}
		if hottest == nil || *devices[i].TempCurrent > *hottest.TempCurrent {
			hottest = &devices[i]
		}
		if coldest == nil || *devices[i].TempCurrent < *coldest.TempCurrent {
			coldest = &devices[i]
		}
	}
	return hottest, coldest
}
