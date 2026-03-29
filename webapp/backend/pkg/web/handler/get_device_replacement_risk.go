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
// given device based on its latest SMART data.
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

	smartResults, err := deviceRepo.GetLatestSmartSubmission(c, device.WWN)
	if err != nil {
		logger.Errorln("An error occurred retrieving latest SMART submission for replacement risk", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false})
		return
	}

	var latestAttrs map[string]measurements.SmartAttribute
	if len(smartResults) > 0 {
		latestAttrs = smartResults[0].Attributes
	}

	weights := thresholds.ReplacementRiskWeightsForProtocol(device.DeviceProtocol)

	contributions := make([]models.AttributeContribution, 0)
	totalScore := 0.0

	if weights != nil {
		contributions = make([]models.AttributeContribution, 0, len(weights))
		for _, w := range weights {
			contrib := models.AttributeContribution{
				AttributeID: w.AttributeID,
				DisplayName: w.DisplayName,
				Weight:      w.Weight,
			}

			if latestAttrs != nil {
				if attr, ok := latestAttrs[w.AttributeID]; ok {
					severity := replacementRiskSeverity(attr, w.AttributeID)
					contrib.Score = math.Round(severity*w.Weight*100) / 100
					contrib.Value = attr.GetTransformedValue()
				}
			}

			totalScore += contrib.Score
			contributions = append(contributions, contrib)
		}
	}

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
		TrendBonus:     0,
		ComputedAt:     time.Now().UTC(),
	}

	c.JSON(http.StatusOK, models.ReplacementRiskResponse{
		Success: true,
		Data:    riskScore,
	})
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
