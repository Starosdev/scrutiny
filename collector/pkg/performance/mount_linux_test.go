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

func fsTypeForPath(t *testing.T, mounts string, path string) string {
	t.Helper()
	entry, err := findMountEntryForPath(strings.NewReader(mounts), path)
	require.NoError(t, err)
	return entry.FsType
}

func TestFindMountEntryForPath_Tmpfs(t *testing.T) {
	require.Equal(t, "devtmpfs", fsTypeForPath(t, sampleProcMounts, "/dev"))
}

func TestFindMountEntryForPath_RealFilesystem(t *testing.T) {
	require.Equal(t, "xfs", fsTypeForPath(t, sampleProcMounts, "/mnt/data"))
}

func TestFindMountEntryForPath_NvmeFilesystem(t *testing.T) {
	require.Equal(t, "ext4", fsTypeForPath(t, sampleProcMounts, "/mnt/fast"))
}

func TestFindMountEntryForPath_LongestMatch(t *testing.T) {
	// /dev/shm should match "tmpfs" (longer match) not "devtmpfs" (shorter /dev match)
	require.Equal(t, "tmpfs", fsTypeForPath(t, sampleProcMounts, "/dev/shm"))
}

func TestFindMountEntryForPath_SubdirMatch(t *testing.T) {
	// A file inside /mnt/data should resolve to the /mnt/data mount point
	require.Equal(t, "xfs", fsTypeForPath(t, sampleProcMounts, "/mnt/data/subdir"))
}

func TestFindMountEntryForPath_NotFound(t *testing.T) {
	_, err := findMountEntryForPath(strings.NewReader(sampleProcMounts), "/nonexistent")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestFindMountEntryForPath_EmptyInput(t *testing.T) {
	_, err := findMountEntryForPath(strings.NewReader(""), "/dev")
	require.Error(t, err)
}

func TestFindMountEntryForPath_ReportsMountOptions(t *testing.T) {
	entry, err := findMountEntryForPath(strings.NewReader(sampleProcMounts), "/mnt/data")
	require.NoError(t, err)
	require.Equal(t, "/dev/sdb1", entry.Source)
	require.False(t, entry.readOnly())
}

func TestFindDeviceMountPointFromReader(t *testing.T) {
	const multiDeviceMounts = `udev /dev devtmpfs rw,nosuid,relatime 0 0
/dev/sda2 / ext4 rw,relatime 0 0
/dev/sdb1 /data ext4 rw,relatime 0 0
/dev/nvme0n1p1 /mnt/fast xfs rw,relatime 0 0
/dev/nvme0n11 /mnt/other-namespace xfs rw,relatime 0 0
/dev/sdd /mnt/whole-disk btrfs rw,relatime 0 0
/dev/sde1 /mnt/readonly xfs ro,relatime 0 0
/dev/md1p1 /mnt/array xfs rw,relatime 0 0
`

	tests := []struct {
		name       string
		device     string
		wantMount  string
		wantErrMsg string
	}{
		{
			// The reported case: a whole disk is not mounted, its partition is.
			name:      "whole disk resolves through its partition",
			device:    "/dev/sdb",
			wantMount: "/data",
		},
		{
			name:      "whole disk mounted directly",
			device:    "/dev/sdd",
			wantMount: "/mnt/whole-disk",
		},
		{
			name:      "nvme namespace resolves through p-separated partition",
			device:    "/dev/nvme0n1",
			wantMount: "/mnt/fast",
		},
		{
			name:      "md array resolves through p-separated partition",
			device:    "/dev/md1",
			wantMount: "/mnt/array",
		},
		{
			name:      "read-only partition still resolves",
			device:    "/dev/sde",
			wantMount: "/mnt/readonly",
		},
		{
			name:       "unmounted disk is not resolved",
			device:     "/dev/sdc",
			wantErrMsg: "no mount point found",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mountPoint, err := findDeviceMountPointFromReader(strings.NewReader(multiDeviceMounts), test.device)
			if test.wantErrMsg != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.wantErrMsg)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.wantMount, mountPoint)
		})
	}
}

func TestFindDeviceMountPointFromReader_PrefersWritableMount(t *testing.T) {
	// The same device bind-mounted twice: the writable mount is the usable one.
	const duplicateMounts = `/dev/sdb1 /mnt/readonly xfs ro,relatime 0 0
/dev/sdb1 /mnt/writable xfs rw,relatime 0 0
`
	mountPoint, err := findDeviceMountPointFromReader(strings.NewReader(duplicateMounts), "/dev/sdb")
	require.NoError(t, err)
	require.Equal(t, "/mnt/writable", mountPoint)
}

func TestFindDeviceMountPointFromReader_PrefersDeviceOverPartition(t *testing.T) {
	const bothMounts = `/dev/sdb1 /mnt/partition xfs rw,relatime 0 0
/dev/sdb /mnt/device btrfs rw,relatime 0 0
`
	mountPoint, err := findDeviceMountPointFromReader(strings.NewReader(bothMounts), "/dev/sdb")
	require.NoError(t, err)
	require.Equal(t, "/mnt/device", mountPoint)
}

func TestIsPartitionOf(t *testing.T) {
	tests := []struct {
		source string
		device string
		want   bool
	}{
		{source: "/dev/sdb1", device: "/dev/sdb", want: true},
		{source: "/dev/sdb10", device: "/dev/sdb", want: true},
		{source: "/dev/nvme0n1p1", device: "/dev/nvme0n1", want: true},
		{source: "/dev/md1p2", device: "/dev/md1", want: true},
		// A sibling namespace is a different device, not a partition.
		{source: "/dev/nvme0n11", device: "/dev/nvme0n1", want: false},
		// A bare suffix on a digit-terminated device is not a partition.
		{source: "/dev/nvme0n1x1", device: "/dev/nvme0n1", want: false},
		{source: "/dev/sdbb1", device: "/dev/sdb", want: false},
		{source: "/dev/sdb", device: "/dev/sdb", want: false},
		{source: "/dev/sda1", device: "/dev/sdb", want: false},
		{source: "/dev/sdb1", device: "", want: false},
	}

	for _, test := range tests {
		t.Run(test.source+"_of_"+test.device, func(t *testing.T) {
			require.Equal(t, test.want, isPartitionOf(test.source, test.device))
		})
	}
}

func TestIsMountEntrySuitable(t *testing.T) {
	tests := []struct {
		name       string
		entry      mountEntry
		suitable   bool
		wantReason string
	}{
		{
			name:     "writable real filesystem",
			entry:    mountEntry{MountPoint: "/data", FsType: "ext4", Options: "rw,relatime"},
			suitable: true,
		},
		{
			name:       "devtmpfs is not a real filesystem",
			entry:      mountEntry{MountPoint: "/dev", FsType: "devtmpfs", Options: "rw,nosuid"},
			wantReason: "not mounted as a real filesystem",
		},
		{
			name:       "tmpfs is not a real filesystem",
			entry:      mountEntry{MountPoint: "/run", FsType: "tmpfs", Options: "rw,nosuid"},
			wantReason: "not mounted as a real filesystem",
		},
		{
			name:       "read-only mount cannot hold a test file",
			entry:      mountEntry{MountPoint: "/mnt/disk1", FsType: "xfs", Options: "ro,noatime"},
			wantReason: "mounted read-only",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			suitable, reason := isMountEntrySuitable(test.entry)
			require.Equal(t, test.suitable, suitable)
			if test.wantReason == "" {
				require.Empty(t, reason)
				return
			}
			require.Contains(t, reason, test.wantReason)
		})
	}
}

func TestMountEntryReadOnly(t *testing.T) {
	// "ro" must match an option, not a substring of one such as "relatime".
	require.False(t, mountEntry{Options: "rw,relatime,errors=remount-ro"}.readOnly())
	require.True(t, mountEntry{Options: "ro,relatime"}.readOnly())
	require.True(t, mountEntry{Options: "noatime,ro"}.readOnly())
	require.False(t, mountEntry{Options: ""}.readOnly())
}
