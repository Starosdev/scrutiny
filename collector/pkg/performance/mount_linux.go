package performance

import (
	"fmt"
	"os"
	"syscall"

	"github.com/analogj/scrutiny/collector/pkg/mounts"
)

// spaceHeadroomBytes is kept free above the benchmark test file, so filling a
// filesystem to zero available bytes is never a possible outcome of a benchmark.
const spaceHeadroomBytes = 64 * 1024 * 1024

// isMountPointSuitable checks whether a mount point is appropriate for benchmarking.
// It rejects a path that is not a directory, a pseudo-filesystem or container
// runtime mount, a read-only mount, and a filesystem without room for the test
// file. A mount table that cannot be read is also a rejection: the alternative is
// writing to a filesystem whose properties are unknown.
func isMountPointSuitable(mountPoint string, requiredBytes uint64) (bool, string) {
	info, err := os.Stat(mountPoint)
	if err != nil {
		return false, fmt.Sprintf("could not inspect %s: %v", mountPoint, err)
	}
	if !info.IsDir() {
		return false, fmt.Sprintf("%s is not a directory (benchmark test file cannot be created inside it)", mountPoint)
	}

	entries, err := mounts.Load()
	if err != nil {
		return false, fmt.Sprintf("could not read %s: %v", mounts.ProcMounts, err)
	}
	entry, found := mounts.FindForPath(entries, mountPoint)
	if !found {
		return false, fmt.Sprintf("no mount point in %s owns %s", mounts.ProcMounts, mountPoint)
	}
	if suitable, reason := isMountEntrySuitable(entry); !suitable {
		return false, reason
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs(mountPoint, &stat); err != nil {
		return false, fmt.Sprintf("could not check available space: %v", err)
	}
	bsize := stat.Bsize
	if bsize <= 0 {
		return false, fmt.Sprintf("unexpected block size %d for mount point %s", bsize, mountPoint)
	}
	availableBytes := stat.Bavail * uint64(bsize)
	if availableBytes < requiredBytes+spaceHeadroomBytes {
		return false, fmt.Sprintf("insufficient space: %d bytes available, %d bytes required (test file plus %d bytes headroom)",
			availableBytes, requiredBytes+spaceHeadroomBytes, uint64(spaceHeadroomBytes))
	}

	return true, ""
}

// isMountEntrySuitable rejects mounts that cannot hold a benchmark test file:
// pseudo-filesystems and container runtime mounts, which are not host storage,
// and read-only mounts, where the write tests cannot create their test file.
func isMountEntrySuitable(entry mounts.Entry) (bool, string) {
	if mounts.ExcludedFSType(entry.FSType) {
		return false, fmt.Sprintf("filesystem type is %s (device not mounted as a real filesystem)", entry.FSType)
	}
	if mounts.ExcludedMountPoint(entry.MountPoint) {
		return false, fmt.Sprintf("%s is a container runtime mount, not host storage", entry.MountPoint)
	}
	if entry.ReadOnly() {
		return false, fmt.Sprintf("%s is mounted read-only (benchmark test file cannot be created)", entry.MountPoint)
	}
	return true, ""
}

// deviceMountCandidates returns the mount points backed by devicePath, best
// first, read from /proc/mounts. A whole-disk device such as /dev/sdb is rarely
// mounted itself -- its partitions are -- so partitions of the device are
// matched too. Reading the mount table also works where the device node is
// absent from a container's /dev, which df requires.
//
// Every candidate is returned, not just the best one, because suitability is
// decided later: a device can back both a filesystem that is too full and one
// the benchmark can use.
func deviceMountCandidates(devicePath string) ([]string, error) {
	entries, err := mounts.Load()
	if err != nil {
		return nil, err
	}
	return rankDeviceMountCandidates(entries, devicePath), nil
}

// rankDeviceMountCandidates orders the mounts backed by devicePath, device
// before partition, preserving mount table order within each group.
//
// Unusable mounts are dropped rather than ranked. A read-only mount cannot hold
// the test file at all, and excluded filesystems are not host storage: Docker
// reports its single-file binds (/etc/resolv.conf and friends) with the backing
// block device as their source, so a device lookup matches a file otherwise.
func rankDeviceMountCandidates(entries []mounts.Entry, devicePath string) []string {
	exactMounts := make([]string, 0)
	partitionMounts := make([]string, 0)

	for _, entry := range entries {
		exact := entry.Source == devicePath
		if !exact && !isPartitionOf(entry.Source, devicePath) {
			continue
		}
		if suitable, _ := isMountEntrySuitable(entry); !suitable {
			continue
		}
		if exact {
			exactMounts = append(exactMounts, entry.MountPoint)
			continue
		}
		partitionMounts = append(partitionMounts, entry.MountPoint)
	}

	return append(exactMounts, partitionMounts...)
}

// isPartitionOf reports whether source names a partition of devicePath.
// The kernel separates the partition number with "p" when the device name already
// ends in a digit (/dev/nvme0n1 -> /dev/nvme0n1p1, /dev/md1 -> /dev/md1p1) and
// appends it directly otherwise (/dev/sdb -> /dev/sdb1). Requiring that separator
// keeps namespace siblings such as /dev/nvme0n11 from matching /dev/nvme0n1.
func isPartitionOf(source string, devicePath string) bool {
	if devicePath == "" || len(source) <= len(devicePath) || source[:len(devicePath)] != devicePath {
		return false
	}
	suffix := source[len(devicePath):]
	if endsWithDigit(devicePath) {
		if suffix[0] != 'p' {
			return false
		}
		suffix = suffix[1:]
	}
	return isAllDigits(suffix)
}

func endsWithDigit(s string) bool {
	return s != "" && s[len(s)-1] >= '0' && s[len(s)-1] <= '9'
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
