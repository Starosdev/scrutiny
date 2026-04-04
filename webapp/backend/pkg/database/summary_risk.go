package database

import (
	"math"
	"strings"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
)

// summaryRiskScore computes a simplified 0-100 replacement risk score using the
// subset of SMART attributes available in the dashboard summary query.
// This is less precise than the full /api/device/:id/replacement-risk endpoint
// (no trend analysis, fewer attributes) but is fast and suitable for dashboard badges.
//
// Covered attributes per protocol (partial weights, scaled to 0-100):
//
//	ATA (max covered weight 65): attr 5 (25pt), attr 197 (20pt), attr 198 (20pt)
//	NVMe (max covered weight 60): percentage_used (40pt), media_errors (20pt)
//	SCSI (max covered weight 40): scsi_grown_defect_list (40pt)
func summaryRiskScore(protocol string, values map[string]interface{}) (int, models.RiskCategory) {
	var raw, maxRaw float64

	switch {
	case strings.EqualFold(protocol, "NVMe"):
		maxRaw = 60.0
		if val := summaryInt64(values, "attr.percentage_used.value"); val > 0 {
			raw += math.Min(float64(val)/100.0, 1.0) * 40.0
		}
		if val := summaryInt64(values, "attr.media_errors.value"); val > 0 {
			raw += summaryCounterSeverity(val) * 20.0
		}
	case strings.EqualFold(protocol, "SCSI"):
		maxRaw = 40.0
		if val := summaryInt64(values, "attr.scsi_grown_defect_list.value"); val > 0 {
			raw += summaryCounterSeverity(val) * 40.0
		}
	default: // ATA
		maxRaw = 65.0
		if val := summaryInt64(values, "attr.5.raw_value"); val > 0 {
			raw += summaryCounterSeverity(val) * 25.0
		}
		if val := summaryInt64(values, "attr.197.raw_value"); val > 0 {
			raw += summaryCounterSeverity(val) * 20.0
		}
		if val := summaryInt64(values, "attr.198.raw_value"); val > 0 {
			raw += summaryCounterSeverity(val) * 20.0
		}
	}

	if maxRaw == 0 || raw == 0 {
		return 0, models.RiskCategoryHealthy
	}

	scaled := int(math.Round(raw / maxRaw * 100))
	if scaled > 100 {
		scaled = 100
	}
	return scaled, models.ScoreToRiskCategory(scaled)
}

// summaryInt64 extracts an int64 from the InfluxDB result values map.
// Returns 0 if the field is absent or not an int64.
func summaryInt64(values map[string]interface{}, field string) int64 {
	val, ok := values[field]
	if !ok || val == nil {
		return 0
	}
	n, ok := val.(int64)
	if !ok {
		return 0
	}
	return n
}

// summaryCounterSeverity is the same stepped scale used by the full risk scorer.
//
//	0     -> 0.00
//	1–4   -> 0.25
//	5–19  -> 0.50
//	20–49 -> 0.75
//	50+   -> 1.00
func summaryCounterSeverity(value int64) float64 {
	switch {
	case value <= 0:
		return 0.0
	case value < 5:
		return 0.25
	case value < 20:
		return 0.50
	case value < 50:
		return 0.75
	default:
		return 1.0
	}
}
