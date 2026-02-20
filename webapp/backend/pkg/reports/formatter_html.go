package reports

import (
	"fmt"
	"strings"
)

// FormatHTMLReport generates an HTML email body for the report.
func FormatHTMLReport(report *ReportData) string {
	if report == nil {
		return "<p>No report data available.</p>"
	}

	var b strings.Builder

	b.WriteString(`<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="margin:0;padding:0;background-color:#f4f4f4;font-family:Arial,Helvetica,sans-serif;">
<table width="100%" cellpadding="0" cellspacing="0" style="background-color:#f4f4f4;padding:20px 0;">
<tr><td align="center">
<table width="600" cellpadding="0" cellspacing="0" style="background-color:#ffffff;border-radius:4px;overflow:hidden;">
`)

	// Status banner
	status := report.OverallStatus()
	bannerColor := statusColor(status)
	bannerLabel := strings.ToUpper(status)
	periodLabel := capitalizePeriod(report.PeriodType)

	b.WriteString(fmt.Sprintf(`<tr><td style="background-color:%s;padding:20px 30px;">
<h1 style="margin:0;color:#ffffff;font-size:22px;">Scrutiny %s Report</h1>
<p style="margin:4px 0 0;color:#ffffffcc;font-size:13px;">%s to %s &middot; Status: %s</p>
</td></tr>
`, bannerColor, periodLabel,
		report.PeriodStart.Format("Jan 2, 2006"),
		report.PeriodEnd.Format("Jan 2, 2006"),
		bannerLabel))

	// Summary counts
	b.WriteString(`<tr><td style="padding:20px 30px;">
<table width="100%" cellpadding="0" cellspacing="0">
<tr>`)
	writeSummaryCell(&b, "Total", report.TotalDevices, "#212529")
	writeSummaryCell(&b, "Passed", report.PassedDevices, "#28a745")
	writeSummaryCell(&b, "Warning", report.WarningDevices, "#ffc107")
	writeSummaryCell(&b, "Failed", report.FailedDevices, "#dc3545")
	b.WriteString(`</tr></table>`)

	if report.ArchivedDevices > 0 {
		b.WriteString(fmt.Sprintf(`<p style="margin:8px 0 0;color:#6c757d;font-size:12px;">%d archived device(s) excluded from report</p>`, report.ArchivedDevices))
	}
	b.WriteString(`</td></tr>`)

	// Failures section
	writeAlertHTMLSection(&b, report, "failed", "Failures", "#dc3545")
	writeAlertHTMLSection(&b, report, "warning", "Warnings", "#ffc107")

	// Device table
	if len(report.Devices) > 0 {
		writeDeviceHTMLTable(&b, report)
	}

	// Temperature summary
	writeTempHTMLSummary(&b, report.Devices)

	// ZFS section
	if len(report.ZFSPools) > 0 {
		writeZFSHTMLSection(&b, report.ZFSPools)
	}

	// Footer
	b.WriteString(fmt.Sprintf(`<tr><td style="padding:15px 30px;background-color:#f8f9fa;border-top:1px solid #dee2e6;">
<p style="margin:0;color:#6c757d;font-size:11px;">Generated %s by Scrutiny</p>
</td></tr>`, report.GeneratedAt.Format("Jan 2, 2006 15:04 MST")))

	b.WriteString(`</table>
</td></tr></table>
</body></html>`)

	return b.String()
}

func statusColor(status string) string {
	switch status {
	case "critical":
		return "#dc3545"
	case "warning":
		return "#e6a817"
	default:
		return "#28a745"
	}
}

func writeSummaryCell(b *strings.Builder, label string, count int, color string) {
	b.WriteString(fmt.Sprintf(`<td width="25%%" align="center" style="padding:10px 0;">
<div style="font-size:24px;font-weight:bold;color:%s;">%d</div>
<div style="font-size:11px;color:#6c757d;text-transform:uppercase;">%s</div>
</td>`, color, count, label))
}

func writeAlertHTMLSection(b *strings.Builder, report *ReportData, status, header, color string) {
	alerts := collectAlerts(report, status)
	if len(alerts) == 0 {
		return
	}

	b.WriteString(fmt.Sprintf(`<tr><td style="padding:10px 30px 0;">
<h3 style="margin:0 0 8px;color:%s;font-size:14px;">%s</h3>
<table width="100%%" cellpadding="4" cellspacing="0" style="font-size:12px;border-collapse:collapse;">`, color, header))

	for _, entry := range alerts {
		b.WriteString(fmt.Sprintf(`<tr>
<td style="border-bottom:1px solid #eee;color:#212529;"><strong>%s</strong></td>
<td style="border-bottom:1px solid #eee;color:#495057;">%s</td>
</tr>`, escapeHTML(entry.deviceName), escapeHTML(entry.alertLine)))
	}

	b.WriteString(`</table></td></tr>`)
}

func writeDeviceHTMLTable(b *strings.Builder, report *ReportData) {
	b.WriteString(`<tr><td style="padding:15px 30px 0;">
<h3 style="margin:0 0 8px;color:#212529;font-size:14px;">Devices</h3>
<table width="100%" cellpadding="5" cellspacing="0" style="font-size:11px;border-collapse:collapse;">
<tr style="background-color:#f0f0f0;">
<th align="left" style="padding:6px;border:1px solid #dee2e6;">Name</th>
<th align="center" style="padding:6px;border:1px solid #dee2e6;">Status</th>
<th align="center" style="padding:6px;border:1px solid #dee2e6;">Temp</th>
<th align="center" style="padding:6px;border:1px solid #dee2e6;">Power-On (h)</th>
<th align="center" style="padding:6px;border:1px solid #dee2e6;">Alerts</th>
</tr>`)

	for i := range report.Devices {
		d := &report.Devices[i]
		rowColor := "#212529"
		if d.Status > 0 {
			rowColor = "#dc3545"
		}

		name := d.DisplayName()
		if len(name) > 30 {
			name = name[:30] + "..."
		}

		alertCount := len(d.ActiveFailures) + len(d.NewAlerts)
		b.WriteString(fmt.Sprintf(`<tr>
<td style="padding:5px 6px;border:1px solid #dee2e6;color:%s;">%s</td>
<td align="center" style="padding:5px 6px;border:1px solid #dee2e6;color:%s;">%s</td>
<td align="center" style="padding:5px 6px;border:1px solid #dee2e6;">%dC</td>
<td align="center" style="padding:5px 6px;border:1px solid #dee2e6;">%d</td>
<td align="center" style="padding:5px 6px;border:1px solid #dee2e6;">%d</td>
</tr>`, rowColor, escapeHTML(name), rowColor, d.StatusString(), d.TempCurrent, d.PowerOnHours, alertCount))
	}

	b.WriteString(`</table></td></tr>`)
}

func writeTempHTMLSummary(b *strings.Builder, devices []DeviceReport) {
	if len(devices) == 0 {
		return
	}
	hottest, coldest := tempExtremes(devices)
	if hottest == nil {
		return
	}

	b.WriteString(`<tr><td style="padding:15px 30px 0;">
<h3 style="margin:0 0 8px;color:#212529;font-size:14px;">Temperature Summary</h3>
<table cellpadding="3" cellspacing="0" style="font-size:12px;">`)

	b.WriteString(fmt.Sprintf(`<tr><td style="color:#6c757d;">Highest:</td><td><strong>%s</strong> at %dC (avg %.0fC)</td></tr>`,
		escapeHTML(hottest.DisplayName()), hottest.TempCurrent, hottest.TempAvg))

	if coldest != nil && coldest.TempCurrent != hottest.TempCurrent {
		b.WriteString(fmt.Sprintf(`<tr><td style="color:#6c757d;">Lowest:</td><td><strong>%s</strong> at %dC (avg %.0fC)</td></tr>`,
			escapeHTML(coldest.DisplayName()), coldest.TempCurrent, coldest.TempAvg))
	}

	b.WriteString(`</table></td></tr>`)
}

func writeZFSHTMLSection(b *strings.Builder, pools []ZFSPoolReport) {
	b.WriteString(`<tr><td style="padding:15px 30px 0;">
<h3 style="margin:0 0 8px;color:#212529;font-size:14px;">ZFS Pools</h3>
<table width="100%" cellpadding="5" cellspacing="0" style="font-size:11px;border-collapse:collapse;">
<tr style="background-color:#f0f0f0;">
<th align="left" style="padding:6px;border:1px solid #dee2e6;">Pool</th>
<th align="center" style="padding:6px;border:1px solid #dee2e6;">Health</th>
<th align="center" style="padding:6px;border:1px solid #dee2e6;">Capacity</th>
<th align="center" style="padding:6px;border:1px solid #dee2e6;">Errors</th>
</tr>`)

	for _, pool := range pools {
		healthColor := "#28a745"
		if pool.Health != "ONLINE" {
			healthColor = "#dc3545"
		}

		errors := ""
		if pool.ErrorsRead > 0 || pool.ErrorsWrite > 0 || pool.ErrorsChecksum > 0 {
			errors = fmt.Sprintf("%d/%d/%d", pool.ErrorsRead, pool.ErrorsWrite, pool.ErrorsChecksum)
		} else {
			errors = "0"
		}

		b.WriteString(fmt.Sprintf(`<tr>
<td style="padding:5px 6px;border:1px solid #dee2e6;">%s</td>
<td align="center" style="padding:5px 6px;border:1px solid #dee2e6;color:%s;">%s</td>
<td align="center" style="padding:5px 6px;border:1px solid #dee2e6;">%.1f%%</td>
<td align="center" style="padding:5px 6px;border:1px solid #dee2e6;">%s</td>
</tr>`, escapeHTML(pool.Name), healthColor, pool.Health, pool.Capacity, errors))
	}

	b.WriteString(`</table></td></tr>`)
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}
