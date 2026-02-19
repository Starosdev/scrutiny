package performance

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const sampleProcMounts = `sysfs /sys sysfs rw,nosuid,nodev,noexec,relatime 0 0
proc /proc proc rw,nosuid,nodev,noexec,relatime 0 0
udev /dev devtmpfs rw,nosuid,relatime,size=16384k 0 0
tmpfs /dev/shm tmpfs rw,nosuid,nodev 0 0
tmpfs /run tmpfs rw,nosuid,nodev,size=3276880k,mode=755 0 0
/dev/sda1 /boot ext4 rw,relatime 0 0
/dev/sdb1 /mnt/data xfs rw,relatime,attr2,inode64,logbufs=8 0 0
/dev/nvme0n1p1 /mnt/fast ext4 rw,relatime 0 0
overlay /var/lib/docker/overlay2/abc123/merged overlay rw,relatime 0 0
`

func TestGetFsTypeFromReader_Tmpfs(t *testing.T) {
	reader := strings.NewReader(sampleProcMounts)
	fsType, err := getFsTypeFromReader(reader, "/dev")
	require.NoError(t, err)
	require.Equal(t, "devtmpfs", fsType)
}

func TestGetFsTypeFromReader_TmpfsShm(t *testing.T) {
	reader := strings.NewReader(sampleProcMounts)
	fsType, err := getFsTypeFromReader(reader, "/dev/shm")
	require.NoError(t, err)
	require.Equal(t, "tmpfs", fsType)
}

func TestGetFsTypeFromReader_RealFilesystem(t *testing.T) {
	reader := strings.NewReader(sampleProcMounts)
	fsType, err := getFsTypeFromReader(reader, "/mnt/data")
	require.NoError(t, err)
	require.Equal(t, "xfs", fsType)
}

func TestGetFsTypeFromReader_NvmeFilesystem(t *testing.T) {
	reader := strings.NewReader(sampleProcMounts)
	fsType, err := getFsTypeFromReader(reader, "/mnt/fast")
	require.NoError(t, err)
	require.Equal(t, "ext4", fsType)
}

func TestGetFsTypeFromReader_LongestMatch(t *testing.T) {
	// /dev/shm should match "tmpfs" (longer match) not "devtmpfs" (shorter /dev match)
	reader := strings.NewReader(sampleProcMounts)
	fsType, err := getFsTypeFromReader(reader, "/dev/shm")
	require.NoError(t, err)
	require.Equal(t, "tmpfs", fsType)
}

func TestGetFsTypeFromReader_SubdirMatch(t *testing.T) {
	// A file inside /mnt/data should resolve to the /mnt/data mount point
	reader := strings.NewReader(sampleProcMounts)
	fsType, err := getFsTypeFromReader(reader, "/mnt/data/subdir")
	require.NoError(t, err)
	require.Equal(t, "xfs", fsType)
}

func TestGetFsTypeFromReader_NotFound(t *testing.T) {
	reader := strings.NewReader(sampleProcMounts)
	_, err := getFsTypeFromReader(reader, "/nonexistent")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestGetFsTypeFromReader_EmptyInput(t *testing.T) {
	reader := strings.NewReader("")
	_, err := getFsTypeFromReader(reader, "/dev")
	require.Error(t, err)
}
