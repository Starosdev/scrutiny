package measurements_test

import (
	"testing"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	"github.com/stretchr/testify/require"
)

// TestApplyDeltaEvaluation_UnchangedValue tests that a cumulative counter attribute
// (attribute 199 - UltraDMA CRC Error Count) is suppressed when the value hasn't
// changed since the last measurement.
func TestApplyDeltaEvaluation_UnchangedValue(t *testing.T) {
	smart := measurements.Smart{
		Date:           time.Now(),
		DeviceWWN:      "test-wwn",
		DeviceProtocol: pkg.DeviceProtocolAta,
		Status:         pkg.DeviceStatusFailedScrutiny,
		Attributes: map[string]measurements.SmartAttribute{
			"199": &measurements.SmartAtaAttribute{
				AttributeId:  199,
				RawValue:     5,
				Status:       pkg.AttributeStatusWarningScrutiny,
				StatusReason: "Observed Failure Rate for Non-Critical Attribute is greater than 10%",
			},
		},
	}

	previousValues := map[string]int64{
		"199": 5, // Same value as current
	}

	smart.ApplyDeltaEvaluation(previousValues)

	// Attribute should be suppressed to passed
	attr := smart.Attributes["199"].(*measurements.SmartAtaAttribute)
	require.Equal(t, pkg.AttributeStatusPassed, attr.Status)
	require.Equal(t, "Cumulative counter unchanged since last measurement", attr.StatusReason)

	// Device status should also be recalculated to passed
	require.Equal(t, pkg.DeviceStatusPassed, smart.Status)
}

// TestApplyDeltaEvaluation_IncreasedValue tests that a cumulative counter attribute
// is NOT suppressed when the value has increased since the last measurement.
func TestApplyDeltaEvaluation_IncreasedValue(t *testing.T) {
	smart := measurements.Smart{
		Date:           time.Now(),
		DeviceWWN:      "test-wwn",
		DeviceProtocol: pkg.DeviceProtocolAta,
		Status:         pkg.DeviceStatusFailedScrutiny,
		Attributes: map[string]measurements.SmartAttribute{
			"199": &measurements.SmartAtaAttribute{
				AttributeId:  199,
				RawValue:     6,
				Status:       pkg.AttributeStatusWarningScrutiny,
				StatusReason: "Observed Failure Rate for Non-Critical Attribute is greater than 10%",
			},
		},
	}

	previousValues := map[string]int64{
		"199": 5, // Previous value was lower - counter increased
	}

	smart.ApplyDeltaEvaluation(previousValues)

	// Attribute should remain warning - value increased
	attr := smart.Attributes["199"].(*measurements.SmartAtaAttribute)
	require.Equal(t, pkg.AttributeStatusWarningScrutiny, attr.Status)
	require.Contains(t, attr.StatusReason, "Observed Failure Rate")
}

// TestApplyDeltaEvaluation_NoPreviousData tests that delta evaluation is skipped
// when no previous data exists (e.g., first submission).
func TestApplyDeltaEvaluation_NoPreviousData(t *testing.T) {
	smart := measurements.Smart{
		Date:           time.Now(),
		DeviceWWN:      "test-wwn",
		DeviceProtocol: pkg.DeviceProtocolAta,
		Status:         pkg.DeviceStatusFailedScrutiny,
		Attributes: map[string]measurements.SmartAttribute{
			"199": &measurements.SmartAtaAttribute{
				AttributeId:  199,
				RawValue:     5,
				Status:       pkg.AttributeStatusWarningScrutiny,
				StatusReason: "Observed Failure Rate for Non-Critical Attribute is greater than 10%",
			},
		},
	}

	// Empty previous values - simulates first submission
	previousValues := map[string]int64{}

	smart.ApplyDeltaEvaluation(previousValues)

	// Attribute should remain warning - no previous data to compare
	attr := smart.Attributes["199"].(*measurements.SmartAtaAttribute)
	require.Equal(t, pkg.AttributeStatusWarningScrutiny, attr.Status)
}

// TestApplyDeltaEvaluation_NilPreviousData tests that delta evaluation handles nil map.
func TestApplyDeltaEvaluation_NilPreviousData(t *testing.T) {
	smart := measurements.Smart{
		Date:           time.Now(),
		DeviceWWN:      "test-wwn",
		DeviceProtocol: pkg.DeviceProtocolAta,
		Status:         pkg.DeviceStatusFailedScrutiny,
		Attributes: map[string]measurements.SmartAttribute{
			"199": &measurements.SmartAtaAttribute{
				AttributeId:  199,
				RawValue:     5,
				Status:       pkg.AttributeStatusWarningScrutiny,
				StatusReason: "Observed Failure Rate for Non-Critical Attribute is greater than 10%",
			},
		},
	}

	smart.ApplyDeltaEvaluation(nil)

	// Attribute should remain warning
	attr := smart.Attributes["199"].(*measurements.SmartAtaAttribute)
	require.Equal(t, pkg.AttributeStatusWarningScrutiny, attr.Status)
}

// TestApplyDeltaEvaluation_ManufacturerFailureNotOverridden tests that manufacturer
// SMART failures (AttributeStatusFailedSmart) are never suppressed by delta evaluation.
func TestApplyDeltaEvaluation_ManufacturerFailureNotOverridden(t *testing.T) {
	smart := measurements.Smart{
		Date:           time.Now(),
		DeviceWWN:      "test-wwn",
		DeviceProtocol: pkg.DeviceProtocolAta,
		Status:         pkg.DeviceStatusFailedSmart,
		Attributes: map[string]measurements.SmartAttribute{
			"199": &measurements.SmartAtaAttribute{
				AttributeId:  199,
				RawValue:     5,
				Status:       pkg.AttributeStatusFailedSmart,
				StatusReason: "Attribute is failing manufacturer SMART threshold",
			},
		},
	}

	previousValues := map[string]int64{
		"199": 5, // Same value
	}

	smart.ApplyDeltaEvaluation(previousValues)

	// Manufacturer failure should NOT be suppressed
	attr := smart.Attributes["199"].(*measurements.SmartAtaAttribute)
	require.True(t, pkg.AttributeStatusHas(attr.Status, pkg.AttributeStatusFailedSmart))
}

// TestApplyDeltaEvaluation_NonDeltaAttributeUnaffected tests that attributes without
// UseDeltaEvaluation=true are not affected by delta evaluation.
func TestApplyDeltaEvaluation_NonDeltaAttributeUnaffected(t *testing.T) {
	smart := measurements.Smart{
		Date:           time.Now(),
		DeviceWWN:      "test-wwn",
		DeviceProtocol: pkg.DeviceProtocolAta,
		Status:         pkg.DeviceStatusFailedScrutiny,
		Attributes: map[string]measurements.SmartAttribute{
			// Attribute 5 (Reallocated Sector Count) does NOT have UseDeltaEvaluation
			"5": &measurements.SmartAtaAttribute{
				AttributeId:  5,
				RawValue:     10,
				Status:       pkg.AttributeStatusFailedScrutiny,
				StatusReason: "Observed Failure Rate for Critical Attribute is greater than 10%",
			},
		},
	}

	previousValues := map[string]int64{
		"5": 10, // Same value
	}

	smart.ApplyDeltaEvaluation(previousValues)

	// Attribute 5 should remain failed - it's not a delta-evaluated attribute
	attr := smart.Attributes["5"].(*measurements.SmartAtaAttribute)
	require.Equal(t, pkg.AttributeStatusFailedScrutiny, attr.Status)
}

// TestApplyDeltaEvaluation_NonATAProtocolSkipped tests that delta evaluation
// is skipped for non-ATA protocols (NVMe, SCSI).
func TestApplyDeltaEvaluation_NonATAProtocolSkipped(t *testing.T) {
	smart := measurements.Smart{
		Date:           time.Now(),
		DeviceWWN:      "test-wwn",
		DeviceProtocol: pkg.DeviceProtocolNvme,
		Status:         pkg.DeviceStatusFailedScrutiny,
		Attributes: map[string]measurements.SmartAttribute{
			"media_errors": &measurements.SmartNvmeAttribute{
				AttributeId: "media_errors",
				Value:       5,
				Status:      pkg.AttributeStatusFailedScrutiny,
			},
		},
	}

	previousValues := map[string]int64{
		"media_errors": 5,
	}

	smart.ApplyDeltaEvaluation(previousValues)

	// NVMe attribute should be unchanged - delta evaluation is ATA-only
	attr := smart.Attributes["media_errors"].(*measurements.SmartNvmeAttribute)
	require.Equal(t, pkg.AttributeStatusFailedScrutiny, attr.Status)
}

// TestApplyDeltaEvaluation_PassedAttributeSkipped tests that attributes already
// in passed status are not modified by delta evaluation.
func TestApplyDeltaEvaluation_PassedAttributeSkipped(t *testing.T) {
	smart := measurements.Smart{
		Date:           time.Now(),
		DeviceWWN:      "test-wwn",
		DeviceProtocol: pkg.DeviceProtocolAta,
		Status:         pkg.DeviceStatusPassed,
		Attributes: map[string]measurements.SmartAttribute{
			"199": &measurements.SmartAtaAttribute{
				AttributeId: 199,
				RawValue:    0,
				Status:      pkg.AttributeStatusPassed,
			},
		},
	}

	previousValues := map[string]int64{
		"199": 0,
	}

	smart.ApplyDeltaEvaluation(previousValues)

	// Should remain passed (no change needed)
	attr := smart.Attributes["199"].(*measurements.SmartAtaAttribute)
	require.Equal(t, pkg.AttributeStatusPassed, attr.Status)
}

// TestApplyDeltaEvaluation_MixedAttributes tests delta evaluation with a mix
// of delta and non-delta attributes, ensuring device status is correctly
// recalculated when only the delta attribute is suppressed.
func TestApplyDeltaEvaluation_MixedAttributes(t *testing.T) {
	smart := measurements.Smart{
		Date:           time.Now(),
		DeviceWWN:      "test-wwn",
		DeviceProtocol: pkg.DeviceProtocolAta,
		Status:         pkg.DeviceStatusFailedScrutiny,
		Attributes: map[string]measurements.SmartAttribute{
			// Attribute 199 (delta-evaluated) with unchanged value
			"199": &measurements.SmartAtaAttribute{
				AttributeId: 199,
				RawValue:    5,
				Status:      pkg.AttributeStatusWarningScrutiny,
			},
			// Attribute 5 (non-delta) still failing
			"5": &measurements.SmartAtaAttribute{
				AttributeId: 5,
				RawValue:    10,
				Status:      pkg.AttributeStatusFailedScrutiny,
			},
		},
	}

	previousValues := map[string]int64{
		"199": 5,  // Unchanged
		"5":   10, // Unchanged but not delta-evaluated
	}

	smart.ApplyDeltaEvaluation(previousValues)

	// Attribute 199 should be suppressed
	attr199 := smart.Attributes["199"].(*measurements.SmartAtaAttribute)
	require.Equal(t, pkg.AttributeStatusPassed, attr199.Status)

	// Attribute 5 should remain failed (not delta-evaluated)
	attr5 := smart.Attributes["5"].(*measurements.SmartAtaAttribute)
	require.Equal(t, pkg.AttributeStatusFailedScrutiny, attr5.Status)

	// Device should still be failed because attribute 5 is still failing
	require.True(t, pkg.DeviceStatusHas(smart.Status, pkg.DeviceStatusFailedScrutiny))
}

// TestApplyDeltaEvaluation_PreservesManufacturerSmartFailure tests that device-level
// manufacturer SMART failure is preserved during status recalculation.
func TestApplyDeltaEvaluation_PreservesManufacturerSmartFailure(t *testing.T) {
	smart := measurements.Smart{
		Date:           time.Now(),
		DeviceWWN:      "test-wwn",
		DeviceProtocol: pkg.DeviceProtocolAta,
		Status:         pkg.DeviceStatusFailedSmart | pkg.DeviceStatusFailedScrutiny,
		Attributes: map[string]measurements.SmartAttribute{
			"199": &measurements.SmartAtaAttribute{
				AttributeId: 199,
				RawValue:    5,
				Status:      pkg.AttributeStatusWarningScrutiny, // Only Scrutiny warning, not SMART failure
			},
		},
	}

	previousValues := map[string]int64{
		"199": 5,
	}

	smart.ApplyDeltaEvaluation(previousValues)

	// Attribute 199 should be suppressed
	attr := smart.Attributes["199"].(*measurements.SmartAtaAttribute)
	require.Equal(t, pkg.AttributeStatusPassed, attr.Status)

	// Device-level manufacturer SMART failure should be preserved
	require.True(t, pkg.DeviceStatusHas(smart.Status, pkg.DeviceStatusFailedSmart))
	// But Scrutiny failure should be cleared since the only failing attribute was suppressed
	require.False(t, pkg.DeviceStatusHas(smart.Status, pkg.DeviceStatusFailedScrutiny))
}

// TestApplyDeltaEvaluation_FailedScrutinyStatus tests that FailedScrutiny (not just
// Warning) is also suppressed by delta evaluation.
func TestApplyDeltaEvaluation_FailedScrutinyStatus(t *testing.T) {
	smart := measurements.Smart{
		Date:           time.Now(),
		DeviceWWN:      "test-wwn",
		DeviceProtocol: pkg.DeviceProtocolAta,
		Status:         pkg.DeviceStatusFailedScrutiny,
		Attributes: map[string]measurements.SmartAttribute{
			"199": &measurements.SmartAtaAttribute{
				AttributeId:  199,
				RawValue:     50, // High value that would trigger FailedScrutiny (>20% AFR)
				Status:       pkg.AttributeStatusFailedScrutiny,
				StatusReason: "Observed Failure Rate for Non-Critical Attribute is greater than 20%",
			},
		},
	}

	previousValues := map[string]int64{
		"199": 50, // Same value
	}

	smart.ApplyDeltaEvaluation(previousValues)

	// Even FailedScrutiny should be suppressed when value unchanged
	attr := smart.Attributes["199"].(*measurements.SmartAtaAttribute)
	require.Equal(t, pkg.AttributeStatusPassed, attr.Status)
	require.Equal(t, pkg.DeviceStatusPassed, smart.Status)
}
