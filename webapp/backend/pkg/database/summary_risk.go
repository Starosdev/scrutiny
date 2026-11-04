package database

import (
	"math"
	"strings"

	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/thresholds"
)

// summaryRiskScore computes a simplified 0-100 replacement risk score using the
// subset of SMART attributes available in the dashboard summary query.
// This is less precise than the full /api/device/:id/replacement-risk endpoint
// (no trend analysis, fewer attributes) but is fast and suitable for dashboard badges.
//
// Covered attributes per protocol, each weighted exactly as the full scorer
// weights it and rescaled to 0-100 over the covered subset:
//
//	ATA:  attr 5, attr 197, attr 198
//	NVMe: percentage_used, media_errors
//	SCSI: scsi_grown_defect_list
//
// The weights come from thresholds.ReplacementRiskWeightFor rather than from
// literals here, so a rebalance of the full table cannot leave the badge
// scoring an attribute at a weight the detail view no longer uses.
func summaryRiskScore(protocol string, values map[string]interface{}) (int, models.RiskCategory) {
	var raw, maxRaw float64

	switch {
	case strings.EqualFold(protocol, "NVMe"):
		maxRaw, raw = summaryMaxWeight("NVMe", nvmeSummaryAttributes), nvmeRiskRaw(values)
	case strings.EqualFold(protocol, "SCSI"):
		maxRaw, raw = summaryMaxWeight("SCSI", scsiSummaryAttributes), scsiRiskRaw(values)
	default: // ATA
		maxRaw, raw = summaryMaxWeight("ATA", ataSummaryAttributes), ataRiskRaw(values)
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

// Attributes each protocol's summary query can score, in the order they are
// summed. These are attribute IDs in the full replacement-risk tables.
var (
	ataSummaryAttributes  = []string{"5", "197", "198"}
	nvmeSummaryAttributes = []string{"percentage_used", "media_errors"}
	scsiSummaryAttributes = []string{"scsi_grown_defect_list"}
)

// summaryMaxWeight is the total weight of the attributes the summary query can
// see, which is what a fully-degraded covered set would score.
func summaryMaxWeight(protocol string, attributeIDs []string) float64 {
	var total float64
	for _, attributeID := range attributeIDs {
		weight, _ := thresholds.ReplacementRiskWeightFor(protocol, attributeID)
		total += weight
	}
	return total
}

// summaryWeight is the full scorer's weight for one attribute. A missing
// attribute returns 0, which drops it from the badge rather than scoring it at
// an invented weight.
func summaryWeight(protocol string, attributeID string) float64 {
	weight, _ := thresholds.ReplacementRiskWeightFor(protocol, attributeID)
	return weight
}

// nvmeRiskRaw sums the NVMe partial risk weights.
func nvmeRiskRaw(values map[string]interface{}) float64 {
	var raw float64
	if val := summaryInt64(values, "attr.percentage_used.value"); val > 0 {
		raw += math.Min(float64(val)/100.0, 1.0) * summaryWeight("NVMe", "percentage_used")
	}
	if val := summaryInt64(values, "attr.media_errors.value"); val > 0 {
		raw += summaryCounterSeverity(val) * summaryWeight("NVMe", "media_errors")
	}
	return raw
}

// scsiRiskRaw returns the SCSI partial risk weight.
func scsiRiskRaw(values map[string]interface{}) float64 {
	if val := summaryInt64(values, "attr.scsi_grown_defect_list.value"); val > 0 {
		return summaryCounterSeverity(val) * summaryWeight("SCSI", "scsi_grown_defect_list")
	}
	return 0
}

// ataRiskRaw sums the ATA partial risk weights.
func ataRiskRaw(values map[string]interface{}) float64 {
	var raw float64
	for _, attributeID := range ataSummaryAttributes {
		val := summaryInt64(values, "attr."+attributeID+".raw_value")
		if val <= 0 {
			continue
		}
		raw += summaryCounterSeverity(val) * summaryWeight("ATA", attributeID)
	}
	return raw
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
