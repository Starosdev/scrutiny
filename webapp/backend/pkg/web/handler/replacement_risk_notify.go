package handler

import (
	"math"

	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	"github.com/analogj/scrutiny/webapp/backend/pkg/notify"
	"github.com/analogj/scrutiny/webapp/backend/pkg/thresholds"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// maybeNotifyReplacementRisk computes the replacement risk score for the device
// using the freshly-saved SMART attributes and, if the score meets the configured
// category threshold, dispatches a notification.
func maybeNotifyReplacementRisk(
	c *gin.Context,
	logger logrus.FieldLogger,
	appConfig config.Interface,
	deviceRepo database.DeviceRepo,
	device models.Device,
	latestAttrs map[string]measurements.SmartAttribute,
	settings *models.Settings,
) {
	if device.Muted {
		return
	}

	notifyCategory := settings.Metrics.ReplacementRiskNotifyCategory
	if notifyCategory == "" {
		notifyCategory = "replace_soon"
	}

	// Retrieve oldest snapshot over the 30-day trend window (best-effort).
	history, err := deviceRepo.GetSmartAttributeHistory(c, device.WWN, "month", 0, 0, nil)
	if err != nil {
		logger.Warnf("Could not retrieve SMART history for replacement risk notification: %v", err)
	}

	var oldestAttrs map[string]measurements.SmartAttribute
	if len(history) > 0 {
		oldestAttrs = history[0].Attributes
	}

	weights := thresholds.ReplacementRiskWeightsForProtocol(device.DeviceProtocol)
	_, totalScore, _ := computeRiskContributions(weights, latestAttrs, oldestAttrs)

	score := int(math.Round(totalScore))
	if score > 100 {
		score = 100
	}
	category := models.ScoreToRiskCategory(score)

	if !notify.ReplacementRiskMeetsThreshold(category, notifyCategory) {
		return
	}

	riskNotify := notify.NewReplacementRisk(logger, appConfig, device, score, category)
	riskNotify.LoadDatabaseUrls(c, deviceRepo)

	if gateVal, exists := c.Get("NOTIFICATION_GATE"); exists {
		if gate, ok := gateVal.(*notify.NotificationGate); ok {
			gate.TrySend(&riskNotify, settings, false)
			return
		}
	}

	if sendErr := riskNotify.Send(); sendErr != nil {
		logger.Warnf("Failed to send replacement risk notification for device %s: %v", device.DeviceID, sendErr)
	}
}
