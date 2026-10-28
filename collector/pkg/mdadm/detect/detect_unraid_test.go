package detect

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const linuxMdstat = `Personalities : [raid1] [linear] [multipath] [raid0] [raid6] [raid5] [raid4] [raid10]
md0 : active raid1 sdb[1] sda[0]
      1048512 blocks super 1.2 [2/2] [UU]

unused devices: <none>`

func loadMdstatFixture(t *testing.T, name string) string {
	t.Helper()
	content, err := os.ReadFile("testdata/" + name)
	require.NoError(t, err)
	return string(content)
}

func TestIsUnraidMdstat(t *testing.T) {
	assert.True(t, isUnraidMdstat(loadMdstatFixture(t, "mdstat_unraid_started.txt")))
	assert.False(t, isUnraidMdstat(linuxMdstat))
	assert.False(t, isUnraidMdstat(""))
}

// TestBuildUnraidArray_Started uses a sanitized copy of a live Unraid host's
// /proc/mdstat: 1 parity disk, 11 data disks, and an empty second parity slot.
func TestBuildUnraidArray_Started(t *testing.T) {
	array, metrics := buildUnraidArray(loadMdstatFixture(t, "mdstat_unraid_started.txt"))

	assert.Equal(t, "unraid:0000-0000-0000-000000000000", array.UUID)
	assert.Equal(t, "unraid-array", array.Name)
	assert.Equal(t, "unraid-parity1", array.Level)
	assert.Equal(t, []string{
		"/dev/sda", "/dev/sdf", "/dev/sdo", "/dev/sdn", "/dev/sdh", "/dev/sdk",
		"/dev/sdj", "/dev/sdg", "/dev/sde", "/dev/sdc", "/dev/sdi", "/dev/sdb",
	}, array.Devices)

	assert.Equal(t, "active", metrics.State)
	assert.Equal(t, 12, metrics.ActiveDevices)
	assert.Equal(t, 12, metrics.WorkingDevices)
	assert.Equal(t, 0, metrics.FailedDevices)
	assert.Equal(t, 0, metrics.SpareDevices)
	assert.Equal(t, float64(0), metrics.SyncProgress)
	// Data disks only; the parity disk contributes no usable capacity.
	assert.Equal(t, int64(104008147398656), metrics.ArraySize)
	assert.Equal(t, int64(0), metrics.UsedBytes)
}

// TestBuildUnraidArray_EmptyParitySlotIsNotAFailure pins the behavior that
// makes mdNumDisabled unusable as a health signal: a single-parity array
// reports its empty second parity slot as DISK_NP_DSBL, and mdNumDisabled
// counts it, so a healthy host reads mdNumDisabled=1.
func TestBuildUnraidArray_EmptyParitySlotIsNotAFailure(t *testing.T) {
	content := loadMdstatFixture(t, "mdstat_unraid_started.txt")
	require.Contains(t, content, "mdNumDisabled=1")
	require.Contains(t, content, "rdevStatus.29=DISK_NP_DSBL")

	_, metrics := buildUnraidArray(content)

	assert.Equal(t, 0, metrics.FailedDevices)
	assert.Equal(t, "active", metrics.State)
}

func TestBuildUnraidArray_Degraded(t *testing.T) {
	array, metrics := buildUnraidArray(loadMdstatFixture(t, "mdstat_unraid_degraded.txt"))

	assert.Equal(t, "unraid:1111-2222-3333-444455556666", array.UUID)
	assert.Equal(t, "unraid-parity1", array.Level)
	assert.Equal(t, []string{"/dev/sda", "/dev/sdf", "/dev/sdg"}, array.Devices)

	assert.Equal(t, "active, degraded", metrics.State)
	assert.Equal(t, 2, metrics.ActiveDevices)
	assert.Equal(t, 2, metrics.WorkingDevices)
	assert.Equal(t, 1, metrics.FailedDevices)
	assert.Equal(t, int64(8001573953536), metrics.ArraySize)
	assert.Equal(t, float64(0), metrics.SyncProgress)
}

func TestBuildUnraidArray_Checking(t *testing.T) {
	_, metrics := buildUnraidArray(loadMdstatFixture(t, "mdstat_unraid_checking.txt"))

	assert.Equal(t, "active, checking", metrics.State)
	assert.Equal(t, 3, metrics.ActiveDevices)
	assert.Equal(t, 0, metrics.FailedDevices)
	assert.InDelta(t, 40.0, metrics.SyncProgress, 0.001)
}

func TestBuildUnraidArray_CheckingPaused(t *testing.T) {
	content := strings.Replace(
		loadMdstatFixture(t, "mdstat_unraid_checking.txt"),
		"mdResync=15625879500", "mdResync=0", 1)

	_, metrics := buildUnraidArray(content)

	assert.Equal(t, "active, checking, paused", metrics.State)
	// Progress survives the pause rather than collapsing to zero.
	assert.InDelta(t, 40.0, metrics.SyncProgress, 0.001)
}

func TestBuildUnraidArray_Stopped(t *testing.T) {
	content := strings.Replace(
		loadMdstatFixture(t, "mdstat_unraid_started.txt"),
		"mdState=STARTED", "mdState=STOPPED", 1)

	_, metrics := buildUnraidArray(content)

	assert.Equal(t, "inactive (stopped)", metrics.State)
	// Membership is still reported while the array is stopped.
	assert.Equal(t, 12, metrics.ActiveDevices)
}

func TestBuildUnraidArray_MissingLabelLeavesUUIDEmptyForCallerFallback(t *testing.T) {
	content := strings.Replace(
		loadMdstatFixture(t, "mdstat_unraid_degraded.txt"),
		"sbLabel=1111-2222-3333-444455556666", "sbLabel=", 1)

	array, _ := buildUnraidArray(content)

	assert.Equal(t, "", array.UUID)
}

func TestUnraidLevel(t *testing.T) {
	assert.Equal(t, "unraid-noparity", unraidLevel(0))
	assert.Equal(t, "unraid-parity1", unraidLevel(1))
	assert.Equal(t, "unraid-parity2", unraidLevel(2))
}

// TestUnraidSyncProgress pins the percentage to mdResyncPos over
// mdResyncSize, which is how Unraid's own webgui computes it
// (/usr/local/emhttp/plugins/dynamix/nchan/parity_list). mdResyncSize always
// holds the parity disk size, so it is never the in-progress signal.
func TestUnraidSyncProgress(t *testing.T) {
	assert.Equal(t, float64(0), unraidSyncProgress(map[string]string{"mdResyncSize": "400", "mdResyncPos": "0"}))
	assert.InDelta(t, 25.0, unraidSyncProgress(map[string]string{"mdResyncSize": "400", "mdResyncPos": "100"}), 0.001)
	// A position past the end is clamped rather than reported above 100%.
	assert.Equal(t, float64(100), unraidSyncProgress(map[string]string{"mdResyncSize": "100", "mdResyncPos": "250"}))
	assert.Equal(t, float64(0), unraidSyncProgress(map[string]string{"mdResyncSize": "0", "mdResyncPos": "100"}))
	assert.Equal(t, float64(0), unraidSyncProgress(map[string]string{}))
	// A paused operation keeps its progress: mdResync is zero, mdResyncPos is not.
	paused := map[string]string{"mdResync": "0", "mdResyncSize": "400", "mdResyncPos": "100"}
	assert.InDelta(t, 25.0, unraidSyncProgress(paused), 0.001)
}

// TestUnraidSyncAction covers the operations Unraid reports in mdResyncAction.
func TestUnraidSyncAction(t *testing.T) {
	tests := map[string]string{
		"check P":     "checking",
		"check":       "checking",
		"recon P":     "recovery",
		"clear":       "resync",
		"nocorrect P": "resync",
	}

	for action, expected := range tests {
		keys := map[string]string{"mdResync": "1000", "mdResyncPos": "1", "mdResyncAction": action}
		assert.Equal(t, expected, unraidSyncAction(keys), "action %q", action)
	}

	// Idle: mdResyncAction keeps the last operation's name after it completes,
	// so mdResyncPos is what decides.
	idle := map[string]string{"mdResync": "0", "mdResyncPos": "0", "mdResyncAction": "check P"}
	assert.Equal(t, "", unraidSyncAction(idle))

	// Paused: mdResync drops to zero but the operation is still in progress.
	paused := map[string]string{"mdResync": "0", "mdResyncPos": "500", "mdResyncAction": "check P"}
	assert.Equal(t, "checking", unraidSyncAction(paused))
}

func TestUnraidSyncPaused(t *testing.T) {
	assert.True(t, unraidSyncPaused(map[string]string{"mdResyncPos": "500", "mdResync": "0"}))
	assert.False(t, unraidSyncPaused(map[string]string{"mdResyncPos": "500", "mdResync": "15625879500"}))
	assert.False(t, unraidSyncPaused(map[string]string{"mdResyncPos": "0", "mdResync": "0"}))
}

// TestUnraidStateUsesScrutinyVocabulary pins the state strings the web UI
// colors on (webapp/frontend/src/app/modules/mdadm/mdadm-status.util.ts).
func TestUnraidStateUsesScrutinyVocabulary(t *testing.T) {
	started := map[string]string{"mdState": "STARTED", "mdResync": "0", "mdResyncPos": "0"}
	assert.Equal(t, "active", unraidState(started, 0))
	assert.Equal(t, "active, degraded", unraidState(started, 2))

	syncing := map[string]string{
		"mdState": "STARTED", "mdResync": "1000", "mdResyncPos": "100", "mdResyncAction": "recon P",
	}
	assert.Equal(t, "active, recovery", unraidState(syncing, 0))
	// Degraded takes color priority over a running rebuild, which is why both
	// terms are emitted.
	assert.Equal(t, "active, degraded, recovery", unraidState(syncing, 1))

	paused := map[string]string{
		"mdState": "STARTED", "mdResync": "0", "mdResyncPos": "100", "mdResyncAction": "check P",
	}
	assert.Equal(t, "active, checking, paused", unraidState(paused, 0))

	assert.Equal(t, "inactive (new)", unraidState(map[string]string{"mdState": "NEW"}, 0))
	assert.Equal(t, "inactive (unknown)", unraidState(map[string]string{}, 0))
}

// TestUnraidRawSummaryDropsPerSlotKeys keeps raw_mdstat bounded: it is stored
// on every metrics point, and a live Unraid mdstat is ~450 lines.
func TestUnraidRawSummaryDropsPerSlotKeys(t *testing.T) {
	content := loadMdstatFixture(t, "mdstat_unraid_started.txt")
	summary := unraidRawSummary(content)

	assert.Less(t, len(strings.Split(summary, "\n")), 40)
	assert.Contains(t, summary, "mdState=STARTED")
	assert.Contains(t, summary, "mdResyncAction=check P")
	assert.NotContains(t, summary, "rdevStatus.0=")
	assert.NotContains(t, summary, "diskName.1=")
}

func TestParseUnraidMdstatSortsSlotsByIndex(t *testing.T) {
	arrayKeys, slots := parseUnraidMdstat(loadMdstatFixture(t, "mdstat_unraid_degraded.txt"))

	assert.Equal(t, "STARTED", arrayKeys["mdState"])
	require.Len(t, slots, 4)
	assert.Equal(t, []int{0, 1, 2, 29}, []int{slots[0].index, slots[1].index, slots[2].index, slots[3].index})

	assert.False(t, slots[0].isData(), "slot 0 is parity and exports no md device")
	assert.True(t, slots[1].isData())
	assert.True(t, slots[2].present())
	assert.False(t, slots[2].healthy())
	assert.False(t, slots[3].present(), "an empty parity slot reports DISK_NP_DSBL")
}
