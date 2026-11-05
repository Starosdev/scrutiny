package detect

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func readFixture(t *testing.T, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return string(content)
}

func TestUnraidArrayDeviceMapPairsArrayDevicesWithTheirDisks(t *testing.T) {
	deviceMap := unraidArrayDeviceMap(readFixture(t, "mdstat_unraid_started.txt"))

	// diskName.1=md1p1 with rdevName.1=sdf, which is the pairing sysfs cannot
	// express for an Unraid array device.
	require.Equal(t, "sdf", deviceMap["md1p1"])
	require.Equal(t, "sdo", deviceMap["md2p1"])
	require.Equal(t, "sdn", deviceMap["md3p1"])

	// Slot 0 is parity: it has an rdevName but no diskName, so it contributes
	// no mapping and its disk is never credited an array filesystem.
	require.NotContains(t, deviceMap, "")
	for arrayDevice := range deviceMap {
		require.NotEmpty(t, arrayDevice)
	}
}

func TestUnraidArrayDeviceMapSkipsSlotsWithNoLiveDisk(t *testing.T) {
	// A disabled or missing disk keeps its recorded identity but has no
	// rdevName, so mapping its array device to nothing is the correct answer:
	// attributing the filesystem to a guessed disk would be worse.
	deviceMap := unraidArrayDeviceMap(readFixture(t, "mdstat_unraid_missing.txt"))

	for arrayDevice, disk := range deviceMap {
		require.NotEmpty(t, disk, "array device %s mapped to an empty disk", arrayDevice)
	}
}

func TestUnraidArrayDeviceMapIgnoresAStandardLinuxMdstat(t *testing.T) {
	// The standard format carries none of these keys, and reading it as Unraid
	// would invent pairings.
	linux := "Personalities : [raid1]\nmd0 : active raid1 sdb1[1] sda1[0]\n      976630464 blocks super 1.2 [2/2] [UU]\n\nunused devices: <none>\n"
	require.False(t, isUnraidMdstat(linux))
	require.Empty(t, unraidArrayDeviceMap(linux))
}

func TestUnraidArrayDeviceMapIgnoresUnindexedAndEmptyKeys(t *testing.T) {
	content := "mdState=STARTED\ndiskName.1=md1p1\nrdevName.1=sdf\ndiskName=ignored\nrdevName.2=\ndiskName.2=md2p1\ndiskName.=skipped\n"
	deviceMap := unraidArrayDeviceMap(content)

	require.Equal(t, map[string]string{"md1p1": "sdf"}, deviceMap)
}
