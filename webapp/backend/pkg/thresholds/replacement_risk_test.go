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
	// Issue #1084: attribute 201 was marked critical in metadata, warned on a real
	// drive, and carried no weight, so it could not reach the score or the
	// contributions table. Residual weights are derived from metadata to keep that
	// from recurring when a new critical attribute is added.
	weighted := make(map[string]struct{}, len(AtaReplacementRiskWeights))
	for _, weight := range AtaReplacementRiskWeights {
		weighted[weight.AttributeID] = struct{}{}
	}

	for attributeID, metadata := range AtaMetadata {
		if !metadata.Critical {
			continue
		}
		_, ok := weighted[strconv.Itoa(attributeID)]
		require.True(t, ok, "critical attribute %d (%s) has no replacement-risk weight", attributeID, metadata.DisplayName)
	}
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
