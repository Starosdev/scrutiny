package database

import (
	"testing"

	"github.com/analogj/scrutiny/webapp/backend/pkg/thresholds"
	"github.com/stretchr/testify/require"
)

func TestSummaryRiskWeightsMatchTheFullScorer(t *testing.T) {
	// The dashboard badge scores a subset of attributes and used to repeat their
	// weights as literals, which went stale the first time the ATA table was
	// rebalanced. Every covered attribute must resolve to a real table weight.
	for protocol, attributeIDs := range map[string][]string{
		"ATA":  ataSummaryAttributes,
		"NVMe": nvmeSummaryAttributes,
		"SCSI": scsiSummaryAttributes,
	} {
		for _, attributeID := range attributeIDs {
			weight, ok := thresholds.ReplacementRiskWeightFor(protocol, attributeID)
			require.True(t, ok, "%s summary scores attribute %s, which carries no replacement-risk weight", protocol, attributeID)
			require.Greater(t, weight, 0.0)
		}
		require.Greater(t, summaryMaxWeight(protocol, attributeIDs), 0.0)
	}
}

func TestSummaryRiskScoreScalesOverCoveredWeight(t *testing.T) {
	// A fully degraded covered set scores 100; a single mid-severity counter scores
	// its own share of the covered weight.
	full := map[string]interface{}{
		"attr.5.raw_value":   int64(100),
		"attr.197.raw_value": int64(100),
		"attr.198.raw_value": int64(100),
	}
	score, _ := summaryRiskScore("ATA", full)
	require.Equal(t, 100, score)

	partial := map[string]interface{}{"attr.5.raw_value": int64(18)}
	attr5Weight, ok := thresholds.ReplacementRiskWeightFor("ATA", "5")
	require.True(t, ok)

	score, _ = summaryRiskScore("ATA", partial)
	expected := int(0.5 * attr5Weight / summaryMaxWeight("ATA", ataSummaryAttributes) * 100)
	require.InDelta(t, expected, score, 1)
}
