package database

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// fixes #851: workload queries group by device_id and device_wwn, so one device can return a first
// and a last snapshot for its tagged points and another pair for its older untagged points.
func TestRecordWorkloadSnapshotKeepsEarliestFirstAndLatestLast(t *testing.T) {
	early := &workloadSnapshot{Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	late := &workloadSnapshot{Time: early.Time.Add(24 * time.Hour)}

	for name, order := range map[string][]*workloadSnapshot{
		"early first": {early, late},
		"late first":  {late, early},
	} {
		t.Run(name, func(t *testing.T) {
			firstPoints := map[string]*workloadSnapshot{}
			lastPoints := map[string]*workloadSnapshot{}
			for _, snap := range order {
				recordWorkloadSnapshot(firstPoints, "device-1", snap, true)
				recordWorkloadSnapshot(lastPoints, "device-1", snap, false)
			}
			require.Same(t, early, firstPoints["device-1"])
			require.Same(t, late, lastPoints["device-1"])
		})
	}
}

func TestNewestWorkloadSnapshotsMergesGroupsNewestFirst(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	snapshotAt := func(hours int) *workloadSnapshot {
		return &workloadSnapshot{Time: base.Add(time.Duration(hours) * time.Hour)}
	}
	// the tagged group followed by the untagged group, each newest first on its own
	points := []*workloadSnapshot{snapshotAt(5), snapshotAt(4), snapshotAt(3), snapshotAt(6), snapshotAt(1)}

	newest := newestWorkloadSnapshots(points)

	require.Len(t, newest, workloadRecentPointLimit)
	require.Equal(t,
		[]time.Time{base.Add(6 * time.Hour), base.Add(5 * time.Hour), base.Add(4 * time.Hour)},
		[]time.Time{newest[0].Time, newest[1].Time, newest[2].Time})
}
