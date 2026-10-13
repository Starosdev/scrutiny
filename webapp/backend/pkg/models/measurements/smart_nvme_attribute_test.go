package measurements

import (
	"testing"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/stretchr/testify/require"
)

func TestNvmeCriticalWarning(t *testing.T) {
	attr := (&SmartNvmeAttribute{AttributeId: "critical_warning", Value: 0, Threshold: 0}).PopulateAttributeStatus()
	require.Equal(t, pkg.AttributeStatusPassed, attr.Status)
	require.Empty(t, attr.StatusReason)

	attr = (&SmartNvmeAttribute{AttributeId: "critical_warning", Value: 0b001001, Threshold: 0}).PopulateAttributeStatus()
	require.Equal(t, pkg.AttributeStatusFailedSmart|pkg.AttributeStatusFailedScrutiny, attr.Status)
	require.Contains(t, attr.StatusReason, "available spare capacity is below the spare threshold")
	require.Contains(t, attr.StatusReason, "read-only mode")
	require.NotContains(t, attr.StatusReason, "temperature")

	attr = (&SmartNvmeAttribute{AttributeId: "critical_warning", Value: 0x80, Threshold: 0}).PopulateAttributeStatus()
	require.Contains(t, attr.StatusReason, "0x80")
}

func TestNvmePercentageUsedBands(t *testing.T) {
	for value, want := range map[int64]pkg.AttributeStatus{
		89:  pkg.AttributeStatusPassed,
		90:  pkg.AttributeStatusWarningScrutiny,
		100: pkg.AttributeStatusWarningScrutiny,
		101: pkg.AttributeStatusFailedScrutiny,
	} {
		attr := (&SmartNvmeAttribute{AttributeId: "percentage_used", Value: value, Threshold: 100}).PopulateAttributeStatus()
		require.Equal(t, want, attr.Status, "value %d", value)
	}
}
