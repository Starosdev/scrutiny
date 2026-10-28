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

	for _, disk := range []string{"sda", "sdb"} {
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

	stacked("md0", "sda1", "sdb1")
	stacked("dm-0", "md0")
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
