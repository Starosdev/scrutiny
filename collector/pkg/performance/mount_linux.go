package performance

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
)

// isMountPointSuitable checks whether a mount point is appropriate for benchmarking.
// Returns false if the mount point is a tmpfs/devtmpfs (indicating the device is
// not actually mounted as a real filesystem) or if available space is insufficient.
func isMountPointSuitable(mountPoint string, requiredBytes uint64) (bool, string) {
	fsType, err := getFsType(mountPoint)
	if err == nil && (fsType == "tmpfs" || fsType == "devtmpfs") {
		return false, fmt.Sprintf("filesystem type is %s (device not mounted as a real filesystem)", fsType)
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

// getFsType returns the filesystem type for a given mount point by reading /proc/mounts.
func getFsType(mountPoint string) (string, error) {
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return "", err
	}
	defer file.Close()
	return getFsTypeFromReader(file, mountPoint)
}

// getFsTypeFromReader parses mount entries from a reader (e.g. /proc/mounts) and
// returns the filesystem type for the longest-matching mount point.
func getFsTypeFromReader(reader io.Reader, mountPoint string) (string, error) {
	bestMatch := ""
	bestFsType := ""

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		mp := fields[1]
		fstype := fields[2]

		if mountPoint == mp || strings.HasPrefix(mountPoint, mp+"/") {
			if len(mp) > len(bestMatch) {
				bestMatch = mp
				bestFsType = fstype
			}
		}
	}

	if bestMatch == "" {
		return "", fmt.Errorf("mount point %s not found in /proc/mounts", mountPoint)
	}
	return bestFsType, nil
}
