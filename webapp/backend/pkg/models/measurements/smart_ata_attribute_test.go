package measurements

import (
	"testing"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/analogj/scrutiny/webapp/backend/pkg/thresholds"
	"github.com/stretchr/testify/require"
)

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
