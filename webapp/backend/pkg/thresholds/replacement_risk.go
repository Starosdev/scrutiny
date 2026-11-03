package thresholds

import (
	"sort"
	"strconv"
)

// ReplacementRiskWeight defines how much a single SMART attribute can contribute
// to the overall replacement risk score. The weight is a fraction of 100; the
// scorer multiplies it by the attribute's normalized severity (0.0-1.0) to get
// the points added to the total score.
type ReplacementRiskWeight struct {
	// AttributeID is the protocol-specific attribute identifier (int key for ATA,
	// string key for NVMe and SCSI).
	AttributeID string

	// DisplayName is the human-readable attribute name.
	DisplayName string

	// Weight is the maximum number of points (out of 100) this attribute can
	// contribute. All weights across a drive type should sum to 100.
	Weight float64

	// TrendMultiplier scales the additional score added when the attribute value
	// is worsening. A value of 1.0 means the trend can double the attribute's
	// contribution at maximum deterioration rate. Set to 0 for attributes where
	// trend analysis is not meaningful.
	TrendMultiplier float64
}

// ataCuratedReplacementRiskWeights defines the hand-tuned attribute weights for
// ATA/SATA hard drives and SSDs. Weights are informed by Backblaze failure
// correlation research
// (https://www.backblaze.com/cloud-storage/resources/hard-drive-test-data),
// whose five key SMART stats (5, 187, 188, 197, 198) all carry weight here.
//
// Attributes with direct failure correlation receive higher weights. Attributes
// that are noisy or vendor-specific (Raw Read Error Rate, Seek Error Rate)
// receive lower weights and rely more on trend analysis.
//
// These weights sum to 100. AtaReplacementRiskWeights appends a residual weight
// for every remaining critical attribute that carries observed thresholds, so
// the ATA table's theoretical maximum exceeds 100; the scorer clamps the final
// score, and the trend bonus it reports, to 100.
var ataCuratedReplacementRiskWeights = []ReplacementRiskWeight{
	{
		AttributeID:     "5",
		DisplayName:     "Reallocated Sector Count",
		Weight:          22,
		TrendMultiplier: 1.5,
	},
	{
		AttributeID:     "197",
		DisplayName:     "Current Pending Sector Count",
		Weight:          18,
		TrendMultiplier: 1.5,
	},
	{
		AttributeID:     "198",
		DisplayName:     "Offline Uncorrectable",
		Weight:          15,
		TrendMultiplier: 1.2,
	},
	{
		AttributeID:     "187",
		DisplayName:     "Reported Uncorrectable Errors",
		Weight:          15,
		TrendMultiplier: 1.5,
	},
	{
		AttributeID:     "188",
		DisplayName:     "Command Timeout",
		Weight:          7,
		TrendMultiplier: 1.0,
	},
	{
		AttributeID:     "196",
		DisplayName:     "Reallocated Event Count",
		Weight:          7,
		TrendMultiplier: 1.0,
	},
	{
		AttributeID:     "10",
		DisplayName:     "Spin Retry Count",
		Weight:          5,
		TrendMultiplier: 1.0,
	},
	{
		AttributeID:     "199",
		DisplayName:     "UDMA CRC Error Count",
		Weight:          4,
		TrendMultiplier: 0.5,
	},
	{
		AttributeID:     "1",
		DisplayName:     "Raw Read Error Rate",
		Weight:          3,
		TrendMultiplier: 0.8,
	},
	{
		AttributeID:     "7",
		DisplayName:     "Seek Error Rate",
		Weight:          2,
		TrendMultiplier: 0.8,
	},
	{
		AttributeID:     "194",
		DisplayName:     "Temperature",
		Weight:          2,
		TrendMultiplier: 0.3,
	},
}

// AtaCriticalResidualWeight is the weight given to an ATA attribute that metadata
// marks critical but the curated table does not cover. It is small on purpose:
// enough that a warning or failure on the attribute reaches the score and the
// contributions table, not enough to rival a Backblaze-correlated signal.
const AtaCriticalResidualWeight = 3

// AtaCriticalResidualTrendMultiplier is the trend multiplier for residual critical
// attributes. They are scored from status, so their trend score comes from a
// status regression over the window rather than from a counter delta.
const AtaCriticalResidualTrendMultiplier = 1.0

// AtaReplacementRiskWeights is the curated ATA table plus a residual weight for
// every attribute that AtaMetadata marks critical and the curated table omits.
// Deriving the remainder from metadata means a critical attribute can never again
// be invisible to replacement-risk scoring just because nobody added it here by
// hand.
//
// Observed thresholds are deliberately not required. They were, because
// ValidateThreshold used to warn on any critical attribute whose value matched no
// bucket, so a critical attribute with no buckets at all was WARN on every drive
// that reported it and weighting that warning would have charged healthy drives
// for a metadata gap. #1088 removed that unconditional warning: a bucket-less
// attribute now leaves the status untouched and the manufacturer threshold decides
// it, which TestValidateThreshold_Attr177_NoBucketWarning_Eliminated pins. Keeping
// the requirement after that change silently dropped attributes 184 and 201, both
// critical, from scoring altogether.
var AtaReplacementRiskWeights = ataReplacementRiskWeights()

func ataReplacementRiskWeights() []ReplacementRiskWeight {
	curated := make(map[string]struct{}, len(ataCuratedReplacementRiskWeights))
	for _, weight := range ataCuratedReplacementRiskWeights {
		curated[weight.AttributeID] = struct{}{}
	}

	// Ranged by key: AtaAttributeMetadata is large enough that copying one per
	// iteration trips gocritic's rangeValCopy.
	residualIDs := []int{}
	for attributeID := range AtaMetadata {
		if !AtaMetadata[attributeID].Critical {
			continue
		}
		if _, ok := curated[strconv.Itoa(attributeID)]; ok {
			continue
		}
		residualIDs = append(residualIDs, attributeID)
	}
	sort.Ints(residualIDs)

	weights := make([]ReplacementRiskWeight, 0, len(ataCuratedReplacementRiskWeights)+len(residualIDs))
	weights = append(weights, ataCuratedReplacementRiskWeights...)
	for _, attributeID := range residualIDs {
		weights = append(weights, ReplacementRiskWeight{
			AttributeID:     strconv.Itoa(attributeID),
			DisplayName:     AtaMetadata[attributeID].DisplayName,
			Weight:          AtaCriticalResidualWeight,
			TrendMultiplier: AtaCriticalResidualTrendMultiplier,
		})
	}
	return weights
}

// NvmeReplacementRiskWeights defines attribute weights for NVMe drives.
// NVMe drives expose direct wear indicators (Percentage Used, Available Spare)
// making prediction more straightforward than ATA.
var NvmeReplacementRiskWeights = []ReplacementRiskWeight{
	{
		AttributeID:     "percentage_used",
		DisplayName:     "Percentage Used",
		Weight:          40,
		TrendMultiplier: 1.0,
	},
	{
		AttributeID:     "available_spare",
		DisplayName:     "Available Spare",
		Weight:          30,
		TrendMultiplier: 1.2,
	},
	{
		AttributeID:     "media_errors",
		DisplayName:     "Media and Data Integrity Errors",
		Weight:          20,
		TrendMultiplier: 1.5,
	},
	{
		AttributeID:     "critical_warning",
		DisplayName:     "Critical Warning",
		Weight:          7,
		TrendMultiplier: 0,
	},
	{
		AttributeID:     "unsafe_shutdowns",
		DisplayName:     "Unsafe Shutdowns",
		Weight:          3,
		TrendMultiplier: 0.5,
	},
}

// ScsiReplacementRiskWeights defines attribute weights for SCSI/SAS drives.
var ScsiReplacementRiskWeights = []ReplacementRiskWeight{
	{
		AttributeID:     "scsi_grown_defect_list",
		DisplayName:     "Grown Defect List",
		Weight:          40,
		TrendMultiplier: 1.5,
	},
	{
		AttributeID:     "read_total_uncorrected_errors",
		DisplayName:     "Read Total Uncorrected Errors",
		Weight:          25,
		TrendMultiplier: 1.5,
	},
	{
		AttributeID:     "write_total_uncorrected_errors",
		DisplayName:     "Write Total Uncorrected Errors",
		Weight:          25,
		TrendMultiplier: 1.5,
	},
	{
		AttributeID:     "read_errors_corrected_by_eccdelayed",
		DisplayName:     "Read Errors Corrected by ECC (Delayed)",
		Weight:          5,
		TrendMultiplier: 1.0,
	},
	{
		AttributeID:     "write_errors_corrected_by_eccdelayed",
		DisplayName:     "Write Errors Corrected by ECC (Delayed)",
		Weight:          5,
		TrendMultiplier: 1.0,
	},
}

// ReplacementRiskWeightsForProtocol returns the correct weight table for the
// given device protocol string ("ATA", "NVMe", "SCSI"). Returns nil for
// unrecognized protocols.
func ReplacementRiskWeightsForProtocol(protocol string) []ReplacementRiskWeight {
	switch protocol {
	case "ATA":
		return AtaReplacementRiskWeights
	case "NVMe":
		return NvmeReplacementRiskWeights
	case "SCSI":
		return ScsiReplacementRiskWeights
	default:
		return nil
	}
}

// ReplacementRiskWeightFor returns the weight a protocol's table gives one
// attribute. Callers that score a subset of attributes use it instead of
// repeating the numbers, which is how the dashboard summary scorer fell out of
// step with the ATA table.
func ReplacementRiskWeightFor(protocol string, attributeID string) (float64, bool) {
	for _, weight := range ReplacementRiskWeightsForProtocol(protocol) {
		if weight.AttributeID == attributeID {
			return weight.Weight, true
		}
	}
	return 0, false
}
