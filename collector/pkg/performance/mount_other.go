//go:build !linux

package performance

import (
	"fmt"
	"os/exec"
	"strings"
)

// isMountPointSuitable always returns true on non-Linux platforms.
// The tmpfs/container detection scenario only applies to Linux containers.
func isMountPointSuitable(mountPoint string, requiredBytes uint64) (bool, string) {
	return true, ""
}

// findDeviceMountPoint resolves the mount point for a device using df, which
// reports the filesystem of a mounted device node. Linux reads /proc/mounts
// instead, so it can also resolve a whole-disk device through its partitions.
func findDeviceMountPoint(devicePath string) (string, error) {
	output, err := exec.Command("df", devicePath).Output()
	if err != nil {
		return "", fmt.Errorf("df command failed for %s: %w", devicePath, err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) < 2 {
		return "", fmt.Errorf("unexpected df output for %s", devicePath)
	}

	// The mount point is the last field in the second line
	fields := strings.Fields(lines[1])
	if len(fields) == 0 {
		return "", fmt.Errorf("could not parse df output for %s", devicePath)
	}

	return fields[len(fields)-1], nil
}
