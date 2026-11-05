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

func TestAtaReplacementRiskWeightsCoverEveryAttributeScrutinyCanActOn(t *testing.T) {
	// Issue #1095: an attribute carrying observed thresholds can be marked WARN or
	// FAILED by ValidateThreshold. One that carries no weight shows that status on
	// the detail page and never appears in the replacement-risk table, which is the
	// second half of the complaint in #1084.
	weighted := make(map[string]float64, len(AtaReplacementRiskWeights))
	for _, weight := range AtaReplacementRiskWeights {
		weighted[weight.AttributeID] = weight.Weight
	}

	for attributeID := range AtaMetadata {
		metadata := AtaMetadata[attributeID]
		if !metadata.Critical && len(metadata.ObservedThresholds) == 0 {
			continue
		}
		weight, ok := weighted[strconv.Itoa(attributeID)]
		require.True(t, ok, "attribute %d (%s) can take a status but carries no weight", attributeID, metadata.DisplayName)
		require.Greater(t, weight, 0.0)
	}
}

func TestAtaResidualWeightsRankCriticalAboveNonCritical(t *testing.T) {
	// Non-critical cutoffs warn where a critical attribute would already have
	// failed, so a non-critical residual carries less weight.
	require.Greater(t, float64(AtaCriticalResidualWeight), float64(AtaObservedThresholdResidualWeight))

	curated := make(map[string]struct{}, len(ataCuratedReplacementRiskWeights))
	for _, weight := range ataCuratedReplacementRiskWeights {
		curated[weight.AttributeID] = struct{}{}
	}

	checked := 0
	for _, weight := range AtaReplacementRiskWeights {
		if _, isCurated := curated[weight.AttributeID]; isCurated {
			continue
		}
		attributeID, err := strconv.Atoi(weight.AttributeID)
		require.NoError(t, err)
		if AtaMetadata[attributeID].Critical {
			require.Equal(t, float64(AtaCriticalResidualWeight), weight.Weight, "attribute %d", attributeID)
		} else {
			require.Equal(t, float64(AtaObservedThresholdResidualWeight), weight.Weight, "attribute %d", attributeID)
		}
		checked++
	}
	require.NotZero(t, checked, "no residual weights found; this test would pass vacuously")
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
		require.Contains(t,
			[]float64{float64(AtaCriticalResidualWeight), float64(AtaObservedThresholdResidualWeight)},
			weight.Weight)
	}
}
