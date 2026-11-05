package notify

import (
	"testing"

	colmodels "github.com/analogj/scrutiny/webapp/backend/pkg/models/collector"
	"github.com/stretchr/testify/require"
)

const mdadmTestUUID = "ad005595:61305895:4de01d37:669bf162"

func degraded(active, failed int) colmodels.MDADMMetrics {
	return colmodels.MDADMMetrics{State: "clean, degraded", ActiveDevices: active, FailedDevices: failed}
}

// evaluate runs one collection and reports whether a send was attempted.
func evaluate(tracker *MDADMTracker, metrics colmodels.MDADMMetrics, repeat, delivered bool) bool {
	attempted := false
	tracker.Evaluate(mdadmTestUUID, metrics, repeat, func() bool {
		attempted = true
		return delivered
	})
	return attempted
}

func TestMDADMTrackerAnnouncesAnUnchangedDegradationOnce(t *testing.T) {
	tracker := NewMDADMTracker()
	require.True(t, evaluate(tracker, degraded(1, 1), false, true))
	require.False(t, evaluate(tracker, degraded(1, 1), false, true))
}

// With repeat notifications on, a degraded array is announced on every collection, as a
// failing SMART attribute is (#1109).
func TestMDADMTrackerAnnouncesEveryCollectionWhenRepeatIsOn(t *testing.T) {
	tracker := NewMDADMTracker()
	require.True(t, evaluate(tracker, degraded(1, 1), true, true))
	require.True(t, evaluate(tracker, degraded(1, 1), true, true))
}

// A failed delivery is not recorded, so the next collection tries again (#1109).
func TestMDADMTrackerRetriesAfterAFailedDelivery(t *testing.T) {
	tracker := NewMDADMTracker()
	require.True(t, evaluate(tracker, degraded(1, 1), false, false))
	require.True(t, evaluate(tracker, degraded(1, 1), false, true))
	require.False(t, evaluate(tracker, degraded(1, 1), false, true))
}

func TestMDADMTrackerAnnouncesAWorseDegradation(t *testing.T) {
	tracker := NewMDADMTracker()
	require.True(t, evaluate(tracker, degraded(3, 1), false, true))
	require.True(t, evaluate(tracker, degraded(2, 2), false, true), "lost a member")
	require.True(t, evaluate(tracker, degraded(2, 3), false, true), "gained a failed member")
}

// An improvement is not announced but becomes the baseline, so losing the member again is.
func TestMDADMTrackerAnnouncesARelapseAfterAnImprovement(t *testing.T) {
	tracker := NewMDADMTracker()
	require.True(t, evaluate(tracker, degraded(2, 2), false, true))
	require.False(t, evaluate(tracker, degraded(3, 1), false, true))
	require.True(t, evaluate(tracker, degraded(2, 2), false, true))
}

func TestMDADMTrackerAnnouncesAgainAfterRecovery(t *testing.T) {
	tracker := NewMDADMTracker()
	require.True(t, evaluate(tracker, degraded(1, 1), false, true))
	tracker.Forget(mdadmTestUUID)
	require.True(t, evaluate(tracker, degraded(1, 1), false, true))
}
