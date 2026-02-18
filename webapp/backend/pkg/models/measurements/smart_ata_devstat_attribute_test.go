package measurements

import (
	"testing"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/stretchr/testify/require"
)

func TestSmartAtaDeviceStatAttribute_Flatten(t *testing.T) {
	// Test that device statistics are flattened with string-based attribute IDs
	attr := SmartAtaDeviceStatAttribute{
		AttributeId:      "devstat_7_8",
		Value:            19,
		Threshold:        100,
		TransformedValue: 19,
		Status:           pkg.AttributeStatusPassed,
	}

	flattened := attr.Flatten()

	require.Equal(t, "devstat_7_8", flattened["attr.devstat_7_8.attribute_id"])
	require.Equal(t, int64(19), flattened["attr.devstat_7_8.value"])
	require.Equal(t, int64(100), flattened["attr.devstat_7_8.thresh"])
	require.Equal(t, int64(19), flattened["attr.devstat_7_8.transformed_value"])
	require.Equal(t, int64(pkg.AttributeStatusPassed), flattened["attr.devstat_7_8.status"])
}

func TestSmartAtaDeviceStatAttribute_Inflate(t *testing.T) {
	// Test that device statistics can be inflated from InfluxDB data
	attr := SmartAtaDeviceStatAttribute{}

	attr.Inflate("attr.devstat_7_8.attribute_id", "devstat_7_8")
	attr.Inflate("attr.devstat_7_8.value", int64(25))
	attr.Inflate("attr.devstat_7_8.thresh", int64(100))
	attr.Inflate("attr.devstat_7_8.transformed_value", int64(25))
	attr.Inflate("attr.devstat_7_8.status", int64(pkg.AttributeStatusPassed))
	attr.Inflate("attr.devstat_7_8.status_reason", "")
	attr.Inflate("attr.devstat_7_8.failure_rate", float64(0))

	require.Equal(t, "devstat_7_8", attr.AttributeId)
	require.Equal(t, int64(25), attr.Value)
	require.Equal(t, int64(100), attr.Threshold)
	require.Equal(t, int64(25), attr.TransformedValue)
	require.Equal(t, pkg.AttributeStatusPassed, attr.Status)
}

func TestSmartAtaDeviceStatAttribute_FlattenInflateRoundtrip(t *testing.T) {
	// Test that flatten/inflate roundtrip preserves data
	original := SmartAtaDeviceStatAttribute{
		AttributeId:      "devstat_7_8",
		Value:            42,
		Threshold:        100,
		TransformedValue: 42,
		Status:           pkg.AttributeStatusWarningScrutiny,
		StatusReason:     "Test warning",
		FailureRate:      0.5,
	}

	flattened := original.Flatten()

	restored := SmartAtaDeviceStatAttribute{}
	for key, val := range flattened {
		restored.Inflate(key, val)
	}

	require.Equal(t, original.AttributeId, restored.AttributeId)
	require.Equal(t, original.Value, restored.Value)
	require.Equal(t, original.Threshold, restored.Threshold)
	require.Equal(t, original.TransformedValue, restored.TransformedValue)
	require.Equal(t, original.Status, restored.Status)
	require.Equal(t, original.StatusReason, restored.StatusReason)
	require.Equal(t, original.FailureRate, restored.FailureRate)
}

func TestSmartAtaDeviceStatAttribute_PopulateAttributeStatus_BelowThreshold(t *testing.T) {
	// Test that percentage used below threshold passes
	attr := SmartAtaDeviceStatAttribute{
		AttributeId: "devstat_7_8", // Percentage Used Endurance Indicator
		Value:       19,            // 19% used
		Threshold:   100,
	}

	attr.PopulateAttributeStatus()

	require.Equal(t, pkg.AttributeStatusPassed, attr.Status)
	require.Equal(t, int64(19), attr.TransformedValue)
}

func TestSmartAtaDeviceStatAttribute_PopulateAttributeStatus_AtThreshold(t *testing.T) {
	// Test that percentage used at threshold fails
	attr := SmartAtaDeviceStatAttribute{
		AttributeId: "devstat_7_8", // Percentage Used Endurance Indicator
		Value:       100,           // 100% used - device end of life
		Threshold:   100,
	}

	attr.PopulateAttributeStatus()

	require.True(t, pkg.AttributeStatusHas(attr.Status, pkg.AttributeStatusFailedScrutiny))
	require.NotEmpty(t, attr.StatusReason)
}

func TestSmartAtaDeviceStatAttribute_PopulateAttributeStatus_AboveThreshold(t *testing.T) {
	// Test that percentage used above threshold fails
	attr := SmartAtaDeviceStatAttribute{
		AttributeId: "devstat_7_8", // Percentage Used Endurance Indicator
		Value:       150,           // 150% used - past end of life
		Threshold:   100,
	}

	attr.PopulateAttributeStatus()

	require.True(t, pkg.AttributeStatusHas(attr.Status, pkg.AttributeStatusFailedScrutiny))
}

func TestSmartAtaDeviceStatAttribute_PopulateAttributeStatus_UnknownAttribute(t *testing.T) {
	// Test that unknown device statistics don't cause errors
	attr := SmartAtaDeviceStatAttribute{
		AttributeId: "devstat_99_99", // Unknown device statistic
		Value:       42,
	}

	attr.PopulateAttributeStatus()

	// Should pass since we don't have metadata for this attribute
	require.Equal(t, pkg.AttributeStatusPassed, attr.Status)
	require.Equal(t, int64(42), attr.TransformedValue)
}

func TestSmartAtaDeviceStatAttribute_GetTransformedValue(t *testing.T) {
	attr := SmartAtaDeviceStatAttribute{
		TransformedValue: 123,
	}
	require.Equal(t, int64(123), attr.GetTransformedValue())
}

func TestSmartAtaDeviceStatAttribute_GetStatus(t *testing.T) {
	attr := SmartAtaDeviceStatAttribute{
		Status: pkg.AttributeStatusWarningScrutiny,
	}
	require.Equal(t, pkg.AttributeStatusWarningScrutiny, attr.GetStatus())
}

func TestSmartAtaDeviceStatAttribute_PopulateAttributeStatus_InvalidValue_TooHigh(t *testing.T) {
	// Test that impossibly high values are marked as invalid (issue #84)
	// Some drives report corrupted values like 420 billion for percentage used
	attr := SmartAtaDeviceStatAttribute{
		AttributeId: "devstat_7_8", // Percentage Used Endurance Indicator (has Ideal: low)
		Value:       420_000_000_000, // 420 billion - obviously corrupted
	}

	attr.PopulateAttributeStatus()

	// Should be marked as invalid, NOT as failed
	require.True(t, pkg.AttributeStatusHas(attr.Status, pkg.AttributeStatusInvalidValue),
		"Impossibly high value should be marked as invalid")
	require.False(t, pkg.AttributeStatusHas(attr.Status, pkg.AttributeStatusFailedScrutiny),
		"Invalid values should not trigger failure status")
	require.Contains(t, attr.StatusReason, "exceeds reasonable maximum")
}

func TestSmartAtaDeviceStatAttribute_PopulateAttributeStatus_InvalidValue_AtBoundary(t *testing.T) {
	// Test the boundary value (1 million)
	attr := SmartAtaDeviceStatAttribute{
		AttributeId: "devstat_7_8", // Percentage Used Endurance Indicator (has Ideal: low)
		Value:       MaxReasonableFailureCount + 1, // Just over the limit
	}

	attr.PopulateAttributeStatus()

	require.True(t, pkg.AttributeStatusHas(attr.Status, pkg.AttributeStatusInvalidValue),
		"Value just over limit should be marked as invalid")
}

func TestSmartAtaDeviceStatAttribute_PopulateAttributeStatus_ValidHighValue(t *testing.T) {
	// Test that values at the boundary are still evaluated normally
	attr := SmartAtaDeviceStatAttribute{
		AttributeId: "devstat_7_8", // Percentage Used Endurance Indicator (has Ideal: low, Critical: true)
		Value:       MaxReasonableFailureCount, // Exactly at limit - still valid (though suspicious)
	}

	attr.PopulateAttributeStatus()

	// Should be evaluated normally (and fail since it's >= 100 threshold)
	require.False(t, pkg.AttributeStatusHas(attr.Status, pkg.AttributeStatusInvalidValue),
		"Value at limit should not be marked as invalid")
	require.True(t, pkg.AttributeStatusHas(attr.Status, pkg.AttributeStatusFailedScrutiny),
		"High but valid value should trigger failure for critical attribute")
}

func TestSmartAtaDeviceStatAttribute_PopulateAttributeStatus_InvalidValue_NonCritical(t *testing.T) {
	// Test that non-critical attributes with high values are also marked invalid
	attr := SmartAtaDeviceStatAttribute{
		AttributeId: "devstat_5_88", // Time in Over-temperature (has Ideal: low, Critical: false)
		Value:       999_999_999_999,
	}

	attr.PopulateAttributeStatus()

	require.True(t, pkg.AttributeStatusHas(attr.Status, pkg.AttributeStatusInvalidValue),
		"Non-critical attributes with impossibly high values should also be marked invalid")
}

func TestSmartAtaDeviceStatAttribute_PopulateAttributeStatus_MetadataThreshold(t *testing.T) {
	// Test that devstat_7_8 uses the metadata threshold (100) even without struct Threshold set
	attr := SmartAtaDeviceStatAttribute{
		AttributeId: "devstat_7_8",
		Value:       100,
		// Threshold NOT set (0) - should use metadata Threshold: 100
	}

	attr.PopulateAttributeStatus()

	require.True(t, pkg.AttributeStatusHas(attr.Status, pkg.AttributeStatusFailedScrutiny),
		"devstat_7_8 at 100%% should fail via metadata threshold")
}

func TestSmartAtaDeviceStatAttribute_PopulateAttributeStatus_ErrorCount_NonZero(t *testing.T) {
	// Discussion #215: devstat_4_8 with value 452 was incorrectly marked as FAILED
	// because of a hardcoded threshold of 100. Error counts should WARN, not FAIL.
	attr := SmartAtaDeviceStatAttribute{
		AttributeId: "devstat_4_8", // Number of Reported Uncorrectable Errors (Critical, Ideal: low)
		Value:       452,
	}

	attr.PopulateAttributeStatus()

	require.True(t, pkg.AttributeStatusHas(attr.Status, pkg.AttributeStatusWarningScrutiny),
		"Non-zero error count on critical devstat should warn")
	require.False(t, pkg.AttributeStatusHas(attr.Status, pkg.AttributeStatusFailedScrutiny),
		"Error count devstat should NOT fail without a fixed threshold")
	require.Contains(t, attr.StatusReason, "non-zero error count")
}

func TestSmartAtaDeviceStatAttribute_PopulateAttributeStatus_ErrorCount_Zero(t *testing.T) {
	// devstat_4_8 with value 0 should pass
	attr := SmartAtaDeviceStatAttribute{
		AttributeId: "devstat_4_8",
		Value:       0,
	}

	attr.PopulateAttributeStatus()

	require.Equal(t, pkg.AttributeStatusPassed, attr.Status)
}

func TestSmartAtaDeviceStatAttribute_PopulateAttributeStatus_ReallocatedSectors_NonZero(t *testing.T) {
	// devstat_3_32 with non-zero value should warn (not fail)
	attr := SmartAtaDeviceStatAttribute{
		AttributeId: "devstat_3_32", // Number of Reallocated Logical Sectors (Critical, Ideal: low)
		Value:       5,
	}

	attr.PopulateAttributeStatus()

	require.True(t, pkg.AttributeStatusHas(attr.Status, pkg.AttributeStatusWarningScrutiny),
		"Non-zero reallocated sectors should warn")
	require.False(t, pkg.AttributeStatusHas(attr.Status, pkg.AttributeStatusFailedScrutiny),
		"Reallocated sectors should NOT fail without a fixed threshold")
}

func TestSmartAtaDeviceStatAttribute_PopulateAttributeStatus_MechanicalFailures_Zero(t *testing.T) {
	// devstat_3_48 with value 0 should pass
	attr := SmartAtaDeviceStatAttribute{
		AttributeId: "devstat_3_48", // Number of Mechanical Start Failures (Critical, Ideal: low)
		Value:       0,
	}

	attr.PopulateAttributeStatus()

	require.Equal(t, pkg.AttributeStatusPassed, attr.Status)
}
