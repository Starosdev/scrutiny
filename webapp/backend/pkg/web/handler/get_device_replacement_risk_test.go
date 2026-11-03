package handler

import (
	"fmt"
	"testing"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	mock_config "github.com/analogj/scrutiny/webapp/backend/pkg/config/mock"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	"github.com/analogj/scrutiny/webapp/backend/pkg/thresholds"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestCounterSeverityWithProfileUsesOverride(t *testing.T) {
	profile := &thresholds.ConsumerDriveProfile{
		AtaCounterSeverityOverrides: map[string]thresholds.CounterSeverityProfile{
			"5": {Low: 0, Moderate: 2, High: 4, Critical: 8},
		},
	}

	require.Equal(t, 0.25, counterSeverityWithProfile(1, profile, "5"))
	require.Equal(t, 0.50, counterSeverityWithProfile(4, profile, "5"))
	require.Equal(t, 0.75, counterSeverityWithProfile(8, profile, "5"))
	require.Equal(t, 1.0, counterSeverityWithProfile(9, profile, "5"))
}

func TestCounterSeverityWithProfileFallsBack(t *testing.T) {
	require.Equal(t, 0.50, counterSeverityWithProfile(7, nil, "5"))
}

func TestComputeRiskContributions_UsesConsumerDriveProfile(t *testing.T) {
	weights := []thresholds.ReplacementRiskWeight{
		{AttributeID: "5", DisplayName: "Reallocated Sector Count", Weight: 25, TrendMultiplier: 1.5},
	}
	latest := map[string]measurements.SmartAttribute{
		"5": &measurements.SmartAtaAttribute{AttributeId: 5, RawValue: 3},
	}
	oldest := map[string]measurements.SmartAttribute{
		"5": &measurements.SmartAtaAttribute{AttributeId: 5, RawValue: 0},
	}
	profile := &thresholds.ConsumerDriveProfile{
		AtaCounterSeverityOverrides: map[string]thresholds.CounterSeverityProfile{
			"5": {Low: 0, Moderate: 1, High: 2, Critical: 3},
		},
	}

	contributions, totalScore, totalTrendBonus := computeRiskContributions(weights, latest, oldest, profile)

	require.Len(t, contributions, 1)
	require.Equal(t, 18.75, contributions[0].Score)
	require.InDelta(t, 28.125, contributions[0].TrendScore, 0.01)
	require.InDelta(t, 46.875, totalScore, 0.01)
	require.InDelta(t, 28.125, totalTrendBonus, 0.01)
}

func TestComputeRiskContributions_FallbackWithoutProfile(t *testing.T) {
	weights := []thresholds.ReplacementRiskWeight{
		{AttributeID: "5", DisplayName: "Reallocated Sector Count", Weight: 25, TrendMultiplier: 1.5},
	}
	latest := map[string]measurements.SmartAttribute{
		"5": &measurements.SmartAtaAttribute{AttributeId: 5, RawValue: 3},
	}

	contributions, totalScore, totalTrendBonus := computeRiskContributions(weights, latest, nil, nil)

	require.Len(t, contributions, 1)
	require.Equal(t, 6.25, contributions[0].Score)
	require.Equal(t, 0.0, contributions[0].TrendScore)
	require.Equal(t, 6.25, totalScore)
	require.Equal(t, 0.0, totalTrendBonus)
}

func TestWD102KRYZProfileReplacementRiskBoundaries(t *testing.T) {
	profile, ok := thresholds.LookupConsumerDriveProfile("ATA", "", "WDC WD102KRYZ-01A5AB0")
	require.True(t, ok)
	require.NotNil(t, profile)

	weightsByAttribute := make(map[string]thresholds.ReplacementRiskWeight)
	for _, weight := range thresholds.ReplacementRiskWeightsForProtocol("ATA") {
		weightsByAttribute[weight.AttributeID] = weight
	}

	testCases := []struct {
		attributeID string
		values      []int64
		severities  []float64
	}{
		{attributeID: "5", values: []int64{0, 1, 2, 4, 5, 10, 11}, severities: []float64{0, 0.25, 0.50, 0.50, 0.75, 0.75, 1}},
		{attributeID: "10", values: []int64{0, 1, 2, 3, 4, 5}, severities: []float64{0, 0.25, 0.50, 0.75, 0.75, 1}},
		{attributeID: "196", values: []int64{0, 1, 2, 4, 5, 10, 11}, severities: []float64{0, 0.25, 0.50, 0.50, 0.75, 0.75, 1}},
		{attributeID: "197", values: []int64{0, 1, 2, 3, 4, 8, 9}, severities: []float64{0, 0.25, 0.50, 0.50, 0.75, 0.75, 1}},
		{attributeID: "198", values: []int64{0, 1, 2, 3, 4, 8, 9}, severities: []float64{0, 0.25, 0.50, 0.50, 0.75, 0.75, 1}},
	}

	for _, testCase := range testCases {
		weight, found := weightsByAttribute[testCase.attributeID]
		require.True(t, found, "missing replacement-risk weight for attribute %s", testCase.attributeID)

		for index, value := range testCase.values {
			t.Run(fmt.Sprintf("attribute_%s_value_%d", testCase.attributeID, value), func(t *testing.T) {
				latest := map[string]measurements.SmartAttribute{
					testCase.attributeID: &measurements.SmartAtaAttribute{RawValue: value},
				}

				contributions, totalScore, totalTrendBonus := computeRiskContributions(
					[]thresholds.ReplacementRiskWeight{weight},
					latest,
					nil,
					profile,
				)

				expectedScore := testCase.severities[index] * weight.Weight
				require.Len(t, contributions, 1)
				require.Equal(t, expectedScore, contributions[0].Score)
				require.Equal(t, expectedScore, totalScore)
				require.Zero(t, totalTrendBonus)
			})
		}
	}
}

func TestConsumerDriveProfilesEnabledDefaultsTrueWhenUnset(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	cfg := mock_config.NewMockInterface(mockCtrl)
	key := config.DB_USER_SETTINGS_SUBKEY + ".metrics.consumer_drive_profiles_enabled"
	cfg.EXPECT().IsSet(key).Return(false)

	require.True(t, consumerDriveProfilesEnabled(cfg))
}

func TestConsumerDriveProfilesEnabledUsesStoredValue(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	cfg := mock_config.NewMockInterface(mockCtrl)
	key := config.DB_USER_SETTINGS_SUBKEY + ".metrics.consumer_drive_profiles_enabled"
	cfg.EXPECT().IsSet(key).Return(true)
	cfg.EXPECT().GetBool(key).Return(false)

	require.False(t, consumerDriveProfilesEnabled(cfg))
}

func TestOldestAttributesFromHistoryReadsTheLastEntry(t *testing.T) {
	// GetSmartAttributeHistory returns newest-first. Reading the first entry
	// compares the latest submission against itself, which zeroes every trend.
	history := []measurements.Smart{
		{Attributes: map[string]measurements.SmartAttribute{
			"5": &measurements.SmartAtaAttribute{AttributeId: 5, RawValue: 18},
		}},
		{Attributes: map[string]measurements.SmartAttribute{
			"5": &measurements.SmartAtaAttribute{AttributeId: 5, RawValue: 1},
		}},
	}

	oldest := oldestAttributesFromHistory(history, time.Now().UTC().AddDate(0, 0, -30))

	require.NotNil(t, oldest)
	require.Equal(t, int64(1), oldest["5"].(*measurements.SmartAtaAttribute).RawValue)
	require.Nil(t, oldestAttributesFromHistory(nil, time.Now().UTC()))
}

func TestOldestAttributesFromHistorySkipsEntriesOlderThanTheWindow(t *testing.T) {
	// A 90-day window queries the "year" duration key, whose range reaches back a
	// full year, so the last entry in the slice can predate the window by months.
	now := time.Now().UTC()
	history := []measurements.Smart{
		{Date: now, Attributes: map[string]measurements.SmartAttribute{
			"5": &measurements.SmartAtaAttribute{AttributeId: 5, RawValue: 18},
		}},
		{Date: now.AddDate(0, 0, -80), Attributes: map[string]measurements.SmartAttribute{
			"5": &measurements.SmartAtaAttribute{AttributeId: 5, RawValue: 9},
		}},
		{Date: now.AddDate(0, 0, -300), Attributes: map[string]measurements.SmartAttribute{
			"5": &measurements.SmartAtaAttribute{AttributeId: 5, RawValue: 1},
		}},
	}

	oldest := oldestAttributesFromHistory(history, trendWindowCutoff(models.TrendWindow90Days, now))
	require.Equal(t, int64(9), oldest["5"].(*measurements.SmartAtaAttribute).RawValue)

	oldest = oldestAttributesFromHistory(history, trendWindowCutoff(models.TrendWindow7Days, now))
	require.Equal(t, int64(18), oldest["5"].(*measurements.SmartAtaAttribute).RawValue)
}

func TestClampRiskTotalsKeepsTheBonusUnderTheCeiling(t *testing.T) {
	// The score clamps to 100, so reporting the raw bonus beside it lets a card
	// read "100 /100 +118 trend".
	score, bonus := clampRiskTotals(218, 118)
	require.Equal(t, 100, score)
	require.Equal(t, 0.0, bonus)

	score, bonus = clampRiskTotals(130, 60)
	require.Equal(t, 100, score)
	require.Equal(t, 30.0, bonus)

	// Under the ceiling, both pass through, rounded for display.
	score, bonus = clampRiskTotals(31.25, 18.75)
	require.Equal(t, 31, score)
	require.Equal(t, 18.75, bonus)
}

func TestComputeRiskContributionsScoresTheTrendOverTheWindow(t *testing.T) {
	// Issue #1084: a drive whose reallocated sector count climbed 1 -> 18 over the
	// window reported no trend at all, because the oldest snapshot was the newest one.
	weights := []thresholds.ReplacementRiskWeight{
		{AttributeID: "5", DisplayName: "Reallocated Sector Count", Weight: 25, TrendMultiplier: 1.5},
	}
	history := []measurements.Smart{
		{Attributes: map[string]measurements.SmartAttribute{
			"5": &measurements.SmartAtaAttribute{AttributeId: 5, RawValue: 18},
		}},
		{Attributes: map[string]measurements.SmartAttribute{
			"5": &measurements.SmartAtaAttribute{AttributeId: 5, RawValue: 1},
		}},
	}
	latest := map[string]measurements.SmartAttribute{
		"5": &measurements.SmartAtaAttribute{AttributeId: 5, RawValue: 18},
	}

	contributions, totalScore, totalTrendBonus := computeRiskContributions(
		weights, latest, oldestAttributesFromHistory(history, time.Now().UTC().AddDate(0, 0, -30)), nil)

	require.Len(t, contributions, 1)
	require.Equal(t, 12.5, contributions[0].Score)
	require.Equal(t, 18.75, contributions[0].TrendScore)
	require.Equal(t, 31.25, totalScore)
	require.Equal(t, 18.75, totalTrendBonus)
}

func TestComputeRiskContributionsReportsTheValueItScored(t *testing.T) {
	// Issue #1084: the contributions table read TransformedValue, which is zero for
	// every attribute without a metadata transform, so a score of 12.5 was reported
	// next to a value of 0.
	weights := []thresholds.ReplacementRiskWeight{
		{AttributeID: "5", DisplayName: "Reallocated Sector Count", Weight: 25, TrendMultiplier: 1.5},
	}
	latest := map[string]measurements.SmartAttribute{
		"5": &measurements.SmartAtaAttribute{AttributeId: 5, RawValue: 18, TransformedValue: 0},
	}

	contributions, _, _ := computeRiskContributions(weights, latest, nil, nil)

	require.Len(t, contributions, 1)
	require.Equal(t, int64(18), contributions[0].Value)
	require.Equal(t, 12.5, contributions[0].Score)
}

func TestComputeRiskContributionsReportsChartedValueForStatusScoredAttributes(t *testing.T) {
	// Attribute 184 is a residual critical attribute: scored from its status, and
	// charted from its normalized value rather than its raw value.
	weights := thresholds.ReplacementRiskWeightsForProtocol("ATA")
	latest := map[string]measurements.SmartAttribute{
		"184": &measurements.SmartAtaAttribute{
			AttributeId: 184,
			Value:       92,
			RawValue:    4096,
			Status:      pkg.AttributeStatusWarningScrutiny,
		},
	}

	contributions, _, _ := computeRiskContributions(weights, latest, nil, nil)

	var endToEnd *models.AttributeContribution
	for index := range contributions {
		if contributions[index].AttributeID == "184" {
			endToEnd = &contributions[index]
		}
	}

	require.NotNil(t, endToEnd, "attribute 184 must carry a replacement-risk weight")
	require.Equal(t, int64(92), endToEnd.Value, "normalized value, not the raw value")
	require.Equal(t, 1.5, endToEnd.Score)
}

func TestContributionsStayPreClampWhileTotalsCarryTheCeiling(t *testing.T) {
	// Documented relationship: rows report per-attribute contributions, the
	// response totals carry the clamp, so on a saturated drive the rows sum past
	// the reported score and trend bonus.
	weights := []thresholds.ReplacementRiskWeight{
		{AttributeID: "percentage_used", DisplayName: "Percentage Used", Weight: 40, TrendMultiplier: 1.0},
		{AttributeID: "media_errors", DisplayName: "Media and Data Integrity Errors", Weight: 20, TrendMultiplier: 1.5},
	}
	oldest := map[string]measurements.SmartAttribute{
		"percentage_used": &measurements.SmartNvmeAttribute{AttributeId: "percentage_used", Value: 90},
		"media_errors":    &measurements.SmartNvmeAttribute{AttributeId: "media_errors", Value: 0},
	}
	latest := map[string]measurements.SmartAttribute{
		"percentage_used": &measurements.SmartNvmeAttribute{AttributeId: "percentage_used", Value: 100},
		"media_errors":    &measurements.SmartNvmeAttribute{AttributeId: "media_errors", Value: 100},
	}

	contributions, totalScore, totalTrendBonus := computeRiskContributions(weights, latest, oldest, nil)
	score, reportedBonus := clampRiskTotals(totalScore, totalTrendBonus)

	var rowScore, rowTrend float64
	for _, contrib := range contributions {
		rowScore += contrib.Score
		rowTrend += contrib.TrendScore
	}

	require.Equal(t, 100, score)
	require.Equal(t, rowTrend, totalTrendBonus, "the raw bonus is the sum of the rows")
	require.Less(t, reportedBonus, rowTrend, "the reported bonus is clamped below the row total")
	require.Greater(t, rowScore+rowTrend, float64(score), "rows are pre-clamp by design")
}

func TestComputeRiskContributionsReportsNvmeAndScsiValues(t *testing.T) {
	nvmeWeights := []thresholds.ReplacementRiskWeight{
		{AttributeID: "percentage_used", DisplayName: "Percentage Used", Weight: 40},
		{AttributeID: "available_spare", DisplayName: "Available Spare", Weight: 30},
		{AttributeID: "media_errors", DisplayName: "Media and Data Integrity Errors", Weight: 20},
		{AttributeID: "critical_warning", DisplayName: "Critical Warning", Weight: 7},
	}
	nvmeLatest := map[string]measurements.SmartAttribute{
		"percentage_used":  &measurements.SmartNvmeAttribute{AttributeId: "percentage_used", Value: 42},
		"available_spare":  &measurements.SmartNvmeAttribute{AttributeId: "available_spare", Value: 7, Status: pkg.AttributeStatusFailedScrutiny},
		"media_errors":     &measurements.SmartNvmeAttribute{AttributeId: "media_errors", Value: 9},
		"critical_warning": &measurements.SmartNvmeAttribute{AttributeId: "critical_warning", Value: 1},
	}

	contributions, _, _ := computeRiskContributions(nvmeWeights, nvmeLatest, nil, nil)
	values := map[string]int64{}
	for _, contrib := range contributions {
		values[contrib.AttributeID] = contrib.Value
	}
	require.Equal(t, int64(42), values["percentage_used"])
	require.Equal(t, int64(7), values["available_spare"], "status-scored, still reports its own value")
	require.Equal(t, int64(9), values["media_errors"])
	require.Equal(t, int64(1), values["critical_warning"])

	scsiWeights := []thresholds.ReplacementRiskWeight{
		{AttributeID: "scsi_grown_defect_list", DisplayName: "Grown Defect List", Weight: 40},
		{AttributeID: "read_errors_corrected_by_eccdelayed", DisplayName: "Read Errors Corrected by ECC (Delayed)", Weight: 5},
	}
	scsiLatest := map[string]measurements.SmartAttribute{
		"scsi_grown_defect_list":              &measurements.SmartScsiAttribute{AttributeId: "scsi_grown_defect_list", Value: 21},
		"read_errors_corrected_by_eccdelayed": &measurements.SmartScsiAttribute{AttributeId: "read_errors_corrected_by_eccdelayed", Value: 3, Status: pkg.AttributeStatusWarningScrutiny},
	}

	contributions, _, _ = computeRiskContributions(scsiWeights, scsiLatest, nil, nil)
	values = map[string]int64{}
	for _, contrib := range contributions {
		values[contrib.AttributeID] = contrib.Value
	}
	require.Equal(t, int64(21), values["scsi_grown_defect_list"])
	require.Equal(t, int64(3), values["read_errors_corrected_by_eccdelayed"], "status-scored, still reports its own value")
}

func TestAttribute187IsCounterScoredNotStatusScored(t *testing.T) {
	// 187 Reported Uncorrectable Errors is a Backblaze key stat with a raw counter.
	// Scoring it from status would leave a PASSED drive with 40 reported errors at
	// zero risk points.
	require.True(t, isAtaCounterAttribute("187"))
	require.False(t, isAtaCounterAttribute("188"), "188 raw is a packed transform, so it stays status-scored")

	weights := thresholds.ReplacementRiskWeightsForProtocol("ATA")
	latest := map[string]measurements.SmartAttribute{
		"187": &measurements.SmartAtaAttribute{AttributeId: 187, RawValue: 40, Status: pkg.AttributeStatusPassed},
	}

	contributions, totalScore, _ := computeRiskContributions(weights, latest, nil, nil)

	var reported *models.AttributeContribution
	for index := range contributions {
		if contributions[index].AttributeID == "187" {
			reported = &contributions[index]
		}
	}
	require.NotNil(t, reported)
	require.Equal(t, int64(40), reported.Value)
	require.Equal(t, 11.25, reported.Score, "counterSeverity(40)=0.75 against weight 15")
	require.Equal(t, 11.25, totalScore)
}
