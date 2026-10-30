// Package blockdev resolves a mount source to the physical disks behind it.
//
// A filesystem's source device is rarely the disk a user cares about. /dev/sda1
// is a partition, /dev/mapper/vg-root is a logical volume that may span several
// disks, /dev/md0 is an array, and a LUKS volume sits on top of any of those.
// Scrutiny registers whole disks, so attributing a filesystem to a drive means
// walking the kernel's holder/slave tree down to the disks at the bottom.
//
// The walk reads sysfs, so it only produces results on Linux. Everywhere else
// the sysfs root is absent and resolution returns nothing, which callers treat
// as "unknown" rather than as "no disks".
package blockdev

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SysBlock is the kernel's block device directory. Every registered block
// device appears here as a symlink into /sys/devices, including partitions,
// device-mapper targets and md arrays.
const SysBlock = "/sys/class/block"

// DevRoot is the device node directory.
const DevRoot = "/dev"

// maxDepth bounds the holder-tree walk. A real stack is at most a few levels
// deep (partition -> md -> LUKS -> LVM); the bound is what keeps a malformed or
// hostile sysfs tree from turning the walk into an unbounded one. The visited
// set already prevents cycles.
const maxDepth = 16

// Resolve returns the kernel names of the whole disks backing a mount source,
// sorted and deduplicated. It returns nil when the source is not a block device
// node (a network mount, a zfs dataset, tmpfs) or when sysfs cannot be read.
func Resolve(source string) []string {
	return resolve(source, SysBlock, DevRoot)
}

func resolve(source string, sysBlock string, devRoot string) []string {
	name := kernelName(source, devRoot)
	if name == "" {
		return nil
	}

	disks := make(map[string]struct{})
	visited := map[string]struct{}{}
	walk(name, sysBlock, disks, visited, 0)

	if len(disks) == 0 {
		return nil
	}
	names := make([]string, 0, len(disks))
	for disk := range disks {
		names = append(names, disk)
	}
	sort.Strings(names)
	return names
}

// kernelName maps a mount source to the name sysfs knows the device by.
//
// The source is frequently a symlink: /dev/mapper/vg-root points at /dev/dm-0,
// and a fstab using /dev/disk/by-uuid/... points at a partition node. sysfs only
// knows the target's name, so the link is followed when the node is there. A
// source that is not under the device root is not a block device and has no
// kernel name.
//
// The node is often not there. In the omnibus container /dev holds only the
// devices the deployment passes through, which are whole disks, while a mount
// source is usually a partition (/dev/md1p1, /dev/sdm1) or an array the
// container was never given. Requiring the node made attribution impossible in
// the one deployment shape that matters, because sysfs carries the whole holder
// tree regardless (#1054). So a flat source falls back to its own last segment,
// which is the kernel name by construction: /dev/md1p1 is the node for md1p1
// whether or not that node was created here.
//
// A nested source still needs the node, because only the symlink says what it
// points at, and guessing "mapper" or a by-uuid string as a device name would be
// worse than reporting nothing. Either way the name is a claim, not a fact:
// walk() looks it up under sysfs and drops anything the kernel does not know.
func kernelName(source string, devRoot string) string {
	if source == "" || !strings.HasPrefix(source, devRoot+"/") {
		return ""
	}

	resolved, err := filepath.EvalSymlinks(source)
	if err != nil {
		// No node to follow. Only a source naming one segment under the device
		// root can be read without it.
		name := strings.TrimPrefix(source, devRoot+"/")
		if name == "" || strings.Contains(name, "/") {
			return ""
		}
		return name
	}

	// The device root is resolved too, so a symlinked path to it does not make
	// every node under it look like it sits somewhere else. A root that cannot
	// be resolved is compared as written rather than abandoning the lookup.
	resolvedRoot, rootErr := filepath.EvalSymlinks(devRoot)
	if rootErr != nil {
		resolvedRoot = devRoot
	}
	// A device node sits directly under the device root and is named by its
	// base. A node resolving to anywhere else is not a block device node, so it
	// has no kernel name to look up.
	if filepath.Dir(resolved) != resolvedRoot {
		return ""
	}
	return filepath.Base(resolved)
}

// walk descends from one block device to the whole disks under it, adding each
// disk it reaches to disks.
//
// Three cases cover every stack. A device with a "slaves" directory is stacked
// on other devices (device-mapper, md) and the walk continues into each of
// them. A device with a "partition" file is a partition and the walk continues
// into the disk that owns it. Anything else that sysfs knows about is itself a
// whole disk.
func walk(name string, sysBlock string, disks map[string]struct{}, visited map[string]struct{}, depth int) {
	if name == "" || depth > maxDepth {
		return
	}
	if _, seen := visited[name]; seen {
		return
	}
	visited[name] = struct{}{}

	devicePath := filepath.Join(sysBlock, name)
	if _, err := os.Stat(devicePath); err != nil {
		// Not a device sysfs knows about. Reporting nothing is correct: a
		// guessed disk name would be attributed to the wrong drive.
		return
	}

	if slaves := readSlaves(devicePath); len(slaves) > 0 {
		for _, slave := range slaves {
			if isSpareArrayMember(devicePath, slave) {
				continue
			}
			walk(slave, sysBlock, disks, visited, depth+1)
		}
		return
	}

	if parent := partitionParent(devicePath); parent != "" {
		walk(parent, sysBlock, disks, visited, depth+1)
		return
	}

	if isSyntheticDisk(name) {
		return
	}
	disks[name] = struct{}{}
}

// isSyntheticDisk reports whether a whole-disk name belongs to a kernel-
// synthesised device rather than to a drive.
//
// loop devices back every snap package, and a snap-based host carries dozens of
// them; zram and ram devices are memory. None is a drive Scrutiny can register,
// so a link row naming one could never match a device and is pure noise
// proportional to the host's snap count.
func isSyntheticDisk(name string) bool {
	for _, prefix := range []string{"loop", "zram", "ram"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// isSpareArrayMember reports whether a slave is an md array member holding none
// of the array's data.
//
// "slaves" lists every member of an array, and a hot spare is a member: it is
// attached and idle until a disk fails. Attributing the array's filesystem to a
// spare would tell a user that a disk holding nothing has the array's used and
// free space on it. md records each member's role in
// md/dev-<name>/state, so the spare is skipped and the data-carrying
// members are not.
//
// A device with no md directory, and a member whose state cannot be read, are
// both kept. The cost of wrongly keeping a member is one over-attributed drive;
// the cost of wrongly dropping one is a drive whose filesystems vanish from the
// UI, so the unreadable case resolves toward keeping.
func isSpareArrayMember(devicePath string, slave string) bool {
	state, err := os.ReadFile(filepath.Join(devicePath, "md", "dev-"+slave, "state"))
	if err != nil {
		return false
	}
	for _, flag := range strings.Split(strings.TrimSpace(string(state)), ",") {
		if flag == "spare" {
			return true
		}
	}
	return false
}

// readSlaves lists the devices a stacked device is built from.
func readSlaves(devicePath string) []string {
	entries, err := os.ReadDir(filepath.Join(devicePath, "slaves"))
	if err != nil {
		return nil
	}
	slaves := make([]string, 0, len(entries))
	for _, entry := range entries {
		slaves = append(slaves, entry.Name())
	}
	return slaves
}

// partitionParent returns the disk that owns a partition, or "" when the device
// is not a partition.
//
// The owning disk is the parent directory of the partition in /sys/devices, so
// the sysfs symlink is followed and the parent's name taken from there. Deriving
// it by trimming digits off the partition name instead would be wrong for every
// device whose name already ends in a digit (nvme0n1p1, md1p1, mmcblk0p1).
func partitionParent(devicePath string) string {
	if _, err := os.Stat(filepath.Join(devicePath, "partition")); err != nil {
		return ""
	}
	resolved, err := filepath.EvalSymlinks(devicePath)
	if err != nil {
		return ""
	}
	parent := filepath.Base(filepath.Dir(resolved))
	if parent == "." || parent == string(filepath.Separator) {
		return ""
	}
	return parent
}
