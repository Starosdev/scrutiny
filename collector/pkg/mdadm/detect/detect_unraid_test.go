package detect

import (
	"fmt"
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

	assert.Equal(t, "unraid:5aee7f08742c76a8b3ddab54750e383846b7ee9e", array.UUID)
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

	assert.Equal(t, "unraid:1fdf355dcd74e04627ded844745d556f3f13d840", array.UUID)
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

// TestBuildUnraidArray_Stopped covers a stopped array. Unraid exports the md
// device for a data disk only while the array is started, so every diskName is
// blanked here: classifying parity by diskName would make all 12 disks parity
// and collapse ArraySize to zero.
func TestBuildUnraidArray_Stopped(t *testing.T) {
	content := strings.Replace(
		loadMdstatFixture(t, "mdstat_unraid_started.txt"),
		"mdState=STARTED", "mdState=STOPPED", 1)
	for slot := 1; slot <= 11; slot++ {
		content = strings.Replace(content,
			fmt.Sprintf("diskName.%d=md%dp1", slot, slot),
			fmt.Sprintf("diskName.%d=", slot), 1)
	}
	require.NotContains(t, content, "p1")

	array, metrics := buildUnraidArray(content)

	assert.Equal(t, "inactive (stopped)", metrics.State)
	// Membership, level and capacity survive the stop.
	assert.Equal(t, 12, metrics.ActiveDevices)
	assert.Equal(t, "unraid-parity1", array.Level)
	assert.Equal(t, int64(104008147398656), metrics.ArraySize)
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
		"nocorrect P": "checking",
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

	assert.True(t, slots[0].isParity(), "slot 0 is the parity slot")
	assert.False(t, slots[1].isParity())
	assert.True(t, slots[2].attached())
	assert.False(t, slots[2].healthy())
	assert.True(t, slots[2].assigned())
	assert.False(t, slots[3].attached(), "an empty parity slot reports DISK_NP_DSBL")
	assert.False(t, slots[3].assigned(), "an empty parity slot records no identity")
	assert.False(t, slots[3].missing(), "an unassigned slot is absent, not missing")
}

// TestBuildUnraidArray_MissingDiskIsAFailure is the regression test for the
// case the collector exists to report. A pulled data disk keeps its superblock
// identity (diskId, diskSize) but reports rdevStatus=DISK_NP_MISSING with no
// live device. Treating every DISK_NP* status as an empty slot skipped it
// entirely, so the array read back "active" with zero failed devices and the
// backend sent no notification.
func TestBuildUnraidArray_MissingDiskIsAFailure(t *testing.T) {
	content := loadMdstatFixture(t, "mdstat_unraid_missing.txt")
	require.Contains(t, content, "rdevStatus.2=DISK_NP_MISSING")
	require.Contains(t, content, "mdNumMissing=1")
	require.Contains(t, content, "mdNumDisabled=0")

	array, metrics := buildUnraidArray(content)

	assert.Equal(t, 1, metrics.FailedDevices, "a missing disk must count as failed")
	assert.Equal(t, "active, degraded", metrics.State, "the UI colors on the degraded substring")
	assert.Equal(t, 2, metrics.ActiveDevices)
	assert.Equal(t, 2, metrics.WorkingDevices)
	// The absent disk has no live device name, so only the attached two appear.
	assert.Equal(t, []string{"/dev/sda", "/dev/sdf"}, array.Devices)
	// Its capacity is still part of the array as configured.
	assert.Equal(t, int64(8001573953536), metrics.ArraySize)
}

// TestBuildUnraidArray_MdNumMissingBacksstopsSilentLoss covers the case where a
// future Unraid release stops recording the identity of a missing disk. The
// per-slot scan would then see an empty slot, so mdNumMissing is used as a
// floor on the failure count rather than being ignored.
func TestBuildUnraidArray_MdNumMissingBackstopsSilentLoss(t *testing.T) {
	content := loadMdstatFixture(t, "mdstat_unraid_missing.txt")
	// Strip every trace of the missing disk except Unraid's own counter.
	for _, key := range []string{"diskName.2", "diskSize.2", "diskId.2"} {
		content = strings.Replace(content, key+"=md2p1", key+"=", 1)
		content = strings.Replace(content, key+"=3907018532", key+"=0", 1)
		content = strings.Replace(content, key+"=TESTDISK02_SERIAL02", key+"=", 1)
	}

	_, metrics := buildUnraidArray(content)

	assert.Equal(t, 1, metrics.FailedDevices, "mdNumMissing must still force a failure")
	assert.Equal(t, "active, degraded", metrics.State)
}

// TestUnraidRawSummaryWithholdsTheLicenseIdentifier keeps sbLabel out of
// raw_mdstat. It is derived from the Unraid flash GUID, and raw_mdstat is
// rendered verbatim on the array details page.
func TestUnraidRawSummaryWithholdsTheLicenseIdentifier(t *testing.T) {
	content := loadMdstatFixture(t, "mdstat_unraid_started.txt")
	require.Contains(t, content, "sbLabel=")

	summary := unraidRawSummary(content)

	assert.NotContains(t, summary, "sbLabel")
	assert.NotContains(t, summary, "0000-0000-0000-000000000000")
	// The rest of the array-level block is retained.
	assert.Contains(t, summary, "mdState=STARTED")
}

// TestBuildUnraidArray_UUIDIsADigestNotTheLicenseIdentifier pins that the array
// GUID never reaches the UUID, which the UI renders and notifications include.
func TestBuildUnraidArray_UUIDIsADigestNotTheLicenseIdentifier(t *testing.T) {
	array, _ := buildUnraidArray(loadMdstatFixture(t, "mdstat_unraid_started.txt"))

	assert.NotContains(t, array.UUID, "0000-0000-0000-000000000000")
	assert.Equal(t, "unraid:5aee7f08742c76a8b3ddab54750e383846b7ee9e", array.UUID)
}

// TestUnraidLinesHandlesLongLines covers what bufio.Scanner could not: a line
// beyond its 64KiB token limit would have truncated the parse silently.
func TestUnraidLinesHandlesLongLines(t *testing.T) {
	content := "mdState=STARTED\nnoise=" + strings.Repeat("x", 128*1024) + "\nmdNumDisks=3\n"

	arrayKeys, _ := parseUnraidMdstat(content)

	assert.Equal(t, "STARTED", arrayKeys["mdState"])
	assert.Equal(t, "3", arrayKeys["mdNumDisks"], "a key after an over-long line is still parsed")
}

// TestParseUnraidMdstatIgnoresMalformedInput covers lines the kernel would
// never emit but a corrupted or truncated read might.
func TestParseUnraidMdstatIgnoresMalformedInput(t *testing.T) {
	content := strings.Join([]string{
		"mdState=STARTED",
		"no-equals-sign-at-all",
		"",
		"diskSize.notanumber=123",
		"diskSize.1=-5",
		"diskId.1=TESTDISK01",
		"rdevStatus.1=DISK_OK",
		"diskSize.2=abc",
		"diskId.2=TESTDISK02",
		"rdevStatus.2=DISK_OK",
		"=emptykey",
	}, "\n")

	arrayKeys, slots := parseUnraidMdstat(content)

	assert.Equal(t, "STARTED", arrayKeys["mdState"])
	require.Len(t, slots, 2, "only well-formed slot indexes are kept")
	// A negative or unparseable size contributes nothing rather than shrinking
	// the array.
	assert.Equal(t, int64(0), slots[0].sizeBlocks)
	assert.Equal(t, int64(0), slots[1].sizeBlocks)

	_, metrics := buildUnraidArray(content)
	assert.Equal(t, int64(0), metrics.ArraySize)
	assert.Equal(t, 2, metrics.ActiveDevices)
}
