package filesystem

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTranslateDeviceNamesReducesNVMeNamespacesToTheirController(t *testing.T) {
	// The SMART collector registers nvme0, so a filesystem resolved to the
	// namespace nvme0n1 would otherwise match no drive (#1057).
	require.Equal(t, []string{"nvme0"}, translateDeviceNames([]string{"nvme0n1"}, nil))
	require.Equal(t, []string{"nvme12"}, translateDeviceNames([]string{"nvme12n3"}, nil))

	// Two namespaces on one controller are one physical drive, so they collapse
	// rather than attributing the same drive twice.
	require.Equal(t, []string{"nvme0"}, translateDeviceNames([]string{"nvme0n1", "nvme0n2"}, nil))

	// Two controllers stay distinct.
	require.Equal(t, []string{"nvme0", "nvme1"}, translateDeviceNames([]string{"nvme0n1", "nvme1n1"}, nil))
}

func TestTranslateDeviceNamesLeavesOtherNamesAlone(t *testing.T) {
	for _, name := range []string{"sda", "sdm", "md0", "dm-0", "mmcblk0", "nvme0", "nvmexn1", "nvme0nx", "nvmen1"} {
		require.Equal(t, []string{name}, translateDeviceNames([]string{name}, nil), name)
	}
}

func TestTranslateDeviceNamesMapsUnraidArrayDevicesToTheirDisks(t *testing.T) {
	// Unraid's md1p1 has no holder link to the disk under it, so the pairing
	// comes from its mdstat (#1058).
	unraid := map[string]string{"md1p1": "sdf", "md2p1": "sdo"}

	require.Equal(t, []string{"sdf"}, translateDeviceNames([]string{"md1p1"}, unraid))
	require.Equal(t, []string{"sdf", "sdo"}, translateDeviceNames([]string{"md1p1", "md2p1"}, unraid))

	// An array device absent from the map keeps its own name, which the server
	// then matches against no registered drive.
	require.Equal(t, []string{"md9p1"}, translateDeviceNames([]string{"md9p1"}, unraid))

	// Two array devices on one disk collapse to that disk once.
	require.Equal(t, []string{"sdf"}, translateDeviceNames([]string{"md1p1", "md1p1"}, unraid))
}

func TestTranslateDeviceNamesHandlesAnEmptyInput(t *testing.T) {
	// An unattributed filesystem stays unattributed: nil, never an empty
	// non-nil slice, so omitempty keeps it out of the upload.
	require.Nil(t, translateDeviceNames(nil, nil))
	require.Nil(t, translateDeviceNames([]string{}, map[string]string{"md1p1": "sdf"}))
}

func TestNVMeControllerName(t *testing.T) {
	tests := map[string]string{
		"nvme0n1":   "nvme0",
		"nvme0n1p2": "nvme0n1p2",
		"nvme10n20": "nvme10",
		"nvme0":     "nvme0",
		"nvme":      "nvme",
		"nvmen1":    "nvmen1",
		"nvme0nx":   "nvme0nx",
		"nvmexn1":   "nvmexn1",
		"sda":       "sda",
		"nvme0n1n2": "nvme0n1n2",
	}

	for in, want := range tests {
		require.Equal(t, want, nvmeControllerName(in), in)
	}
}
