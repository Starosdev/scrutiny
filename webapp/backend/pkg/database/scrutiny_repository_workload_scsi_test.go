package database

import (
	"testing"
	"time"

	"github.com/analogj/scrutiny/webapp/backend/pkg"
	"github.com/analogj/scrutiny/webapp/backend/pkg/models"
	"github.com/stretchr/testify/require"
)

// TestParseWorkloadSnapshot_ScsiGigabytesProcessed verifies that the SCSI
// read/write_gigabytes_processed attribute fields are extracted into the workload snapshot.
func TestParseWorkloadSnapshot_ScsiGigabytesProcessed(t *testing.T) {
	values := map[string]interface{}{
		"attr.write_gigabytes_processed.value": int64(243990000000),
		"attr.read_gigabytes_processed.value":  int64(7668000000),
		"attr.percentage_used.value":           int64(3),
	}

	snap := parseWorkloadSnapshot(values)

	require.True(t, snap.hasScsiWriteGigabytesProcessed)
	require.Equal(t, int64(243990000000), snap.ScsiWriteGigabytesProcessed)
	require.True(t, snap.hasScsiReadGigabytesProcessed)
	require.Equal(t, int64(7668000000), snap.ScsiReadGigabytesProcessed)
	require.True(t, snap.hasPercentageUsed)
	require.Equal(t, int64(3), snap.PercentageUsed)
}

// TestComputeSCSIWorkload verifies that cumulative byte deltas are computed correctly from
// two snapshots, mirroring how ATA/NVMe compute their workload deltas.
func TestComputeSCSIWorkload(t *testing.T) {
	repo := &scrutinyRepository{}

	first := &workloadSnapshot{
		ScsiWriteGigabytesProcessed:    100_000_000_000,
		hasScsiWriteGigabytesProcessed: true,
		ScsiReadGigabytesProcessed:     10_000_000_000,
		hasScsiReadGigabytesProcessed:  true,
	}
	last := &workloadSnapshot{
		ScsiWriteGigabytesProcessed:    243_990_000_000,
		hasScsiWriteGigabytesProcessed: true,
		ScsiReadGigabytesProcessed:     7_668_000_000_000, // simulate large cumulative growth
		hasScsiReadGigabytesProcessed:  true,
	}

	writtenBytes, readBytes := repo.computeSCSIWorkload(first, last)
	require.Equal(t, int64(143_990_000_000), writtenBytes)
	require.Equal(t, int64(7_658_000_000_000), readBytes)
}

// TestComputeSCSIWorkload_MissingCounters verifies that a missing counter on either side of the
// delta computation results in a zero delta (e.g. drives that don't report gigabytes_processed).
func TestComputeSCSIWorkload_MissingCounters(t *testing.T) {
	repo := &scrutinyRepository{}

	first := &workloadSnapshot{}
	last := &workloadSnapshot{
		ScsiWriteGigabytesProcessed:    100,
		hasScsiWriteGigabytesProcessed: true,
	}

	writtenBytes, readBytes := repo.computeSCSIWorkload(first, last)
	require.Equal(t, int64(0), writtenBytes)
	require.Equal(t, int64(0), readBytes)
}

// TestEndurancePercentageUsed_Scsi verifies that SCSI SAS SSD endurance (shared
// "percentage_used" attribute ID with NVMe) is picked up by the endurance calculation.
func TestEndurancePercentageUsed_Scsi(t *testing.T) {
	snap := &workloadSnapshot{
		PercentageUsed:    3,
		hasPercentageUsed: true,
	}

	percentageUsed, hasPercentage := endurancePercentageUsed(snap, pkg.DeviceProtocolScsi)
	require.True(t, hasPercentage)
	require.Equal(t, int64(3), percentageUsed)
}

// TestEndurancePercentageUsed_Scsi_NotReported verifies SCSI HDDs (no endurance_used field,
// so no percentage_used attribute) correctly report no endurance data rather than a false 0%.
func TestEndurancePercentageUsed_Scsi_NotReported(t *testing.T) {
	snap := &workloadSnapshot{}

	_, hasPercentage := endurancePercentageUsed(snap, pkg.DeviceProtocolScsi)
	require.False(t, hasPercentage)
}

// TestGetCumulativeWriteBytes_Scsi verifies the SCSI branch of getCumulativeWriteBytes used for
// TBW estimation.
func TestGetCumulativeWriteBytes_Scsi(t *testing.T) {
	repo := &scrutinyRepository{}
	snap := &workloadSnapshot{
		ScsiWriteGigabytesProcessed:    243_990_000_000,
		hasScsiWriteGigabytesProcessed: true,
	}

	require.Equal(t, int64(243_990_000_000), repo.getCumulativeWriteBytes(snap, pkg.DeviceProtocolScsi))
}

// TestComputeWorkloadInsight_Scsi is an end-to-end check (without InfluxDB) that a SCSI SAS SSD
// with gigabytes_processed and percentage_used populated no longer reports "unknown" intensity.
func TestComputeWorkloadInsight_Scsi(t *testing.T) {
	repo := &scrutinyRepository{}

	baseTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	first := &workloadSnapshot{
		Time:                           baseTime,
		PowerOnHours:                   45000,
		ScsiWriteGigabytesProcessed:    100_000_000_000,
		hasScsiWriteGigabytesProcessed: true,
		ScsiReadGigabytesProcessed:     5_000_000_000,
		hasScsiReadGigabytesProcessed:  true,
	}
	last := &workloadSnapshot{
		Time:                           baseTime.Add(48 * time.Hour),
		PowerOnHours:                   45048,
		ScsiWriteGigabytesProcessed:    243_990_000_000,
		hasScsiWriteGigabytesProcessed: true,
		ScsiReadGigabytesProcessed:     7_668_000_000,
		hasScsiReadGigabytesProcessed:  true,
		PercentageUsed:                 3,
		hasPercentageUsed:              true,
	}

	insight := &models.WorkloadInsight{}
	repo.computeWorkloadInsight(insight, first, last, pkg.DeviceProtocolScsi, nil)

	require.NotEqual(t, "unknown", insight.Intensity)
	require.Greater(t, insight.TotalWriteBytes, int64(0))
	require.Greater(t, insight.TotalReadBytes, int64(0))
	require.NotNil(t, insight.Endurance)
	require.True(t, insight.Endurance.Available)
	require.Equal(t, int64(3), insight.Endurance.PercentageUsed)
}
