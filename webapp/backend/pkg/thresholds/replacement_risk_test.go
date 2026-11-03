package thresholds

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAtaCuratedReplacementRiskWeightsSumTo100(t *testing.T) {
	total := 0.0
	for _, weight := range ataCuratedReplacementRiskWeights {
		total += weight.Weight
	}
	require.Equal(t, 100.0, total)
}

func TestAtaReplacementRiskWeightsCoverEveryCriticalAttribute(t *testing.T) {
	// Issue #1084: attributes marked critical in metadata carried no weight, so they
	// could not reach the score or the contributions table. Residual weights are
	// derived from metadata to keep that from recurring when a new critical
	// attribute is added.
	//
	// Observed thresholds are not part of the condition. #1088 stopped a bucket-less
	// critical attribute from warning unconditionally, so every critical attribute is
	// scoreable and must carry a weight.
	weighted := make(map[string]struct{}, len(AtaReplacementRiskWeights))
	for _, weight := range AtaReplacementRiskWeights {
		weighted[weight.AttributeID] = struct{}{}
	}

	for attributeID := range AtaMetadata {
		metadata := AtaMetadata[attributeID]
		if !metadata.Critical {
			continue
		}
		_, ok := weighted[strconv.Itoa(attributeID)]
		require.True(t, ok, "critical attribute %d (%s) has no replacement-risk weight", attributeID, metadata.DisplayName)
	}
}

func TestAtaReplacementRiskWeightsScoreCriticalAttributesWithoutObservedThresholds(t *testing.T) {
	// This test used to assert the opposite. ValidateThreshold once warned on any
	// critical attribute whose value matched no bucket, so an attribute with no
	// buckets at all warned on every drive that reported it, and weighting that
	// warning would have charged healthy drives for a metadata gap. #1088 removed
	// that unconditional warning, so a bucket-less critical attribute now leaves the
	// status untouched and the manufacturer threshold decides it. It therefore
	// contributes only when a drive actually fails that threshold, which is a drive
	// signal and belongs in the score.
	//
	// Attributes 184 and 201 are the live cases. They were silently unscored between
	// #1088 and this fix.
	weighted := map[string]float64{}
	for _, weight := range AtaReplacementRiskWeights {
		weighted[weight.AttributeID] = weight.Weight
	}

	for _, attributeID := range []int{184, 201} {
		require.True(t, AtaMetadata[attributeID].Critical)
		require.Empty(t, AtaMetadata[attributeID].ObservedThresholds)

		weight, ok := weighted[strconv.Itoa(attributeID)]
		require.True(t, ok, "critical attribute %d must be scored from its manufacturer threshold", attributeID)
		require.Equal(t, float64(AtaCriticalResidualWeight), weight, "attribute %d is a residual critical attribute", attributeID)
	}
}

func TestReplacementRiskWeightForMatchesTheTable(t *testing.T) {
	weight, ok := ReplacementRiskWeightFor("ATA", "5")
	require.True(t, ok)
	require.Equal(t, 22.0, weight)

	_, ok = ReplacementRiskWeightFor("ATA", "nonexistent")
	require.False(t, ok)

	_, ok = ReplacementRiskWeightFor("unknown-protocol", "5")
	require.False(t, ok)
}

func TestAtaReplacementRiskWeightsAreUniqueAndOrdered(t *testing.T) {
	seen := make(map[string]struct{}, len(AtaReplacementRiskWeights))
	for _, weight := range AtaReplacementRiskWeights {
		_, duplicate := seen[weight.AttributeID]
		require.False(t, duplicate, "duplicate replacement-risk weight for attribute %s", weight.AttributeID)
		seen[weight.AttributeID] = struct{}{}
		require.NotEmpty(t, weight.DisplayName, "attribute %s has no display name", weight.AttributeID)
	}

	// Residual entries are appended in ascending attribute order, so the table is
	// stable across runs and the API response order does not depend on map iteration.
	previous := -1
	for _, weight := range AtaReplacementRiskWeights[len(ataCuratedReplacementRiskWeights):] {
		attributeID, err := strconv.Atoi(weight.AttributeID)
		require.NoError(t, err)
		require.Greater(t, attributeID, previous)
		previous = attributeID
		require.Equal(t, float64(AtaCriticalResidualWeight), weight.Weight)
	}
}
