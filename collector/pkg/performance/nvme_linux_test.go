package performance

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNvmeNamespacePath_Standard(t *testing.T) {
	require.Equal(t, "/dev/nvme0n1", nvmeNamespacePath("/dev/nvme0"))
}

func TestNvmeNamespacePath_SecondController(t *testing.T) {
	require.Equal(t, "/dev/nvme1n1", nvmeNamespacePath("/dev/nvme1"))
}

func TestNvmeNamespacePath_DoubleDigitController(t *testing.T) {
	require.Equal(t, "/dev/nvme10n1", nvmeNamespacePath("/dev/nvme10"))
}

func TestIsBlockDevice_CharDevice(t *testing.T) {
	// /dev/null is a character device, not a block device
	isBlock, err := isBlockDevice("/dev/null")
	require.NoError(t, err)
	require.False(t, isBlock)
}

func TestIsBlockDevice_NonExistent(t *testing.T) {
	_, err := isBlockDevice("/dev/nonexistent_device_xyz")
	require.Error(t, err)
}

func TestResolveNVMeBlockDevice_NonExistent(t *testing.T) {
	_, err := resolveNVMeBlockDevice("/dev/nvme99")
	require.Error(t, err)
	require.Contains(t, err.Error(), "nvme99n1")
}
