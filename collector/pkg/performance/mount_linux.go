package performance

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
)

const procMounts = "/proc/mounts"

// mountEntry is one parsed line from /proc/mounts.
type mountEntry struct {
	Source     string
	MountPoint string
	FsType     string
	Options    string
}

// readOnly reports whether the filesystem is mounted read-only.
func (m mountEntry) readOnly() bool {
	for _, opt := range strings.Split(m.Options, ",") {
		if opt == "ro" {
			return true
		}
	}
	return false
}

// isMountPointSuitable checks whether a mount point is appropriate for benchmarking.
// Returns false if the mount point is a tmpfs/devtmpfs (indicating the device is
// not actually mounted as a real filesystem), if it is mounted read-only, or if
// available space is insufficient.
func isMountPointSuitable(mountPoint string, requiredBytes uint64) (bool, string) {
	if entry, err := lookupMountEntryForPath(mountPoint); err == nil {
		if suitable, reason := isMountEntrySuitable(entry); !suitable {
			return false, reason
		}
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
	if availableBytes < requiredBytes {
		return false, fmt.Sprintf("insufficient space: %d bytes available, %d bytes required", availableBytes, requiredBytes)
	}

	return true, ""
}

// isMountEntrySuitable rejects mounts that cannot hold a benchmark test file:
// pseudo-filesystems, where the device is not mounted as a real filesystem, and
// read-only mounts, where the write tests cannot create their test file.
func isMountEntrySuitable(entry mountEntry) (bool, string) {
	if entry.FsType == "tmpfs" || entry.FsType == "devtmpfs" {
		return false, fmt.Sprintf("filesystem type is %s (device not mounted as a real filesystem)", entry.FsType)
	}
	if entry.readOnly() {
		return false, fmt.Sprintf("%s is mounted read-only (benchmark test file cannot be created)", entry.MountPoint)
	}
	return true, ""
}

// lookupMountEntryForPath resolves the mount entry owning path from /proc/mounts.
func lookupMountEntryForPath(path string) (mountEntry, error) {
	file, err := os.Open(procMounts)
	if err != nil {
		return mountEntry{}, err
	}
	defer file.Close()
	return findMountEntryForPath(file, path)
}

// findMountEntryForPath returns the entry for the longest mount point containing path.
func findMountEntryForPath(reader io.Reader, path string) (mountEntry, error) {
	best := mountEntry{}
	for _, entry := range parseMountEntries(reader) {
		if path != entry.MountPoint && !strings.HasPrefix(path, entry.MountPoint+"/") {
			continue
		}
		if len(entry.MountPoint) > len(best.MountPoint) {
			best = entry
		}
	}

	if best.MountPoint == "" {
		return mountEntry{}, fmt.Errorf("mount point %s not found in %s", path, procMounts)
	}
	return best, nil
}

// findDeviceMountPoint resolves the mount point backed by devicePath from
// /proc/mounts. A whole-disk device such as /dev/sdb is rarely mounted itself --
// its partitions are -- so partitions of the device are matched too. Reading the
// mount table also works where the device node is absent from a container's /dev.
func findDeviceMountPoint(devicePath string) (string, error) {
	file, err := os.Open(procMounts)
	if err != nil {
		return "", err
	}
	defer file.Close()
	return findDeviceMountPointFromReader(file, devicePath)
}

// findDeviceMountPointFromReader returns the mount point for devicePath or one of
// its partitions. The device itself outranks a partition, and a writable mount
// outranks a read-only one, so a device mounted more than once resolves to the
// mount the benchmark can actually use.
func findDeviceMountPointFromReader(reader io.Reader, devicePath string) (string, error) {
	best := mountEntry{}
	bestRank := -1

	for _, entry := range parseMountEntries(reader) {
		var rank int
		switch {
		case entry.Source == devicePath:
			rank = 2
		case isPartitionOf(entry.Source, devicePath):
			rank = 1
		default:
			continue
		}
		if entry.readOnly() {
			rank--
		}
		if rank > bestRank {
			best = entry
			bestRank = rank
		}
	}

	if bestRank < 0 {
		return "", fmt.Errorf("no mount point found for %s or its partitions in %s", devicePath, procMounts)
	}
	return best.MountPoint, nil
}

// isPartitionOf reports whether source names a partition of devicePath.
// The kernel separates the partition number with "p" when the device name already
// ends in a digit (/dev/nvme0n1 -> /dev/nvme0n1p1, /dev/md1 -> /dev/md1p1) and
// appends it directly otherwise (/dev/sdb -> /dev/sdb1). Requiring that separator
// keeps namespace siblings such as /dev/nvme0n11 from matching /dev/nvme0n1.
func isPartitionOf(source string, devicePath string) bool {
	if devicePath == "" || !strings.HasPrefix(source, devicePath) {
		return false
	}
	suffix := source[len(devicePath):]
	if endsWithDigit(devicePath) {
		if !strings.HasPrefix(suffix, "p") {
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

// parseMountEntries reads mount entries from a reader such as /proc/mounts.
func parseMountEntries(reader io.Reader) []mountEntry {
	var entries []mountEntry
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		entry := mountEntry{
			Source:     fields[0],
			MountPoint: fields[1],
			FsType:     fields[2],
		}
		if len(fields) > 3 {
			entry.Options = fields[3]
		}
		entries = append(entries, entry)
	}
	return entries
}
