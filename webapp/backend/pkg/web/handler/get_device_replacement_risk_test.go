package handler

import (
	"fmt"
	"testing"

	"github.com/analogj/scrutiny/webapp/backend/pkg/config"
	mock_config "github.com/analogj/scrutiny/webapp/backend/pkg/config/mock"
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
