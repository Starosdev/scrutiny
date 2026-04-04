package models

import "time"

// RiskCategory represents the replacement urgency level for a drive.
type RiskCategory string

const (
	// RiskCategoryHealthy indicates the drive shows no significant warning signs (score 0-25).
	RiskCategoryHealthy RiskCategory = "healthy"

	// RiskCategoryMonitor indicates the drive has mild degradation and should be watched (score 26-50).
	RiskCategoryMonitor RiskCategory = "monitor"

	// RiskCategoryPlanReplacement indicates measurable degradation; budget for a replacement (score 51-75).
	RiskCategoryPlanReplacement RiskCategory = "plan_replacement"

	// RiskCategoryReplaceSoon indicates critical degradation; replace before data loss occurs (score 76-100).
	RiskCategoryReplaceSoon RiskCategory = "replace_soon"
)

// RiskCategoryThresholds maps score boundaries to risk categories.
// Scores are integers in the range [0, 100].
//
//	0-25  -> Healthy
//	26-50 -> Monitor
//	51-75 -> PlanReplacement
//	76-100 -> ReplaceSoon
var RiskCategoryThresholds = map[RiskCategory][2]int{
	RiskCategoryHealthy:         {0, 25},
	RiskCategoryMonitor:         {26, 50},
	RiskCategoryPlanReplacement: {51, 75},
	RiskCategoryReplaceSoon:     {76, 100},
}

// ScoreToRiskCategory converts a numeric score (0-100) to its RiskCategory.
func ScoreToRiskCategory(score int) RiskCategory {
	switch {
	case score <= 25:
		return RiskCategoryHealthy
	case score <= 50:
		return RiskCategoryMonitor
	case score <= 75:
		return RiskCategoryPlanReplacement
	default:
		return RiskCategoryReplaceSoon
	}
}

// AttributeContribution records how a single SMART attribute contributed to the
// overall replacement risk score.
type AttributeContribution struct {
	// AttributeID is the attribute identifier (integer for ATA, string key for NVMe/SCSI).
	AttributeID string `json:"attribute_id"`

	// DisplayName is the human-readable attribute name.
	DisplayName string `json:"display_name"`

	// Weight is the maximum number of points this attribute can contribute (0.0-1.0 fraction of total).
	Weight float64 `json:"weight"`

	// Score is the weighted score contribution from this attribute (0-100 scaled).
	Score float64 `json:"score"`

	// Value is the raw or normalized value used to compute the score.
	Value int64 `json:"value"`

	// TrendScore is the additional score added due to rate-of-change analysis.
	// A positive value means the attribute is worsening over the observation window.
	TrendScore float64 `json:"trend_score"`
}

// TrendWindow represents a time window used for rate-of-change analysis.
type TrendWindow string

const (
	TrendWindow7Days  TrendWindow = "7d"
	TrendWindow30Days TrendWindow = "30d"
	TrendWindow90Days TrendWindow = "90d"
)

// ReplacementRiskScore is the core domain model for a computed drive replacement risk.
// It is computed on-the-fly from the latest SMART data and historical trends;
// it is NOT persisted to the database in this definition phase.
type ReplacementRiskScore struct {
    // ComputedAt is the timestamp when this score was calculated.
    ComputedAt time.Time `json:"computed_at"`

    // DeviceWWN identifies the drive this score belongs to.
    DeviceWWN string `json:"device_wwn"`

    // DeviceProtocol is the protocol type used for scoring (ATA, NVMe, SCSI).
    DeviceProtocol string `json:"device_protocol"`

    // Category is the human-readable risk bucket derived from Score.
    Category RiskCategory `json:"category"`

    // TrendWindow is the time window over which rate-of-change was evaluated.
    TrendWindow TrendWindow `json:"trend_window"`

    // Contributions lists per-attribute score breakdowns.
    Contributions []AttributeContribution `json:"contributions"`

    // TrendBonus is the total additional score added by trend analysis across all attributes.
    TrendBonus float64 `json:"trend_bonus"`

    // Score is the overall replacement risk score, 0 (best) to 100 (worst).
    Score int `json:"score"`
}

// ReplacementRiskResponse is the API response envelope for the
// GET /api/device/:wwn/replacement-risk endpoint.
type ReplacementRiskResponse struct {
	Data    ReplacementRiskScore `json:"data"`
	Success bool                 `json:"success"`
}
