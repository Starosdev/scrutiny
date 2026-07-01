package handler

import (
	"net/http"
	"sort"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	"github.com/analogj/scrutiny/webapp/backend/pkg/database"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/thresholds"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// GetDeviceDriveProfile returns consumer drive profile match diagnostics for a
// device: which catalog entry matched (if any), the match method, the
// confidence gate result, which overrides would be applied, and why the drive
// falls back to generic ATA rules when no profile is in effect.
//
// The catalog match is always computed (even when the feature is globally
// disabled) so operators can inspect what WOULD happen; the Applied flag
// reflects the effective state.
//
// Response: models.DriveProfileInspectionResponse
func GetDeviceDriveProfile(c *gin.Context) {
	logger := c.MustGet("LOGGER").(*logrus.Entry)
	deviceRepo := c.MustGet("DEVICE_REPOSITORY").(database.DeviceRepo)
	appConfig := c.MustGet("CONFIG").(config.Interface)

	device, err := ResolveDevice(c, logger, deviceRepo)
	if err != nil {
		return
	}

	profilesEnabled := consumerDriveProfilesEnabled(appConfig)
	denied := consumerDriveProfileDenylist(appConfig)

	inspection := models.DriveProfileInspection{
		DeviceWWN:       device.WWN,
		DeviceProtocol:  device.DeviceProtocol,
		ModelFamily:     device.ModelFamily,
		ModelName:       device.ModelName,
		ProfilesEnabled: profilesEnabled,
		Denylist:        sortedDenylistKeys(denied),
		CatalogVersion:  thresholds.ConsumerDriveProfileCatalogVersion(),
	}

	match := thresholds.MatchConsumerDriveProfile(device.DeviceProtocol, device.ModelFamily, device.ModelName, denied)
	populateDriveProfileMatch(&inspection, match, profilesEnabled)

	c.JSON(http.StatusOK, models.DriveProfileInspectionResponse{
		Success: true,
		Data:    inspection,
	})
}

// populateDriveProfileMatch fills match, override, and fallback-reason fields
// on the inspection from a catalog match result.
func populateDriveProfileMatch(inspection *models.DriveProfileInspection, match *thresholds.ConsumerDriveProfileMatch, profilesEnabled bool) {
	if match == nil {
		switch {
		case inspection.DeviceProtocol != pkg.DeviceProtocolAta:
			inspection.FallbackReason = "Device protocol is not ATA; consumer drive profiles apply to ATA drives only."
		case !profilesEnabled:
			inspection.FallbackReason = "Consumer drive profiles are disabled globally in Settings."
		default:
			inspection.FallbackReason = "No profile in the bundled catalog matched this drive's model family or model name."
		}
		return
	}

	profile := match.Profile
	inspection.Matched = true
	inspection.MatchMethod = string(match.Method)
	inspection.MatchedValue = match.MatchedValue
	inspection.ProfileFamily = profile.ModelFamily
	inspection.ProfileVendor = profile.Vendor
	inspection.ProfileSource = profile.Source
	inspection.SampleCount = profile.SampleCount
	inspection.MinSamples = profile.EffectiveMinSamples()
	inspection.ConfidenceMet = profile.MeetsConfidenceThreshold()
	inspection.ObservedThresholdAttributes = sortedIntKeys(profile.AtaObservedThresholds)
	inspection.CounterSeverityAttributes = sortedStringKeys(profile.AtaCounterSeverityOverrides)

	if !profilesEnabled {
		inspection.FallbackReason = "Consumer drive profiles are disabled globally in Settings; the matched profile is not applied."
		return
	}

	switch match.SkipReason {
	case thresholds.ProfileSkipReasonFamilyDenylisted:
		inspection.FallbackReason = "Matched family is denylisted in Settings; generic ATA rules are in effect."
	case thresholds.ProfileSkipReasonBelowConfidence:
		inspection.FallbackReason = "Matched profile does not meet its minimum sample-count confidence gate; generic ATA rules are in effect."
	default:
		inspection.Applied = match.Applied
	}
}

func sortedDenylistKeys(denied map[string]struct{}) []string {
	if len(denied) == 0 {
		return nil
	}
	keys := make([]string, 0, len(denied))
	for key := range denied {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedIntKeys(m map[int][]thresholds.ObservedThreshold) []int {
	if len(m) == 0 {
		return nil
	}
	keys := make([]int, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	return keys
}

func sortedStringKeys(m map[string]thresholds.CounterSeverityProfile) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
