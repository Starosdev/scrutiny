package performance

import (
	"os"
	"strings"
	"testing"

	"github.com/analogj/scrutiny/collector/pkg/mounts"
	"github.com/stretchr/testify/require"
)

func parseFixture(t *testing.T, table string) []mounts.Entry {
	t.Helper()
	entries, err := mounts.Parse(strings.NewReader(table))
	require.NoError(t, err)
	return entries
}

func TestRankDeviceMountCandidates(t *testing.T) {
	// A container's mount table. /dev/sda2 backs the container runtime's
	// single-file binds, /dev/sdb1 is a bind-mounted data filesystem, /dev/sdd is
	// mounted whole, /dev/sde1 is read-only, /dev/md1p1 is an array partition.
	const table = `overlay / overlay rw,relatime 0 0
tmpfs /dev tmpfs rw,nosuid,mode=755 0 0
/dev/sda2 /etc/resolv.conf ext4 rw,relatime 0 0
/dev/sda2 /etc/hostname ext4 rw,relatime 0 0
/dev/sda2 /opt/scrutiny/config ext4 rw,relatime 0 0
/dev/sdb1 /data ext4 rw,relatime 0 0
/dev/nvme0n1p1 /mnt/fast xfs rw,relatime 0 0
/dev/nvme0n11 /mnt/other-namespace xfs rw,relatime 0 0
/dev/sdd /mnt/whole-disk btrfs rw,relatime 0 0
/dev/sde1 /mnt/readonly xfs ro,relatime 0 0
/dev/md1p1 /mnt/array xfs rw,relatime 0 0
`

	tests := []struct {
		name   string
		device string
		want   []string
	}{
		{
			// The reported case: the whole disk is not mounted, its partition is.
			name:   "whole disk resolves through its partition",
			device: "/dev/sdb",
			want:   []string{"/data"},
		},
		{
			name:   "whole disk mounted directly",
			device: "/dev/sdd",
			want:   []string{"/mnt/whole-disk"},
		},
		{
			name:   "nvme namespace resolves through p-separated partition",
			device: "/dev/nvme0n1",
			want:   []string{"/mnt/fast"},
		},
		{
			name:   "md array resolves through p-separated partition",
			device: "/dev/md1",
			want:   []string{"/mnt/array"},
		},
		{
			// Candidates are not filtered here; the caller rejects them with a
			// reason. isMountEntrySuitable is what keeps them from being used.
			name:   "container runtime file binds are returned for the caller to reject",
			device: "/dev/sda",
			want:   []string{"/etc/resolv.conf", "/etc/hostname", "/opt/scrutiny/config"},
		},
		{
			name:   "read-only mount is returned for the caller to reject",
			device: "/dev/sde",
			want:   []string{"/mnt/readonly"},
		},
		{
			name:   "unmounted disk has no candidates",
			device: "/dev/sdc",
			want:   []string{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, rankDeviceMountCandidates(parseFixture(t, table), test.device))
		})
	}
}

func TestRankDeviceMountCandidates_ReadOnlyDeviceDoesNotHideWritablePartition(t *testing.T) {
	// A read-only mount cannot hold the test file. It may be offered first,
	// because the device outranks its partition, but the writable partition
	// must still be in the list for the caller to fall through to.
	const table = `/dev/sdb /mnt/ro xfs ro,relatime 0 0
/dev/sdb1 /mnt/rw xfs rw,relatime 0 0
`
	candidates := rankDeviceMountCandidates(parseFixture(t, table), "/dev/sdb")
	require.Equal(t, []string{"/mnt/ro", "/mnt/rw"}, candidates)

	// The read-only one is rejected with a reason naming the cause, so the log
	// says "mounted read-only" rather than "no mount point found".
	entries := parseFixture(t, table)
	readOnly, found := mounts.FindForPath(entries, "/mnt/ro")
	require.True(t, found)
	suitable, reason := isMountEntrySuitable(readOnly)
	require.False(t, suitable)
	require.Contains(t, reason, "mounted read-only")
}

func TestRankDeviceMountCandidates_DeviceBeforePartition(t *testing.T) {
	const table = `/dev/sdb1 /mnt/partition xfs rw,relatime 0 0
/dev/sdb /mnt/device btrfs rw,relatime 0 0
`
	require.Equal(t, []string{"/mnt/device", "/mnt/partition"}, rankDeviceMountCandidates(parseFixture(t, table), "/dev/sdb"))
}

func TestRankDeviceMountCandidates_ReturnsEveryCandidateInOrder(t *testing.T) {
	// Both partitions are plausible targets. The caller checks free space and
	// falls through to the next one, so neither may be dropped here: /boot/efi
	// is far too small for a test file, and / is usable.
	const table = `/dev/sda1 /boot/efi vfat rw,relatime 0 0
/dev/sda2 / ext4 rw,relatime 0 0
`
	require.Equal(t, []string{"/boot/efi", "/"}, rankDeviceMountCandidates(parseFixture(t, table), "/dev/sda"))
}

func TestRankDeviceMountCandidates_MultiDeviceBtrfsResolvesToPoolMount(t *testing.T) {
	// A multi-device btrfs pool is listed with whichever member was passed at
	// mount time, so one member resolves to the pool and the others do not.
	const table = `/dev/sdb1 /mnt/pool btrfs rw,relatime,subvol=/ 0 0
`
	entries := parseFixture(t, table)
	require.Equal(t, []string{"/mnt/pool"}, rankDeviceMountCandidates(entries, "/dev/sdb"))
	require.Empty(t, rankDeviceMountCandidates(entries, "/dev/sdc"))
}

func TestRankDeviceMountCandidates_DecodesEscapedMountPoint(t *testing.T) {
	const table = `/dev/sdb1 /mnt/my\040disk xfs rw,relatime 0 0
`
	require.Equal(t, []string{"/mnt/my disk"}, rankDeviceMountCandidates(parseFixture(t, table), "/dev/sdb"))
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
		{source: "", device: "/dev/sdb", want: false},
		// LVM and ZFS sources name no device, which is why they need explicit
		// mount_points configuration.
		{source: "/dev/mapper/vg0-data", device: "/dev/sdb", want: false},
		{source: "tank/data", device: "/dev/sdb", want: false},
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
		entry      mounts.Entry
		suitable   bool
		wantReason string
	}{
		{
			name:     "writable real filesystem",
			entry:    mounts.Entry{MountPoint: "/data", FSType: "ext4", Options: "rw,relatime"},
			suitable: true,
		},
		{
			name:       "devtmpfs is not a real filesystem",
			entry:      mounts.Entry{MountPoint: "/dev", FSType: "devtmpfs", Options: "rw,nosuid"},
			wantReason: "not mounted as a real filesystem",
		},
		{
			name:       "tmpfs is not a real filesystem",
			entry:      mounts.Entry{MountPoint: "/run", FSType: "tmpfs", Options: "rw,nosuid"},
			wantReason: "not mounted as a real filesystem",
		},
		{
			name:       "container overlay is not host storage",
			entry:      mounts.Entry{MountPoint: "/", FSType: "overlay", Options: "rw,relatime"},
			wantReason: "not mounted as a real filesystem",
		},
		{
			name:       "container runtime file bind is not host storage",
			entry:      mounts.Entry{MountPoint: "/etc/resolv.conf", FSType: "ext4", Options: "rw,relatime"},
			wantReason: "container runtime mount",
		},
		{
			name:       "scrutiny config volume is not a benchmark target",
			entry:      mounts.Entry{MountPoint: "/opt/scrutiny/config", FSType: "ext4", Options: "rw,relatime"},
			wantReason: "container runtime mount",
		},
		{
			// Only the capacity collector skips zfs, because the dedicated zfs
			// collector reports those pools. A dataset is a real filesystem and
			// a benchmark can write a test file to it.
			name:     "zfs dataset is a benchmark target",
			entry:    mounts.Entry{MountPoint: "/tank/data", FSType: "zfs", Options: "rw,noatime"},
			suitable: true,
		},
		{
			name:       "read-only mount cannot hold a test file",
			entry:      mounts.Entry{MountPoint: "/mnt/disk1", FSType: "xfs", Options: "ro,noatime"},
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

func TestIsMountPointSuitable_RejectsNonDirectory(t *testing.T) {
	// Defence in depth behind the exclusion list: a file can never hold the
	// test file, whatever the mount table says.
	file := t.TempDir() + "/not-a-directory"
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))

	suitable, reason := isMountPointSuitable(file, 1)
	require.False(t, suitable)
	require.Contains(t, reason, "not a directory")
}

func TestIsMountPointSuitable_RejectsMissingPath(t *testing.T) {
	suitable, reason := isMountPointSuitable(t.TempDir()+"/absent", 1)
	require.False(t, suitable)
	require.Contains(t, reason, "could not inspect")
}

// TestSpaceRejection covers the headroom rule directly. It takes no filesystem,
// so unlike the integration test below it runs on every runner: the mount entry
// checks reject a tmpfs before isMountPointSuitable reaches the space check, and
// CI runners put the temporary directory on tmpfs.
func TestSpaceRejection(t *testing.T) {
	const headroom = uint64(spaceHeadroomBytes)

	// Exactly enough, and one byte short of enough.
	require.Empty(t, spaceRejection(headroom+10, 10))
	require.NotEmpty(t, spaceRejection(headroom+9, 10))

	// An empty filesystem cannot hold even a zero-byte test file, because the
	// headroom is required on top of it.
	require.NotEmpty(t, spaceRejection(0, 0))
	require.Empty(t, spaceRejection(headroom, 0))

	// The rejection has to name both the shortfall and the headroom, because
	// that is what tells an operator the free space is not the whole budget.
	reason := spaceRejection(1024, 1<<60)
	require.Contains(t, reason, "insufficient space")
	require.Contains(t, reason, "headroom")
	require.Contains(t, reason, "1024 bytes available")
}

func TestIsMountPointSuitable_RequiresHeadroomAboveTestFile(t *testing.T) {
	dir := t.TempDir()

	// Confirm isMountPointSuitable actually consults the space rule. Some CI
	// runners put the temporary directory on a tmpfs, which the mount entry
	// checks reject first, and that is an environment difference rather than a
	// failure of the rule; TestSpaceRejection covers the rule itself. An
	// "insufficient space" rejection is not a reason to skip, because reaching
	// it means the space check ran.
	if suitable, reason := isMountPointSuitable(dir, 1); !suitable && !strings.Contains(reason, "insufficient space") {
		t.Skipf("temporary directory %s is not a benchmark target: %s", dir, reason)
	}

	// No filesystem has this much free, so the rejection must name the
	// headroom the check adds on top of the test file size.
	suitable, reason := isMountPointSuitable(dir, 1<<60)
	require.False(t, suitable)
	require.Contains(t, reason, "insufficient space")
	require.Contains(t, reason, "headroom")
}
