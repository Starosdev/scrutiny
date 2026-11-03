package handler

import (
	"time"

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
	device *models.Device,
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
	history, err := deviceRepo.GetSmartAttributeHistory(c, device.DeviceID, "month", 0, 0, nil)
	if err != nil {
		logger.Warnf("Could not retrieve SMART history for replacement risk notification: %v", err)
	}

	// The query above asks for the "month" bucket, so the notification path uses
	// the same 30-day window as the endpoint's default.
	oldestAttrs := oldestAttributesFromHistory(history, trendWindowCutoff(models.TrendWindow30Days, time.Now().UTC()))

	weights := thresholds.ReplacementRiskWeightsForProtocol(device.DeviceProtocol)
	var profile *thresholds.ConsumerDriveProfile
	if consumerDriveProfilesEnabled(appConfig) {
		if match := thresholds.MatchConsumerDriveProfile(device.DeviceProtocol, device.ModelFamily, device.ModelName, consumerDriveProfileDenylist(appConfig)); match != nil && match.Applied {
			profile = match.Profile
		}
	}
	_, totalScore, totalTrendBonus := computeRiskContributions(weights, latestAttrs, oldestAttrs, profile)

	score, _ := clampRiskTotals(totalScore, totalTrendBonus)
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
