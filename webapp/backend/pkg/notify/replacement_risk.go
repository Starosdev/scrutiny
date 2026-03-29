package notify

import (
	"fmt"
	"strings"

	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/sirupsen/logrus"
)

// NewReplacementRisk constructs a Notify instance for a drive replacement risk alert.
func NewReplacementRisk(logger logrus.FieldLogger, appconfig config.Interface, device models.Device, score int, category models.RiskCategory) Notify {
	payload := NewPayload(device, false)
	payload.FailureType = NotifyFailureTypeReplacementRisk

	deviceIdentifier := device.DeviceName
	if label := strings.TrimSpace(device.Label); len(label) > 0 {
		deviceIdentifier = fmt.Sprintf(fmtLabelWithName, label, device.DeviceName)
	}

	if hostId := strings.TrimSpace(device.HostId); len(hostId) > 0 {
		payload.Subject = fmt.Sprintf(
			"Scrutiny replacement risk (%s, score %d) detected on [host]device: [%s]%s",
			category, score, hostId, deviceIdentifier,
		)
	} else {
		payload.Subject = fmt.Sprintf(
			"Scrutiny replacement risk (%s, score %d) detected on device: %s",
			category, score, deviceIdentifier,
		)
	}

	parts := []string{
		fmt.Sprintf("Scrutiny replacement risk notification for device: %s", device.DeviceName),
	}
	if hostId := strings.TrimSpace(device.HostId); len(hostId) > 0 {
		parts = append(parts, fmt.Sprintf(fmtHostId, hostId))
	}
	parts = append(parts,
		fmt.Sprintf("Failure Type: %s", NotifyFailureTypeReplacementRisk),
		fmt.Sprintf("Risk Category: %s", category),
		fmt.Sprintf("Risk Score: %d/100", score),
		fmt.Sprintf("Device Name: %s", device.DeviceName),
		fmt.Sprintf(fmtDeviceSerial, device.SerialNumber),
		fmt.Sprintf("Device Type: %s", device.DeviceType),
	)
	if label := strings.TrimSpace(device.Label); len(label) > 0 {
		parts = append(parts, fmt.Sprintf(fmtDeviceLabel, label))
	}
	parts = append(parts, "", fmt.Sprintf(fmtDate, payload.Date))
	payload.Message = strings.Join(parts, "\n")

	return Notify{
		Logger:  logger,
		Config:  appconfig,
		Payload: payload,
	}
}

// riskCategoryOrder maps risk categories to a numeric rank for threshold comparison.
var riskCategoryOrder = map[models.RiskCategory]int{
	models.RiskCategoryHealthy:         0,
	models.RiskCategoryMonitor:          1,
	models.RiskCategoryPlanReplacement:  2,
	models.RiskCategoryReplaceSoon:      3,
}

// ReplacementRiskMeetsThreshold returns true if category is at or above minCategory.
func ReplacementRiskMeetsThreshold(category models.RiskCategory, minCategory string) bool {
	min := models.RiskCategory(minCategory)
	minOrder, minOk := riskCategoryOrder[min]
	catOrder, catOk := riskCategoryOrder[category]
	if !minOk || !catOk {
		return false
	}
	return catOrder >= minOrder
}
