package thresholds

import (
	"sort"
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

func TestAtaReplacementRiskWeightsCoverCriticalAttributesWithoutObservedThresholds(t *testing.T) {
	// Issue #1091: residuals used to require observed thresholds, because a critical
	// attribute with no buckets warned on every drive that reported it. #1088 made
	// ValidateThreshold return without a status when no bucket applies, so these
	// attributes now carry only real signals (manufacturer FAILING_NOW, operator
	// overrides) and excluding them suppressed those signals instead of noise.
	bucketless := []int{}
	for attributeID := range AtaMetadata {
		if AtaMetadata[attributeID].Critical && len(AtaMetadata[attributeID].ObservedThresholds) == 0 {
			bucketless = append(bucketless, attributeID)
		}
	}
	sort.Ints(bucketless)
	require.NotEmpty(t, bucketless, "attributes 184 and 201 are the live cases; this test is meaningless without one")

	weighted := make(map[string]struct{}, len(AtaReplacementRiskWeights))
	for _, weight := range AtaReplacementRiskWeights {
		weighted[weight.AttributeID] = struct{}{}
	}
	for _, attributeID := range bucketless {
		_, ok := weighted[strconv.Itoa(attributeID)]
		require.True(t, ok, "critical attribute %d carries no weight", attributeID)
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
