package measurements_test

import (
	"testing"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models/measurements"
	"github.com/stretchr/testify/require"
)

// ataSmart builds an ATA Smart measurement with the given attributes for rollover tests.
func ataSmart(attrs map[string]measurements.SmartAttribute) measurements.Smart {
	return measurements.Smart{
		Date:           time.Now(),
		DeviceWWN:      "test-wwn",
		DeviceProtocol: pkg.DeviceProtocolAta,
		Attributes:     attrs,
	}
}

// TestDetectPowerOnHoursRollover_HistoryDecrease flags a wrap when Power-On Hours drops.
func TestDetectPowerOnHoursRollover_HistoryDecrease(t *testing.T) {
	current := ataSmart(map[string]measurements.SmartAttribute{
		"9": &measurements.SmartAtaAttribute{AttributeId: 9, TransformedValue: 12},
	})
	previous := ataSmart(map[string]measurements.SmartAttribute{
		"9": &measurements.SmartAtaAttribute{AttributeId: 9, TransformedValue: 65000},
	})

	current.DetectPowerOnHoursRollover(&previous)

	attr := current.Attributes["9"].(*measurements.SmartAtaAttribute)
	require.True(t, pkg.AttributeStatusHas(attr.Status, pkg.AttributeStatusWarningScrutiny))
	require.Contains(t, attr.StatusReason, "decreased from 65000h to 12h")
}

// TestDetectPowerOnHoursRollover_FarmExceeds flags a wrap when tamper-proof FARM hours exceed PoH.
func TestDetectPowerOnHoursRollover_FarmExceeds(t *testing.T) {
	current := ataSmart(map[string]measurements.SmartAttribute{
		"9":        &measurements.SmartAtaAttribute{AttributeId: 9, TransformedValue: 12},
		"farm_poh": &measurements.SmartFarmAttribute{AttributeId: "farm_poh", Value: 65548},
	})

	current.DetectPowerOnHoursRollover(nil)

	attr := current.Attributes["9"].(*measurements.SmartAtaAttribute)
	require.True(t, pkg.AttributeStatusHas(attr.Status, pkg.AttributeStatusWarningScrutiny))
	require.Contains(t, attr.StatusReason, "FARM Power-On Hours (65548h)")
}

// TestDetectPowerOnHoursRollover_HeadFlyingExceeds flags a wrap when Head Flying Hours exceed PoH.
func TestDetectPowerOnHoursRollover_HeadFlyingExceeds(t *testing.T) {
	current := ataSmart(map[string]measurements.SmartAttribute{
		"9":   &measurements.SmartAtaAttribute{AttributeId: 9, TransformedValue: 12},
		"240": &measurements.SmartAtaAttribute{AttributeId: 240, RawString: "70000h+00m+00.000s"},
	})

	current.DetectPowerOnHoursRollover(nil)

	attr := current.Attributes["9"].(*measurements.SmartAtaAttribute)
	require.True(t, pkg.AttributeStatusHas(attr.Status, pkg.AttributeStatusWarningScrutiny))
	require.Contains(t, attr.StatusReason, "Head Flying Hours (70000h)")
}

// TestDetectPowerOnHoursRollover_NormalIncrease leaves a normally-increasing PoH untouched.
func TestDetectPowerOnHoursRollover_NormalIncrease(t *testing.T) {
	current := ataSmart(map[string]measurements.SmartAttribute{
		"9": &measurements.SmartAtaAttribute{AttributeId: 9, TransformedValue: 1100},
	})
	previous := ataSmart(map[string]measurements.SmartAttribute{
		"9": &measurements.SmartAtaAttribute{AttributeId: 9, TransformedValue: 1000},
	})

	current.DetectPowerOnHoursRollover(&previous)

	attr := current.Attributes["9"].(*measurements.SmartAtaAttribute)
	require.Equal(t, pkg.AttributeStatusPassed, attr.Status)
	require.Empty(t, attr.StatusReason)
}

// TestDetectPowerOnHoursRollover_WithinMargin does not flag small cross-attribute differences.
func TestDetectPowerOnHoursRollover_WithinMargin(t *testing.T) {
	current := ataSmart(map[string]measurements.SmartAttribute{
		"9":        &measurements.SmartAtaAttribute{AttributeId: 9, TransformedValue: 1000},
		"farm_poh": &measurements.SmartFarmAttribute{AttributeId: "farm_poh", Value: 1050}, // +50, below 100 margin
	})

	current.DetectPowerOnHoursRollover(nil)

	attr := current.Attributes["9"].(*measurements.SmartAtaAttribute)
	require.Equal(t, pkg.AttributeStatusPassed, attr.Status)
	require.Empty(t, attr.StatusReason)
}

// TestDetectPowerOnHoursRollover_NilPreviousNoCrossAttrs is a no-op without history or cross-attrs.
func TestDetectPowerOnHoursRollover_NilPreviousNoCrossAttrs(t *testing.T) {
	current := ataSmart(map[string]measurements.SmartAttribute{
		"9": &measurements.SmartAtaAttribute{AttributeId: 9, TransformedValue: 1000},
	})

	current.DetectPowerOnHoursRollover(nil)

	attr := current.Attributes["9"].(*measurements.SmartAtaAttribute)
	require.Equal(t, pkg.AttributeStatusPassed, attr.Status)
	require.Empty(t, attr.StatusReason)
}

// TestDetectPowerOnHoursRollover_NonAta is a no-op for non-ATA protocols.
func TestDetectPowerOnHoursRollover_NonAta(t *testing.T) {
	current := measurements.Smart{
		Date:           time.Now(),
		DeviceWWN:      "test-wwn",
		DeviceProtocol: pkg.DeviceProtocolNvme,
		Attributes: map[string]measurements.SmartAttribute{
			"power_on_hours": &measurements.SmartNvmeAttribute{AttributeId: "power_on_hours", Value: 12},
		},
	}

	current.DetectPowerOnHoursRollover(nil)

	attr := current.Attributes["power_on_hours"].(*measurements.SmartNvmeAttribute)
	require.Equal(t, pkg.AttributeStatusPassed, attr.Status)
}

// TestDetectPowerOnHoursRollover_PreservesExistingReason appends to an existing status reason.
func TestDetectPowerOnHoursRollover_PreservesExistingReason(t *testing.T) {
	current := ataSmart(map[string]measurements.SmartAttribute{
		"9": &measurements.SmartAtaAttribute{
			AttributeId:      9,
			TransformedValue: 12,
			Status:           pkg.AttributeStatusWarningScrutiny,
			StatusReason:     "Attribute has previously failed manufacturer SMART threshold",
		},
	})
	previous := ataSmart(map[string]measurements.SmartAttribute{
		"9": &measurements.SmartAtaAttribute{AttributeId: 9, TransformedValue: 65000},
	})

	current.DetectPowerOnHoursRollover(&previous)

	attr := current.Attributes["9"].(*measurements.SmartAtaAttribute)
	require.Contains(t, attr.StatusReason, "Attribute has previously failed manufacturer SMART threshold")
	require.Contains(t, attr.StatusReason, "possible 16-bit counter rollover")
}
