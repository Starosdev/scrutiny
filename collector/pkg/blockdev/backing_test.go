package blockdev

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// fakeTree builds a sysfs and /dev layout matching a host with two disks, a
// partition on each, an md array across both partitions, and an LVM logical
// volume on the array. Partitions and disks live under a devices tree the sysfs
// class directory symlinks into, exactly as the kernel lays them out, so the
// partition-to-disk step exercises the real symlink walk.
func fakeTree(t *testing.T) (sysBlock string, devRoot string) {
	t.Helper()
	root := t.TempDir()
	devices := filepath.Join(root, "devices")
	sysBlock = filepath.Join(root, "sys", "class", "block")
	devRoot = filepath.Join(root, "dev")

	require.NoError(t, os.MkdirAll(sysBlock, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(devRoot, "mapper"), 0o755))

	link := func(name string, target string) {
		require.NoError(t, os.Symlink(target, filepath.Join(sysBlock, name)))
	}

	for _, disk := range []string{"sda", "sdb", "sdc"} {
		partition := filepath.Join(devices, "pci", disk, disk+"1")
		require.NoError(t, os.MkdirAll(partition, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(partition, "partition"), []byte("1\n"), 0o644))
		link(disk, filepath.Join(devices, "pci", disk))
		link(disk+"1", partition)
		require.NoError(t, os.WriteFile(filepath.Join(devRoot, disk), nil, 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(devRoot, disk+"1"), nil, 0o644))
	}

	// An nvme namespace: a whole disk whose name already ends in a digit, with
	// a "p"-separated partition.
	nvme := filepath.Join(devices, "pci", "nvme0n1")
	require.NoError(t, os.MkdirAll(filepath.Join(nvme, "nvme0n1p2"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(nvme, "nvme0n1p2", "partition"), []byte("2\n"), 0o644))
	link("nvme0n1", nvme)
	link("nvme0n1p2", filepath.Join(nvme, "nvme0n1p2"))
	require.NoError(t, os.WriteFile(filepath.Join(devRoot, "nvme0n1p2"), nil, 0o644))

	stacked := func(name string, slaves ...string) {
		path := filepath.Join(devices, "virtual", "block", name)
		for _, slave := range slaves {
			require.NoError(t, os.MkdirAll(filepath.Join(path, "slaves", slave), 0o755))
		}
		link(name, path)
		require.NoError(t, os.WriteFile(filepath.Join(devRoot, name), nil, 0o644))
	}

	// A snap mount: a loop device with no slaves and no partition file, which
	// the walk would otherwise call a whole disk.
	loop := filepath.Join(devices, "virtual", "block", "loop3")
	require.NoError(t, os.MkdirAll(loop, 0o755))
	link("loop3", loop)
	require.NoError(t, os.WriteFile(filepath.Join(devRoot, "loop3"), nil, 0o644))

	stacked("md0", "sda1", "sdb1")
	stacked("dm-0", "md0")

	// md1 is an array across sda1 and sdb1 with sdc1 attached as a hot spare.
	// The kernel lists all three under slaves and records each member's role
	// under md/dev-<name>/state.
	stacked("md1", "sda1", "sdb1", "sdc1")
	md1 := filepath.Join(devices, "virtual", "block", "md1", "md")
	for member, state := range map[string]string{"sda1": "in_sync", "sdb1": "writemostly,in_sync", "sdc1": "spare"} {
		require.NoError(t, os.MkdirAll(filepath.Join(md1, "dev-"+member), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(md1, "dev-"+member, "state"), []byte(state+"\n"), 0o644))
	}
	require.NoError(t, os.Symlink(filepath.Join(devRoot, "dm-0"), filepath.Join(devRoot, "mapper", "vg-root")))

	return sysBlock, devRoot
}

func TestResolve(t *testing.T) {
	sysBlock, devRoot := fakeTree(t)

	tests := []struct {
		name   string
		source string
		want   []string
	}{
		{name: "whole disk", source: "sda", want: []string{"sda"}},
		{name: "partition maps to its disk", source: "sda1", want: []string{"sda"}},
		// Regression: trimming trailing digits would turn nvme0n1p2 into
		// "nvme0n1p" or "nvme0n", never the real disk.
		{name: "nvme partition maps to its namespace", source: "nvme0n1p2", want: []string{"nvme0n1"}},
		{name: "md array spans both disks", source: "md0", want: []string{"sda", "sdb"}},
		{name: "logical volume descends through the array", source: "dm-0", want: []string{"sda", "sdb"}},
		{name: "mapper symlink resolves to its target", source: "mapper/vg-root", want: []string{"sda", "sdb"}},
		// A hot spare holds none of the array's data, so the array's
		// filesystem must not be attributed to it.
		{name: "array skips its hot spare", source: "md1", want: []string{"sda", "sdb"}},
		// Scrutiny registers drives, so a loop device can never match one and
		// a link row naming it would be noise, one per snap package.
		{name: "loop device is not a drive", source: "loop3", want: nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, resolve(filepath.Join(devRoot, test.source), sysBlock, devRoot))
		})
	}
}

func TestResolve_NonBlockSourcesAreUnknown(t *testing.T) {
	sysBlock, devRoot := fakeTree(t)

	// A zfs dataset, an NFS export and a tmpfs have no block device behind
	// them. Returning nothing marks the filesystem unattributed; returning a
	// guess would credit free space to a drive that does not hold it.
	for _, source := range []string{"tank/data", "nas:/export", "tmpfs", ""} {
		require.Nil(t, resolve(source, sysBlock, devRoot))
	}
}

func TestResolve_UnknownDeviceIsNotGuessed(t *testing.T) {
	sysBlock, devRoot := fakeTree(t)

	// The node exists but sysfs has no entry for it, which is what a container
	// seeing a bind-mounted /dev without the host's sysfs looks like.
	require.NoError(t, os.WriteFile(filepath.Join(devRoot, "sdz"), nil, 0o644))
	require.Nil(t, resolve(filepath.Join(devRoot, "sdz"), sysBlock, devRoot))
}

func TestResolve_MissingSysfsReturnsNothing(t *testing.T) {
	_, devRoot := fakeTree(t)
	require.Nil(t, resolve(filepath.Join(devRoot, "sda1"), filepath.Join(devRoot, "no-sysfs-here"), devRoot))
}

func TestWalk_StopsOnCycles(t *testing.T) {
	root := t.TempDir()
	sysBlock := filepath.Join(root, "sys", "class", "block")
	// Two stacked devices listing each other as slaves. A real kernel cannot
	// produce this; the visited set is what keeps a corrupt tree from looping.
	require.NoError(t, os.MkdirAll(filepath.Join(sysBlock, "dm-0", "slaves", "dm-1"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(sysBlock, "dm-1", "slaves", "dm-0"), 0o755))

	disks := map[string]struct{}{}
	walk("dm-0", sysBlock, disks, map[string]struct{}{}, 0)
	require.Empty(t, disks)
}

func TestIsSpareArrayMember(t *testing.T) {
	root := t.TempDir()
	write := func(member string, state string) {
		require.NoError(t, os.MkdirAll(filepath.Join(root, "md", "dev-"+member), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(root, "md", "dev-"+member, "state"), []byte(state), 0o644))
	}

	write("sda1", "in_sync\n")
	write("sdb1", "writemostly,in_sync\n")
	write("sdc1", "spare\n")
	write("sdd1", "spare,write_error\n")
	// A failed member still holds the array's data; only an idle spare does not.
	write("sde1", "faulty\n")

	require.False(t, isSpareArrayMember(root, "sda1"))
	require.False(t, isSpareArrayMember(root, "sdb1"))
	require.True(t, isSpareArrayMember(root, "sdc1"))
	require.True(t, isSpareArrayMember(root, "sdd1"))
	require.False(t, isSpareArrayMember(root, "sde1"))
	// A device that is not an array, and a member with no readable state, are
	// both kept: dropping a data-carrying disk hides its filesystems entirely.
	require.False(t, isSpareArrayMember(root, "absent"))
	require.False(t, isSpareArrayMember(filepath.Join(root, "not-an-array"), "sda1"))
}

func TestIsSyntheticDisk(t *testing.T) {
	for _, name := range []string{"loop0", "loop12", "zram0", "ram15"} {
		require.True(t, isSyntheticDisk(name), name)
	}
	for _, name := range []string{"sda", "nvme0n1", "md0", "dm-0", "mmcblk0", "sdram"} {
		require.False(t, isSyntheticDisk(name), name)
	}
}

// TestResolve_WorksWithoutDeviceNodes is the container case: sysfs carries the
// whole holder tree while /dev holds only the devices the deployment passed
// through. Attribution has to work from sysfs alone, or the feature does
// nothing in the one deployment shape that ships (#1054).
func TestResolve_WorksWithoutDeviceNodes(t *testing.T) {
	sysBlock, devRoot := fakeTree(t)

	// Keep only the whole-disk nodes, the way a compose device list does, and
	// remove the partition and array nodes a mount source actually names.
	for _, name := range []string{"sda1", "sdb1", "sdc1", "nvme0n1p2", "md0", "md1", "dm-0", "loop3"} {
		require.NoError(t, os.Remove(filepath.Join(devRoot, name)))
	}
	require.NoError(t, os.Remove(filepath.Join(devRoot, "mapper", "vg-root")))

	tests := []struct {
		name   string
		source string
		want   []string
	}{
		{name: "partition with no node", source: "sda1", want: []string{"sda"}},
		{name: "nvme partition with no node", source: "nvme0n1p2", want: []string{"nvme0n1"}},
		{name: "array with no node", source: "md0", want: []string{"sda", "sdb"}},
		{name: "array skips its spare with no node", source: "md1", want: []string{"sda", "sdb"}},
		{name: "logical volume with no node", source: "dm-0", want: []string{"sda", "sdb"}},
		{name: "loop device is still not a drive", source: "loop3", want: nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, resolve(filepath.Join(devRoot, test.source), sysBlock, devRoot))
		})
	}

	// A nested source cannot be read without its symlink: only the link says
	// what it points at, and "mapper" is not a device name.
	require.Nil(t, resolve(filepath.Join(devRoot, "mapper", "vg-root"), sysBlock, devRoot))

	// The fallback is a claim about a name, not a licence to invent a device.
	// sysfs still decides.
	require.Nil(t, resolve(filepath.Join(devRoot, "sdz9"), sysBlock, devRoot))
}
