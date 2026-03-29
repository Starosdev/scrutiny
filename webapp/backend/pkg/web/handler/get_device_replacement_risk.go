package handler

import (
	"math"
	"net/http"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	"github.com/analogj/scrutiny/webapp/backend/pkg/thresholds"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// GetDeviceReplacementRisk computes and returns a replacement risk score for the
// given device based on its latest SMART data, plus a trend bonus derived from
// the rate of change observed over the requested time window.
//
// Query parameters:
//   - trend_window: time window for rate-of-change analysis ("7d", "30d", "90d").
//     Defaults to "30d".
//
// Response: models.ReplacementRiskResponse
func GetDeviceReplacementRisk(c *gin.Context) {
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)

	device, err := ResolveDevice(c, logger, deviceRepo)
	if err != nil {
		return
	}

	trendWindowParam := c.DefaultQuery("trend_window", string(models.TrendWindow30Days))
	trendWindow := models.TrendWindow(trendWindowParam)
	switch trendWindow {
	case models.TrendWindow7Days, models.TrendWindow30Days, models.TrendWindow90Days:
		// valid
	default:
		trendWindow = models.TrendWindow30Days
	}

	// Latest SMART snapshot for static attribute scoring.
	latestResults, err := deviceRepo.GetLatestSmartSubmission(c, device.WWN)
	if err != nil {
		logger.Errorln("An error occurred retrieving latest SMART submission for replacement risk", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	var latestAttrs map[string]measurements.SmartAttribute
	if len(latestResults) > 0 {
		latestAttrs = latestResults[0].Attributes
	}

	// Historical snapshots over the trend window for rate-of-change analysis.
	// We request all entries so we can compare oldest vs. newest within the window.
	durationKey := trendWindowToDurationKey(trendWindow)
	history, err := deviceRepo.GetSmartAttributeHistory(c, device.WWN, durationKey, 0, 0, nil)
	if err != nil {
		// Non-fatal: trend analysis is best-effort; proceed with static score only.
		logger.Warnf("Could not retrieve SMART history for trend analysis: %v", err)
		history = nil
	}

	// Build oldest-attribute lookup from the first entry in the history slice.
	// GetSmartAttributeHistory returns entries ordered oldest-first.
	var oldestAttrs map[string]measurements.SmartAttribute
	if len(history) > 0 {
		oldestAttrs = history[0].Attributes
	}

	weights := thresholds.ReplacementRiskWeightsForProtocol(device.DeviceProtocol)
	contributions, totalScore, totalTrendBonus := computeRiskContributions(weights, latestAttrs, oldestAttrs)

	score := int(math.Round(totalScore))
	if score > 100 {
		score = 100
	}

	riskScore := models.ReplacementRiskScore{
		DeviceWWN:      device.WWN,
		DeviceProtocol: device.DeviceProtocol,
		Score:          score,
		Category:       models.ScoreToRiskCategory(score),
		Contributions:  contributions,
		TrendWindow:    trendWindow,
		TrendBonus:     math.Round(totalTrendBonus*100) / 100,
		ComputedAt:     time.Now().UTC(),
	}

	c.JSON(http.StatusOK, models.ReplacementRiskResponse{
		Success: true,
		Data:    riskScore,
	})
}

// computeRiskContributions builds the per-attribute contribution list and returns
// the total score and total trend bonus.
func computeRiskContributions(
	weights []thresholds.ReplacementRiskWeight,
	latestAttrs map[string]measurements.SmartAttribute,
	oldestAttrs map[string]measurements.SmartAttribute,
) ([]models.AttributeContribution, float64, float64) {
	if weights == nil {
		return []models.AttributeContribution{}, 0, 0
	}

	contributions := make([]models.AttributeContribution, 0, len(weights))
	totalScore := 0.0
	totalTrendBonus := 0.0

	for _, w := range weights {
		contrib := computeSingleContribution(w, latestAttrs, oldestAttrs)
		totalScore += contrib.Score + contrib.TrendScore
		totalTrendBonus += contrib.TrendScore
		contributions = append(contributions, contrib)
	}

	return contributions, totalScore, totalTrendBonus
}

// computeSingleContribution computes the static score and trend score for one
// weighted attribute.
func computeSingleContribution(
	w thresholds.ReplacementRiskWeight,
	latestAttrs map[string]measurements.SmartAttribute,
	oldestAttrs map[string]measurements.SmartAttribute,
) models.AttributeContribution {
	contrib := models.AttributeContribution{
		AttributeID: w.AttributeID,
		DisplayName: w.DisplayName,
		Weight:      w.Weight,
	}

	latest, hasLatest := latestAttrs[w.AttributeID]
	if hasLatest {
		severity := replacementRiskSeverity(latest, w.AttributeID)
		contrib.Score = math.Round(severity*w.Weight*100) / 100
		contrib.Value = latest.GetTransformedValue()
	}

	if hasLatest && oldestAttrs != nil {
		if oldest, ok := oldestAttrs[w.AttributeID]; ok {
			trendSev := trendSeverity(oldest, latest, w.AttributeID)
			rawTrend := math.Round(trendSev*w.Weight*w.TrendMultiplier*100) / 100
			maxTrend := math.Max(0, w.Weight*2-contrib.Score)
			contrib.TrendScore = math.Min(rawTrend, maxTrend)
		}
	}

	return contrib
}

// trendWindowToDurationKey maps a TrendWindow constant to the InfluxDB duration
// key used by GetSmartAttributeHistory.
func trendWindowToDurationKey(tw models.TrendWindow) string {
	switch tw {
	case models.TrendWindow7Days:
		return "week"
	case models.TrendWindow90Days:
		return "year"
	default: // TrendWindow30Days
		return "month"
	}
}

// trendSeverity computes a severity in [0.0, 1.0] representing how much an
// attribute has worsened between two SMART snapshots. Returns 0 if the
// attribute has not changed or has improved.
func trendSeverity(old, new measurements.SmartAttribute, attributeID string) float64 {
	switch o := old.(type) {
	case *measurements.SmartAtaAttribute:
		n, ok := new.(*measurements.SmartAtaAttribute)
		if !ok {
			return 0.0
		}
		return ataTrendSeverity(o, n, attributeID)
	case *measurements.SmartNvmeAttribute:
		n, ok := new.(*measurements.SmartNvmeAttribute)
		if !ok {
			return 0.0
		}
		return nvmeTrendSeverity(o, n, attributeID)
	case *measurements.SmartScsiAttribute:
		n, ok := new.(*measurements.SmartScsiAttribute)
		if !ok {
			return 0.0
		}
		return scsiTrendSeverity(o, n, attributeID)
	}
	return 0.0
}

func ataTrendSeverity(old, new *measurements.SmartAtaAttribute, id string) float64 {
	switch id {
	case "5", "196", "197", "198", "10":
		// Counter attributes: score the delta between oldest and newest raw value.
		delta := new.RawValue - old.RawValue
		return counterSeverity(delta)
	default:
		// For status-based attributes, detect a status regression.
		return statusRegressionSeverity(old.GetStatus(), new.GetStatus())
	}
}

func nvmeTrendSeverity(old, new *measurements.SmartNvmeAttribute, id string) float64 {
	switch id {
	case "percentage_used":
		// Rate of wear increase: 10% growth in the window = max severity.
		delta := new.Value - old.Value
		if delta <= 0 {
			return 0.0
		}
		return math.Min(float64(delta)/10.0, 1.0)
	case "media_errors":
		delta := new.Value - old.Value
		return counterSeverity(delta)
	default:
		return statusRegressionSeverity(old.GetStatus(), new.GetStatus())
	}
}

func scsiTrendSeverity(old, new *measurements.SmartScsiAttribute, id string) float64 {
	switch id {
	case "scsi_grown_defect_list",
		"read_total_uncorrected_errors",
		"write_total_uncorrected_errors":
		delta := new.Value - old.Value
		return counterSeverity(delta)
	default:
		return statusRegressionSeverity(old.GetStatus(), new.GetStatus())
	}
}

// statusRegressionSeverity returns a severity if the attribute status has
// worsened (e.g., passed → warning, or warning → failed). Returns 0.0 if
// status is unchanged or improved.
func statusRegressionSeverity(old, new pkg.AttributeStatus) float64 {
	oldFailed := pkg.AttributeStatusHas(old, pkg.AttributeStatusFailedSmart) ||
		pkg.AttributeStatusHas(old, pkg.AttributeStatusFailedScrutiny)
	newFailed := pkg.AttributeStatusHas(new, pkg.AttributeStatusFailedSmart) ||
		pkg.AttributeStatusHas(new, pkg.AttributeStatusFailedScrutiny)
	newWarn := pkg.AttributeStatusHas(new, pkg.AttributeStatusWarningScrutiny)

	if !oldFailed && newFailed {
		return 1.0 // newly failed
	}
	if old == pkg.AttributeStatusPassed && newWarn {
		return 0.5 // newly warned
	}
	return 0.0
}

// replacementRiskSeverity returns a severity in [0.0, 1.0] for a single SMART
// attribute. The caller multiplies this by the attribute's weight to get the
// point contribution to the total score.
func replacementRiskSeverity(attr measurements.SmartAttribute, attributeID string) float64 {
	switch a := attr.(type) {
	case *measurements.SmartAtaAttribute:
		return ataAttributeSeverity(a, attributeID)
	case *measurements.SmartNvmeAttribute:
		return nvmeAttributeSeverity(a, attributeID)
	case *measurements.SmartScsiAttribute:
		return scsiAttributeSeverity(a, attributeID)
	}
	return attributeStatusSeverity(attr.GetStatus())
}

// ataAttributeSeverity computes severity for an ATA SMART attribute.
// Counter attributes (reallocated sectors, pending sectors, etc.) are scored
// by raw value because any non-zero count is a surface degradation signal,
// regardless of whether the drive's internal threshold has been crossed.
func ataAttributeSeverity(a *measurements.SmartAtaAttribute, id string) float64 {
	switch id {
	case "5",  // Reallocated Sector Count
		"196", // Reallocated Event Count
		"197", // Current Pending Sector Count
		"198", // Offline Uncorrectable
		"10":  // Spin Retry Count
		return counterSeverity(a.RawValue)
	default:
		return attributeStatusSeverity(a.GetStatus())
	}
}

// nvmeAttributeSeverity computes severity for an NVMe SMART attribute.
// Wear indicators (percentage_used, available_spare) are scored from their raw
// values. Error counters use the same stepped scale as ATA counters.
func nvmeAttributeSeverity(a *measurements.SmartNvmeAttribute, id string) float64 {
	switch id {
	case "percentage_used":
		// Direct wear indicator: 0% used = 0.0 severity, 100% used = 1.0 severity.
		return math.Min(float64(a.Value)/100.0, 1.0)
	case "available_spare":
		// The drive firmware sets its own threshold; Status reflects whether spare
		// has dropped below that threshold. Use status rather than a fixed scale.
		return attributeStatusSeverity(a.GetStatus())
	case "media_errors":
		return counterSeverity(a.Value)
	case "critical_warning":
		// Bitfield: any non-zero bit indicates a critical controller condition.
		if a.Value > 0 {
			return 1.0
		}
		return 0.0
	default:
		return attributeStatusSeverity(a.GetStatus())
	}
}

// scsiAttributeSeverity computes severity for a SCSI SMART attribute.
func scsiAttributeSeverity(a *measurements.SmartScsiAttribute, id string) float64 {
	switch id {
	case "scsi_grown_defect_list",
		"read_total_uncorrected_errors",
		"write_total_uncorrected_errors":
		return counterSeverity(a.Value)
	default:
		return attributeStatusSeverity(a.GetStatus())
	}
}

// counterSeverity converts a raw counter value to a severity in [0.0, 1.0].
// The scale is stepped rather than linear: even a single event is a warning
// signal on attributes that should remain at zero (reallocated sectors, etc.).
//
//	0     -> 0.00
//	1–4   -> 0.25
//	5–19  -> 0.50
//	20–49 -> 0.75
//	50+   -> 1.00
func counterSeverity(value int64) float64 {
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

// attributeStatusSeverity maps Scrutiny's bitwise AttributeStatus to a severity
// value. Failed takes precedence over Warning.
func attributeStatusSeverity(status pkg.AttributeStatus) float64 {
	if pkg.AttributeStatusHas(status, pkg.AttributeStatusFailedSmart) ||
		pkg.AttributeStatusHas(status, pkg.AttributeStatusFailedScrutiny) {
		return 1.0
	}
	if pkg.AttributeStatusHas(status, pkg.AttributeStatusWarningScrutiny) {
		return 0.5
	}
	return 0.0
}
