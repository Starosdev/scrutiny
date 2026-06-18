//go:build linux
// +build linux

package detect

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// getMountUsage uses device IDs (Major:Minor) to reliably connect the RAID device
// (or its partitions) to a mount point in the container.
func (d *Detect) getMountUsage(devicePath string) (int64, error) {
	// 1. Collect all potential device IDs for this array (main device + p1, p2)
	targetIDs := collectDeviceRdevs(devicePath)
	if len(targetIDs) == 0 {
		return 0, fmt.Errorf("could not stat device %s or its partitions", devicePath)
	}

	// 2. Scan /proc/self/mountinfo to find the mount point with any of these IDs
	mountPoint, err := findMountPointByDeviceID(targetIDs)
	if err != nil {
		return 0, err
	}
	if mountPoint == "" {
		return 0, fmt.Errorf("no mount point found in container for RAID device or partitions")
	}

	// 3. Statfs the discovered mount point
	var stat syscall.Statfs_t
	if err := syscall.Statfs(mountPoint, &stat); err != nil {
		return 0, fmt.Errorf("statfs(%s): %w", mountPoint, err)
	}

	usedBlocks := stat.Blocks - stat.Bfree
	return int64(usedBlocks) * int64(stat.Bsize), nil //nolint:gosec // filesystem block counts fit in int64
}

// collectDeviceRdevs stats the array device and its first two partitions, returning their Rdev IDs.
func collectDeviceRdevs(devicePath string) []uint64 {
	var targetIDs []uint64
	for _, suffix := range []string{"", "p1", "p2"} {
		var devStat syscall.Stat_t
		if err := syscall.Stat(devicePath+suffix, &devStat); err == nil {
			targetIDs = append(targetIDs, devStat.Rdev)
		}
	}
	return targetIDs
}

// findMountPointByDeviceID scans /proc/self/mountinfo and returns the mount point whose device ID
// matches any of targetIDs, or "" when none match.
func findMountPointByDeviceID(targetIDs []uint64) (string, error) {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 5 {
			continue
		}

		mm := strings.Split(fields[2], ":")
		if len(mm) != 2 {
			continue
		}
		major, _ := strconv.ParseUint(mm[0], 10, 32)
		minor, _ := strconv.ParseUint(mm[1], 10, 32)
		id := uint64(unixMkdev(uint32(major), uint32(minor)))

		if containsUint64(targetIDs, id) {
			return fields[4], nil
		}
	}
	return "", nil
}

// containsUint64 reports whether target is present in ids.
func containsUint64(ids []uint64, target uint64) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

// unixMkdev mimics the Linux MKDEV macro
func unixMkdev(major, minor uint32) uint64 {
	return uint64((minor & 0xff) | ((major & 0xfff) << 8) | ((minor & ^uint32(0xff)) << 12) | ((major & ^uint32(0xfff)) << 32))
}
