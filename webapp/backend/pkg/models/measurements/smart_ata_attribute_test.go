package measurements

import (
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
	sa.PopulateAttributeStatus()

	require.False(t, pkg.AttributeStatusHas(sa.Status, pkg.AttributeStatusWarningScrutiny))
	require.False(t, pkg.AttributeStatusHas(sa.Status, pkg.AttributeStatusFailedScrutiny))
}
