package measurements

import (
	"sort"
	"testing"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/analogj/scrutiny/webapp/backend/pkg/thresholds"
	"github.com/stretchr/testify/require"
)

// Power-On Hours (Attribute 9) Transform Tests

func TestAttribute9Transform_StandardHours(t *testing.T) {
	transform := thresholds.AtaMetadata[9].Transform
	require.NotNil(t, transform)
	result := transform(100, 1730, "1730")
	require.Equal(t, int64(1730), result)
}

func TestAttribute9Transform_ZeroHours(t *testing.T) {
	transform := thresholds.AtaMetadata[9].Transform
	result := transform(100, 0, "0")
	require.Equal(t, int64(0), result)
}

func TestAttribute9Transform_PackedValue(t *testing.T) {
	// From smart-sat.json: rawValue=167031278144165 (0x97ea00000aa5), actual hours=2725
	transform := thresholds.AtaMetadata[9].Transform
	result := transform(97, 167031278144165, "2725 (151 234 0)")
	require.Equal(t, int64(2725), result)
}

func TestAttribute9Transform_PackedValueLargeHours(t *testing.T) {
	// Packed format where upper bytes contain flags, lower 32 bits = 10800 hours
	transform := thresholds.AtaMetadata[9].Transform
	result := transform(85, 0x001E00002A30, "10800")
	require.Equal(t, int64(10800), result)
}

func TestAttribute9Transform_HoursMinutesSecondsFormat(t *testing.T) {
	// smartctl h+m+s format
	transform := thresholds.AtaMetadata[9].Transform
	result := transform(97, 1730, "1730h+05m+02.453s")
	require.Equal(t, int64(1730), result)
}

func TestAttribute9Transform_ParenthesisHoursFormat(t *testing.T) {
	// smartctl minutes-converted format
	transform := thresholds.AtaMetadata[9].Transform
	result := transform(90, 103800, "103800 (1730 hours)")
	require.Equal(t, int64(1730), result)
}

func TestAttribute9Transform_LargeStandardValue(t *testing.T) {
	// Old drive with 100,000+ hours (within 32-bit range, not packed)
	transform := thresholds.AtaMetadata[9].Transform
	result := transform(50, 100000, "100000")
	require.Equal(t, int64(100000), result)
}

func TestValidateThreshold_NonZeroAnnualFailureRate_Unchanged(t *testing.T) {
	sa := SmartAtaAttribute{RawValue: 5}
	metadata := thresholds.AtaAttributeMetadata{
		DisplayType: thresholds.AtaSmartAttributeDisplayTypeRaw,
		Critical:    true,
		ObservedThresholds: []thresholds.ObservedThreshold{
			{Low: 0, High: 10, AnnualFailureRate: 0.15, ErrorInterval: []float64{0.12, 0.18}},
		},
	}

	sa.ValidateThreshold(metadata)

	require.InDelta(t, 0.15, sa.FailureRate, 0.001)
	require.True(t, pkg.AttributeStatusHas(sa.Status, pkg.AttributeStatusFailedScrutiny))
}

func TestValidateThreshold_ZeroRate_ZeroInterval_NoChange(t *testing.T) {
	sa := SmartAtaAttribute{RawValue: 5}
	metadata := thresholds.AtaAttributeMetadata{
		DisplayType: thresholds.AtaSmartAttributeDisplayTypeRaw,
		Critical:    false,
		ObservedThresholds: []thresholds.ObservedThreshold{
			{Low: 0, High: 10, AnnualFailureRate: 0, ErrorInterval: []float64{0, 0}},
		},
	}

	sa.ValidateThreshold(metadata)

	require.Equal(t, float64(0), sa.FailureRate)
	require.Equal(t, pkg.AttributeStatusPassed, sa.Status)
}

func TestValidateThreshold_ZeroRate_RealInterval_MidpointUsed(t *testing.T) {
	sa := SmartAtaAttribute{RawValue: 5}
	metadata := thresholds.AtaAttributeMetadata{
		DisplayType: thresholds.AtaSmartAttributeDisplayTypeRaw,
		Critical:    false,
		ObservedThresholds: []thresholds.ObservedThreshold{
			{Low: 0, High: 10, AnnualFailureRate: 0, ErrorInterval: []float64{0.08, 0.12}},
		},
	}

	sa.ValidateThreshold(metadata)

	require.InDelta(t, 0.10, sa.FailureRate, 0.001)
}

func TestValidateThreshold_CriticalAttribute_InferredRate_TriggersFailure(t *testing.T) {
	sa := SmartAtaAttribute{RawValue: 5}
	metadata := thresholds.AtaAttributeMetadata{
		DisplayType: thresholds.AtaSmartAttributeDisplayTypeRaw,
		Critical:    true,
		ObservedThresholds: []thresholds.ObservedThreshold{
			{Low: 0, High: 10, AnnualFailureRate: 0, ErrorInterval: []float64{0.10, 0.14}},
		},
	}

	sa.ValidateThreshold(metadata)

	require.InDelta(t, 0.12, sa.FailureRate, 0.001)
	require.True(t, pkg.AttributeStatusHas(sa.Status, pkg.AttributeStatusFailedScrutiny))
}

func TestValidateThreshold_NonCriticalAttribute_InferredRate_TriggersWarning(t *testing.T) {
	sa := SmartAtaAttribute{RawValue: 5}
	metadata := thresholds.AtaAttributeMetadata{
		DisplayType: thresholds.AtaSmartAttributeDisplayTypeRaw,
		Critical:    false,
		ObservedThresholds: []thresholds.ObservedThreshold{
			{Low: 0, High: 10, AnnualFailureRate: 0, ErrorInterval: []float64{0.08, 0.12}},
		},
	}

	sa.ValidateThreshold(metadata)

	require.InDelta(t, 0.10, sa.FailureRate, 0.001)
	require.True(t, pkg.AttributeStatusHas(sa.Status, pkg.AttributeStatusWarningScrutiny))
}

func TestValidateThreshold_NonCriticalAttribute_InferredRate_TriggersFailure(t *testing.T) {
	sa := SmartAtaAttribute{RawValue: 5}
	metadata := thresholds.AtaAttributeMetadata{
		DisplayType: thresholds.AtaSmartAttributeDisplayTypeRaw,
		Critical:    false,
		ObservedThresholds: []thresholds.ObservedThreshold{
			{Low: 0, High: 10, AnnualFailureRate: 0, ErrorInterval: []float64{0.18, 0.26}},
		},
	}

	sa.ValidateThreshold(metadata)

	require.InDelta(t, 0.22, sa.FailureRate, 0.001)
	require.True(t, pkg.AttributeStatusHas(sa.Status, pkg.AttributeStatusFailedScrutiny))
}

// Attribute 177 (Wear Leveling Count) Threshold Tests

func TestValidateThreshold_Attr177_NewDrive_NormalizedValue100_Passes(t *testing.T) {
	sa := SmartAtaAttribute{AttributeId: 177, Value: 100, RawValue: 0}
	metadata := thresholds.AtaMetadata[177]

	sa.ValidateThreshold(metadata)

	require.Less(t, sa.FailureRate, 0.10)
	require.False(t, pkg.AttributeStatusHas(sa.Status, pkg.AttributeStatusFailedScrutiny))
	require.False(t, pkg.AttributeStatusHas(sa.Status, pkg.AttributeStatusWarningScrutiny))
}

func TestValidateThreshold_Attr177_LightWear_NormalizedValue85_Passes(t *testing.T) {
	sa := SmartAtaAttribute{AttributeId: 177, Value: 85, RawValue: 500}
	metadata := thresholds.AtaMetadata[177]

	sa.ValidateThreshold(metadata)

	require.Less(t, sa.FailureRate, 0.10)
	require.False(t, pkg.AttributeStatusHas(sa.Status, pkg.AttributeStatusFailedScrutiny))
}

func TestValidateThreshold_Attr177_ModerateWear_NormalizedValue40_Passes(t *testing.T) {
	sa := SmartAtaAttribute{AttributeId: 177, Value: 40, RawValue: 3000}
	metadata := thresholds.AtaMetadata[177]

	sa.ValidateThreshold(metadata)

	require.Less(t, sa.FailureRate, 0.10)
	require.False(t, pkg.AttributeStatusHas(sa.Status, pkg.AttributeStatusFailedScrutiny))
}

func TestValidateThreshold_Attr177_SevereWear_NormalizedValue15_Fails(t *testing.T) {
	sa := SmartAtaAttribute{AttributeId: 177, Value: 15, RawValue: 8000}
	metadata := thresholds.AtaMetadata[177]

	sa.ValidateThreshold(metadata)

	require.GreaterOrEqual(t, sa.FailureRate, 0.10)
	require.True(t, pkg.AttributeStatusHas(sa.Status, pkg.AttributeStatusFailedScrutiny))
}

func TestValidateThreshold_Attr177_FullyWorn_NormalizedValue0_Fails(t *testing.T) {
	sa := SmartAtaAttribute{AttributeId: 177, Value: 0, RawValue: 10000}
	metadata := thresholds.AtaMetadata[177]

	sa.ValidateThreshold(metadata)

	require.GreaterOrEqual(t, sa.FailureRate, 0.10)
	require.True(t, pkg.AttributeStatusHas(sa.Status, pkg.AttributeStatusFailedScrutiny))
}

func TestValidateThreshold_Attr177_BoundaryValue25_Passes(t *testing.T) {
	sa := SmartAtaAttribute{AttributeId: 177, Value: 25, RawValue: 5000}
	metadata := thresholds.AtaMetadata[177]

	sa.ValidateThreshold(metadata)

	require.Less(t, sa.FailureRate, 0.10)
	require.False(t, pkg.AttributeStatusHas(sa.Status, pkg.AttributeStatusFailedScrutiny))
}

func TestValidateThreshold_Attr177_BoundaryValue24_Fails(t *testing.T) {
	sa := SmartAtaAttribute{AttributeId: 177, Value: 24, RawValue: 5500}
	metadata := thresholds.AtaMetadata[177]

	sa.ValidateThreshold(metadata)

	require.GreaterOrEqual(t, sa.FailureRate, 0.10)
	require.True(t, pkg.AttributeStatusHas(sa.Status, pkg.AttributeStatusFailedScrutiny))
}

func TestValidateThreshold_Attr177_AboveNormal_NormalizedValue110_Passes(t *testing.T) {
	sa := SmartAtaAttribute{AttributeId: 177, Value: 110, RawValue: 0}
	metadata := thresholds.AtaMetadata[177]

	sa.ValidateThreshold(metadata)

	require.Less(t, sa.FailureRate, 0.10)
	require.False(t, pkg.AttributeStatusHas(sa.Status, pkg.AttributeStatusFailedScrutiny))
}

func TestValidateThreshold_Attr177_NoBucketWarning_Eliminated(t *testing.T) {
	sa := SmartAtaAttribute{AttributeId: 177, Value: 92, RawValue: 200}
	sa.PopulateAttributeStatus(nil, false)

	require.False(t, pkg.AttributeStatusHas(sa.Status, pkg.AttributeStatusWarningScrutiny))
	require.False(t, pkg.AttributeStatusHas(sa.Status, pkg.AttributeStatusFailedScrutiny))
}

func TestPopulateAttributeStatus_UsesConsumerDriveProfileThresholds(t *testing.T) {
	sa := SmartAtaAttribute{AttributeId: 5, RawValue: 40}
	profile := &thresholds.ConsumerDriveProfile{
		AtaObservedThresholds: map[int][]thresholds.ObservedThreshold{
			5: {
				{Low: 0, High: 10, AnnualFailureRate: 0.02, ErrorInterval: []float64{0.01, 0.03}},
				{Low: 10, High: 64, AnnualFailureRate: 0.30, ErrorInterval: []float64{0.20, 0.40}},
			},
		},
	}

	sa.PopulateAttributeStatus(profile, false)

	require.InDelta(t, 0.30, sa.FailureRate, 0.001)
	require.True(t, pkg.AttributeStatusHas(sa.Status, pkg.AttributeStatusFailedScrutiny))
}

func TestPopulateAttributeStatus_FallsBackWithoutProfile(t *testing.T) {
	sa := SmartAtaAttribute{AttributeId: 5, RawValue: 40}

	sa.PopulateAttributeStatus(nil, false)

	require.InDelta(t, 0.23589260654405794, sa.FailureRate, 0.001)
	require.True(t, pkg.AttributeStatusHas(sa.Status, pkg.AttributeStatusFailedScrutiny))
}

func TestAtaAttributeNamesSemanticallyMatch(t *testing.T) {
	require.True(t, ataAttributeNamesSemanticallyMatch("High_Fly_Writes", "High Fly Writes"))
	require.True(t, ataAttributeNamesSemanticallyMatch("Runtime_Bad_Block", "SATA Downshift Error Count or Runtime Bad Block"))
	require.True(t, ataAttributeNamesSemanticallyMatch("Factory_Bad_Block_Ct", "Factory Bad Block Count"))
	require.False(t, ataAttributeNamesSemanticallyMatch("Factory_Bad_Block_Ct", "High Fly Writes"))
}

func TestPopulateAttributeStatus_SkipsGenericObservedThresholdsForVendorSpecificAtaSsd(t *testing.T) {
	sa := SmartAtaAttribute{
		AttributeId: 189,
		Name:        "Factory_Bad_Block_Ct",
		Value:       100,
		RawValue:    81,
	}

	sa.PopulateAttributeStatus(nil, true)

	require.Equal(t, pkg.AttributeStatusPassed, sa.Status)
	require.Equal(t, ataSsdVendorSpecificThresholdSkipReason, sa.StatusReason)
	require.Zero(t, sa.FailureRate)
}

// Observed-threshold envelope tests (#1078)

// ataSeverityRank orders the statuses observed-threshold scoring can produce, so a sweep can
// assert severity never drops.
func ataSeverityRank(status pkg.AttributeStatus) int {
	switch {
	case pkg.AttributeStatusHas(status, pkg.AttributeStatusFailedScrutiny):
		return 2
	case pkg.AttributeStatusHas(status, pkg.AttributeStatusWarningScrutiny):
		return 1
	default:
		return 0
	}
}

// ataAttributeWithThresholdValue builds an attribute carrying value in the field the metadata's
// display type is scored against.
func ataAttributeWithThresholdValue(attributeID int, metadata thresholds.AtaAttributeMetadata, value int64) SmartAtaAttribute {
	sa := SmartAtaAttribute{AttributeId: attributeID}
	switch metadata.DisplayType {
	case thresholds.AtaSmartAttributeDisplayTypeNormalized:
		sa.Value = value
	case thresholds.AtaSmartAttributeDisplayTypeTransformed:
		sa.TransformedValue = value
	default:
		sa.RawValue = value
	}
	return sa
}

// ataThresholdProbeValues returns the values worth scoring for a bucket list, ascending: every
// bucket edge, one value either side of it, and zero. A non-monotonic table shows up at an edge.
func ataThresholdProbeValues(obsThresholds []thresholds.ObservedThreshold) []int64 {
	seen := map[int64]struct{}{0: {}}
	for _, obsThresh := range obsThresholds {
		for _, candidate := range []int64{
			obsThresh.Low - 1, obsThresh.Low, obsThresh.Low + 1,
			obsThresh.High, obsThresh.High + 1,
		} {
			seen[candidate] = struct{}{}
		}
	}

	values := make([]int64, 0, len(seen))
	for value := range seen {
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	return values
}

func TestValidateThreshold_ObservedSeverityNeverImprovesAsAttributeWorsens(t *testing.T) {
	for attributeID, metadata := range thresholds.AtaMetadata {
		if len(metadata.ObservedThresholds) == 0 {
			continue
		}

		// walk the probes from the healthiest value to the worst one, whichever direction that is
		values := ataThresholdProbeValues(metadata.ObservedThresholds)
		if metadata.Ideal == thresholds.ObservedThresholdIdealHigh {
			for i, j := 0, len(values)-1; i < j; i, j = i+1, j-1 {
				values[i], values[j] = values[j], values[i]
			}
		}

		worstSoFar := 0
		for _, value := range values {
			sa := ataAttributeWithThresholdValue(attributeID, metadata, value)
			sa.ValidateThreshold(metadata)

			rank := ataSeverityRank(sa.Status)
			require.GreaterOrEqual(t, rank, worstSoFar,
				"attribute %d (%s) reports a milder status at %d than at a healthier value",
				attributeID, metadata.DisplayName, value)
			worstSoFar = rank
		}
	}
}

func TestAtaMetadata_BucketedAttributesDeclareAnIdealDirection(t *testing.T) {
	for attributeID, metadata := range thresholds.AtaMetadata {
		if len(metadata.ObservedThresholds) == 0 {
			continue
		}

		require.Contains(t,
			[]string{thresholds.ObservedThresholdIdealLow, thresholds.ObservedThresholdIdealHigh},
			metadata.Ideal,
			"attribute %d (%s) carries observed thresholds but declares no Ideal, so its buckets are scored one at a time",
			attributeID, metadata.DisplayName)
	}
}

func TestValidateThreshold_Attr198_BeyondLastBucket_Fails(t *testing.T) {
	// seventeen offline uncorrectable sectors is past the last bucket, and used to be scored
	// milder than one sector (#1078).
	sa := SmartAtaAttribute{AttributeId: 198, RawValue: 17}

	sa.ValidateThreshold(thresholds.AtaMetadata[198])

	require.True(t, pkg.AttributeStatusHas(sa.Status, pkg.AttributeStatusFailedScrutiny))
	require.False(t, pkg.AttributeStatusHas(sa.Status, pkg.AttributeStatusWarningScrutiny))
}

func TestValidateThreshold_Attr198_ZeroSectors_Passes(t *testing.T) {
	sa := SmartAtaAttribute{AttributeId: 198, RawValue: 0}

	sa.ValidateThreshold(thresholds.AtaMetadata[198])

	require.Equal(t, pkg.AttributeStatusPassed, sa.Status)
}

func TestValidateThreshold_Attr5_BucketEdgeUnchanged(t *testing.T) {
	// the envelope must not move an existing edge: the 23.6% bucket still starts at 17.
	passing := SmartAtaAttribute{AttributeId: 5, RawValue: 16}
	passing.ValidateThreshold(thresholds.AtaMetadata[5])
	require.Equal(t, pkg.AttributeStatusPassed, passing.Status)

	failing := SmartAtaAttribute{AttributeId: 5, RawValue: 17}
	failing.ValidateThreshold(thresholds.AtaMetadata[5])
	require.True(t, pkg.AttributeStatusHas(failing.Status, pkg.AttributeStatusFailedScrutiny))
}

func TestValidateThreshold_Attr4_UsageCounterNeverScoresAVerdict(t *testing.T) {
	// 69 cycles on two healthy WD Ultrastar DC HC550 drives warned on every collector run, which
	// is the real-world effect reported in #1078. Monotonic scoring spread that warning to every
	// count from 27 upward rather than ending it, so #1093 removed the buckets.
	for _, value := range []int64{0, 9, 27, 69, 95, 500, 1860} {
		sa := SmartAtaAttribute{AttributeId: 4, RawValue: value}

		sa.ValidateThreshold(thresholds.AtaMetadata[4])

		require.Equal(t, pkg.AttributeStatusPassed, sa.Status, "start/stop count %d", value)
		require.Zero(t, sa.FailureRate, "start/stop count %d", value)
	}
}

func TestValidateThreshold_Attr192_UsageCounterNeverScoresAVerdict(t *testing.T) {
	// 7 retracts upward all scored the 6-16 bucket's 11.97% once severity became monotonic, while
	// the 650-1600 bucket reads 0.52%. The rate described the fleet, not the drive (#1093).
	for _, value := range []int64{0, 7, 17, 234, 909, 1600, 5000} {
		sa := SmartAtaAttribute{AttributeId: 192, RawValue: value}

		sa.ValidateThreshold(thresholds.AtaMetadata[192])

		require.Equal(t, pkg.AttributeStatusPassed, sa.Status, "power-off retract count %d", value)
		require.Zero(t, sa.FailureRate, "power-off retract count %d", value)
	}
}

func TestAtaMetadata_UsageCountersCarryNoBuckets(t *testing.T) {
	// A start/stop cycle and a power-off retract are normal operation, so a population failure
	// rate for them measures fleet age. Attributes 3, 184 and 195 are excluded for a different
	// reason and have their own test.
	for _, attributeID := range []int{4, 192} {
		require.Empty(t, thresholds.AtaMetadata[attributeID].ObservedThresholds,
			"attribute %d", attributeID)
	}
}

func TestAtaMetadata_VendorScaledNormalizedAttributesCarryNoBuckets(t *testing.T) {
	// 3, 184 and 195 are bucketed on the normalized value, which every vendor scales itself.
	for _, attributeID := range []int{3, 184, 195} {
		require.Empty(t, thresholds.AtaMetadata[attributeID].ObservedThresholds,
			"attribute %d", attributeID)
	}
}

func TestValidateThreshold_Attr3_VendorNormalizedSpinUpTime_Passes(t *testing.T) {
	// a healthy WD Ultrastar DC HC550 reports normalized 83 for 349ms spin-up (#1078).
	sa := SmartAtaAttribute{AttributeId: 3, Value: 83, RawValue: 349}

	sa.ValidateThreshold(thresholds.AtaMetadata[3])

	require.Equal(t, pkg.AttributeStatusPassed, sa.Status)
}

// Profile-supplied buckets replace the bundled ones for the drives a profile matches, so they are
// scored by the same envelope. This sweeps every bundled profile that overrides buckets through
// PopulateAttributeStatus, which is the path a real drive takes, rather than calling
// ValidateThreshold with hand-built metadata.
func TestPopulateAttributeStatus_ProfileBucketsNeverScoreMilderAsAttributeWorsens(t *testing.T) {
	families := []string{
		"Hitachi Deskstar 7K1000.D",
		"WDC Red",
		"WDC Red Plus",
		"Seagate Desktop SSHD",
		"Seagate Barracuda 7200.14 (AF)",
		"WDC Caviar Blue",
		"WDC Caviar Green",
		"Seagate Barracuda 7200.12",
		"Seagate Desktop HDD.15",
		"Crucial/Micron RealSSD m4/C400/P400",
	}

	swept := 0
	for _, family := range families {
		profile, ok := thresholds.LookupConsumerDriveProfile("ATA", family, "")
		require.True(t, ok, "profile %q is no longer in the bundled catalog", family)
		require.NotEmpty(t, profile.AtaObservedThresholds, "profile %q no longer overrides buckets", family)

		for attributeID, obsThresholds := range profile.AtaObservedThresholds {
			metadata := thresholds.AtaMetadata[attributeID]

			// walk the probes from the healthiest value to the worst one, whichever direction that is
			values := ataThresholdProbeValues(obsThresholds)
			if metadata.Ideal == thresholds.ObservedThresholdIdealHigh {
				for i, j := 0, len(values)-1; i < j; i, j = i+1, j-1 {
					values[i], values[j] = values[j], values[i]
				}
			}

			worstSoFar := 0
			for _, value := range values {
				sa := ataAttributeWithThresholdValue(attributeID, metadata, value)
				sa.PopulateAttributeStatus(profile, false)

				rank := ataSeverityRank(sa.Status)
				require.GreaterOrEqual(t, rank, worstSoFar,
					"profile %q attribute %d (%s) reports a milder status at %d than at a healthier value",
					family, attributeID, metadata.DisplayName, value)
				worstSoFar = rank
			}
			swept++
		}
	}

	require.GreaterOrEqual(t, swept, len(families),
		"every bundled profile that overrides buckets must be swept")
}

func TestValidateThreshold_CriticalWithoutObservedThresholds_StaysPassed(t *testing.T) {
	// Issue #1091: replacement-risk scoring now weights critical attributes that
	// carry no observed thresholds, which is only safe while a missing bucket set
	// leaves the status alone. Restoring an unconditional warning here would mark
	// every drive reporting attribute 201 as degraded and charge it risk points.
	sa := SmartAtaAttribute{AttributeId: 201, Value: 23, RawValue: 23}
	metadata := thresholds.AtaMetadata[201]
	require.True(t, metadata.Critical)
	require.Empty(t, metadata.ObservedThresholds)

	sa.ValidateThreshold(metadata)

	require.Equal(t, pkg.AttributeStatusPassed, sa.Status)
	require.Zero(t, sa.FailureRate)
	require.Empty(t, sa.StatusReason)
}
