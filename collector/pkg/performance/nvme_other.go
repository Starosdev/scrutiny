//go:build !linux

package performance

import "fmt"

// resolveNVMeBlockDevice is a no-op on non-Linux platforms.
// On macOS, NVMe drives appear as "disk0" (not "nvme0") and the DeviceName
// prefix check in resolveTargetPath prevents this function from being called.
// On FreeBSD, NVMe uses different naming conventions (nvd0, nvme0ns1).
func resolveNVMeBlockDevice(controllerPath string) (string, error) {
	return "", fmt.Errorf("NVMe namespace resolution is not supported on this platform")
}
