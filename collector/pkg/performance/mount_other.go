//go:build !linux

package performance

// isMountPointSuitable always returns true on non-Linux platforms.
// The tmpfs/container detection scenario only applies to Linux containers.
func isMountPointSuitable(mountPoint string, requiredBytes uint64) (bool, string) {
	return true, ""
}
