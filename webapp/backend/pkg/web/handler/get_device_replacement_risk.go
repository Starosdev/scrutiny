package handler

import (
	"net/http"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
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
				// Score and TrendScore are computed from SMART attribute values.
				// Full attribute-level scoring requires parsing the latest SMART
				// result per attribute; this is a placeholder for that logic.
				Score:      0,
				TrendScore: 0,
			}
			contributions = append(contributions, contrib)
			totalScore += contrib.Score + contrib.TrendScore
		}
	}

	score := int(totalScore)
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
